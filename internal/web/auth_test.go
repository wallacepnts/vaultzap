package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wallacepnts/vaultzap/internal/config"
	"github.com/wallacepnts/vaultzap/internal/store"
)

func openTestHandler(t *testing.T, cfg config.Config) *Handler {
	t.Helper()
	s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "vaultzap.db"))
	if err != nil {
		t.Fatalf("abrir banco: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return NewHandler(s, cfg, nil)
}

func TestWithAuth_unconfiguredPassesThrough(t *testing.T) {
	h := openTestHandler(t, config.Config{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.Routes().ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized {
		t.Fatal("sem VAULTZAP_BASIC_AUTH configurado, não deveria exigir credenciais")
	}
}

func TestWithAuth_noCredentialsReturns401(t *testing.T) {
	h := openTestHandler(t, config.Config{BasicAuthUser: "ana", BasicAuthPassword: "segredo"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperava 401 sem credenciais, veio %d", rec.Code)
	}
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Error("esperava header WWW-Authenticate no desafio")
	}
}

func TestWithAuth_wrongCredentialsReturn401(t *testing.T) {
	h := openTestHandler(t, config.Config{BasicAuthUser: "ana", BasicAuthPassword: "segredo"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("ana", "senha-errada")
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperava 401 com senha errada, veio %d", rec.Code)
	}
}

func TestWithAuth_correctCredentialsPass(t *testing.T) {
	h := openTestHandler(t, config.Config{BasicAuthUser: "ana", BasicAuthPassword: "segredo"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("ana", "segredo")
	h.Routes().ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized {
		t.Fatal("credenciais corretas não deveriam ser rejeitadas")
	}
}

// Guards against html/template failing its escaping analysis and answering 200 with an
// empty body: checking rec.Code alone would never catch it.
func TestHome_rendersFullPage(t *testing.T) {
	h := openTestHandler(t, config.Config{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, esperado 200", rec.Code)
	}
	body := rec.Body.String()
	if len(body) < 1000 {
		t.Fatalf("corpo suspeitosamente pequeno (%d bytes) — página não renderizou: %q", len(body), body)
	}
	for _, want := range []string{"VaultZap", "cropper-overlay", "</html>"} {
		if !strings.Contains(body, want) {
			t.Errorf("corpo não contém %q — página não renderizou completa", want)
		}
	}
}
