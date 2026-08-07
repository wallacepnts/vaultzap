// Package ingest connects the parser to the database: given an export file, it produces
// (or reuses) a chat and inserts new messages idempotently.
package ingest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wallacepnts/vaultzap/internal/parser"
	"github.com/wallacepnts/vaultzap/internal/store"
)

type Report struct {
	Path        string
	ChatID      int64
	ChatName    string
	Added       int
	Skipped     int
	Warnings    int
	AlreadyDone bool // file's sha256 already seen; nothing was done
}

// ImportFile parses an ingestion unit (loose .txt, .zip, or a subfolder with a .txt) and
// saves it. path is where it sits on disk; record goes in imports.path and derives the
// chat name. Idempotent via UNIQUE(chat_id, hash).
//
// dateOrder is VAULTZAP_DATE_ORDER, threaded through because config validates it at boot:
// dropping it here would turn a documented setting into a silent no-op.
func ImportFile(ctx context.Context, s *store.Store, path, record, mediaDir string, dateOrder parser.DateOrder) (Report, error) {
	return ImportIntoChat(ctx, s, path, record, mediaDir, 0, dateOrder)
}

// targetChat > 0 sends the messages there instead of deriving a chat from the file name.
func ImportIntoChat(ctx context.Context, s *store.Store, path, record, mediaDir string, targetChat int64, dateOrder parser.DateOrder) (Report, error) {
	// MEDIA_DIR has to exist before resolveUnit: a zip is extracted into a temp directory
	// created inside it.
	if err := os.MkdirAll(mediaDir, 0o755); err != nil {
		return Report{}, fmt.Errorf("criar diretório de mídia: %w", err)
	}

	unit, err := resolveUnit(path, mediaDir)
	if err != nil {
		_ = recordFailedImport(ctx, s, record, fallbackHash(path, record), err)
		return Report{}, err
	}
	defer unit.cleanup()

	fileHash, err := identityHash(path, unit.txtPath)
	if err != nil {
		return Report{}, fmt.Errorf("calcular identidade do import: %w", err)
	}
	report := Report{Path: record}

	// Only a *successful* import short-circuits. A row left by a previous failure must not
	// count: the caller would get AlreadyDone with a nil error and move (or delete) the file
	// while the database holds no message from it.
	// The guard is for the scanner only — "Atualizar conversa" (targetChat > 0) exists
	// precisely to point an already-seen file at another chat.
	if targetChat == 0 {
		var alreadyExists int
		err = s.DB().QueryRowContext(ctx,
			`SELECT COUNT(*) FROM imports WHERE sha256 = ? AND status = 'done'`, fileHash).Scan(&alreadyExists)
		if err != nil {
			return Report{}, fmt.Errorf("checar import existente: %w", err)
		}
		if alreadyExists > 0 {
			report.AlreadyDone = true
			return report, nil
		}
	}

	txtContent, err := os.ReadFile(unit.txtPath)
	if err != nil {
		return Report{}, fmt.Errorf("ler %s: %w", unit.txtPath, err)
	}

	chatName := parser.ChatNameFromFile(record)

	result, err := parser.Parse(bytes.NewReader(txtContent), parser.Options{
		ChatName:         chatName,
		DefaultDateOrder: dateOrder,
	})
	if err != nil {
		_ = recordFailedImport(ctx, s, record, fileHash, err)
		return Report{}, fmt.Errorf("analisar %s: %w", unit.txtPath, err)
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		return Report{}, err
	}
	defer tx.Rollback()

	chatID := targetChat
	if chatID > 0 {
		if err := tx.QueryRowContext(ctx, `SELECT name FROM chats WHERE id = ?`, chatID).Scan(&chatName); err != nil {
			return Report{}, fmt.Errorf("chat de destino %d: %w", chatID, err)
		}
	} else {
		if MeaninglessName(chatName) {
			if guessed := chatNameFromSenders(ctx, tx, result); guessed != "" {
				slog.Info("nome da conversa deduzido dos remetentes", "pasta", chatName, "nome", guessed)
				chatName = guessed
			}
		}

		chatID, err = findOrCreateChat(ctx, tx, chatName, result.IsGroup, result.Source, now)
		if err != nil {
			return Report{}, fmt.Errorf("chat: %w", err)
		}
	}

	if err := importProfilePhoto(ctx, tx, mediaDir, chatID, unit.avatarSource); err != nil {
		return Report{}, fmt.Errorf("importar foto de perfil: %w", err)
	}

	progress.setPhase(PhaseImporting, int64(len(result.Messages)))

	added, skipped := 0, 0
	cited, missing := 0, 0
	for _, msg := range result.Messages {
		progress.add(1)
		kind := msg.Kind
		var attachmentID any

		if msg.AttachmentName != "" {
			cited++
			id, ok, err := resolveAttachment(ctx, tx, mediaDir, chatID, unit.mediaDir, msg.AttachmentName, msg.MediaKind)
			if err != nil {
				return Report{}, fmt.Errorf("anexo %s: %w", msg.AttachmentName, err)
			}
			if ok {
				attachmentID = id
			} else {
				kind = "media_omitted"
				missing++
			}
		}

		hash := messageHash(msg.SentAt, msg.Sender, msg.Body, msg.AttachmentName)
		res, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO messages (chat_id, sent_at, seq, sender, body, kind, attachment_id, hash)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			chatID, msg.SentAt, msg.Seq, msg.Sender, msg.Body, kind, attachmentID, hash)
		if err != nil {
			return Report{}, fmt.Errorf("inserir mensagem: %w", err)
		}
		rows, _ := res.RowsAffected()
		if rows > 0 {
			added++
		} else {
			skipped++
		}
	}

	if err := refreshChatSummary(ctx, tx, chatID); err != nil {
		return Report{}, fmt.Errorf("atualizar resumo do chat: %w", err)
	}

	if err := inferOwner(ctx, tx, chatID, chatName, result); err != nil {
		return Report{}, fmt.Errorf("inferir dono do chat: %w", err)
	}

	checks := result.Checks
	// Only the ingest side sees the media folder. An export made "without media" cites
	// everything and delivers nothing — normal, and not a finding; a file where some media
	// arrived and some did not is what deserves a look.
	if missing > 0 && missing < cited {
		checks = append(checks, parser.Check{Code: parser.CheckMediaMissing, Count: missing})
	}

	if err := recordImport(ctx, tx, record, fileHash, chatID, now, added, skipped, result.Warnings, checks, unitSize(path)); err != nil {
		return Report{}, fmt.Errorf("registrar import: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return Report{}, err
	}

	report.ChatID = chatID
	report.ChatName = chatName
	report.Added = added
	report.Skipped = skipped
	report.Warnings = len(result.Warnings)
	return report, nil
}

