package export

import (
	"io"

	"github.com/wallacepnts/vaultzap/internal/render"
	"github.com/wallacepnts/vaultzap/internal/store"
)

// writer writes a document incrementally: header, one divider per new day, one line per
// message, footer. Each format implements only its own syntax; the traversal and date
// grouping are shared in export.go.
type writer interface {
	header(w io.Writer, chat store.Chat) error
	dateDivider(w io.Writer, isoDate string) error
	line(w io.Writer, l line) error
	footer(w io.Writer, chat store.Chat) error
}

func writerFor(format Format, lang render.Locale) writer {
	switch format {
	case Markdown:
		return markdownWriter{lang: lang}
	case HTML:
		return htmlWriter{lang: lang}
	case Print:
		return htmlWriter{lang: lang, print: true}
	case Typst:
		return typstWriter{lang: lang}
	default:
		return txtWriter{lang: lang}
	}
}
