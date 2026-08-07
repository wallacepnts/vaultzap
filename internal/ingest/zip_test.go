package ingest

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createTestZip(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for name, content := range entries {
		escritor, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := escritor.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExtractZipSafely_extractsNormally(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "normal.zip")
	createTestZip(t, zipPath, map[string]string{
		"_chat.txt":                  "26/07/2026 14:32 - Vitor: oi",
		"IMG-20260726-WA0001.jpg":    "conteudo-fake-da-imagem",
		"subpasta/outro-arquivo.txt": "não é o chat",
	})

	dest, err := extractZipSafely(zipPath, t.TempDir())
	if err != nil {
		t.Fatalf("extractZipSafely: %v", err)
	}
	defer os.RemoveAll(dest)

	content, err := os.ReadFile(filepath.Join(dest, "_chat.txt"))
	if err != nil {
		t.Fatalf("ler _chat.txt extraído: %v", err)
	}
	if string(content) != "26/07/2026 14:32 - Vitor: oi" {
		t.Errorf("conteúdo extraído inesperado: %q", content)
	}
}

func TestExtractZipSafely_rejectsPathTraversal(t *testing.T) {
	cases := []string{
		"../fora-do-destino.txt",
		"../../etc/passwd",
		"subpasta/../../fora.txt",
	}
	for _, maliciousName := range cases {
		t.Run(maliciousName, func(t *testing.T) {
			zipPath := filepath.Join(t.TempDir(), "malicioso.zip")
			createTestZip(t, zipPath, map[string]string{
				"_chat.txt":   "conteudo normal",
				maliciousName: "tentando escapar",
			})

			dest, err := extractZipSafely(zipPath, t.TempDir())
			if err == nil {
				os.RemoveAll(dest)
				t.Fatalf("esperava erro para entrada %q, extraiu sem problema", maliciousName)
			}
			if dest != "" {
				t.Errorf("diretório temporário não deveria ter sobrado após falha: %q", dest)
			}
		})
	}
}

func TestExtractZipSafely_rejectsAbsolutePath(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "absoluto.zip")
	createTestZip(t, zipPath, map[string]string{
		"/etc/passwd": "tentando escrever em caminho absoluto",
	})

	if _, err := extractZipSafely(zipPath, t.TempDir()); err == nil {
		t.Error("esperava erro para entrada com caminho absoluto")
	}
}

func TestExtractZipSafely_rejectsTooManyEntries(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "muitas-entradas.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for i := 0; i < maxZipEntries+1; i++ {
		if _, err := w.Create(fmt.Sprintf("arquivo-%d.txt", i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if _, err := extractZipSafely(zipPath, t.TempDir()); err == nil {
		t.Error("esperava erro para zip com excesso de entradas")
	}
}

func TestSafePath(t *testing.T) {
	base := "/tmp/destino-base"
	cases := []struct {
		name     string
		input    string
		querErro bool
	}{
		{"normal", "arquivo.txt", false},
		{"subpasta normal", "sub/arquivo.txt", false},
		{"traversal simples", "../fora.txt", true},
		{"traversal apos subpasta", "sub/../../fora.txt", true},
		{"absoluto unix", "/etc/passwd", true},
		{"so pontos", "..", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := safePath(base, c.input)
			if c.querErro && err == nil {
				t.Errorf("safePath(%q) deveria falhar", c.input)
			}
			if !c.querErro && err != nil {
				t.Errorf("safePath(%q) falhou inesperadamente: %v", c.input, err)
			}
		})
	}
}

// A real export is media, which is already compressed: it comes out near 1:1 and used to
// be refused by absolute caps. What has to be refused is the expansion ratio.
func TestExtractZipSafely_zipBomb(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "bomba.zip")
	createTestZip(t, zipPath, map[string]string{
		"grande.bin": string(make([]byte, 32<<20)), // 32 MiB de zeros -> alguns KB no zip
	})

	dest, err := extractZipSafely(zipPath, "")
	if dest != "" {
		os.RemoveAll(dest)
	}
	if err == nil {
		t.Fatal("esperava recusa do zip bomb")
	}
	if !strings.Contains(err.Error(), "zip bomb") {
		t.Errorf("erro deveria mencionar zip bomb, veio: %v", err)
	}
}

func TestExpandsTooMuch(t *testing.T) {
	casos := []struct {
		nome                     string
		uncompressed, compressed uint64
		quer                     bool
	}{
		{"export real: mídia não comprime", 7 << 30, 6900 << 20, false},
		{"txt de chat comprime ~10x", 80 << 20, 8 << 20, false},
		{"bomba: 1000x", 32 << 20, 32 << 10, true},
		{"arquivo pequeno e vazio não conta", 1 << 10, 8, false},
		{"limite exato não dispara", ratioFloor, ratioFloor / maxExpansionRatio, false},
	}
	for _, c := range casos {
		if got := expandsTooMuch(c.uncompressed, c.compressed); got != c.quer {
			t.Errorf("%s: expandsTooMuch(%d, %d) = %v, queria %v",
				c.nome, c.uncompressed, c.compressed, got, c.quer)
		}
	}
}

// The stored hash must not change with the streaming rewrite: imports.sha256 and
// seen_files.sha256 hold values computed by the old in-memory version, and a different
// result would reimport every file already in the archive.
func TestHashFileHex_matchesInMemory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "arquivo.bin")
	conteudo := []byte("qualquer conteúdo, com acento e bytes\x00\x01\x02")
	if err := os.WriteFile(path, conteudo, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := hashFileHex(path)
	if err != nil {
		t.Fatal(err)
	}
	if quer := sha256Hex(conteudo); got != quer {
		t.Errorf("hashFileHex = %s, queria %s", got, quer)
	}
}
