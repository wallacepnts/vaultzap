-- Full-text search index (FTS5) and its sync triggers.

CREATE VIRTUAL TABLE messages_fts USING fts5(
  body, content='messages', content_rowid='id',
  tokenize="unicode61 remove_diacritics 2"
);

CREATE TRIGGER messages_fts_ai AFTER INSERT ON messages BEGIN
  INSERT INTO messages_fts(rowid, body) VALUES (new.id, new.body);
END;

CREATE TRIGGER messages_fts_ad AFTER DELETE ON messages BEGIN
  INSERT INTO messages_fts(messages_fts, rowid, body) VALUES('delete', old.id, old.body);
END;

CREATE TRIGGER messages_fts_au AFTER UPDATE ON messages BEGIN
  INSERT INTO messages_fts(messages_fts, rowid, body) VALUES('delete', old.id, old.body);
  INSERT INTO messages_fts(rowid, body) VALUES (new.id, new.body);
END;

-- Indexes messages that already existed before this migration.
INSERT INTO messages_fts(messages_fts) VALUES('rebuild');
