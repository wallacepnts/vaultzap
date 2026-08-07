package export

import (
	"fmt"
	"io"
	"strings"

	"github.com/wallacepnts/vaultzap/internal/render"
	"github.com/wallacepnts/vaultzap/internal/store"
)

type markdownWriter struct{ lang render.Locale }

func (markdownWriter) header(w io.Writer, chat store.Chat) error {
	_, err := fmt.Fprintf(w, "# %s\n", escapeMarkdown(chat.Name))
	return err
}

func (e markdownWriter) dateDivider(w io.Writer, isoDate string) error {
	_, err := fmt.Fprintf(w, "\n## %s\n\n", escapeMarkdown(render.LongDate(isoDate, e.lang)))
	return err
}

func (markdownWriter) line(w io.Writer, l line) error {
	if l.Sender == "" {
		_, err := fmt.Fprintf(w, "_%s_\n\n", markdownBody(l.Body))
		return err
	}
	_, err := fmt.Fprintf(w, "**%s** `%s`  \n%s\n\n", escapeMarkdown(l.Sender), l.Time, markdownBody(l.Body))
	return err
}

func (e markdownWriter) footer(w io.Writer, _ store.Chat) error {
	_, err := fmt.Fprintf(w, "---\n\n%s\n", escapeMarkdown(exportedOn(e.lang)))
	return err
}

// mdEscapable are the characters CommonMark lets you escape with \.
//
// "<" and "&" are here for a reason that isn't cosmetic: CommonMark allows raw HTML, so
// a body starting with a tag opens an HTML block that the renderer emits verbatim — an
// "<img src=x onerror=...>" typed by someone in the chat becomes live markup in the
// exported .md, and a benign "<div>" swallows every message after it until it closes.
const mdEscapable = "\\`*_{}[]()#+-.!~|>$<&"

func escapeMarkdown(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if strings.ContainsRune(mdEscapable, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func markdownBody(body string) string {
	return strings.ReplaceAll(escapeMarkdown(body), "\n", "  \n")
}
