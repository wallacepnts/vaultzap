package store

import (
	"context"
	"database/sql"
	"errors"
)

// SettingMe is the sender the user answers to, the UI's counterpart of VAULTZAP_ME.
const SettingMe = "me"

func (s *Store) Setting(ctx context.Context, key string) (string, error) {
	var value string
	err := s.read.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return value, err
}

// SetSetting stores the value, or removes the row when it is empty — an absent setting is
// what makes the env var take over again.
func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	if value == "" {
		_, err := s.write.ExecContext(ctx, `DELETE FROM settings WHERE key = ?`, key)
		return err
	}
	_, err := s.write.ExecContext(ctx, `
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}
