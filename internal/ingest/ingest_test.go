package ingest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wallacepnts/vaultzap/internal/parser"
	"github.com/wallacepnts/vaultzap/internal/store"
)

func TestImportFile_incrementalReingest(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "vaultzap.db"))
	if err != nil {
		t.Fatalf("abrir banco: %v", err)
	}
	defer s.Close()

	path := filepath.Join(t.TempDir(), "Conversa do WhatsApp com Reingest.txt")
	mediaDir := t.TempDir()

	copyContent(t, "testdata/reingest-parte1.txt", path)
	r1, err := ImportFile(ctx, s, path, path, mediaDir, "")
	if err != nil {
		t.Fatalf("primeira importação: %v", err)
	}
	if r1.Added != 5 {
		t.Errorf("primeira importação: %d adicionadas, esperado 5", r1.Added)
	}
	if r1.Skipped != 0 {
		t.Errorf("primeira importação: %d ignoradas, esperado 0", r1.Skipped)
	}

	copyContent(t, "testdata/reingest-parte2.txt", path)
	r2, err := ImportFile(ctx, s, path, path, mediaDir, "")
	if err != nil {
		t.Fatalf("segunda importação: %v", err)
	}
	if r2.Added != 10 {
		t.Errorf("segunda importação: %d adicionadas, esperado 10", r2.Added)
	}
	if r2.Skipped != 5 {
		t.Errorf("segunda importação: %d ignoradas, esperado 5", r2.Skipped)
	}
	if r1.ChatID != r2.ChatID {
		t.Errorf("chats diferentes entre importações: %d != %d", r1.ChatID, r2.ChatID)
	}

	var total int
	err = s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM messages WHERE chat_id = ?`, r2.ChatID).Scan(&total)
	if err != nil {
		t.Fatalf("contar mensagens: %v", err)
	}
	if total != 15 {
		t.Errorf("total de mensagens = %d, esperado 15 (sem duplicatas)", total)
	}

	var chatCount int
	if err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM chats`).Scan(&chatCount); err != nil {
		t.Fatalf("contar chats: %v", err)
	}
	if chatCount != 1 {
		t.Errorf("contagem de chats = %d, esperado 1 (não deveria duplicar o chat)", chatCount)
	}

	r3, err := ImportFile(ctx, s, path, path, mediaDir, "")
	if err != nil {
		t.Fatalf("terceira importação: %v", err)
	}
	if !r3.AlreadyDone {
		t.Error("terceira importação deveria ser um no-op (mesmo sha256 do arquivo)")
	}
}

func TestImportFile_phoneLikeNameFillsPhone(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "vaultzap.db"))
	if err != nil {
		t.Fatalf("abrir banco: %v", err)
	}
	defer s.Close()

	path := filepath.Join(t.TempDir(), "Conversa do WhatsApp com +55 11 91234-5678.txt")
	mediaDir := t.TempDir()

	copyContent(t, "testdata/reingest-parte1.txt", path)
	r1, err := ImportFile(ctx, s, path, path, mediaDir, "")
	if err != nil {
		t.Fatalf("importação: %v", err)
	}

	var name, phone string
	err = s.DB().QueryRowContext(ctx, `SELECT name, COALESCE(phone, '') FROM chats WHERE id = ?`, r1.ChatID).Scan(&name, &phone)
	if err != nil {
		t.Fatalf("consultar chat: %v", err)
	}
	if name != "+55 11 91234-5678" {
		t.Errorf("name = %q, esperado o telefone como nome (sem contato salvo)", name)
	}
	if phone != "+55 11 91234-5678" {
		t.Errorf("phone = %q, esperado preenchido com o mesmo telefone", phone)
	}

	if err := s.SetPhone(ctx, r1.ChatID, "(11) 90000-0000"); err != nil {
		t.Fatalf("editar telefone: %v", err)
	}

	copyContent(t, "testdata/reingest-parte2.txt", path)
	if _, err := ImportFile(ctx, s, path, path, mediaDir, ""); err != nil {
		t.Fatalf("reimportação: %v", err)
	}
	if err := s.DB().QueryRowContext(ctx, `SELECT COALESCE(phone, '') FROM chats WHERE id = ?`, r1.ChatID).Scan(&phone); err != nil {
		t.Fatalf("consultar chat após reimportação: %v", err)
	}
	if phone != "(11) 90000-0000" {
		t.Errorf("phone = %q após reimportação, esperado preservar a edição do usuário", phone)
	}
}

