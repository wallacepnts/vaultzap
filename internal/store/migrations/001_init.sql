-- Initial schema: chats, messages, attachments, imports, seen_files.

CREATE TABLE chats (
  id               INTEGER PRIMARY KEY,
  name             TEXT    NOT NULL,          -- from the file name / do nome do arquivo
  is_group         INTEGER NOT NULL DEFAULT 0,
  source           TEXT    NOT NULL,          -- 'android' | 'ios'
  owner            TEXT,                      -- sender treated as "me" (bubble on the right) / remetente tratado como "eu" (bolha à direita)
  first_message_at TEXT,
  last_message_at  TEXT,
  message_count    INTEGER NOT NULL DEFAULT 0,
  created_at       TEXT    NOT NULL,
  UNIQUE (name, is_group)
);

CREATE TABLE attachments (
  id          INTEGER PRIMARY KEY,
  chat_id     INTEGER NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
  filename    TEXT    NOT NULL,               -- original name cited in the .txt / nome original citado no .txt
  sha256      TEXT    NOT NULL,
  media_kind  TEXT    NOT NULL,               -- image|video|audio|voice|sticker|gif|document
  mime        TEXT,
  size_bytes  INTEGER,
  width       INTEGER,
  height      INTEGER,
  stored_path TEXT    NOT NULL,               -- relative to MEDIA_DIR / relativo a MEDIA_DIR
  UNIQUE (chat_id, sha256)
);

CREATE TABLE messages (
  id            INTEGER PRIMARY KEY,
  chat_id       INTEGER NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
  sent_at       TEXT    NOT NULL,
  seq           INTEGER NOT NULL,             -- order in the file; breaks ties on equal timestamps / ordem no arquivo; desempata timestamps iguais
  sender        TEXT,                         -- NULL = system message / NULL = mensagem de sistema
  body          TEXT    NOT NULL,
  kind          TEXT    NOT NULL,             -- text|system|media|media_omitted|deleted|call|location|contact
  attachment_id INTEGER REFERENCES attachments(id),
  hash          TEXT    NOT NULL,             -- sha256(sent_at|sender|body|filename)
  UNIQUE (chat_id, hash)
);
CREATE INDEX idx_msg_chat_order ON messages(chat_id, sent_at, seq);

CREATE TABLE imports (
  id           INTEGER PRIMARY KEY,
  path         TEXT    NOT NULL,              -- path relative to INBOX / caminho relativo à INBOX
  sha256       TEXT    NOT NULL UNIQUE,       -- re-ingesting the same content is a no-op / reingerir o mesmo conteúdo é no-op
  chat_id      INTEGER REFERENCES chats(id) ON DELETE SET NULL,
  added        INTEGER NOT NULL DEFAULT 0,
  skipped      INTEGER NOT NULL DEFAULT 0,
  warnings     INTEGER NOT NULL DEFAULT 0,
  status       TEXT    NOT NULL,              -- running|done|error
  error        TEXT,
  started_at   TEXT    NOT NULL,
  finished_at  TEXT
);

CREATE TABLE seen_files (
  path        TEXT PRIMARY KEY,               -- relative to INBOX / relativo à INBOX
  size_bytes  INTEGER NOT NULL,
  mtime       TEXT    NOT NULL,
  sha256      TEXT,                           -- NULL until the file settles / NULL até o arquivo estabilizar
  state       TEXT    NOT NULL,               -- pending|stable|done|error|ignored
  last_seen   TEXT    NOT NULL
);

