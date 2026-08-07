-- Index on messages.hash alone, separate from UNIQUE(chat_id, hash).
CREATE INDEX IF NOT EXISTS idx_msg_hash ON messages(hash);
