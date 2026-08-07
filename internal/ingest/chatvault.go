package ingest

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/wallacepnts/vaultzap/internal/parser"
)

// chatvault (vitormarcal/chatvault) archives arrive as one folder per conversation, named
// with the chat's UUID, holding _chat.txt, media and profile-image.*. This file resolves
// the chat name and imports the profile photo for that layout.

var reUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func MeaninglessName(name string) bool {
	return reUUID.MatchString(strings.TrimSpace(name))
}

// chatNameFromSenders returns the contact's name in a 1:1 chat by comparing the two
// senders against the most frequent chats.owner in the database — the same user is in
// every conversation, so one resolved chat identifies the others. Empty when there are
// not exactly two senders or no owner is known: guessing would be wrong half the time.
func chatNameFromSenders(ctx context.Context, tx *sql.Tx, result parser.Result) string {
	if result.IsGroup {
		return ""
	}

	seen := map[string]bool{}
	for _, m := range result.Messages {
		if m.Sender != nil {
			seen[strings.TrimSpace(*m.Sender)] = true
		}
	}
	if len(seen) != 2 {
		return ""
	}

	var me string
	if err := tx.QueryRowContext(ctx, `
		SELECT owner FROM chats WHERE owner IS NOT NULL AND owner <> ''
		GROUP BY owner ORDER BY COUNT(*) DESC LIMIT 1`).Scan(&me); err != nil {
		return ""
	}

	for sender := range seen {
		if !parser.SameName(sender, me) {
			return sender
		}
	}
	return ""
}

// Does nothing if an avatar is already set: the user's choice wins over the file.
func importProfilePhoto(ctx context.Context, tx *sql.Tx, mediaDir string, chatID int64, source string) error {
	if source == "" {
		return nil
	}

	var current sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT avatar_path FROM chats WHERE id = ?`, chatID).Scan(&current); err != nil {
		return err
	}
	if current.Valid && current.String != "" {
		return nil
	}

	destRel := filepath.Join("avatars", fmt.Sprintf("%d%s", chatID, strings.ToLower(filepath.Ext(source))))
	if err := copyFile(source, filepath.Join(mediaDir, destRel)); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `UPDATE chats SET avatar_path = ? WHERE id = ?`, filepath.ToSlash(destRel), chatID)
	if err == nil {
		slog.Info("foto de perfil importada junto da conversa", "chat_id", chatID)
	}
	return err
}
