-- Display name the user picked for a group participant. messages.sender itself is
-- never touched: it is export data and feeds the dedupe hash.
CREATE TABLE apelidos (
  chat_id   INTEGER NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
  remetente TEXT    NOT NULL,   -- exactly as it comes in the export / exatamente como vem no export
  nome      TEXT    NOT NULL,   -- how to display it / como exibir
  PRIMARY KEY (chat_id, remetente)
);
