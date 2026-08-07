-- Archived is UI state only; no message or file is touched.
ALTER TABLE chats ADD COLUMN archived INTEGER NOT NULL DEFAULT 0;
