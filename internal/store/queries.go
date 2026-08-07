package store

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

type Chat struct {
	ID             int64
	Name           string
	IsGroup        bool
	Source         string
	Owner          *string
	AvatarPath     string // relative to MEDIA_DIR; empty = no custom photo
	Phone          string
	FirstMessageAt string
	LastMessageAt  string
	MessageCount   int
	PreviewBody    string
	PreviewKind    string
	Archived       bool
	Pinned         bool
	Favorite       bool
}

type Message struct {
	ID                  int64
	SentAt              string
	Seq                 int
	Sender              *string
	Body                string
	Kind                string
	AttachmentID        *int64
	AttachmentName      string
	AttachmentMediaKind string
	AttachmentMime      string
	AttachmentWidth     int
	AttachmentHeight    int
	Favorite            bool // starred by the user; UI state, not export data
	Pinned              bool // pinned by the user; UI state, not export data
}

// Attachment is an attachments row; SentAt comes from the message that cites it.
type Attachment struct {
	ID         int64
	ChatID     int64
	Filename   string
	SHA256     string
	MediaKind  string
	Mime       string
	StoredPath string
	SizeBytes  int64
	SentAt     string
}

// ChatFilter is which slice of the chat list to return; the zero value is the main list.
type ChatFilter struct {
	// Search filters by chat name or some message's content (FTS).
	Search    string
	Archived  bool
	Favorites bool
	Groups    bool
	ListID    int64
}

// Pinned first, then by most recent message.
func (s *Store) ListChats(ctx context.Context, filter ChatFilter) ([]Chat, error) {
	query := `
		SELECT
			c.id, c.name, c.is_group, c.source, c.owner, COALESCE(c.avatar_path, ''),
			COALESCE(c.first_message_at, ''), COALESCE(c.last_message_at, ''), c.message_count,
			COALESCE((SELECT body FROM messages m WHERE m.chat_id = c.id ORDER BY m.sent_at DESC, m.seq DESC LIMIT 1), ''),
			COALESCE((SELECT kind FROM messages m WHERE m.chat_id = c.id ORDER BY m.sent_at DESC, m.seq DESC LIMIT 1), ''),
			c.archived, c.pinned, c.favorite
		FROM chats c
		WHERE c.archived = ?`

	args := []any{filter.Archived}

	if filter.Favorites {
		query += ` AND c.favorite = 1`
	}
	if filter.Groups {
		query += ` AND c.is_group = 1`
	}
	if filter.ListID > 0 {
		query += ` AND EXISTS (SELECT 1 FROM chat_lists cl WHERE cl.chat_id = c.id AND cl.list_id = ?)`
		args = append(args, filter.ListID)
	}

	if search := strings.TrimSpace(filter.Search); search != "" {
		// messages_fts can't carry an alias alongside MATCH.
		query += `
		  AND (c.name LIKE '%' || ? || '%' ESCAPE '\'
		       OR EXISTS (
		            SELECT 1 FROM messages m2 JOIN messages_fts ON messages_fts.rowid = m2.id
		            WHERE m2.chat_id = c.id AND messages_fts MATCH ?
		          ))`
		args = append(args, escapeLike(search), toFTSQuery(search))
	}
	query += ` ORDER BY c.pinned DESC, c.last_message_at DESC, c.name ASC`

	rows, err := s.read.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []Chat
	for rows.Next() {
		var c Chat
		var owner sql.NullString
		if err := rows.Scan(
			&c.ID, &c.Name, &c.IsGroup, &c.Source, &owner, &c.AvatarPath,
			&c.FirstMessageAt, &c.LastMessageAt, &c.MessageCount,
			&c.PreviewBody, &c.PreviewKind, &c.Archived, &c.Pinned, &c.Favorite,
		); err != nil {
			return nil, err
		}
		if owner.Valid {
			c.Owner = &owner.String
		}
		chats = append(chats, c)
	}
	return chats, rows.Err()
}

// Same is_group only (a group and a 1:1 never match), chatID itself excluded.
// Empty search returns the most recent chats of that kind, capped at 30.
func (s *Store) SearchMergeCandidates(ctx context.Context, chatID int64, isGroup bool, search string) ([]Chat, error) {
	query := `
		SELECT id, name, is_group, COALESCE(avatar_path, ''), message_count
		FROM chats
		WHERE id != ? AND archived = 0 AND is_group = ?`
	args := []any{chatID, isGroup}

	if search = strings.TrimSpace(search); search != "" {
		query += ` AND name LIKE '%' || ? || '%' ESCAPE '\'`
		args = append(args, escapeLike(search))
	}
	query += ` ORDER BY last_message_at DESC LIMIT 30`

	rows, err := s.read.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []Chat
	for rows.Next() {
		var c Chat
		if err := rows.Scan(&c.ID, &c.Name, &c.IsGroup, &c.AvatarPath, &c.MessageCount); err != nil {
			return nil, err
		}
		chats = append(chats, c)
	}
	return chats, rows.Err()
}

