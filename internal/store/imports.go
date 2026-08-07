package store

import (
	"context"
	"database/sql"
	"strings"
)

type Import struct {
	ID       int64
	Path     string
	ChatID   *int64
	ChatName string
	Added    int
	Skipped  int
	Warnings int
	// The parser's warnings ("linha 42: ...").
	WarningText []string
	// Checks are the coherence findings about the file, as stored JSON.
	Checks     string
	Status     string
	Error      string
	StartedAt  string
	SizeBytes  int64
	FinishedAt string
}

// ListImports returns the import history, most recent first.
func (s *Store) ListImports(ctx context.Context) ([]Import, error) {
	rows, err := s.read.QueryContext(ctx, `
		SELECT i.id, i.path, i.chat_id, COALESCE(c.name, ''),
			i.added, i.skipped, i.warnings, i.status, COALESCE(i.error, ''),
			i.started_at, COALESCE(i.finished_at, ''), COALESCE(i.warnings_text, ''),
			COALESCE(i.checks_text, ''), COALESCE(i.size_bytes, 0)
		FROM imports i LEFT JOIN chats c ON c.id = i.chat_id
		ORDER BY i.started_at DESC, i.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Import
	for rows.Next() {
		var imp Import
		var chatID sql.NullInt64
		var warnings string
		if err := rows.Scan(
			&imp.ID, &imp.Path, &chatID, &imp.ChatName,
			&imp.Added, &imp.Skipped, &imp.Warnings, &imp.Status, &imp.Error,
			&imp.StartedAt, &imp.FinishedAt, &warnings, &imp.Checks, &imp.SizeBytes,
		); err != nil {
			return nil, err
		}
		if chatID.Valid {
			imp.ChatID = &chatID.Int64
		}
		if warnings != "" {
			imp.WarningText = strings.Split(warnings, "\n")
		}
		list = append(list, imp)
	}
	return list, rows.Err()
}

// Counts imports with an error or still running.
func (s *Store) CountImportsNeedingAttention(ctx context.Context) (int, error) {
	var n int
	err := s.read.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM imports WHERE status IN ('error', 'running')`).Scan(&n)
	return n, err
}