// A derived name that is actually a phone number is stored in chats.phone too, but only
// on creation: a reimport must never overwrite a phone the user edited.
func findOrCreateChat(ctx context.Context, tx *sql.Tx, name string, isGroup bool, source, now string) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM chats WHERE name = ? AND is_group = ?`, name, boolToInt(isGroup)).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}

	var phone sql.NullString
	if !isGroup && parser.LooksLikePhoneNumber(name) {
		phone = sql.NullString{String: name, Valid: true}
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO chats (name, is_group, source, created_at, phone) VALUES (?, ?, ?, ?, ?)`,
		name, boolToInt(isGroup), source, now, phone)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// In a 1:1 chat the owner is the sender that doesn't match the file name. Only writes with
// no group, exactly two senders, and owner still null.
func inferOwner(ctx context.Context, tx *sql.Tx, chatID int64, chatName string, result parser.Result) error {
	if result.IsGroup {
		return nil
	}

	var already sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT owner FROM chats WHERE id = ?`, chatID).Scan(&already); err != nil {
		return err
	}
	if already.Valid && already.String != "" {
		return nil
	}

	seen := map[string]bool{}
	for _, m := range result.Messages {
		if m.Sender != nil {
			seen[strings.TrimSpace(*m.Sender)] = true
		}
	}
	if len(seen) != 2 {
		return nil
	}

	target := strings.TrimSpace(chatName)
	var me string
	foundOther := false
	for sender := range seen {
		if parser.SameName(sender, target) {
			foundOther = true
		} else {
			me = sender
		}
	}
	if !foundOther || me == "" {
		return nil
	}

	slog.Info("dono inferido pelo nome do export", "chat_id", chatID, "contato", target)
	_, err := tx.ExecContext(ctx, `UPDATE chats SET owner = ? WHERE id = ?`, me, chatID)
	return err
}

// The date range ignores an empty sent_at, which is what a line with an unparseable date gets.
// The empty string sorts below every real timestamp, so one corrupt line would make MIN()
// return it and the chat lose its start date — and with it the calendar's lower bound.
// message_count still counts everything: the message exists.
func refreshChatSummary(ctx context.Context, tx *sql.Tx, chatID int64) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE chats SET
			first_message_at = (SELECT MIN(sent_at) FROM messages WHERE chat_id = ? AND sent_at <> ''),
			last_message_at  = (SELECT MAX(sent_at) FROM messages WHERE chat_id = ? AND sent_at <> ''),
			message_count    = (SELECT COUNT(*) FROM messages WHERE chat_id = ?)
		WHERE id = ?`,
		chatID, chatID, chatID, chatID)
	return err
}

