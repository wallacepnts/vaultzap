package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOpen_appliesMigrationsIdempotently(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "vaultzap.db")

	s1, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("primeira abertura: %v", err)
	}

	var table string
	err = s1.DB().QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type='table' AND name='chats'`).Scan(&table)
	if err != nil {
		t.Fatalf("tabela chats não foi criada: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("fechar: %v", err)
	}

	// Reopening the same database must not reapply a migration.
	s2, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("segunda abertura: %v", err)
	}
	defer s2.Close()

	files, err := filepath.Glob("migrations/*.sql")
	if err != nil {
		t.Fatalf("listar migrations: %v", err)
	}

	var versoes int
	if err := s2.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&versoes); err != nil {
		t.Fatalf("consultar schema_migrations: %v", err)
	}
	if versoes != len(files) {
		t.Errorf("schema_migrations tem %d linhas, esperado %d (uma por arquivo em migrations/)", versoes, len(files))
	}
}

// SQLite does not create the directory, and its error ("unable to open database file
// (14)") does not say which path is missing — a confusing first run.
func TestOpen_createsDatabaseDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sem", "esta", "pasta", "vaultzap.db")
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open deveria criar o diretório: %v", err)
	}
	defer s.Close()

	if _, err := os.Stat(path); err != nil {
		t.Errorf("banco não foi criado em %s: %v", path, err)
	}
}
