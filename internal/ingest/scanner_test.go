package ingest

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/wallacepnts/vaultzap/internal/config"
	"github.com/wallacepnts/vaultzap/internal/store"
)

func testScanner(store *store.Store, cfg config.Config) *Scanner {
	sc := NewScanner(store, cfg)
	sc.passGap = 0
	return sc
}

func openTestScanner(t *testing.T, policy config.PostImportPolicy) (*Scanner, *store.Store, string) {
	t.Helper()
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "vaultzap.db"))
	if err != nil {
		t.Fatalf("abrir banco: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	inbox := t.TempDir()
	mediaDir := t.TempDir()
	sc := testScanner(s, config.Config{
		Inbox:            inbox,
		MediaDir:         mediaDir,
		PostImportPolicy: policy,
	})
	return sc, s, inbox
}

func writeToInbox(t *testing.T, inbox, name, source string, mtimeAge time.Duration) string {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(inbox, name)
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if mtimeAge > 0 {
		old := time.Now().Add(-mtimeAge)
		if err := os.Chtimes(dest, old, old); err != nil {
			t.Fatal(err)
		}
	}
	return dest
}

func countImports(t *testing.T, s *store.Store) int {
	t.Helper()
	var n int
	if err := s.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM imports`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestScanner_partialFileNeverImports(t *testing.T) {
	sc, s, inbox := openTestScanner(t, config.PolicyKeep)
	writeToInbox(t, inbox, "Conversa.txt.part", "testdata/reingest-parte1.txt", 1*time.Minute)

	sc.Scan(context.Background())
	sc.Scan(context.Background())

	if n := countImports(t, s); n != 0 {
		t.Errorf("imports = %d, esperado 0 (.part nunca deveria ser processado)", n)
	}
}

func TestScanner_needsTwoPassesAndAge(t *testing.T) {
	sc, s, inbox := openTestScanner(t, config.PolicyKeep)
	writeToInbox(t, inbox, "Conversa do WhatsApp com Teste.txt", "testdata/reingest-parte1.txt", 1*time.Minute)
	ctx := context.Background()

	sc.scanOnce(ctx)
	if n := countImports(t, s); n != 0 {
		t.Fatalf("imports após 1ª passada = %d, esperado 0 (ainda pending)", n)
	}
	file, ok, err := s.GetSeenFile(ctx, "Conversa do WhatsApp com Teste.txt")
	if err != nil || !ok {
		t.Fatalf("seen_files não registrou o arquivo: ok=%v err=%v", ok, err)
	}
	if file.State != store.StatePending {
		t.Errorf("estado após 1ª passada = %q, esperado %q", file.State, store.StatePending)
	}

	sc.scanOnce(ctx)
	if n := countImports(t, s); n != 1 {
		t.Fatalf("imports após 2ª passada = %d, esperado 1", n)
	}
	file, _, err = s.GetSeenFile(ctx, "Conversa do WhatsApp com Teste.txt")
	if err != nil {
		t.Fatal(err)
	}
	if file.State != store.StateDone {
		t.Errorf("estado após import = %q, esperado %q", file.State, store.StateDone)
	}
}

func TestScanner_oneScanIsEnoughToImport(t *testing.T) {
	sc, s, inbox := openTestScanner(t, config.PolicyKeep)
	writeToInbox(t, inbox, "Conversa do WhatsApp com Nova.txt", "testdata/reingest-parte1.txt", 1*time.Minute)

	sc.Scan(context.Background())

	if n := countImports(t, s); n != 1 {
		t.Errorf("imports após UMA varredura = %d, esperado 1", n)
	}
}

func TestScanner_incrementalReingest(t *testing.T) {
	sc, s, inbox := openTestScanner(t, config.PolicyKeep)
	ctx := context.Background()
	name := "Conversa do WhatsApp com Reingest.txt"

	writeToInbox(t, inbox, name, "testdata/reingest-parte1.txt", 1*time.Minute)
	sc.Scan(ctx)
	sc.Scan(ctx)

	var chatID int64
	err := s.DB().QueryRowContext(ctx, `SELECT chat_id FROM imports ORDER BY id DESC LIMIT 1`).Scan(&chatID)
	if err != nil {
		t.Fatalf("primeira importação não aconteceu: %v", err)
	}
	var total int
	s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM messages WHERE chat_id = ?`, chatID).Scan(&total)
	if total != 5 {
		t.Fatalf("mensagens após parte1 = %d, esperado 5", total)
	}

	writeToInbox(t, inbox, name, "testdata/reingest-parte2.txt", 1*time.Minute)
	sc.Scan(ctx)
	sc.Scan(ctx)

	s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM messages WHERE chat_id = ?`, chatID).Scan(&total)
	if total != 15 {
		t.Errorf("mensagens após parte2 = %d, esperado 15 (10 novas, sem duplicar as 5 antigas)", total)
	}
	if n := countImports(t, s); n != 2 {
		t.Errorf("imports = %d, esperado 2 (uma linha por sha256 de conteúdo diferente)", n)
	}
}

func TestScanner_movePolicyMovesToImported(t *testing.T) {
	sc, s, inbox := openTestScanner(t, config.PolicyMove)
	ctx := context.Background()
	name := "Conversa do WhatsApp com Mover.txt"
	path := writeToInbox(t, inbox, name, "testdata/reingest-parte1.txt", 1*time.Minute)

	sc.Scan(ctx)
	sc.Scan(ctx)

	if n := countImports(t, s); n != 1 {
		t.Fatalf("imports = %d, esperado 1", n)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("arquivo ainda está na inbox, esperado que tivesse sido movido")
	}

	dest := filepath.Join(inbox, ".imported", time.Now().Format("2006-01"), name)
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("arquivo não está em .imported/AAAA-MM: %v", err)
	}
}

func TestScanner_deletePolicySkipsFilesWithWarnings(t *testing.T) {
	sc, s, inbox := openTestScanner(t, config.PolicyDelete)
	ctx := context.Background()
	name := "Conversa do WhatsApp com Avisos.txt"
	path := writeToInbox(t, inbox, name, "../parser/testdata/linha-corrompida.txt", 1*time.Minute)

	sc.Scan(ctx)
	sc.Scan(ctx)

	var warnings int
	err := s.DB().QueryRowContext(ctx, `SELECT warnings FROM imports ORDER BY id DESC LIMIT 1`).Scan(&warnings)
	if err != nil {
		t.Fatalf("import não aconteceu: %v", err)
	}
	if warnings == 0 {
		t.Fatal("fixture deveria ter gerado ao menos um aviso do parser")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("arquivo com avisos foi apagado da inbox, não deveria: %v", err)
	}
}

func TestScanner_malformedFileMarksErrorAndKeepsGoing(t *testing.T) {
	sc, s, inbox := openTestScanner(t, config.PolicyKeep)
	ctx := context.Background()

	ruim := filepath.Join(inbox, "Conversa do WhatsApp com Ruim.zip")
	if err := os.WriteFile(ruim, []byte("isso não é um zip de verdade"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-1 * time.Minute)
	os.Chtimes(ruim, old, old)

	writeToInbox(t, inbox, "Conversa do WhatsApp com Bom.txt", "testdata/reingest-parte1.txt", 1*time.Minute)

	sc.Scan(ctx)
	sc.Scan(ctx)

	badSeen, ok, err := s.GetSeenFile(ctx, "Conversa do WhatsApp com Ruim.zip")
	if err != nil || !ok {
		t.Fatalf("seen_files não registrou o arquivo ruim: ok=%v err=%v", ok, err)
	}
	if badSeen.State != store.StateError {
		t.Errorf("estado do zip inválido = %q, esperado %q", badSeen.State, store.StateError)
	}

	goodSeen, ok, err := s.GetSeenFile(ctx, "Conversa do WhatsApp com Bom.txt")
	if err != nil || !ok {
		t.Fatalf("seen_files não registrou o arquivo bom: ok=%v err=%v", ok, err)
	}
	if goodSeen.State != store.StateDone {
		t.Errorf("estado do arquivo bom = %q, esperado %q (a varredura não deveria ter parado no ruim)", goodSeen.State, store.StateDone)
	}
}

func TestScanner_importedDirInsideInboxDoesNotFeedBack(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "vaultzap.db"))
	if err != nil {
		t.Fatalf("abrir banco: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	inbox := t.TempDir()
	importados := filepath.Join(inbox, "importados") // no leading dot, so the dotfile filter lets it through
	sc := testScanner(s, config.Config{
		Inbox:            inbox,
		MediaDir:         t.TempDir(),
		PostImportPolicy: config.PolicyMove,
		ImportedDir:      importados,
	})

	name := "Conversa do WhatsApp com Loop.txt"
	writeToInbox(t, inbox, name, "testdata/reingest-parte1.txt", 1*time.Minute)

	sc.Scan(ctx)
	sc.Scan(ctx)
	if n := countImports(t, s); n != 1 {
		t.Fatalf("imports após o primeiro move = %d, esperado 1", n)
	}
	if _, err := os.Stat(filepath.Join(importados, time.Now().Format("2006-01"), name)); err != nil {
		t.Fatalf("arquivo não foi movido para a pasta configurada: %v", err)
	}

	sc.Scan(ctx)
	sc.Scan(ctx)
	sc.Scan(ctx)
	if n := countImports(t, s); n != 1 {
		t.Errorf("imports = %d após varreduras extras, esperado 1 — a pasta de importados está realimentando a inbox", n)
	}
}

func TestScanner_movesAlreadyImportedFileLeftInInbox(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "vaultzap.db"))
	if err != nil {
		t.Fatalf("abrir banco: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	inbox, mediaDir := t.TempDir(), t.TempDir()
	base := config.Config{Inbox: inbox, MediaDir: mediaDir}

	name := "Conversa do WhatsApp com Antiga.txt"
	path := writeToInbox(t, inbox, name, "testdata/reingest-parte1.txt", 1*time.Minute)

	comKeep := base
	comKeep.PostImportPolicy = config.PolicyKeep
	scKeep := testScanner(s, comKeep)
	scKeep.Scan(ctx)
	scKeep.Scan(ctx)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("com keep o arquivo deveria continuar na inbox: %v", err)
	}

	comMove := base
	comMove.PostImportPolicy = config.PolicyMove
	testScanner(s, comMove).Scan(ctx)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("arquivo já importado continuou na inbox depois de ligar move")
	}
	dest := filepath.Join(inbox, ".imported", time.Now().Format("2006-01"), name)
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("arquivo não foi para a pasta de importados: %v", err)
	}
	if n := countImports(t, s); n != 1 {
		t.Errorf("imports = %d, esperado 1 — mover não pode reimportar", n)
	}
}

func TestScanner_deleteIsNotRetroactive(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "vaultzap.db"))
	if err != nil {
		t.Fatalf("abrir banco: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	inbox, mediaDir := t.TempDir(), t.TempDir()
	base := config.Config{Inbox: inbox, MediaDir: mediaDir}
	name := "Conversa do WhatsApp com Avisos.txt"
	path := writeToInbox(t, inbox, name, "../parser/testdata/linha-corrompida.txt", 1*time.Minute)

	comKeep := base
	comKeep.PostImportPolicy = config.PolicyKeep
	scKeep := testScanner(s, comKeep)
	scKeep.Scan(ctx)
	scKeep.Scan(ctx)

	comDelete := base
	comDelete.PostImportPolicy = config.PolicyDelete
	testScanner(s, comDelete).Scan(ctx)

	if _, err := os.Stat(path); err != nil {
		t.Errorf("delete apagou retroativamente um arquivo já importado: %v", err)
	}
}

// WhatsApp always reuses the same export file name, so "move" lands two identical names
// in the same folder. Renaming straight over would silently erase the archived export.
func TestScanner_moveDoesNotOverwriteArchivedExport(t *testing.T) {
	sc, _, inbox := openTestScanner(t, config.PolicyMove)
	ctx := context.Background()
	name := "Conversa do WhatsApp com Mover.txt"

	writeToInbox(t, inbox, name, "testdata/reingest-parte1.txt", 1*time.Minute)
	sc.Scan(ctx)

	archivedFiles := filepath.Join(inbox, ".imported", time.Now().Format("2006-01"))
	first, err := os.ReadFile(filepath.Join(archivedFiles, name))
	if err != nil {
		t.Fatalf("primeiro export não foi arquivado: %v", err)
	}

	writeToInbox(t, inbox, name, "testdata/reingest-parte2.txt", 1*time.Minute)
	sc.Scan(ctx)

	depois, err := os.ReadFile(filepath.Join(archivedFiles, name))
	if err != nil {
		t.Fatalf("primeiro export sumiu de .imported: %v", err)
	}
	if string(depois) != string(first) {
		t.Error("o segundo export sobrescreveu o primeiro em .imported")
	}

	entries, err := os.ReadDir(archivedFiles)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("arquivos em .imported = %v, esperado os dois exports", names)
	}
}

// The startup scan and the "scan now" button are two callers of the same Scanner.
func TestScanner_concurrentScansDoNotCollide(t *testing.T) {
	sc, s, inbox := openTestScanner(t, config.PolicyMove)
	ctx := context.Background()

	fixtures := map[string]string{
		"Conversa do WhatsApp com Um.txt":   "testdata/reingest-parte1.txt",
		"Conversa do WhatsApp com Dois.txt": "testdata/reingest-parte2.txt",
		"Conversa do WhatsApp com Tres.txt": "../parser/testdata/android-pt.txt",
	}
	names := make([]string, 0, len(fixtures))
	for name, fixture := range fixtures {
		writeToInbox(t, inbox, name, fixture, 1*time.Minute)
		names = append(names, name)
	}

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sc.Scan(ctx)
		}()
	}
	wg.Wait()

	if n := countImports(t, s); n != len(names) {
		t.Errorf("imports = %d, esperado %d", n, len(names))
	}

	for _, name := range names {
		seen, existed, err := s.GetSeenFile(ctx, name)
		if err != nil || !existed {
			t.Fatalf("seen_files não tem %q: %v", name, err)
		}
		if seen.State != store.StateDone {
			t.Errorf("%q: state = %q, esperado done", name, seen.State)
		}
	}

	archivedFiles := filepath.Join(inbox, ".imported", time.Now().Format("2006-01"))
	entries, err := os.ReadDir(archivedFiles)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(names) {
		found := make([]string, 0, len(entries))
		for _, e := range entries {
			found = append(found, e.Name())
		}
		t.Errorf("arquivos em .imported = %v, esperado exatamente %v", found, names)
	}
}

// A directory's mtime only changes when an entry is added or removed, so a folder whose
// last file is still being copied looks perfectly still.
func TestScanner_folderWithFileBeingWrittenIsNotStable(t *testing.T) {
	sc, s, inbox := openTestScanner(t, config.PolicyKeep)
	ctx := context.Background()

	folder := filepath.Join(inbox, "Conversa do WhatsApp com Pasta")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("testdata/reingest-parte1.txt")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "_chat.txt"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	media := filepath.Join(folder, "IMG-20260726-WA0001.jpg")
	if err := os.WriteFile(media, []byte("primeiro pedaco"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-1 * time.Minute)
	if err := os.Chtimes(folder, old, old); err != nil {
		t.Fatal(err)
	}

	sc.Scan(ctx)
	if n := countImports(t, s); n != 0 {
		t.Fatalf("imports = %d, esperado 0: a pasta tem arquivo sendo escrito", n)
	}

	if err := os.WriteFile(media, []byte("primeiro pedaco + o resto do arquivo"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{folder, media, filepath.Join(folder, "_chat.txt")} {
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}

	sc.Scan(ctx)
	if n := countImports(t, s); n != 1 {
		t.Errorf("imports = %d, esperado 1 depois que a cópia terminou", n)
	}
}
