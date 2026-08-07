package store

import (
	"context"
	"fmt"
)

// MergeReport summarizes what a merge moved.
type MergeReport struct {
	MessagesMoved         int
	MessagesDuplicated    int
	AttachmentsMoved      int
	AttachmentsDuplicated int
}

// Moves everything from source into dest and deletes source. Irreversible.
func (s *Store) MergeChats(ctx context.Context, dest, source int64) (MergeReport, error) {
	var rep MergeReport
	if dest == source {
		return rep, fmt.Errorf("destino e origem são a mesma conversa")
	}

	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return rep, err
	}
	defer tx.Rollback()

	// 1. Repoint source's messages to dest's matching attachment (same sha256), before
	// deleting the duplicate: the other order orphans messages.attachment_id.
	if _, err := tx.ExecContext(ctx, `
		UPDATE messages SET attachment_id = (
			SELECT ad.id FROM attachments ad, attachments ao
			WHERE ao.id = messages.attachment_id AND ad.sha256 = ao.sha256 AND ad.chat_id = ?
		)
		WHERE chat_id = ? AND attachment_id IN (
			SELECT ao.id FROM attachments ao JOIN attachments ad ON ad.sha256 = ao.sha256
			WHERE ao.chat_id = ? AND ad.chat_id = ?
		)`, dest, source, source, dest); err != nil {
		return rep, fmt.Errorf("reapontar anexos duplicados: %w", err)
	}

	res, err := tx.ExecContext(ctx, `
		DELETE FROM attachments WHERE chat_id = ?
		  AND sha256 IN (SELECT sha256 FROM attachments WHERE chat_id = ?)`, source, dest)
	if err != nil {
		return rep, fmt.Errorf("apagar anexos duplicados: %w", err)
	}
	n, _ := res.RowsAffected()
	rep.AttachmentsDuplicated = int(n)

	res, err = tx.ExecContext(ctx, `UPDATE attachments SET chat_id = ? WHERE chat_id = ?`, dest, source)
	if err != nil {
		return rep, fmt.Errorf("mover anexos: %w", err)
	}
	n, _ = res.RowsAffected()
	rep.AttachmentsMoved = int(n)

	// 2. Move messages, skipping ones dest already has.
	res, err = tx.ExecContext(ctx, `UPDATE OR IGNORE messages SET chat_id = ? WHERE chat_id = ?`, dest, source)
	if err != nil {
		return rep, fmt.Errorf("mover mensagens: %w", err)
	}
	n, _ = res.RowsAffected()
	rep.MessagesMoved = int(n)

	// 2b. Rescue the attachment of every source message that couldn't move because dest
	// already holds the same hash. Those rows are deleted right below, and their attachment
	// already belongs to dest (step 1), so without this it ends up referenced by nobody and
	// the media vanishes from the gallery.
	//
	// This is the merge people actually do: the media-less .txt of a chat and the .zip of
	// the same chat, where the messages are identical and only the attachment differs.
	if _, err := tx.ExecContext(ctx, `
		UPDATE messages AS md SET
			attachment_id = (SELECT mo.attachment_id FROM messages mo
			                 WHERE mo.chat_id = ? AND mo.hash = md.hash AND mo.attachment_id IS NOT NULL),
			kind          = (SELECT mo.kind          FROM messages mo
			                 WHERE mo.chat_id = ? AND mo.hash = md.hash AND mo.attachment_id IS NOT NULL)
		WHERE md.chat_id = ? AND md.attachment_id IS NULL
		  AND EXISTS (SELECT 1 FROM messages mo
		              WHERE mo.chat_id = ? AND mo.hash = md.hash AND mo.attachment_id IS NOT NULL)`,
		source, source, dest, source); err != nil {
		return rep, fmt.Errorf("resgatar anexos das mensagens duplicadas: %w", err)
	}

	res, err = tx.ExecContext(ctx, `DELETE FROM messages WHERE chat_id = ?`, source)
	if err != nil {
		return rep, fmt.Errorf("apagar mensagens duplicadas: %w", err)
	}
	n, _ = res.RowsAffected()
	rep.MessagesDuplicated = int(n)

	// 3. Move customizations and links dest doesn't have yet.
	for _, query := range []string{
		`INSERT OR IGNORE INTO nicknames (chat_id, sender, name) SELECT ?, sender, name FROM nicknames WHERE chat_id = ?`,
		`INSERT OR IGNORE INTO chat_lists (chat_id, list_id) SELECT ?, list_id FROM chat_lists WHERE chat_id = ?`,
		`UPDATE imports SET chat_id = ? WHERE chat_id = ?`,
	} {
		if _, err := tx.ExecContext(ctx, query, dest, source); err != nil {
			return rep, fmt.Errorf("mover vínculos: %w", err)
		}
	}

	// 4. dest inherits owner, avatar and phone from source where dest has none.
	if _, err := tx.ExecContext(ctx, `
		UPDATE chats SET
			owner = COALESCE(NULLIF(owner, ''), (SELECT owner FROM chats WHERE id = ?)),
			avatar_path = COALESCE(NULLIF(avatar_path, ''), (SELECT avatar_path FROM chats WHERE id = ?)),
			phone = COALESCE(NULLIF(phone, ''), (SELECT phone FROM chats WHERE id = ?))
		WHERE id = ?`, source, source, source, dest); err != nil {
		return rep, fmt.Errorf("herdar personalizações: %w", err)
	}

	// 4b. Both sides can be at the pin limit, and the merged chat would come out at twice
	// it — nothing else ever repairs that, since TogglePinnedMessage only trims on pin.
	if _, err := tx.ExecContext(ctx, `
		UPDATE messages SET pinned = 0, pinned_at = NULL
		WHERE id IN (
		    SELECT id FROM messages
		    WHERE chat_id = ? AND pinned = 1
		    ORDER BY pinned_at DESC, id DESC
		    LIMIT -1 OFFSET ?
		)`, dest, MaxPinnedMessages); err != nil {
		return rep, fmt.Errorf("reaplicar teto de fixadas: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM chats WHERE id = ?`, source); err != nil {
		return rep, fmt.Errorf("apagar conversa de origem: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE chats SET
			-- sent_at = '' (an unparseable date) stays out of the
			-- intervalo; ver refreshChatSummary em internal/ingest.
			first_message_at = (SELECT MIN(sent_at) FROM messages WHERE chat_id = ? AND sent_at <> ''),
			last_message_at  = (SELECT MAX(sent_at) FROM messages WHERE chat_id = ? AND sent_at <> ''),
			message_count    = (SELECT COUNT(*) FROM messages WHERE chat_id = ?)
		WHERE id = ?`, dest, dest, dest, dest); err != nil {
		return rep, fmt.Errorf("atualizar resumo: %w", err)
	}

	return rep, tx.Commit()
}
