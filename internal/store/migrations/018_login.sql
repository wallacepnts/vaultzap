-- Password for the login screen. PBKDF2-HMAC-SHA256 over a per-install random salt, with
-- the iteration count stored alongside so it can be raised later without invalidating the
-- password already set. A single row: this is a one-user archive, not an accounts table.
CREATE TABLE credentials (
  id            INTEGER PRIMARY KEY CHECK (id = 1),
  password_hash BLOB    NOT NULL,
  salt          BLOB    NOT NULL,
  iterations    INTEGER NOT NULL,
  updated_at    TEXT    NOT NULL
);

-- Browser sessions. Only the token's digest is kept, never the token itself: a database
-- that leaks (it is a plain file the user copies around for backup) must not hand over
-- cookies that still work.
CREATE TABLE sessions (
  token_hash BLOB PRIMARY KEY,
  created_at TEXT NOT NULL,
  expires_at TEXT NOT NULL
);

CREATE INDEX idx_sessions_expiry ON sessions(expires_at);
