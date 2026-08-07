package render

import (
	"strings"
	"testing"
)

func TestFormat_escapesBeforeMarking(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		absent  []string
		present []string
	}{
		{
			name:    "xss via tag img",
			input:   `<img src=x onerror=alert(1)>`,
			absent:  []string{"<img"},
			present: []string{"&lt;img"},
		},
		{
			name:    "negrito",
			input:   "isso é *importante* aqui",
			present: []string{"<b>importante</b>"},
		},
		{
			name:    "italico",
			input:   "isso é _importante_ aqui",
			present: []string{"<i>importante</i>"},
		},
		{
			name:    "riscado",
			input:   "isso é ~importante~ aqui",
			present: []string{"<s>importante</s>"},
		},
		{
			name:    "mono",
			input:   "roda ```go run main.go``` no terminal",
			present: []string{"<code>go run main.go</code>"},
		},
		{
			name:    "mono protege marcação interna",
			input:   "```*não vira negrito*```",
			present: []string{"<code>*não vira negrito*</code>"},
			absent:  []string{"<b>"},
		},
		{
			name:    "autolink",
			input:   "veja https://example.com/pagina.",
			present: []string{`<a href="https://example.com/pagina" target="_blank" rel="noopener noreferrer">https://example.com/pagina</a>.`},
		},
		{
			name:    "emoji inline maior que o texto",
			input:   "Oi 👋 tudo bem?",
			present: []string{`Oi <span class="emoji-inline">👋</span> tudo bem?`},
		},
		{
			name:    "mono protege emoji de virar inline",
			input:   "```👋```",
			present: []string{"<code>👋</code>"},
			absent:  []string{"emoji-inline"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			saida := string(Format(c.input))
			for _, s := range c.present {
				if !strings.Contains(saida, s) {
					t.Errorf("saída não contém %q\nsaída: %s", s, saida)
				}
			}
			for _, s := range c.absent {
				if strings.Contains(saida, s) {
					t.Errorf("saída contém %q e não deveria\nsaída: %s", s, saida)
				}
			}
		})
	}
}

func TestFormatSnippet(t *testing.T) {
	cases := []struct {
		name  string
		input string
		saida string
	}{
		{
			name:  "um destaque",
			input: "vamos pro " + snippetMarkStart + "cinema" + snippetMarkEnd + " hoje",
			saida: "vamos pro <mark>cinema</mark> hoje",
		},
		{
			name:  "dois destaques",
			input: snippetMarkStart + "oi" + snippetMarkEnd + " tudo bem, " + snippetMarkStart + "oi" + snippetMarkEnd,
			saida: "<mark>oi</mark> tudo bem, <mark>oi</mark>",
		},
		{
			name:  "sem destaque",
			input: "texto qualquer",
			saida: "texto qualquer",
		},
		{
			name:  "escapa html mesmo dentro da marcação",
			input: snippetMarkStart + "<script>" + snippetMarkEnd,
			saida: "<mark>&lt;script&gt;</mark>",
		},
		{
			name:  "escapa html fora da marcação",
			input: "<img onerror=x> " + snippetMarkStart + "termo" + snippetMarkEnd,
			saida: "&lt;img onerror=x&gt; <mark>termo</mark>",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := string(FormatSnippet(c.input)); got != c.saida {
				t.Errorf("FormatSnippet(%q) = %q, esperado %q", c.input, got, c.saida)
			}
		})
	}
}

func TestIsOnlyEmoji(t *testing.T) {
	cases := map[string]bool{
		"😀🎉":         true,
		"😀 🎉":        true,
		"Oi 👋":       false,
		"":           false,
		"   ":        false,
		"texto puro": false,
		"👨‍👩‍👧":      true, // family, joined with ZWJ
	}
	for input, want := range cases {
		if got := IsOnlyEmoji(input); got != want {
			t.Errorf("IsOnlyEmoji(%q) = %v, esperado %v", input, got, want)
		}
	}
}

func TestExtractURLs(t *testing.T) {
	cases := []struct {
		text string
		want []string
	}{
		{"sem link aqui", nil},
		{"olha https://exemplo.com/a", []string{"https://exemplo.com/a"}},
		{"dois: http://a.com e https://b.com/x?q=1", []string{"http://a.com", "https://b.com/x?q=1"}},
		{"vai em https://exemplo.com.", []string{"https://exemplo.com"}},
		{"(veja https://exemplo.com/p)", []string{"https://exemplo.com/p"}},
	}
	for _, c := range cases {
		got := ExtractURLs(c.text)
		if len(got) != len(c.want) {
			t.Errorf("ExtractURLs(%q) = %v, esperado %v", c.text, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("ExtractURLs(%q)[%d] = %q, esperado %q", c.text, i, got[i], c.want[i])
			}
		}
	}
}

// A NUL in the body would forge a mono-block placeholder with no block behind it.
func TestFormat_controlBytesDoNotBreakConversation(t *testing.T) {
	cases := []string{
		"\x000\x00",
		"antes \x0099\x00 depois",
		"mistura ```codigo``` com \x001\x00 forjado",
		"\x00",
	}
	for _, input := range cases {
		got := string(Format(input))
		if strings.Contains(got, "\x00") {
			t.Errorf("Format(%q) = %q, ainda contém NUL", input, got)
		}
	}

	if got := string(Format("roda ```go test``` aqui")); !strings.Contains(got, "<code>go test</code>") {
		t.Errorf("bloco mono quebrou: %q", got)
	}
}

// A URL carrying "_", "*" or "~" is routine; marking before autolinking injects tags
// into the address and truncates the link.
func TestFormat_urlWithMarkupIsNotBroken(t *testing.T) {
	cases := []string{
		"https://pt.wikipedia.org/wiki/Guerra_dos_Cem_Anos",
		"https://exemplo.com/a_b_c/d_e",
		"https://exemplo.com/busca?q=a*b*c",
		"https://exemplo.com/x~y~z",
	}
	for _, url := range cases {
		got := string(Format("olha " + url + " aqui"))
		want := `<a href="` + url + `" target="_blank" rel="noopener noreferrer">` + url + `</a>`
		if !strings.Contains(got, want) {
			t.Errorf("Format(%q) = %q\nnão contém o link inteiro", url, got)
		}
		for _, tag := range []string{"<i>", "<b>", "<s>"} {
			if strings.Contains(got, tag) {
				t.Errorf("Format(%q) injetou %s dentro da URL: %q", url, tag, got)
			}
		}
	}

	got := string(Format("*importante*: https://exemplo.com/a_b e _fim_"))
	for _, want := range []string{"<b>importante</b>", "<i>fim</i>", `href="https://exemplo.com/a_b"`} {
		if !strings.Contains(got, want) {
			t.Errorf("esperava %q em %q", want, got)
		}
	}
}
