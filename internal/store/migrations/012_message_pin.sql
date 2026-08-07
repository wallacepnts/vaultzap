-- Pinned messages. Same reasoning as 011_message_favorite.sql: a column on messages, not
-- a side table, so it survives a merge and a reimport.
ALTER TABLE messages ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0;

CREATE INDEX idx_msg_pinned ON messages(chat_id, sent_at, seq) WHERE pinned = 1;
