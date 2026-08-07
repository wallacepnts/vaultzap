// Package store handles SQLite access: migrations and domain queries.
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Store wraps the SQLite connection.
type Store struct {
	// Two handles over the same file. WAL lets readers run while a writer holds its
	// transaction, but a single pooled connection would queue them behind it: during a
	// 6.7 GiB import every page that touches the database hung for minutes (§11.36).
	//
	// read has several connections; write has exactly one, which is what keeps writes
	// serialized by the pool instead of racing into SQLITE_BUSY.
	read  *sql.DB
	write *sql.DB
}

// Open opens (or creates) the database, applies pending migrations and sets the required
// PRAGMAs (WAL, foreign_keys).
func Open(ctx context.Context, path string) (*Store, error) {
	// SQLite does not create the directory, and the driver's failure ("unable to open
	// database file (14)") says nothing about which path or why.
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("criar diretório do banco %s: %w", dir, err)
		}
	}

	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)"

	write, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("abrir banco: %w", err)
	}
	write.SetMaxOpenConns(1)

	// Migrations run before the read handle exists: the file has to have its schema (and
	// journal_mode=WAL) before anything reads it.
	if err := applyMigrations(ctx, write); err != nil {
		write.Close()
		return nil, fmt.Errorf("aplicar migrations: %w", err)
	}

	read, err := sql.Open("sqlite", dsn)
	if err != nil {
		write.Close()
		return nil, fmt.Errorf("abrir banco para leitura: %w", err)
	}
	read.SetMaxOpenConns(maxReaders)

	return &Store{read: read, write: write}, nil
}

// What the UI needs at once: a page load fires a handful of fragments in parallel.
const maxReaders = 4

func (s *Store) Close() error {
	errRead := s.read.Close()
	if err := s.write.Close(); err != nil {
		return err
	}
	return errRead
}

// DB is the write handle, serialized by its single connection. Reads use s.read.
func (s *Store) DB() *sql.DB {
	return s.write
}

// Creates schema_migrations if needed and applies each pending file in order. The file
// name is the version key, so a migration must never be renamed.
func applyMigrations(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			versao      TEXT PRIMARY KEY,
			aplicada_em TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now'))
		)`); err != nil {
		return err
	}

	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var alreadyApplied bool
		row := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE versao = ?)`, name)
		if err := row.Scan(&alreadyApplied); err != nil {
			return err
		}
		if alreadyApplied {
			continue
		}

		content, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (versao) VALUES (?)`, name); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}
