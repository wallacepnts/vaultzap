package web

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Runs a POST through withOriginProtection alone, so the result is the origin check and
// nothing else.
func postThroughOriginCheck(t *testing.T, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	handler := withOriginProtection(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/setup", nil)
	req.Host = "192.168.1.12:8927"
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestTrustedOrigin(t *testing.T) {
	for _, c := range []struct {
		nome    string
		headers map[string]string
		aceita  bool
	}{
		{"same-origin", map[string]string{"Sec-Fetch-Site": "same-origin"}, true},
		{"none", map[string]string{"Sec-Fetch-Site": "none"}, true},
		{"nenhum dos dois cabeçalhos", nil, true},

		// The change: a browser value other than same-origin/none now falls through to the
		// Origin comparison instead of being refused on the spot.
		{"same-site com Origin do mesmo host", map[string]string{
			"Sec-Fetch-Site": "same-site",
			"Origin":         "http://192.168.1.12:8927",
		}, true},
		{"cross-site com Origin do mesmo host", map[string]string{
			"Sec-Fetch-Site": "cross-site",
			"Origin":         "http://192.168.1.12:8927",
		}, true},

		{"same-site com Origin de outro host", map[string]string{
			"Sec-Fetch-Site": "same-site",
			"Origin":         "http://outro.exemplo:8927",
		}, false},
		{"cross-site com Origin de outro host", map[string]string{
			"Sec-Fetch-Site": "cross-site",
			"Origin":         "http://atacante.exemplo",
		}, false},
		{"cross-site sem Origin", map[string]string{"Sec-Fetch-Site": "cross-site"}, true},
		{"só Origin, mesmo host", map[string]string{"Origin": "http://192.168.1.12:8927"}, true},
		{"só Origin, outro host", map[string]string{"Origin": "http://atacante.exemplo"}, false},

		// Behind a reverse proxy the Origin arrives as https:// while Host has no scheme;
		// comparing schemes would break the proxied deployment, which is the main path.
		{"proxy HTTPS preservando o Host", map[string]string{
			"Sec-Fetch-Site": "same-origin",
			"Origin":         "https://192.168.1.12:8927",
		}, true},
		{"Origin https com o mesmo host, sem Sec-Fetch-Site", map[string]string{
			"Origin": "https://192.168.1.12:8927",
		}, true},
	} {
		t.Run(c.nome, func(t *testing.T) {
			rec := postThroughOriginCheck(t, c.headers)
			aceitou := rec.Code != http.StatusForbidden
			if aceitou != c.aceita {
				t.Errorf("aceita = %v (status %d), esperado %v", aceitou, rec.Code, c.aceita)
			}
		})
	}
}

// A refusal that leaves no trace is what made the last one cost half an hour: the user sees
// three words and the log sees nothing, so there is no way to tell which branch fired.
func TestWithOriginProtection_logsTheRefusal(t *testing.T) {
	var buffer bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buffer, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	rec := postThroughOriginCheck(t, map[string]string{
		"Sec-Fetch-Site": "cross-site",
		"Origin":         "http://atacante.exemplo",
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("esperava 403, veio %d", rec.Code)
	}

	logged := buffer.String()
	for _, want := range []string{"cross-site", "atacante.exemplo", "192.168.1.12:8927", "POST", "/setup"} {
		if !strings.Contains(logged, want) {
			t.Errorf("o log da recusa não traz %q: %s", want, logged)
		}
	}
}

// GET is never a mutation, so it never goes through the check at all.
func TestWithOriginProtection_onlyGuardsMutations(t *testing.T) {
	handler := withOriginProtection(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("Origin", "http://atacante.exemplo")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET não deveria passar pela checagem, veio %d", rec.Code)
	}
}
