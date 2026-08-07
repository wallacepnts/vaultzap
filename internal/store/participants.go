package store

import "context"

// ParticipantAvatar is a photo the user attached to a group participant: an upload of
// their own (Path) or another chat's photo (LinkedChatID). Only one is ever set.
type ParticipantAvatar struct {
	Path         string
	LinkedChatID int64
}

func (s *Store) ParticipantAvatars(ctx context.Context, chatID int64) (map[string]ParticipantAvatar, error) {
	rows, err := s.read.QueryContext(ctx, `
		SELECT sender, avatar_path, COALESCE(linked_chat_id, 0)
		FROM participant_avatars WHERE chat_id = ?`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	avatars := map[string]ParticipantAvatar{}
	for rows.Next() {
		var sender string
		var a ParticipantAvatar
		if err := rows.Scan(&sender, &a.Path, &a.LinkedChatID); err != nil {
			return nil, err
		}
		avatars[sender] = a
	}
	return avatars, rows.Err()
}

func (s *Store) SetParticipantAvatarPath(ctx context.Context, chatID int64, sender, path string) error {
	_, err := s.write.ExecContext(ctx, `
		INSERT INTO participant_avatars (chat_id, sender, avatar_path, linked_chat_id)
		VALUES (?, ?, ?, NULL)
		ON CONFLICT (chat_id, sender) DO UPDATE SET avatar_path = excluded.avatar_path, linked_chat_id = NULL`,
		chatID, sender, path)
	return err
}

func (s *Store) LinkParticipantAvatar(ctx context.Context, chatID int64, sender string, linkedChatID int64) error {
	_, err := s.write.ExecContext(ctx, `
		INSERT INTO participant_avatars (chat_id, sender, avatar_path, linked_chat_id)
		VALUES (?, ?, '', ?)
		ON CONFLICT (chat_id, sender) DO UPDATE SET avatar_path = '', linked_chat_id = excluded.linked_chat_id`,
		chatID, sender, linkedChatID)
	return err
}

// The uploaded file is left on disk, like the chat avatar and attachments.
func (s *Store) RemoveParticipantAvatar(ctx context.Context, chatID int64, sender string) error {
	_, err := s.write.ExecContext(ctx,
		`DELETE FROM participant_avatars WHERE chat_id = ? AND sender = ?`, chatID, sender)
	return err
}

// The chats whose photo can be borrowed for a participant.
func (s *Store) ChatsWithPhoto(ctx context.Context, term string) ([]Chat, error) {
	rows, err := s.read.QueryContext(ctx, `
		SELECT id, name, is_group, COALESCE(avatar_path, '')
		FROM chats
		WHERE COALESCE(avatar_path, '') <> '' AND (? = '' OR name LIKE '%' || ? || '%')
		ORDER BY is_group, name
		LIMIT 50`, term, term)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []Chat
	for rows.Next() {
		var c Chat
		if err := rows.Scan(&c.ID, &c.Name, &c.IsGroup, &c.AvatarPath); err != nil {
			return nil, err
		}
		chats = append(chats, c)
	}
	return chats, rows.Err()
}
