-- One row per app-wide preference the user sets in the UI. Today only "me": the sender
-- treated as the user, which VAULTZAP_ME also sets — the setting wins when both exist.
CREATE TABLE settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
