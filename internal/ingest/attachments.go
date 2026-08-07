package ingest

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif" // registers the decoder for image.DecodeConfig
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

// Copies fileName from mediaSrcDir into mediaDir, deduping by sha256 within the chat.
// A missing file returns ok=false.
func resolveAttachment(ctx context.Context, tx *sql.Tx, mediaDir string, chatID int64, mediaSrcDir, fileName, mediaKind string) (attachmentID int64, ok bool, err error) {
	if !safeAttachmentName(fileName) {
		return 0, false, nil
	}
	src := filepath.Join(mediaSrcDir, fileName)
	info, err := os.Stat(src)
	if err != nil || info.IsDir() {
		return 0, false, nil
	}

	sumBytes, size, err := hashAndSizeFile(src)
	if err != nil {
		return 0, false, fmt.Errorf("ler anexo %s: %w", fileName, err)
	}
	sum := hex.EncodeToString(sumBytes)

	var existingID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM attachments WHERE chat_id = ? AND sha256 = ?`, chatID, sum).Scan(&existingID)
	if err == nil {
		return existingID, true, nil
	}
	if err != sql.ErrNoRows {
		return 0, false, err
	}

	ext := filepath.Ext(fileName)
	storedPath := fmt.Sprintf("%d/%s/%s%s", chatID, sum[:2], sum, ext)
	dest := filepath.Join(mediaDir, filepath.FromSlash(storedPath))
	if err := copyFile(src, dest); err != nil {
		return 0, false, fmt.Errorf("copiar anexo %s: %w", fileName, err)
	}

	mimeType := mime.TypeByExtension(ext)
	width, height := imageDimensions(src, mediaKind)

	res, err := tx.ExecContext(ctx, `
		INSERT INTO attachments (chat_id, filename, sha256, media_kind, mime, size_bytes, width, height, stored_path)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		chatID, fileName, sum, mediaKind, nilIfEmpty(mimeType), size, nilIfZero(width), nilIfZero(height), storedPath)
	if err != nil {
		return 0, false, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

// The name quoted in the export is untrusted — it comes verbatim from a .txt the user only
// dropped in the inbox, and "<anexado: ../../data/vaultzap.db>" would otherwise copy an
// arbitrary file into MEDIA_DIR and serve it over /media/{id}. Only a plain name passes; a
// rejected one is treated as a missing file, never as an aborted import.
func safeAttachmentName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	// filepath.Separator covers the platform; '/' is checked explicitly because the export
	// can quote a POSIX path on any host.
	return !strings.ContainsRune(name, '/') && !strings.ContainsRune(name, filepath.Separator)
}

// hashFileHex is the streaming sha256 of a whole file. Reading it into memory instead
// would make peak RSS the size of the largest file in the inbox: measured at 7.2 GB
// importing a 6.7 GiB export, enough to be OOM-killed in a memory-capped container.
func hashFileHex(path string) (string, error) {
	sum, _, err := hashAndSizeFile(path)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(sum), nil
}

func hashAndSizeFile(path string) ([]byte, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return nil, 0, err
	}
	return h.Sum(nil), size, nil
}

func copyFile(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func imageDimensions(path, mediaKind string) (width, height int) {
	if mediaKind != "image" {
		return 0, 0
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nilIfZero(n int) any {
	if n == 0 {
		return nil
	}
	return n
}