func TestImportFile_invalidZipShowsInImports(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "vaultzap.db"))
	if err != nil {
		t.Fatalf("abrir banco: %v", err)
	}
	defer s.Close()

	path := filepath.Join(t.TempDir(), "Ruim.zip")
	if err := os.WriteFile(path, []byte("isso não é um zip"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := ImportFile(ctx, s, path, path, t.TempDir(), ""); err == nil {
		t.Fatal("esperava erro ao importar um zip inválido")
	}

	var status, recordedPath string
	err = s.DB().QueryRowContext(ctx, `SELECT status, path FROM imports ORDER BY id DESC LIMIT 1`).
		Scan(&status, &recordedPath)
	if err != nil {
		t.Fatalf("import não foi registrado: %v", err)
	}
	if status != "error" {
		t.Errorf("status = %q, esperado \"error\"", status)
	}
	if recordedPath != path {
		t.Errorf("path registrado = %q, esperado %q", recordedPath, path)
	}
}

// The guard must not treat a failed import as already done: the scanner would mark the
// file done and move it with nothing in the database.
func TestImportFile_failedFileIsNotAlreadyDone(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "vaultzap.db"))
	if err != nil {
		t.Fatalf("abrir banco: %v", err)
	}
	defer s.Close()

	path := filepath.Join(t.TempDir(), "Conversa do WhatsApp com Ana.txt")
	if err := os.WriteFile(path, []byte("nada aqui parece um export\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mediaDir := t.TempDir()

	if _, err := ImportFile(ctx, s, path, path, mediaDir, ""); err == nil {
		t.Fatal("primeira tentativa: esperava erro de parser")
	}

	report, err := ImportFile(ctx, s, path, path, mediaDir, "")
	if err == nil {
		t.Fatal("segunda tentativa: esperava o mesmo erro, não um import silenciosamente aceito")
	}
	if report.AlreadyDone {
		t.Error("segunda tentativa: AlreadyDone = true para um arquivo que nunca foi importado")
	}

	var lines int
	if err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM imports`).Scan(&lines); err != nil {
		t.Fatal(err)
	}
	if lines != 1 {
		t.Errorf("linhas em imports = %d, esperado 1 (a segunda falha atualiza a existente)", lines)
	}
}

func copyContent(t *testing.T, source, dest string) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestImportIntoChat_forcedTarget(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "vaultzap.db"))
	if err != nil {
		t.Fatalf("abrir banco: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	dir, mediaDir := t.TempDir(), t.TempDir()
	original := filepath.Join(dir, "Conversa do WhatsApp com Ana.txt")
	if err := os.WriteFile(original, []byte("26/07/2026 09:00 - Ana: oi\n26/07/2026 09:01 - Eu: oi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportFile(ctx, s, original, "Conversa do WhatsApp com Ana.txt", mediaDir, ""); err != nil {
		t.Fatalf("import inicial: %v", err)
	}

	var chatID int64
	if err := s.DB().QueryRow(`SELECT id FROM chats WHERE name = 'Ana'`).Scan(&chatID); err != nil {
		t.Fatal(err)
	}

	new := filepath.Join(dir, "export-qualquer.txt")
	if err := os.WriteFile(new, []byte(
		"26/07/2026 09:00 - Ana: oi\n26/07/2026 09:01 - Eu: oi\n"+
			"27/07/2026 10:00 - Ana: nova 1\n27/07/2026 10:01 - Eu: nova 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := ImportIntoChat(ctx, s, new, "export-qualquer.txt", mediaDir, chatID, "")
	if err != nil {
		t.Fatalf("ImportIntoChat: %v", err)
	}
	if report.Added != 2 || report.Skipped != 2 {
		t.Errorf("relatório = %d novas / %d ignoradas, esperado 2/2", report.Added, report.Skipped)
	}

	var chats int
	s.DB().QueryRow(`SELECT COUNT(*) FROM chats`).Scan(&chats)
	if chats != 1 {
		t.Errorf("%d chats no banco — o destino forçado deveria evitar criar um segundo", chats)
	}
	var total int
	s.DB().QueryRow(`SELECT COUNT(*) FROM messages WHERE chat_id = ?`, chatID).Scan(&total)
	if total != 4 {
		t.Errorf("%d mensagens no chat, esperado 4 (2 antigas + 2 novas)", total)
	}
}

func TestRecordImport_keepsWarningText(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "vaultzap.db"))
	if err != nil {
		t.Fatalf("abrir banco: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	dir := t.TempDir()
	path := filepath.Join(dir, "Conversa do WhatsApp com Ana.txt")
	content := "26/07/2026 09:00 - Ana: oi\n" +
		"99/99/2026 09:01 - Ana: data impossível\n" +
		"26/07/2026 09:02 - Eu: fim\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := ImportFile(ctx, s, path, "Conversa do WhatsApp com Ana.txt", t.TempDir(), "")
	if err != nil {
		t.Fatalf("ImportFile: %v", err)
	}
	if report.Warnings == 0 {
		t.Fatal("cenário inválido: o parser não gerou nenhum aviso")
	}

	imports, err := s.ListImports(ctx)
	if err != nil {
		t.Fatalf("ListarImports: %v", err)
	}
	if len(imports) != 1 {
		t.Fatalf("%d imports, esperado 1", len(imports))
	}
	if imports[0].Warnings != report.Warnings {
		t.Errorf("contagem de avisos = %d, esperado %d", imports[0].Warnings, report.Warnings)
	}
	if len(imports[0].WarningText) != report.Warnings {
		t.Fatalf("texto dos avisos = %d linhas, esperado %d", len(imports[0].WarningText), report.Warnings)
	}
	if !strings.Contains(imports[0].WarningText[0], "line 2") {
		t.Errorf("aviso = %q, esperado apontar a linha problemática", imports[0].WarningText[0])
	}
}

// VAULTZAP_DATE_ORDER has to reach the parser: config validates it at boot, so dropping
// it on the way turns a documented setting into a no-op.
func TestImportFile_configuredDateOrder(t *testing.T) {
	const content = "05/06/2026 10:00 - Ana: dia cinco ou seis de junho?\n"

	cases := []struct {
		order parser.DateOrder
		want  string
	}{
		{"", "2026-06-05 10:00:00"},              // the parser's own default (DMY)
		{parser.OrderDMY, "2026-06-05 10:00:00"}, // 5 June
		{parser.OrderMDY, "2026-05-06 10:00:00"}, // 6 May
	}

	for _, c := range cases {
		ctx := context.Background()
		s, err := store.Open(ctx, filepath.Join(t.TempDir(), "vaultzap.db"))
		if err != nil {
			t.Fatalf("abrir banco: %v", err)
		}

		path := filepath.Join(t.TempDir(), "Conversa do WhatsApp com Ambigua.txt")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		report, err := ImportFile(ctx, s, path, path, t.TempDir(), c.order)
		if err != nil {
			t.Fatalf("ordem %q: %v", c.order, err)
		}

		var sentAt string
		err = s.DB().QueryRowContext(ctx, `SELECT sent_at FROM messages WHERE chat_id = ?`, report.ChatID).Scan(&sentAt)
		if err != nil {
			t.Fatalf("ordem %q: consultar mensagem: %v", c.order, err)
		}
		if sentAt != c.want {
			t.Errorf("ordem %q: sent_at = %q, esperado %q", c.order, sentAt, c.want)
		}
		s.Close()
	}
}

// An empty sent_at sorts below every real timestamp, so one corrupt line must not become
// the chat's start date and wipe the calendar's lower bound.
func TestImportFile_corruptLineDoesNotClearChatStart(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "vaultzap.db"))
	if err != nil {
		t.Fatalf("abrir banco: %v", err)
	}
	defer s.Close()

	content := "04/05/2026 09:00 - Ana: primeira\n" +
		"45/07/2026 14:33 - Ana: data impossivel\n" +
		"26/07/2026 14:30 - Ana: ultima\n"
	path := filepath.Join(t.TempDir(), "Conversa do WhatsApp com Ana.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := ImportFile(ctx, s, path, path, t.TempDir(), "")
	if err != nil {
		t.Fatalf("ImportFile: %v", err)
	}

	var first, last string
	var total int
	err = s.DB().QueryRowContext(ctx,
		`SELECT COALESCE(first_message_at,''), COALESCE(last_message_at,''), message_count FROM chats WHERE id = ?`,
		report.ChatID).Scan(&first, &last, &total)
	if err != nil {
		t.Fatal(err)
	}
	if first != "2026-05-04 09:00:00" {
		t.Errorf("first_message_at = %q, esperado a primeira mensagem com data de verdade", first)
	}
	if last != "2026-07-26 14:30:00" {
		t.Errorf("last_message_at = %q, esperado %q", last, "2026-07-26 14:30:00")
	}
	if total != 3 {
		t.Errorf("message_count = %d, esperado 3 (a linha corrompida não é descartada)", total)
	}
}
