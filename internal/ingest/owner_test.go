package ingest

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/wallacepnts/vaultzap/internal/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "vaultzap.db"))
	if err != nil {
		t.Fatalf("abrir banco: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func chatOwner(t *testing.T, s *store.Store, name string) string {
	t.Helper()
	var owner sql.NullString
	if err := s.DB().QueryRow(`SELECT owner FROM chats WHERE name = ?`, name).Scan(&owner); err != nil {
		t.Fatalf("buscar dono de %q: %v", name, err)
	}
	return owner.String
}

func importText(t *testing.T, s *store.Store, fileName, content string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, fileName)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("escrever fixture: %v", err)
	}
	if _, err := ImportFile(context.Background(), s, path, fileName, t.TempDir(), ""); err != nil {
		t.Fatalf("ImportFile(%s): %v", fileName, err)
	}
}

func TestInferOwner(t *testing.T) {
	s := openTestStore(t)

	importText(t, s, "Conversa do WhatsApp com Ana Souza.txt", `26/07/2026 09:00 - Ana Souza: oi
26/07/2026 09:01 - Wallace Pontes: oi, tudo bem?
26/07/2026 09:02 - Ana Souza: tudo!
`)
	if owner := chatOwner(t, s, "Ana Souza"); owner != "Wallace Pontes" {
		t.Errorf("dono = %q, esperado Wallace Pontes (o remetente que não é o nome do arquivo)", owner)
	}
}

func TestInferOwner_casesThatMustNotInfer(t *testing.T) {
	cases := []struct {
		name        string
		file        string
		content     string
		chatNoBanco string
		porque      string
	}{
		{
			name: "grupo",
			file: "Conversa do WhatsApp com Time.txt",
			content: `26/07/2026 09:00 - Time: Ana criou o grupo "Time"
26/07/2026 09:01 - Ana: oi
26/07/2026 09:02 - Bruno: oi
26/07/2026 09:03 - Wallace: oi
`,
			chatNoBanco: "Time",
			porque:      "em grupo o nome do arquivo é do grupo, não de um participante",
		},
		{
			name: "nenhum remetente bate com o nome do arquivo",
			file: "Conversa do WhatsApp com Ana Souza.txt",
			content: `26/07/2026 09:00 - Aninha: oi
26/07/2026 09:01 - Wallace: oi
`,
			chatNoBanco: "Ana Souza",
			porque:      "o apelido no export não bate com o nome do arquivo; fica pro seletor",
		},
		{
			name: "só um remetente",
			file: "Conversa do WhatsApp com Ana.txt",
			content: `26/07/2026 09:00 - Ana: oi
26/07/2026 09:01 - Ana: alguém aí?
`,
			chatNoBanco: "Ana",
			porque:      "sem um segundo remetente não há quem seja 'eu'",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := openTestStore(t)
			importText(t, s, c.file, c.content)
			if owner := chatOwner(t, s, c.chatNoBanco); owner != "" {
				t.Errorf("dono = %q, esperado vazio — %s", owner, c.porque)
			}
		})
	}
}

func TestInferOwner_doesNotOverwriteUserChoice(t *testing.T) {
	s := openTestStore(t)
	const file = "Conversa do WhatsApp com Ana.txt"
	const content = `26/07/2026 09:00 - Ana: oi
26/07/2026 09:01 - Wallace: oi
`
	importText(t, s, file, content)

	if _, err := s.DB().Exec(`UPDATE chats SET owner = 'Ana' WHERE name = 'Ana'`); err != nil {
		t.Fatalf("definir dono manualmente: %v", err)
	}
	importText(t, s, file, content)

	if owner := chatOwner(t, s, "Ana"); owner != "Ana" {
		t.Errorf("dono = %q, esperado Ana — a reimportação sobrescreveu a escolha do usuário", owner)
	}
}
