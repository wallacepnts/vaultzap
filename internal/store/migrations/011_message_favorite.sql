-- Starred messages, mirroring the official app.

-- A column on messages, not a side table: it travels with the row when a chat is merged
-- (which repoints messages.chat_id) and survives reimport, which is INSERT OR IGNORE and
-- never rewrites an existing row.
ALTER TABLE messages ADD COLUMN favorite INTEGER NOT NULL DEFAULT 0;

-- Partial index: only starred rows are indexed, so listing stays cheap without weighing
-- on the insert of every ordinary message during an import.
CREATE INDEX idx_msg_favorite ON messages(chat_id, sent_at, seq) WHERE favorite = 1;
