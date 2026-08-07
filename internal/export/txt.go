package export

import (
	"fmt"
	"io"

	"github.com/wallacepnts/vaultzap/internal/render"
	"github.com/wallacepnts/vaultzap/internal/store"
)

// txtWriter reproduces WhatsApp Android's own export format:
// "DD/MM/YYYY HH:MM - Sender: body", one line per message, no grouping by day.
type txtWriter struct{ lang render.Locale }

func (txtWriter) header(w io.Writer, chat store.Chat) error {
	_, err := fmt.Fprintf(w, "%s\n\n", chat.Name)
	return err
}

func (txtWriter) dateDivider(io.Writer, string) error { return nil }

func (txtWriter) line(w io.Writer, l line) error {
	if l.Sender == "" {
		_, err := fmt.Fprintf(w, "%s %s - %s\n", l.ShortDate, l.Time, l.Body)
		return err
	}
	_, err := fmt.Fprintf(w, "%s %s - %s: %s\n", l.ShortDate, l.Time, l.Sender, l.Body)
	return err
}

func (e txtWriter) footer(w io.Writer, _ store.Chat) error {
	_, err := fmt.Fprintf(w, "\n%s\n", exportedOn(e.lang))
	return err
}