const maxStoredWarnings = 200

// recordImport always upserts on the sha256 conflict. Reaching here means the guard in
// ImportIntoChat let the file through, which happens in exactly two cases, both wanting
// the old row replaced: the forced "Atualizar conversa" path, and a retry of a file whose
// previous import ended in status='error'.
func recordImport(ctx context.Context, tx *sql.Tx, path, fileHash string, chatID int64, now string, added, skipped int, warnings []string, checks []parser.Check, size int64) error {
	checksJSON := ""
	if len(checks) > 0 {
		if encoded, err := json.Marshal(checks); err == nil {
			checksJSON = string(encoded)
		}
	}

	kept := warnings
	if len(kept) > maxStoredWarnings {
		kept = append(kept[:maxStoredWarnings:maxStoredWarnings],
			fmt.Sprintf("... e mais %d avisos", len(warnings)-maxStoredWarnings))
	}

	const query = `
		INSERT INTO imports (path, sha256, chat_id, added, skipped, warnings, warnings_text, checks_text, status, started_at, finished_at, size_bytes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'done', ?, ?, ?)
		ON CONFLICT(sha256) DO UPDATE SET
			path = excluded.path, chat_id = excluded.chat_id, added = excluded.added,
			skipped = excluded.skipped, warnings = excluded.warnings,
			warnings_text = excluded.warnings_text, checks_text = excluded.checks_text,
			size_bytes = excluded.size_bytes,
			status = 'done', error = NULL, finished_at = excluded.finished_at`
	_, err := tx.ExecContext(ctx, query,
		path, fileHash, chatID, added, skipped,
		len(warnings), strings.Join(kept, "\n"), checksJSON, now, now, size)
	return err
}

func recordFailedImport(ctx context.Context, s *store.Store, path, fileHash string, importErr error) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	// Upserts too, so a second failure shows the latest error instead of keeping the first.
	_, err := s.DB().ExecContext(ctx, `
		INSERT INTO imports (path, sha256, status, error, started_at, finished_at)
		VALUES (?, ?, 'error', ?, ?, ?)
		ON CONFLICT(sha256) DO UPDATE SET
			path = excluded.path, status = 'error', error = excluded.error,
			finished_at = excluded.finished_at`,
		path, fileHash, importErr.Error(), now, now)
	return err
}

func messageHash(sentAt string, sender *string, body, fileName string) string {
	from := ""
	if sender != nil {
		from = *sender
	}
	sum := sha256.Sum256([]byte(sentAt + "|" + from + "|" + body + "|" + fileName))
	return hex.EncodeToString(sum[:])
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// identityHash returns the sha256 stored in imports.sha256: the file itself for
// .txt/.zip, or the resolved .txt for a subfolder.
func identityHash(originalPath, txtPath string) (string, error) {
	info, err := os.Stat(originalPath)
	if err != nil {
		return "", err
	}
	target := originalPath
	if info.IsDir() {
		target = txtPath
	}
	return hashFileHex(target)
}

func fallbackHash(path, record string) string {
	if sum, err := hashFileHex(path); err == nil {
		return sum
	}
	return sha256Hex([]byte(record + "|" + time.Now().Format(time.RFC3339Nano)))
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// unitSize is the size of the imported unit: the file itself, or everything inside the
// folder. Recorded at import time because the file may be moved to .imported/ afterwards.
func unitSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	if !info.IsDir() {
		return info.Size()
	}
	var total int64
	_ = filepath.WalkDir(path, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if fi, err := d.Info(); err == nil {
			total += fi.Size()
		}
		return nil
	})
	return total
}
