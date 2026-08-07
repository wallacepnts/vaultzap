-- Pin/favorite as flags on the chat; chat lists live in N:N tables.
ALTER TABLE chats ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0;
ALTER TABLE chats ADD COLUMN favorite INTEGER NOT NULL DEFAULT 0;

CREATE TABLE listas (
  id         INTEGER PRIMARY KEY,
  nome       TEXT    NOT NULL UNIQUE,
  created_at TEXT    NOT NULL
);

CREATE TABLE chat_listas (
  chat_id  INTEGER NOT NULL REFERENCES chats(id)  ON DELETE CASCADE,
  lista_id INTEGER NOT NULL REFERENCES listas(id) ON DELETE CASCADE,
  PRIMARY KEY (chat_id, lista_id)
);

CREATE INDEX idx_chat_listas_lista ON chat_listas(lista_id);
