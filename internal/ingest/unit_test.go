package ingest

import (
	"os"
	"path/filepath"
	"testing"
)

// With the default "move" policy the file is archived under <imported>/YYYY-MM/, and the
// "Reimport…" panel has to find it there — otherwise it opens empty.
func TestListImportedUnits(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{
		"2026-07/Conversa do WhatsApp com Ana.txt",
		"2026-08/Conversa do WhatsApp com Bruno.zip",
		"2026-08/.oculto.txt",
		"2026-08/leiame.md",
	} {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	units, err := ListImportedUnits(root)
	if err != nil {
		t.Fatal(err)
	}
	var nomes []string
	for _, u := range units {
		nomes = append(nomes, u.Name)
	}
	// Newest month first; dotfiles and unknown extensions stay out.
	quer := []string{"2026-08/Conversa do WhatsApp com Bruno.zip", "2026-07/Conversa do WhatsApp com Ana.txt"}
	if len(nomes) != len(quer) {
		t.Fatalf("unidades = %v, queria %v", nomes, quer)
	}
	for i := range quer {
		if nomes[i] != quer[i] {
			t.Errorf("unidade %d = %q, queria %q", i, nomes[i], quer[i])
		}
	}

	if _, err := ListImportedUnits(filepath.Join(root, "nao-existe")); err != nil {
		t.Errorf("pasta inexistente deveria devolver lista vazia, veio erro: %v", err)
	}
}

// The name arrives from a form: it must never resolve outside the imported folder.
func TestResolveImported(t *testing.T) {
	root := t.TempDir()
	dentro := filepath.Join(root, "2026-08", "Conversa do WhatsApp com Ana.zip")
	if err := os.MkdirAll(filepath.Dir(dentro), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dentro, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	fora := filepath.Join(t.TempDir(), "segredo.txt")
	if err := os.WriteFile(fora, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got, err := ResolveImported(root, "2026-08/Conversa do WhatsApp com Ana.zip"); err != nil || got != dentro {
		t.Errorf("caminho válido: got=%q err=%v", got, err)
	}
	for _, nome := range []string{
		"../" + filepath.Base(fora),
		"2026-08/../../" + filepath.Base(fora),
		"/etc/passwd",
		"2026-08/nao-existe.zip",
	} {
		if _, err := ResolveImported(root, nome); err == nil {
			t.Errorf("%q deveria ser recusado", nome)
		}
	}
}
