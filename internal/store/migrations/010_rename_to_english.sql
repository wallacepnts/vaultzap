-- Renames the schema's last Portuguese identifiers to English.

ALTER TABLE chats RENAME COLUMN telefone TO phone;

ALTER TABLE listas RENAME TO lists;
ALTER TABLE lists RENAME COLUMN nome TO name;

ALTER TABLE chat_listas RENAME TO chat_lists;
ALTER TABLE chat_lists RENAME COLUMN lista_id TO list_id;
DROP INDEX IF EXISTS idx_chat_listas_lista;
CREATE INDEX idx_chat_lists_list ON chat_lists(list_id);

ALTER TABLE apelidos RENAME TO nicknames;
ALTER TABLE nicknames RENAME COLUMN remetente TO sender;
ALTER TABLE nicknames RENAME COLUMN nome TO name;

-- schema_migrations (versao/aplicada_em) is left unchanged: the file name is the
-- version key, so renaming would reapply every migration.