func (s *Store) CountArchived(ctx context.Context) (int, error) {
	var n int
	err := s.read.QueryRowContext(ctx, `SELECT COUNT(*) FROM chats WHERE archived = 1`).Scan(&n)
	return n, err
}

func (s *Store) SetPinned(ctx context.Context, chatID int64, pinned bool) error {
	_, err := s.write.ExecContext(ctx, `UPDATE chats SET pinned = ? WHERE id = ?`, pinned, chatID)
	return err
}

func (s *Store) SetFavorite(ctx context.Context, chatID int64, favorite bool) error {
	_, err := s.write.ExecContext(ctx, `UPDATE chats SET favorite = ? WHERE id = ?`, favorite, chatID)
	return err
}

func (s *Store) SetArchived(ctx context.Context, chatID int64, archived bool) error {
	_, err := s.write.ExecContext(ctx, `UPDATE chats SET archived = ? WHERE id = ?`, archived, chatID)
	return err
}

// Messages and attachments go by CASCADE; the attachments' stored_path comes back for the
// caller to delete the files. The imports rows are deleted and seen_files marked 'ignored',
// so the file in the inbox can bring the chat back.
func (s *Store) DeleteChat(ctx context.Context, chatID int64) (mediaPaths []string, inboxPaths []string, err error) {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	mediaPaths, err = collectColumn(ctx, tx, `SELECT stored_path FROM attachments WHERE chat_id = ?`, chatID)
	if err != nil {
		return nil, nil, err
	}
	inboxPaths, err = collectColumn(ctx, tx, `SELECT path FROM imports WHERE chat_id = ?`, chatID)
	if err != nil {
		return nil, nil, err
	}

	for _, path := range inboxPaths {
		if _, err := tx.ExecContext(ctx,
			`UPDATE seen_files SET state = ? WHERE path = ?`, StateIgnored, path); err != nil {
			return nil, nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM imports WHERE chat_id = ?`, chatID); err != nil {
		return nil, nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM chats WHERE id = ?`, chatID); err != nil {
		return nil, nil, err
	}
	return mediaPaths, inboxPaths, tx.Commit()
}

func collectColumn(ctx context.Context, tx *sql.Tx, query string, args ...any) ([]string, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, rows.Err()
}

func (s *Store) GetChat(ctx context.Context, id int64) (Chat, error) {
	var c Chat
	var owner sql.NullString
	err := s.read.QueryRowContext(ctx, `
		SELECT id, name, is_group, source, owner, COALESCE(avatar_path, ''), COALESCE(phone, ''),
			COALESCE(first_message_at, ''), COALESCE(last_message_at, ''), message_count, archived, pinned, favorite
		FROM chats WHERE id = ?`, id,
	).Scan(&c.ID, &c.Name, &c.IsGroup, &c.Source, &owner, &c.AvatarPath, &c.Phone, &c.FirstMessageAt, &c.LastMessageAt, &c.MessageCount, &c.Archived, &c.Pinned, &c.Favorite)
	if err != nil {
		return Chat{}, err
	}
	if owner.Valid {
		c.Owner = &owner.String
	}
	return c, nil
}

const defaultPageSize = 50

// 0 or negative falls back to the default. No cap.
func pageSizeOr(requested int) int {
	if requested <= 0 {
		return defaultPageSize
	}
	return requested
}

const messageColumnsWithAttachment = `
	m.id, m.sent_at, m.seq, m.sender, m.body, m.kind, m.attachment_id,
	a.filename, a.media_kind, a.mime, a.width, a.height, m.favorite, m.pinned`

// The most recent page, chronological, plus whether older messages exist beyond it.
func (s *Store) LastMessagePage(ctx context.Context, chatID int64, size int) ([]Message, bool, error) {
	return s.fetchMessagePage(ctx, pageSizeOr(size), `
		SELECT `+messageColumnsWithAttachment+`
		FROM messages m LEFT JOIN attachments a ON a.id = m.attachment_id
		WHERE m.chat_id = ?
		ORDER BY m.sent_at DESC, m.seq DESC, m.id DESC LIMIT ?`,
		chatID)
}

// The page immediately before the (sentAt, seq, id) cursor.
//
// id is the third component because (sent_at, seq) is not unique: seq is the line's
// position in its own export file, so two messages from different imports of the same
// chat can tie on both. A non-unique cursor skips whichever tied row falls on the page
// boundary, and it can never be reached by scrolling up, at any page size.
func (s *Store) MessagesBefore(ctx context.Context, chatID int64, sentAt string, seq int, id int64, size int) ([]Message, bool, error) {
	return s.fetchMessagePage(ctx, pageSizeOr(size), `
		SELECT `+messageColumnsWithAttachment+`
		FROM messages m LEFT JOIN attachments a ON a.id = m.attachment_id
		WHERE m.chat_id = ? AND (m.sent_at, m.seq, m.id) < (?, ?, ?)
		ORDER BY m.sent_at DESC, m.seq DESC, m.id DESC LIMIT ?`,
		chatID, sentAt, seq, id)
}

// query must end in "... LIMIT ?" and order DESC by sent_at/seq. Fetches one extra
// row to know whether there is more beyond the page; returns chronological.
func (s *Store) fetchMessagePage(ctx context.Context, pageSize int, query string, args ...any) ([]Message, bool, error) {
	args = append(args, pageSize+1)
	rows, err := s.read.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	messages, err := scanMessages(rows)
	if err != nil {
		return nil, false, err
	}

	hasMore := len(messages) > pageSize
	if hasMore {
		messages = messages[:pageSize]
	}
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, hasMore, nil
}

// Does not close rows.
func scanMessages(rows *sql.Rows) ([]Message, error) {
	var messages []Message
	for rows.Next() {
		m, err := scanOneMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

func scanOneMessage(rows *sql.Rows) (Message, error) {
	var m Message
	var sender, attName, attMediaKind, attMime sql.NullString
	var attachmentID sql.NullInt64
	var attWidth, attHeight sql.NullInt64
	if err := rows.Scan(
		&m.ID, &m.SentAt, &m.Seq, &sender, &m.Body, &m.Kind, &attachmentID,
		&attName, &attMediaKind, &attMime, &attWidth, &attHeight, &m.Favorite, &m.Pinned,
	); err != nil {
		return Message{}, err
	}
	if sender.Valid {
		m.Sender = &sender.String
	}
	if attachmentID.Valid {
		id := attachmentID.Int64
		m.AttachmentID = &id
	}
	m.AttachmentName = attName.String
	m.AttachmentMediaKind = attMediaKind.String
	m.AttachmentMime = attMime.String
	m.AttachmentWidth = int(attWidth.Int64)
	m.AttachmentHeight = int(attHeight.Int64)
	return m, nil
}

func (s *Store) GetAttachment(ctx context.Context, id int64) (Attachment, error) {
	var a Attachment
	var mime sql.NullString
	err := s.read.QueryRowContext(ctx, `
		SELECT id, chat_id, filename, sha256, media_kind, mime, stored_path
		FROM attachments WHERE id = ?`, id,
	).Scan(&a.ID, &a.ChatID, &a.Filename, &a.SHA256, &a.MediaKind, &mime, &a.StoredPath)
	if err != nil {
		return Attachment{}, err
	}
	if mime.Valid {
		a.Mime = mime.String
	}
	return a, nil
}

// One page, newest first. An empty beforeSentAt starts at the top; limit 0 means every
// attachment (what the merge and delete paths want), and empty kinds means none.
func (s *Store) ListAttachments(ctx context.Context, chatID int64, kinds []string, beforeSentAt string, beforeID int64, limit int) ([]Attachment, error) {
	if len(kinds) == 0 {
		return nil, nil
	}
	// chatID 0 means every chat: the sidebar's global gallery.
	scope, args := "a.chat_id = ?", []any{chatID}
	if chatID == 0 {
		scope, args = "1 = 1", nil
	}
	for _, k := range kinds {
		args = append(args, k)
	}
	// GROUP BY a.id, not a plain JOIN: an attachment sent several times (the common case
	// for stickers) has one messages row per send, and the ungrouped JOIN repeated the
	// same sticker in the gallery once per message.
	query := `
		SELECT a.id, a.chat_id, a.filename, a.sha256, a.media_kind, COALESCE(a.mime, ''),
		       a.stored_path, COALESCE(a.size_bytes, 0), MAX(m.sent_at) AS sent_at
		FROM attachments a
		JOIN messages m ON m.attachment_id = a.id
		WHERE ` + scope + ` AND a.media_kind IN (?` + strings.Repeat(", ?", len(kinds)-1) + `)
		GROUP BY a.id`
	if beforeSentAt != "" {
		query += ` HAVING sent_at < ? OR (sent_at = ? AND a.id < ?)`
		args = append(args, beforeSentAt, beforeSentAt, beforeID)
	}
	query += ` ORDER BY sent_at DESC, a.id DESC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := s.read.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attachments []Attachment
	for rows.Next() {
		var a Attachment
		if err := rows.Scan(&a.ID, &a.ChatID, &a.Filename, &a.SHA256, &a.MediaKind, &a.Mime,
			&a.StoredPath, &a.SizeBytes, &a.SentAt); err != nil {
			return nil, err
		}
		attachments = append(attachments, a)
	}
	return attachments, rows.Err()
}

// Every media_kind in one pass, for the gallery tabs.
func (s *Store) CountAttachmentsByKind(ctx context.Context, chatID int64) (map[string]int, error) {
	query := `SELECT media_kind, COUNT(*) FROM attachments WHERE chat_id = ? GROUP BY media_kind`
	args := []any{chatID}
	if chatID == 0 {
		query = `SELECT media_kind, COUNT(*) FROM attachments GROUP BY media_kind`
		args = nil
	}
	rows, err := s.read.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var kind string
		var n int
		if err := rows.Scan(&kind, &n); err != nil {
			return nil, err
		}
		counts[kind] = n
	}
	return counts, rows.Err()
}

// The SQL filter is coarse (LIKE '%http%'); render.ExtractURLs does the real extraction
// in the caller.
func (s *Store) ListMessagesWithLink(ctx context.Context, chatID int64) ([]Message, error) {
	query := `
		SELECT id, sent_at, seq, sender, body, kind
		FROM messages
		WHERE chat_id = ? AND body LIKE '%http%'
		ORDER BY sent_at DESC, seq DESC`
	args := []any{chatID}
	if chatID == 0 {
		query = `
		SELECT id, sent_at, seq, sender, body, kind
		FROM messages
		WHERE body LIKE '%http%'
		ORDER BY sent_at DESC, seq DESC`
		args = nil
	}
	rows, err := s.read.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		var sender sql.NullString
		if err := rows.Scan(&m.ID, &m.SentAt, &m.Seq, &sender, &m.Body, &m.Kind); err != nil {
			return nil, err
		}
		if sender.Valid {
			m.Sender = &sender.String
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

func (s *Store) SetOwner(ctx context.Context, chatID int64, owner string) error {
	_, err := s.write.ExecContext(ctx, `UPDATE chats SET owner = ? WHERE id = ?`, owner, chatID)
	return err
}

// Subject to UNIQUE(name, is_group); the caller handles the collision.
func (s *Store) RenameChat(ctx context.Context, chatID int64, name string) error {
	_, err := s.write.ExecContext(ctx, `UPDATE chats SET name = ? WHERE id = ?`, name, chatID)
	return err
}

// An empty path clears it.
func (s *Store) SetAvatar(ctx context.Context, chatID int64, path string) error {
	var value sql.NullString
	if path != "" {
		value = sql.NullString{String: path, Valid: true}
	}
	_, err := s.write.ExecContext(ctx, `UPDATE chats SET avatar_path = ? WHERE id = ?`, value, chatID)
	return err
}

// An empty value clears it.
func (s *Store) SetPhone(ctx context.Context, chatID int64, phone string) error {
	var value sql.NullString
	if phone != "" {
		value = sql.NullString{String: phone, Valid: true}
	}
	_, err := s.write.ExecContext(ctx, `UPDATE chats SET phone = ? WHERE id = ?`, value, chatID)
	return err
}

func (s *Store) SendersByVolume(ctx context.Context, chatID int64) ([]string, error) {
	rows, err := s.read.QueryContext(ctx, `
		SELECT sender FROM messages
		WHERE chat_id = ? AND sender IS NOT NULL
		GROUP BY sender ORDER BY COUNT(*) DESC`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var senders []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return nil, err
		}
		senders = append(senders, r)
	}
	return senders, rows.Err()
}

// Total counts only non-archived chats.
type List struct {
	ID    int64
	Name  string
	Total int
}

func (s *Store) ListLists(ctx context.Context) ([]List, error) {
	rows, err := s.read.QueryContext(ctx, `
		SELECT l.id, l.name,
		       (SELECT COUNT(*) FROM chat_lists cl JOIN chats c ON c.id = cl.chat_id
		         WHERE cl.list_id = l.id AND c.archived = 0)
		FROM lists l ORDER BY l.name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lists []List
	for rows.Next() {
		var l List
		if err := rows.Scan(&l.ID, &l.Name, &l.Total); err != nil {
			return nil, err
		}
		lists = append(lists, l)
	}
	return lists, rows.Err()
}

// A duplicate name violates UNIQUE; the caller handles it.
func (s *Store) CreateList(ctx context.Context, name string) (int64, error) {
	res, err := s.write.ExecContext(ctx,
		`INSERT INTO lists (name, created_at) VALUES (?, ?)`,
		name, time.Now().Format("2006-01-02 15:04:05"))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Associations go by CASCADE; no chat is touched.
func (s *Store) DeleteList(ctx context.Context, listID int64) error {
	_, err := s.write.ExecContext(ctx, `DELETE FROM lists WHERE id = ?`, listID)
	return err
}

func (s *Store) SetChatInList(ctx context.Context, chatID, listID int64, in bool) error {
	if in {
		_, err := s.write.ExecContext(ctx,
			`INSERT OR IGNORE INTO chat_lists (chat_id, list_id) VALUES (?, ?)`, chatID, listID)
		return err
	}
	_, err := s.write.ExecContext(ctx,
		`DELETE FROM chat_lists WHERE chat_id = ? AND list_id = ?`, chatID, listID)
	return err
}

func (s *Store) ChatLists(ctx context.Context, chatID int64) (map[int64]bool, error) {
	rows, err := s.read.QueryContext(ctx, `SELECT list_id FROM chat_lists WHERE chat_id = ?`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	in := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		in[id] = true
	}
	return in, rows.Err()
}

func (s *Store) CountByFilter(ctx context.Context, filter ChatFilter) (int, error) {
	query := `SELECT COUNT(*) FROM chats c WHERE c.archived = ?`
	args := []any{filter.Archived}
	if filter.Favorites {
		query += ` AND c.favorite = 1`
	}
	if filter.Groups {
		query += ` AND c.is_group = 1`
	}
	var n int
	err := s.read.QueryRowContext(ctx, query, args...).Scan(&n)
	return n, err
}

// One query for every chat at once: chat_id -> list_id -> true.
func (s *Store) ListAssociations(ctx context.Context) (map[int64]map[int64]bool, error) {
	rows, err := s.read.QueryContext(ctx, `SELECT chat_id, list_id FROM chat_lists`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	assoc := map[int64]map[int64]bool{}
	for rows.Next() {
		var chatID, listID int64
		if err := rows.Scan(&chatID, &listID); err != nil {
			return nil, err
		}
		if assoc[chatID] == nil {
			assoc[chatID] = map[int64]bool{}
		}
		assoc[chatID][listID] = true
	}
	return assoc, rows.Err()
}

func (s *Store) CountPinned(ctx context.Context) (int, error) {
	var n int
	err := s.read.QueryRowContext(ctx, `SELECT COUNT(*) FROM chats WHERE pinned = 1`).Scan(&n)
	return n, err
}

// Original sender -> display name.
func (s *Store) ChatNicknames(ctx context.Context, chatID int64) (map[string]string, error) {
	rows, err := s.read.QueryContext(ctx, `SELECT sender, name FROM nicknames WHERE chat_id = ?`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	nicknames := map[string]string{}
	for rows.Next() {
		var sender, name string
		if err := rows.Scan(&sender, &name); err != nil {
			return nil, err
		}
		nicknames[sender] = name
	}
	return nicknames, rows.Err()
}

// An empty name clears it. messages.sender stays untouched: it is export data and
// feeds the dedupe hash.
func (s *Store) SetNickname(ctx context.Context, chatID int64, sender, name string) error {
	if name == "" {
		_, err := s.write.ExecContext(ctx,
			`DELETE FROM nicknames WHERE chat_id = ? AND sender = ?`, chatID, sender)
		return err
	}
	_, err := s.write.ExecContext(ctx, `
		INSERT INTO nicknames (chat_id, sender, name) VALUES (?, ?, ?)
		ON CONFLICT(chat_id, sender) DO UPDATE SET name = excluded.name`,
		chatID, sender, name)
	return err
}

// One message at a time, without materializing everything in memory.
func (s *Store) IterateMessages(ctx context.Context, chatID int64, visit func(Message) error) error {
	rows, err := s.read.QueryContext(ctx, `
		SELECT `+messageColumnsWithAttachment+`
		FROM messages m LEFT JOIN attachments a ON a.id = m.attachment_id
		WHERE m.chat_id = ?
		ORDER BY m.sent_at ASC, m.seq ASC`, chatID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		m, err := scanOneMessage(rows)
		if err != nil {
			return err
		}
		if err := visit(m); err != nil {
			return err
		}
	}
	return rows.Err()
}

// Returns the resulting (favorite, pinned, sentAt): the caller needs all three to
// re-render the message-actions fragment without a second query. Scoped by chat_id,
// so a forged id from another chat is a no-op.
func (s *Store) ToggleMessageFavorite(ctx context.Context, chatID, messageID int64) (favorite, pinned bool, sentAt string, err error) {
	err = s.write.QueryRowContext(ctx,
		`UPDATE messages SET favorite = NOT favorite
		 WHERE id = ? AND chat_id = ? AND kind != 'system'
		 RETURNING favorite, pinned, sent_at`, messageID, chatID).Scan(&favorite, &pinned, &sentAt)
	return favorite, pinned, sentAt, err
}

// MaxPinnedMessages is how many messages a chat can keep pinned. Pinning one more
// evicts the one pinned longest ago instead of refusing, as the official app does.
// Exported so the UI can warn before the eviction happens.
const MaxPinnedMessages = 4

// Mirrors ToggleMessageFavorite, plus the eviction the limit needs — both in one
// transaction, or a crash between them would leave the chat over the limit.
func (s *Store) TogglePinnedMessage(ctx context.Context, chatID, messageID int64) (favorite, pinned bool, sentAt string, err error) {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return false, false, "", err
	}
	defer tx.Rollback()

	// Local time, not CURRENT_TIMESTAMP: SQLite's is UTC, while migration 013 backfilled
	// pinned_at from sent_at, which is the export's local time. Mixing the two scales
	// misorders the strip — and that order is what decides which pin gets evicted.
	now := time.Now().Format("2006-01-02 15:04:05")
	err = tx.QueryRowContext(ctx,
		`UPDATE messages
		 SET pinned = NOT pinned,
		     pinned_at = CASE WHEN pinned THEN NULL ELSE ? END
		 WHERE id = ? AND chat_id = ? AND kind != 'system'
		 RETURNING favorite, pinned, sent_at`, now, messageID, chatID).Scan(&favorite, &pinned, &sentAt)
	if err != nil {
		return false, false, "", err
	}

	if pinned {
		// Keeps the (limit - 1) most recently pinned OTHERS, so the message just pinned can
		// never evict itself — excluded by id rather than trusting pinned_at, which a clock
		// jump can invert (a test with a skewed pinned_at did exactly that). Evicts everything
		// past the limit, so the same statement repairs a chat that ended up over it.
		if _, err = tx.ExecContext(ctx, `
			UPDATE messages SET pinned = 0, pinned_at = NULL
			WHERE id IN (
			    SELECT id FROM messages
			    WHERE chat_id = ? AND pinned = 1 AND id != ?
			    ORDER BY pinned_at DESC, id DESC
			    LIMIT -1 OFFSET ?
			)`, chatID, messageID, MaxPinnedMessages-1); err != nil {
			return false, false, "", err
		}
	}

	return favorite, pinned, sentAt, tx.Commit()
}

func (s *Store) ListFavoriteMessages(ctx context.Context, chatID int64) ([]Message, error) {
	rows, err := s.read.QueryContext(ctx, `
		SELECT `+messageColumnsWithAttachment+`
		FROM messages m LEFT JOIN attachments a ON a.id = m.attachment_id
		WHERE m.chat_id = ? AND m.favorite = 1
		ORDER BY m.sent_at DESC, m.seq DESC`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMessages(rows)
}

// Most recently pinned first — the same order the limit evicts from, so the strip's
// last item is always the next one out.
func (s *Store) ListPinnedMessages(ctx context.Context, chatID int64) ([]Message, error) {
	rows, err := s.read.QueryContext(ctx, `
		SELECT `+messageColumnsWithAttachment+`
		FROM messages m LEFT JOIN attachments a ON a.id = m.attachment_id
		WHERE m.chat_id = ? AND m.pinned = 1
		ORDER BY m.pinned_at DESC, m.id DESC`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMessages(rows)
}
