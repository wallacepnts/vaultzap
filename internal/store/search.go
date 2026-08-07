package store

import (
	"context"
	"database/sql"
	"strings"
)

// toFTSQuery turns user input into a MATCH query: each word becomes a quoted phrase.
func toFTSQuery(input string) string {
	words := strings.Fields(input)
	parts := make([]string, len(words))
	for i, w := range words {
		parts[i] = `"` + strings.ReplaceAll(w, `"`, `""`) + `"`
	}
	return strings.Join(parts, " ")
}

// The ESCAPE character declared alongside every LIKE that interpolates user input.
const likeEscape = `\`

// Without this, typing "_" matches any single character and "%" matches everything, so the
// search silently stops filtering — and both show up in real chat names.
func escapeLike(input string) string {
	r := strings.NewReplacer(
		likeEscape, likeEscape+likeEscape,
		"%", likeEscape+"%",
		"_", likeEscape+"_",
	)
	return r.Replace(input)
}

// internal/render.FormatSnippet swaps these marks for <mark>.
const (
	snippetMarkStart = "\x01"
	snippetMarkEnd   = "\x02"
)

// SearchResult is a text search hit inside a chat.
type SearchResult struct {
	Message Message
	Snippet string
}

// FTS5, most relevant first.
func (s *Store) SearchMessages(ctx context.Context, chatID int64, term string) ([]SearchResult, error) {
	ftsQuery := toFTSQuery(term)
	if ftsQuery == "" {
		return nil, nil
	}

	// messages_fts can't be referenced with an alias alongside MATCH.
	rows, err := s.read.QueryContext(ctx, `
		SELECT `+messageColumnsWithAttachment+`,
			snippet(messages_fts, 0, ?, ?, '…', 10) AS snippet
		FROM messages_fts
		JOIN messages m ON m.id = messages_fts.rowid
		LEFT JOIN attachments a ON a.id = m.attachment_id
		WHERE messages_fts MATCH ? AND m.chat_id = ?
		ORDER BY rank
		LIMIT 50`,
		snippetMarkStart, snippetMarkEnd, ftsQuery, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var sender, attName, attMediaKind, attMime sql.NullString
		var attachmentID sql.NullInt64
		var attWidth, attHeight sql.NullInt64
		if err := rows.Scan(
			&r.Message.ID, &r.Message.SentAt, &r.Message.Seq, &sender, &r.Message.Body, &r.Message.Kind, &attachmentID,
			&attName, &attMediaKind, &attMime, &attWidth, &attHeight, &r.Message.Favorite, &r.Message.Pinned,
			&r.Snippet,
		); err != nil {
			return nil, err
		}
		if sender.Valid {
			r.Message.Sender = &sender.String
		}
		if attachmentID.Valid {
			id := attachmentID.Int64
			r.Message.AttachmentID = &id
		}
		r.Message.AttachmentName = attName.String
		r.Message.AttachmentMediaKind = attMediaKind.String
		r.Message.AttachmentMime = attMime.String
		r.Message.AttachmentWidth = int(attWidth.Int64)
		r.Message.AttachmentHeight = int(attHeight.Int64)
		results = append(results, r)
	}
	return results, rows.Err()
}

// A window centered on messageID, chronological, plus whether older messages exist.
func (s *Store) MessagesAround(ctx context.Context, chatID, messageID int64) ([]Message, bool, error) {
	var sentAt string
	var seq int
	err := s.read.QueryRowContext(ctx,
		`SELECT sent_at, seq FROM messages WHERE id = ? AND chat_id = ?`, messageID, chatID,
	).Scan(&sentAt, &seq)
	if err != nil {
		return nil, false, err
	}

	half := defaultPageSize / 2

	// Same three-part cursor as MessagesBefore, so the window splits at exactly the target
	// message even when another ties on (sent_at, seq).
	before, hasMore, err := s.fetchMessagePage(ctx, half, `
		SELECT `+messageColumnsWithAttachment+`
		FROM messages m LEFT JOIN attachments a ON a.id = m.attachment_id
		WHERE m.chat_id = ? AND (m.sent_at, m.seq, m.id) <= (?, ?, ?)
		ORDER BY m.sent_at DESC, m.seq DESC, m.id DESC LIMIT ?`,
		chatID, sentAt, seq, messageID)
	if err != nil {
		return nil, false, err
	}

	afterRows, err := s.read.QueryContext(ctx, `
		SELECT `+messageColumnsWithAttachment+`
		FROM messages m LEFT JOIN attachments a ON a.id = m.attachment_id
		WHERE m.chat_id = ? AND (m.sent_at, m.seq, m.id) > (?, ?, ?)
		ORDER BY m.sent_at ASC, m.seq ASC, m.id ASC LIMIT ?`,
		chatID, sentAt, seq, messageID, half)
	if err != nil {
		return nil, false, err
	}
	defer afterRows.Close()
	after, err := scanMessages(afterRows)
	if err != nil {
		return nil, false, err
	}

	return append(before, after...), hasMore, nil
}
