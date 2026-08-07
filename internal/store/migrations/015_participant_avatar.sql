-- A photo the user attaches to a group participant: either an upload of their own
-- (avatar_path) or a link to another chat's photo (linked_chat_id), never both.
-- Same key as nicknames, but a separate table: clearing a nickname deletes that row,
-- which would take the photo with it.
CREATE TABLE participant_avatars (
  chat_id        INTEGER NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
  sender         TEXT    NOT NULL,
  avatar_path    TEXT    NOT NULL DEFAULT '',
  linked_chat_id INTEGER REFERENCES chats(id) ON DELETE CASCADE,
  PRIMARY KEY (chat_id, sender)
);
