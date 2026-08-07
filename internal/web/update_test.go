package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wallacepnts/vaultzap/internal/config"
	"github.com/wallacepnts/vaultzap/internal/ingest"
	"github.com/wallacepnts/vaultzap/internal/store"
)

// slowScanner blocks inside the import, the way a multi-gigabyte export does.
type slowScanner struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (s *slowScanner) Scan(context.Context) {}

func (s *slowScanner) ImportInto(context.Context, string, string, int64) (ingest.Report, error) {
	s.once.Do(func() { close(s.started) })
	<-s.release
	return ingest.Report{Added: 1}, nil
}

// The import used to run inside the handler: the browser waited minutes with no feedback.
// Now the panel comes back immediately, with the bar.
func TestApplyUpdate_respondsWithoutWaitingForTheImport(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := store.Open(ctx, filepath.Join(dir, "vaultzap.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	res, err := s.DB().ExecContext(ctx,
		`INSERT INTO chats (name, is_group, source, created_at) VALUES ('Ana', 0, 'android', '2026-01-01 00:00:00')`)
	if err != nil {
		t.Fatal(err)
	}
	chatID, _ := res.LastInsertId()

	inbox := filepath.Join(dir, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	nome := "Conversa do WhatsApp com Ana.txt"
	if err := os.WriteFile(filepath.Join(inbox, nome), []byte("26/07/2026 14:32 - Ana: oi\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	scanner := &slowScanner{started: make(chan struct{}), release: make(chan struct{})}
	h := NewHandler(s, config.Config{Inbox: inbox, MediaDir: filepath.Join(dir, "media")}, scanner)

	req := httptest.NewRequest(http.MethodPost, "/chats/1/atualizar",
		strings.NewReader("file="+nome))
	req.SetPathValue("id", "1")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	pronto := make(chan struct{})
	go func() {
		h.applyUpdate(rec, req)
		close(pronto)
	}()

	select {
	case <-pronto:
	case <-time.After(3 * time.Second):
		t.Fatal("o handler ficou esperando o import terminar")
	}
	close(scanner.release)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	corpo := rec.Body.String()
	if !strings.Contains(corpo, "running-import") {
		t.Error("a resposta deveria trazer a barra de progresso")
	}
	if !strings.Contains(corpo, nome) {
		t.Error("a resposta deveria nomear o arquivo em importação")
	}

	select {
	case <-scanner.started:
	case <-time.After(3 * time.Second):
		t.Error("o import deveria ter começado em segundo plano")
	}
	_ = chatID
}

// A 10-year group had 725 senders, and the bar rendered every one as a chip: the picker
// filled the screen and pushed the conversation out of view.
func TestSplitCandidates(t *testing.T) {
	muitos := make([]string, 725)
	for i := range muitos {
		muitos[i] = "+55 11 90000-" + string(rune('0'+i%10))
	}

	chips, outros := splitCandidates(muitos)
	if len(chips) != maxOwnerChips {
		t.Errorf("chips = %d, queria %d", len(chips), maxOwnerChips)
	}
	if len(outros) != len(muitos) {
		t.Errorf("o seletor deveria trazer todos (%d), veio %d", len(muitos), len(outros))
	}
	// The chips are the top of the volume order — the likeliest to be "me".
	for i := range chips {
		if chips[i] != muitos[i] {
			t.Fatalf("chip %d = %q, queria %q", i, chips[i], muitos[i])
		}
	}

	poucos := []string{"Ana", "Bruno", "Carla"}
	chips, outros = splitCandidates(poucos)
	if len(chips) != 3 || outros != nil {
		t.Errorf("com poucos remetentes: chips=%v outros=%v, queria os 3 chips e nenhum seletor", chips, outros)
	}
}

// Saved contacts come before raw numbers: in a big group most senders are numbers the
// exporting phone never had saved, and a person looking for themselves recognizes a name.
func TestNamedFirst(t *testing.T) {
	// Input order is by volume.
	entrada := []string{"+55 11 99999-0001", "Wallace Pontes", "+55 41 9600-2882", "Michael", "E52645"}
	quer := []string{"Wallace Pontes", "Michael", "E52645", "+55 11 99999-0001", "+55 41 9600-2882"}

	got := namedFirst(entrada)
	for i := range quer {
		if got[i] != quer[i] {
			t.Fatalf("posição %d = %q, queria %q (lista: %v)", i, got[i], quer[i], got)
		}
	}
}
