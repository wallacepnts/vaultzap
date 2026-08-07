package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/wallacepnts/vaultzap/internal/config"
	"github.com/wallacepnts/vaultzap/internal/store"
)

func TestToggleMessageFlag_toastSoAoFavoritar(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "vaultzap.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	res, _ := s.DB().Exec(`INSERT INTO chats (name, is_group, source, created_at) VALUES ('Ana',0,'android','2026-07-26 00:00:00')`)
	chatID, _ := res.LastInsertId()
	res, _ = s.DB().Exec(`INSERT INTO messages (chat_id, sent_at, seq, sender, body, kind, hash) VALUES (?,?,?,?,?,?,?)`,
		chatID, "2026-07-26 10:00:00", 1, "Ana", "oi", "text", "h1")
	msgID, _ := res.LastInsertId()

	h := NewHandler(s, config.Config{}, nil, "")

	post := func(rota string) map[string]any {
		t.Helper()
		rec := httptest.NewRecorder()
		url := "/chats/" + strconv.FormatInt(chatID, 10) + "/messages/" + strconv.FormatInt(msgID, 10) + "/" + rota
		req := httptest.NewRequest(http.MethodPost, url, nil)
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		h.Routes().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d", rota, rec.Code)
		}
		hdr := rec.Header().Get("HX-Trigger")
		if hdr == "" {
			return nil // no toast is the expected result for pinning
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(hdr), &payload); err != nil {
			t.Fatalf("%s: HX-Trigger não é JSON válido: %v\n%s", rota, err, hdr)
		}
		toastAny, ok := payload["toast"]
		if !ok {
			return nil
		}
		return toastAny.(map[string]any)
	}

	on := post("favoritar")
	if on == nil {
		t.Fatal("favoritar: nenhum toast disparado")
	}
	if !strings.Contains(on["text"].(string), "favoritada") {
		t.Errorf("favoritar: texto = %q", on["text"])
	}
	if got := on["undo"].(string); !strings.HasSuffix(got, "/favoritar") {
		t.Errorf("undo = %q, esperado terminar em /favoritar", got)
	}
	if got := on["target"].(string); got != "#meta-"+strconv.FormatInt(msgID, 10) {
		t.Errorf("target = %q", got)
	}
	if got := on["swap"].(string); got != "outerHTML" {
		t.Errorf("swap = %q, esperado outerHTML", got)
	}
	off := post("favoritar")
	if off == nil || !strings.Contains(off["text"].(string), "desfavoritada") {
		t.Errorf("desfavoritar: toast = %v", off)
	}

	if got := post("fixar"); got != nil {
		t.Errorf("fixar mensagem não deveria disparar toast, veio %v", got)
	}
}

func TestToggleMessageFlag_desfazerNaoGeraOutroToast(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "vaultzap.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	res, _ := s.DB().Exec(`INSERT INTO chats (name, is_group, source, created_at) VALUES ('Ana',0,'android','2026-07-26 00:00:00')`)
	chatID, _ := res.LastInsertId()
	res, _ = s.DB().Exec(`INSERT INTO messages (chat_id, sent_at, seq, sender, body, kind, hash) VALUES (?,?,?,?,?,?,?)`,
		chatID, "2026-07-26 10:00:00", 1, "Ana", "oi", "text", "h1")
	msgID, _ := res.LastInsertId()

	h := NewHandler(s, config.Config{}, nil, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost,
		"/chats/"+strconv.FormatInt(chatID, 10)+"/messages/"+strconv.FormatInt(msgID, 10)+"/favoritar?undoing=1", nil)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	h.Routes().ServeHTTP(rec, req)
	if got := rec.Header().Get("HX-Trigger"); got != "" {
		t.Errorf("desfazer não deveria disparar toast, veio %q", got)
	}
}

func TestFavoriteChat_desfazerSoAoRemover(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "vaultzap.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	res, _ := s.DB().Exec(`INSERT INTO chats (name, is_group, source, created_at) VALUES ('Ana',0,'android','2026-07-26 00:00:00')`)
	chatID, _ := res.LastInsertId()

	h := NewHandler(s, config.Config{}, nil, "")
	post := func() map[string]any {
		t.Helper()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/chats/"+strconv.FormatInt(chatID, 10)+"/favoritar", nil)
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		h.Routes().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(rec.Header().Get("HX-Trigger")), &payload); err != nil {
			t.Fatalf("HX-Trigger inválido: %v", err)
		}
		return payload["toast"].(map[string]any)
	}

	adicionar := post()
	if !strings.Contains(adicionar["text"].(string), "Adicionada") {
		t.Errorf("adicionar: texto = %q", adicionar["text"])
	}
	if _, tem := adicionar["undo"]; tem {
		t.Errorf("adicionar não deveria oferecer Desfazer, veio %v", adicionar["undo"])
	}

	remover := post()
	if !strings.Contains(remover["text"].(string), "Removida") {
		t.Errorf("remover: texto = %q", remover["text"])
	}
	if got, tem := remover["undo"]; !tem || !strings.HasSuffix(got.(string), "/favoritar") {
		t.Errorf("remover deveria oferecer Desfazer para /favoritar, veio %v", got)
	}
}
