package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Credential is the stored login. Complete reports whether the user finished the setup
// screen; anything short of that sends every request back to it.
type Credential struct {
	Username   string
	Hash       []byte
	Salt       []byte
	Iterations int
}

func (c Credential) Complete() bool {
	return c.Username != "" && len(c.Hash) > 0
}

func (s *Store) Credential(ctx context.Context) (Credential, error) {
	var c Credential
	err := s.read.QueryRowContext(ctx,
		`SELECT username, password_hash, salt, iterations FROM credentials WHERE id = 1`).
		Scan(&c.Username, &c.Hash, &c.Salt, &c.Iterations)
	if errors.Is(err, sql.ErrNoRows) {
		return Credential{}, nil
	}
	return c, err
}

// SetCredential replaces the password and drops every session: changing the password has
// to log out the browsers that were already in, or it protects nothing.
func (s *Store) SetCredential(ctx context.Context, c Credential) error {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO credentials (id, username, password_hash, salt, iterations, updated_at)
		VALUES (1, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			username      = excluded.username,
			password_hash = excluded.password_hash,
			salt          = excluded.salt,
			iterations    = excluded.iterations,
			updated_at    = excluded.updated_at`,
		c.Username, c.Hash, c.Salt, c.Iterations, localNow()); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions`); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CreateSession(ctx context.Context, digest []byte, expiresAt time.Time) error {
	_, err := s.write.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, created_at, expires_at) VALUES (?, ?, ?)`,
		digest, localNow(), expiresAt.Format(timeLayout))
	return err
}

// SessionValid also clears expired rows as it goes, so nothing else has to sweep them.
func (s *Store) SessionValid(ctx context.Context, digest []byte) (bool, error) {
	now := localNow()
	if _, err := s.write.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, now); err != nil {
		return false, err
	}
	var one int
	err := s.read.QueryRowContext(ctx,
		`SELECT 1 FROM sessions WHERE token_hash = ? AND expires_at > ?`, digest, now).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *Store) DeleteSession(ctx context.Context, digest []byte) error {
	_, err := s.write.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, digest)
	return err
}

const timeLayout = "2006-01-02 15:04:05"

// Local time, not CURRENT_TIMESTAMP: SQLite's is UTC while every other timestamp in this
// database is the export's local time, and mixing the two scales here would expire
// sessions hours early or late depending on the zone.
func localNow() string {
	return time.Now().Format(timeLayout)
}
