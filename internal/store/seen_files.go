package store

import (
	"context"
	"database/sql"
)

const (
	StatePending = "pending"
	StateStable  = "stable"
	StateDone    = "done"
	StateError   = "error"
	StateIgnored = "ignored"
)

// SeenFile is the projection of a seen_files row: whether a file has stabilized and can
// be imported.
type SeenFile struct {
	Path     string
	Size     int64
	Mtime    string
	SHA256   *string
	State    string
	LastSeen string
}

// ok=false when there is no record for the path.
func (s *Store) GetSeenFile(ctx context.Context, path string) (file SeenFile, ok bool, err error) {
	var sum sql.NullString
	err = s.read.QueryRowContext(ctx, `
		SELECT path, size_bytes, mtime, sha256, state, last_seen
		FROM seen_files WHERE path = ?`, path,
	).Scan(&file.Path, &file.Size, &file.Mtime, &sum, &file.State, &file.LastSeen)
	if err == sql.ErrNoRows {
		return SeenFile{}, false, nil
	}
	if err != nil {
		return SeenFile{}, false, err
	}
	if sum.Valid {
		file.SHA256 = &sum.String
	}
	return file, true, nil
}

// SaveSeenFile inserts or updates the seen_files entry.
func (s *Store) SaveSeenFile(ctx context.Context, f SeenFile) error {
	_, err := s.write.ExecContext(ctx, `
		INSERT INTO seen_files (path, size_bytes, mtime, sha256, state, last_seen)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			size_bytes = excluded.size_bytes,
			mtime      = excluded.mtime,
			sha256     = excluded.sha256,
			state      = excluded.state,
			last_seen  = excluded.last_seen`,
		f.Path, f.Size, f.Mtime, f.SHA256, f.State, f.LastSeen)
	return err
}
