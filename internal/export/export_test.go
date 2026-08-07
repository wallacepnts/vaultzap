package export

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wallacepnts/vaultzap/internal/render"
	"github.com/wallacepnts/vaultzap/internal/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "vaultzap.db"))
	if err != nil {
		t.Fatalf("abrir banco: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func createTestChat(t *testing.T, s *store.Store, name string, isGroup bool) int64 {
	t.Helper()
	res, err := s.DB().Exec(
		`INSERT INTO chats (name, is_group, source, created_at) VALUES (?, ?, 'android', '2026-07-26 00:00:00')`,
		name, isGroup)
	if err != nil {
		t.Fatalf("criar chat: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func insertTestMessage(t *testing.T, s *store.Store, chatID int64, seq int, sentAt string, sender *string, body, kind string) {
	t.Helper()
	hash := fmt.Sprintf("%d-%d-%s", chatID, seq, sentAt)
	_, err := s.DB().Exec(
		`INSERT INTO messages (chat_id, sent_at, seq, sender, body, kind, hash) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		chatID, sentAt, seq, sender, body, kind, hash)
	if err != nil {
		t.Fatalf("inserir mensagem: %v", err)
	}
}

func pointer(s string) *string { return &s }

// The sender's name carries syntax characters from every format, and so do the bodies.
func buildTestConversation(t *testing.T) (*store.Store, store.Chat) {
	t.Helper()
	s := openTestStore(t)
	chatID := createTestChat(t, s, "Grupo * _teste_ #1", true)

	sender := `Fulano "*_[teste]_*" \com/barra`
	insertTestMessage(t, s, chatID, 1, "2026-07-26 09:00:00", nil, "As mensagens são protegidas.", "system")
	insertTestMessage(t, s, chatID, 2, "2026-07-26 09:01:00", &sender,
		"Corpo com *asterisco*, _underline_, `crase`, $dolar$, #hash, @arroba, [colchete], \\barra e \"aspas\".\nSegunda linha.", "text")
	insertTestMessage(t, s, chatID, 3, "2026-07-27 10:00:00", &sender, "", "media")
	if _, err := s.DB().Exec(`UPDATE messages SET attachment_id = (
		SELECT id FROM attachments LIMIT 1) WHERE chat_id = ? AND seq = 3`, chatID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`INSERT INTO attachments (chat_id, filename, sha256, media_kind, stored_path)
		VALUES (?, 'foto-pessoal-do-fulano.jpg', 'sha', 'image', 'x/sha.jpg')`, chatID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`UPDATE messages SET attachment_id = (SELECT id FROM attachments WHERE chat_id = ?)
		WHERE chat_id = ? AND seq = 3`, chatID, chatID); err != nil {
		t.Fatal(err)
	}

	chat, err := s.GetChat(context.Background(), chatID)
	if err != nil {
		t.Fatalf("BuscarChat: %v", err)
	}
	return s, chat
}

func exportToString(t *testing.T, s *store.Store, chat store.Chat, format Format) string {
	t.Helper()
	var b strings.Builder
	if err := Export(context.Background(), s, chat, Options{}, format, &b); err != nil {
		t.Fatalf("Export(%s): %v", format, err)
	}
	return b.String()
}

func TestExport_doesNotLeakAttachmentFileName(t *testing.T) {
	s, chat := buildTestConversation(t)
	for _, f := range Formats {
		out := exportToString(t, s, chat, f)
		if strings.Contains(out, "foto-pessoal-do-fulano") {
			t.Errorf("[%s] o nome do arquivo do anexo vazou pro export", f)
		}
		if !strings.Contains(out, "foto") {
			t.Errorf("[%s] esperava a palavra 'foto' no lugar do anexo", f)
		}
	}
}

func TestExport_multilineIsPreserved(t *testing.T) {
	s, chat := buildTestConversation(t)
	for _, f := range Formats {
		out := exportToString(t, s, chat, f)
		if !strings.Contains(out, "Segunda linha") {
			t.Errorf("[%s] a segunda linha da mensagem sumiu", f)
		}
	}
}

func TestExportMarkdown_escapesSyntax(t *testing.T) {
	s, chat := buildTestConversation(t)
	out := exportToString(t, s, chat, Markdown)

	if !strings.Contains(out, `\*\_\[teste\]\_\*`) {
		t.Error("asterisco/colchetes/underscore do remetente vazaram sem escapar")
	}
	if !strings.Contains(out, `\*asterisco\*`) {
		t.Error("asterisco do corpo não foi escapado")
	}
	if !strings.Contains(out, `\[colchete\]`) {
		t.Error("colchete do corpo não foi escapado")
	}
}

func TestExportTypst_stringsAreSafe(t *testing.T) {
	s, chat := buildTestConversation(t)
	out := exportToString(t, s, chat, Typst)

	if !strings.Contains(out, `\"aspas\"`) {
		t.Error(`aspas do corpo não foram escapadas para \"`)
	}
	if !strings.Contains(out, `\\com`) {
		t.Error(`barra invertida do remetente não foi escapada para \\`)
	}
	body := out
	insideString := false
	escaping := false
	for _, r := range body {
		switch {
		case escaping:
			escaping = false
		case r == '\\':
			escaping = true
		case r == '"':
			insideString = !insideString
		}
	}
	if insideString {
		t.Error("número ímpar de aspas não-escapadas — o documento Typst gerado é inválido")
	}
}

func TestExportHTML_escapesAndFormats(t *testing.T) {
	s := openTestStore(t)
	chatID := createTestChat(t, s, "Ana", false)
	sender := "Ana"
	insertTestMessage(t, s, chatID, 1, "2026-07-26 09:00:00", &sender,
		`<img src=x onerror=alert(1)> e *negrito real*`, "text")
	chat, _ := s.GetChat(context.Background(), chatID)

	for _, f := range []Format{HTML, Print} {
		out := exportToString(t, s, chat, f)
		if strings.Contains(out, "<img src=x onerror") {
			t.Errorf("[%s] tag crua do corpo da mensagem vazou pro HTML (XSS)", f)
		}
		if !strings.Contains(out, "<b>negrito real</b>") {
			t.Errorf("[%s] a formatação *negrito* do WhatsApp não foi aplicada", f)
		}
	}
}

func TestExportHTML_embeddedStyle(t *testing.T) {
	s, chat := buildTestConversation(t)
	for _, f := range []Format{HTML, Print} {
		out := exportToString(t, s, chat, f)
		if strings.Contains(out, "ZgotmplZ") {
			t.Errorf("[%s] o CSS foi substituído pelo placeholder de segurança do html/template", f)
		}
		if !strings.Contains(out, "background") {
			t.Errorf("[%s] o <style> não contém CSS de verdade", f)
		}
	}
}

func TestExportHTML_ownMessageIsMarked(t *testing.T) {
	s := openTestStore(t)
	chatID := createTestChat(t, s, "Ana", false)
	ana, me := "Ana", "Wallace"
	insertTestMessage(t, s, chatID, 1, "2026-07-26 09:00:00", &ana, "oi", "text")
	insertTestMessage(t, s, chatID, 2, "2026-07-26 09:01:00", &me, "oi de volta", "text")
	chat, _ := s.GetChat(context.Background(), chatID)

	var b strings.Builder
	if err := Export(context.Background(), s, chat, Options{Owner: me}, HTML, &b); err != nil {
		t.Fatalf("Export: %v", err)
	}
	out := b.String()
	if !strings.Contains(out, `class="msg own"`) {
		t.Error("a mensagem do dono não ficou marcada como 'own'")
	}
}

func TestExport_nicknameIsApplied(t *testing.T) {
	s := openTestStore(t)
	chatID := createTestChat(t, s, "Grupo", true)
	number := "+55 21 90000-0000"
	insertTestMessage(t, s, chatID, 1, "2026-07-26 09:00:00", &number, "oi", "text")
	chat, _ := s.GetChat(context.Background(), chatID)

	options := Options{Nicknames: map[string]string{number: "Fulano de Tal"}}
	for _, f := range Formats {
		var b strings.Builder
		if err := Export(context.Background(), s, chat, options, f, &b); err != nil {
			t.Fatalf("[%s] Export: %v", f, err)
		}
		out := b.String()
		if strings.Contains(out, number) {
			t.Errorf("[%s] o número original vazou em vez do apelido", f)
		}
		if !strings.Contains(out, "Fulano de Tal") {
			t.Errorf("[%s] o apelido não apareceu no documento", f)
		}
	}
}

func TestExport_invalidFormat(t *testing.T) {
	s, chat := buildTestConversation(t)
	var b strings.Builder
	err := Export(context.Background(), s, chat, Options{}, Format("pdf-mágico"), &b)
	if err != ErrInvalidFormat {
		t.Errorf("err = %v, esperado ErrInvalidFormat", err)
	}
}

func TestFileName(t *testing.T) {
	cases := []struct {
		name     string
		format   Format
		expected string
	}{
		{"Ana Souza", TXT, "Ana Souza.txt"},
		{"Grupo/Teste", Markdown, "Grupo-Teste.md"},
		{`Nome: "esquisito" <raro>`, HTML, "Nome- 'esquisito' -raro-.html"},
		{"", Typst, "conversa-0.typ"},
	}
	for _, c := range cases {
		chat := store.Chat{Name: c.name}
		if got := FileName(chat, c.format); got != c.expected {
			t.Errorf("FileName(%q, %s) = %q, expected %q", c.name, c.format, got, c.expected)
		}
	}
}

func TestExport_emptyConversation(t *testing.T) {
	s := openTestStore(t)
	chatID := createTestChat(t, s, "Vazio", false)
	chat, _ := s.GetChat(context.Background(), chatID)

	for _, f := range Formats {
		var b strings.Builder
		if err := Export(context.Background(), s, chat, Options{}, f, &b); err != nil {
			t.Errorf("[%s] Export numa conversa vazia falhou: %v", f, err)
		}
		if !strings.Contains(b.String(), "Vazio") {
			t.Errorf("[%s] o nome do chat nem apareceu no cabeçalho", f)
		}
	}
}

// CommonMark allows raw HTML, so a tag at the start of a line opens an HTML block the
// renderer emits verbatim — an <img onerror=...> typed in the chat would become live
// markup in the exported .md.
func TestExportMarkdown_escapesRawHTML(t *testing.T) {
	cases := []struct {
		name, input, want string
	}{
		{"tag de imagem", `<img src=x onerror=alert>`, `\<img src=x onerror=alert\>`},
		{"bloco que engole o resto", `<div>`, `\<div\>`},
		{"entidade HTML", `&copy;`, `\&copy;`},
	}
	for _, c := range cases {
		got := escapeMarkdown(c.input)
		if got != c.want {
			t.Errorf("%s: escapeMarkdown(%q) = %q, esperado %q", c.name, c.input, got, c.want)
		}
		if strings.Contains(got, "<") && !strings.Contains(got, `\<`) {
			t.Errorf("%s: sobrou um \"<\" sem escapar em %q", c.name, got)
		}
	}
}

// The export follows the UI language the download was requested in: date dividers,
// attachment labels and the footer. Options{} (no locale) stays on pt-BR.
func TestExport_followsRequestedLocale(t *testing.T) {
	s, chat := buildTestConversation(t)

	cases := []struct {
		lang     render.Locale
		divider  string
		media    string
		footer   string
		shortDay string
	}{
		{render.LocalePTBR, "26 de julho de 2026", "[foto]", "Exportado do VaultZap", "26/07/2026"},
		{render.LocaleEN, "July 26, 2026", "[photo]", "Exported from VaultZap", "07/26/2026"},
		{render.LocaleDE, "26. Juli 2026", "[Foto]", "Exportiert aus VaultZap", "26.07.2026"},
	}

	for _, c := range cases {
		var b strings.Builder
		if err := Export(context.Background(), s, chat, Options{Locale: c.lang}, HTML, &b); err != nil {
			t.Fatalf("Export(%s): %v", c.lang, err)
		}
		out := b.String()
		for _, want := range []string{c.divider, c.media, c.footer} {
			if !strings.Contains(out, want) {
				t.Errorf("[%s] esperava %q no documento", c.lang, want)
			}
		}
		if want := `<html lang="` + string(c.lang) + `">`; !strings.Contains(out, want) {
			t.Errorf("[%s] esperava %s", c.lang, want)
		}

		var txt strings.Builder
		if err := Export(context.Background(), s, chat, Options{Locale: c.lang}, TXT, &txt); err != nil {
			t.Fatalf("Export TXT(%s): %v", c.lang, err)
		}
		if !strings.Contains(txt.String(), c.shortDay) {
			t.Errorf("[%s] esperava a data curta %s no TXT", c.lang, c.shortDay)
		}
	}

	// Zero value: nobody passing a locale still gets pt-BR, as before.
	if out := exportToString(t, s, chat, HTML); !strings.Contains(out, "26 de julho de 2026") {
		t.Error("Options{} sem locale deveria continuar em pt-BR")
	}
}
