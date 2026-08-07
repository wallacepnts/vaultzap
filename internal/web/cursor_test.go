package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/wallacepnts/vaultzap/internal/config"
	"github.com/wallacepnts/vaultzap/internal/store"
)

func TestEncodeDecodeCursor(t *testing.T) {
	cases := []struct {
		sentAt string
		seq    int
		id     int64
	}{
		{"2026-07-26 14:32:00", 1, 1},
		{"2026-05-04 09:00:00", 85, 85},
		{"2026-01-01 00:00:00", 0, 9007199254740991},
	}
	for _, c := range cases {
		encoded, err := url.QueryUnescape(encodeCursor(c.sentAt, c.seq, c.id))
		if err != nil {
			t.Fatalf("desescapar cursor: %v", err)
		}
		sentAt, seq, id, err := decodeCursor(encoded)
		if err != nil {
			t.Fatalf("decodeCursor(%q): %v", encoded, err)
		}
		if sentAt != c.sentAt || seq != c.seq || id != c.id {
			t.Errorf("ida e volta = (%q, %d, %d), esperado (%q, %d, %d)",
				sentAt, seq, id, c.sentAt, c.seq, c.id)
		}
	}
}

func TestDecodeCursor_legacyFormat(t *testing.T) {
	sentAt, seq, id, err := decodeCursor("2026-07-26 14:32:00,7")
	if err != nil {
		t.Fatalf("cursor de duas partes deveria continuar aceito: %v", err)
	}
	if sentAt != "2026-07-26 14:32:00" || seq != 7 || id != 0 {
		t.Errorf("= (%q, %d, %d), esperado (%q, 7, 0)", sentAt, seq, id, "2026-07-26 14:32:00")
	}
}

func TestDecodeCursor_invalid(t *testing.T) {
	for _, invalid := range []string{"", "só-a-data", "2026-07-26 14:32:00,x", "2026-07-26 14:32:00,1,y", "a,1,2,3"} {
		if _, _, _, err := decodeCursor(invalid); err == nil {
			t.Errorf("decodeCursor(%q) não devolveu erro", invalid)
		}
	}
}

var sentinelCursor = regexp.MustCompile(`messages\?before=([^"]*)`)

var messageBody = regexp.MustCompile(`mensagem (\d+)`)

func TestMessages_scrollReachesEverything(t *testing.T) {
	const total = 120
	h, chatID := handlerWithChat(t, total)

	seen := map[string]bool{}
	body := getFragment(t, h, fmt.Sprintf("/chats/%d", chatID))
	pages := 1

	for {
		for _, m := range messageBody.FindAllStringSubmatch(body, -1) {
			seen[m[1]] = true
		}

		match := sentinelCursor.FindStringSubmatch(body)
		if match == nil {
			break
		}
		if pages > 10 {
			t.Fatal("mais de 10 páginas: a rolagem não está terminando")
		}
		cursor := strings.ReplaceAll(match[1], "&#43;", "+")
		body = getFragment(t, h, fmt.Sprintf("/chats/%d/messages?before=%s", chatID, cursor))
		pages++
	}

	if len(seen) != total {
		t.Errorf("a rolagem alcançou %d de %d mensagens em %d páginas", len(seen), total, pages)
	}
}

func TestMessages_invalidCursorReturns400(t *testing.T) {
	h, chatID := handlerWithChat(t, 3)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/chats/%d/messages?before=lixo", chatID), nil)
	req.Header.Set("HX-Request", "true")
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, esperado 400 para um cursor malformado", rec.Code)
	}
}

func handlerWithChat(t *testing.T, n int) (*Handler, int64) {
	t.Helper()
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "vaultzap.db"))
	if err != nil {
		t.Fatalf("abrir banco: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	res, err := s.DB().Exec(
		`INSERT INTO chats (name, is_group, source, created_at) VALUES ('Ana', 0, 'android', '2026-07-26 00:00:00')`)
	if err != nil {
		t.Fatal(err)
	}
	chatID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}

	for i := 1; i <= n; i++ {
		_, err := s.DB().Exec(
			`INSERT INTO messages (chat_id, sent_at, seq, sender, body, kind, hash) VALUES (?,?,?,?,?,?,?)`,
			chatID, fmt.Sprintf("2026-07-26 %02d:%02d:00", i/60, i%60), i, "Ana",
			fmt.Sprintf("mensagem %d", i), "text", fmt.Sprintf("hash-%d", i))
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.DB().Exec(
		`UPDATE chats SET message_count = ?, first_message_at = (SELECT MIN(sent_at) FROM messages),
		 last_message_at = (SELECT MAX(sent_at) FROM messages) WHERE id = ?`, n, chatID); err != nil {
		t.Fatal(err)
	}

	return NewHandler(s, config.Config{}, nil), chatID
}

func getFragment(t *testing.T, h *Handler, path string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("HX-Request", "true")
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, esperado 200\n%s", path, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}
