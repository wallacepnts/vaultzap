package ingest

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/wallacepnts/vaultzap/internal/store"
)

func TestImportFile_zipAttachmentResolvesAndDedupes(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "vaultzap.db"))
	if err != nil {
		t.Fatalf("abrir banco: %v", err)
	}
	defer s.Close()

	zipDir := t.TempDir()
	mediaDir := t.TempDir()

	zipPath := filepath.Join(zipDir, "Conversa do WhatsApp com FotoTeste.zip")
	createTestZip(t, zipPath, map[string]string{
		"_chat.txt": "26/07/2026 14:32 - Vitor: olha essa foto\n" +
			"26/07/2026 14:33 - Vitor: IMG-20260726-WA0001.jpg (arquivo anexado)\n" +
			"26/07/2026 14:34 - Ana: bonita!\n",
		"IMG-20260726-WA0001.jpg": "bytes-fake-de-imagem",
	})

	report, err := ImportFile(ctx, s, zipPath, zipPath, mediaDir, "")
	if err != nil {
		t.Fatalf("ImportFile: %v", err)
	}
	if report.Added != 3 {
		t.Fatalf("adicionadas = %d, esperado 3", report.Added)
	}

	var kind string
	var attachmentID sql.NullInt64
	err = s.DB().QueryRowContext(ctx,
		`SELECT kind, attachment_id FROM messages WHERE body LIKE 'IMG-%'`).Scan(&kind, &attachmentID)
	if err != nil {
		t.Fatalf("consultar mensagem com anexo: %v", err)
	}
	if kind != "media" {
		t.Errorf("kind = %q, esperado media (anexo deveria ter sido resolvido)", kind)
	}
	if !attachmentID.Valid {
		t.Fatal("attachment_id deveria estar preenchido")
	}

	var attachmentCount int
	if err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM attachments`).Scan(&attachmentCount); err != nil {
		t.Fatalf("contar attachments: %v", err)
	}
	if attachmentCount != 1 {
		t.Errorf("total de attachments = %d, esperado 1", attachmentCount)
	}

	var storedPath string
	if err := s.DB().QueryRowContext(ctx, `SELECT stored_path FROM attachments`).Scan(&storedPath); err != nil {
		t.Fatalf("ler stored_path: %v", err)
	}
	copiedContent, err := os.ReadFile(filepath.Join(mediaDir, storedPath))
	if err != nil {
		t.Fatalf("ler arquivo copiado: %v", err)
	}
	if string(copiedContent) != "bytes-fake-de-imagem" {
		t.Errorf("conteúdo copiado difere do original: %q", copiedContent)
	}
}

func TestImportFile_missingAttachmentBecomesOmitted(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "vaultzap.db"))
	if err != nil {
		t.Fatalf("abrir banco: %v", err)
	}
	defer s.Close()

	txtDir := t.TempDir()
	mediaDir := t.TempDir()
	txtPath := filepath.Join(txtDir, "Conversa do WhatsApp com SemMidia.txt")
	content := "26/07/2026 14:32 - Vitor: IMG-20260726-WA0009.jpg (arquivo anexado)\n"
	if err := os.WriteFile(txtPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := ImportFile(ctx, s, txtPath, txtPath, mediaDir, "")
	if err != nil {
		t.Fatalf("ImportFile não deveria falhar com anexo ausente: %v", err)
	}
	if report.Added != 1 {
		t.Fatalf("adicionadas = %d, esperado 1", report.Added)
	}

	var kind string
	if err := s.DB().QueryRowContext(ctx, `SELECT kind FROM messages`).Scan(&kind); err != nil {
		t.Fatalf("consultar mensagem: %v", err)
	}
	if kind != "media_omitted" {
		t.Errorf("kind = %q, esperado media_omitted", kind)
	}
}

// The attachment name is untrusted input: it comes verbatim from a .txt the user only
// dropped in the inbox.
func TestImportFile_attachmentPathTraversalStaysInFolder(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "vaultzap.db"))
	if err != nil {
		t.Fatalf("abrir banco: %v", err)
	}
	defer s.Close()

	inbox := t.TempDir()
	mediaDir := t.TempDir()

	secret := filepath.Join(inbox, "segredo.txt")
	if err := os.WriteFile(secret, []byte("conteudo que nao deveria vazar"), 0o644); err != nil {
		t.Fatal(err)
	}

	folder := filepath.Join(inbox, "Conversa do WhatsApp com Ana")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "26/07/2026 14:32 - Vitor: <anexado: ../segredo.txt>\n" +
		"26/07/2026 14:33 - Ana: e ai?\n"
	if err := os.WriteFile(filepath.Join(folder, "_chat.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := ImportFile(ctx, s, folder, folder, mediaDir, ""); err != nil {
		t.Fatalf("ImportFile não deveria falhar por causa do nome recusado: %v", err)
	}

	var attachments int
	if err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM attachments`).Scan(&attachments); err != nil {
		t.Fatal(err)
	}
	if attachments != 0 {
		t.Errorf("attachments = %d, esperado 0 (o nome deveria ter sido recusado)", attachments)
	}

	var kind string
	err = s.DB().QueryRowContext(ctx, `SELECT kind FROM messages WHERE body LIKE '%segredo%'`).Scan(&kind)
	if err != nil {
		t.Fatalf("consultar mensagem: %v", err)
	}
	if kind != "media_omitted" {
		t.Errorf("kind = %q, esperado media_omitted", kind)
	}

	copied := 0
	filepath.Walk(mediaDir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			copied++
		}
		return nil
	})
	if copied != 0 {
		t.Errorf("%d arquivos copiados pro MEDIA_DIR, esperado 0", copied)
	}
}

func TestSafeAttachmentName(t *testing.T) {
	cases := map[string]bool{
		"IMG-20260726-WA0001.jpg":  true,
		"00000042-PHOTO-2026.jpeg": true,
		"arquivo com espaço.pdf":   true,
		"../../data/vaultzap.db":   false,
		"../segredo.txt":           false,
		"/etc/passwd":              false,
		"sub/dir/foto.jpg":         false,
		"..":                       false,
		".":                        false,
		"":                         false,
	}
	for name, want := range cases {
		if got := safeAttachmentName(name); got != want {
			t.Errorf("safeAttachmentName(%q) = %v, esperado %v", name, got, want)
		}
	}
}

func TestResolveUnit_zipAndFolder(t *testing.T) {
	zipDir := t.TempDir()
	zipPath := filepath.Join(zipDir, "teste.zip")
	createTestZip(t, zipPath, map[string]string{
		"_chat.txt":  "conteudo",
		"imagem.jpg": "bytes",
	})

	u, err := resolveUnit(zipPath, t.TempDir())
	if err != nil {
		t.Fatalf("resolveUnit (zip): %v", err)
	}
	defer u.cleanup()
	if filepath.Base(u.txtPath) != "_chat.txt" {
		t.Errorf("caminhoTxt = %q, esperado terminar em _chat.txt", u.txtPath)
	}

	folderDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(folderDir, "Conversa do WhatsApp com X.txt"), []byte("oi"), 0o644); err != nil {
		t.Fatal(err)
	}
	u2, err := resolveUnit(folderDir, t.TempDir())
	if err != nil {
		t.Fatalf("resolveUnit (pasta): %v", err)
	}
	defer u2.cleanup()
	if u2.mediaDir != folderDir {
		t.Errorf("dirMidia = %q, esperado %q", u2.mediaDir, folderDir)
	}
}
