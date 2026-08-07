// Package export generates a read-only document of an entire conversation.
// Five formats, two axes:
//
//   - TXT and Markdown: plain text.
//   - HTML (screen) and HTML for printing: same structure, different stylesheet.
//   - Typst: .typ source.
//
// No media is embedded or copied: a message with an attachment becomes just "[foto]",
// "[áudio]" etc., without the file name.
package export

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/wallacepnts/vaultzap/internal/locale"
	"github.com/wallacepnts/vaultzap/internal/render"
	"github.com/wallacepnts/vaultzap/internal/store"
)

type Format string

const (
	TXT      Format = "txt"
	Markdown Format = "markdown"
	HTML     Format = "html"
	Print    Format = "impressao"
	Typst    Format = "typst"
)

var Formats = []Format{TXT, Markdown, HTML, Print, Typst}

func (f Format) Label() string {
	switch f {
	case TXT:
		return "Texto (.txt)"
	case Markdown:
		return "Markdown (.md)"
	case HTML:
		return "HTML"
	case Print:
		return "HTML para impressão / PDF"
	case Typst:
		return "Typst (.typ)"
	default:
		return string(f)
	}
}

func (f Format) extension() string {
	switch f {
	case TXT:
		return ".txt"
	case Markdown:
		return ".md"
	case HTML, Print:
		return ".html"
	case Typst:
		return ".typ"
	default:
		return ""
	}
}

func (f Format) ContentType() string {
	if f == HTML || f == Print {
		return "text/html; charset=utf-8"
	}
	return "text/plain; charset=utf-8"
}

func (f Format) valid() bool {
	for _, x := range Formats {
		if x == f {
			return true
		}
	}
	return false
}

// ErrInvalidFormat is returned by Export when format isn't one of Formats.
var ErrInvalidFormat = fmt.Errorf("formato de export desconhecido")

// Resolved by the caller, so this package depends on neither config nor HTTP.
type Options struct {
	// Empty means nobody is "me" in the document.
	Owner string
	// Locale is the UI language the export was requested in: date dividers, attachment
	// labels and the footer follow it. The zero value normalizes to pt-BR.
	Locale render.Locale

	// Sender as the export has it -> the display name the user chose.
	Nicknames map[string]string
}

// Streams from the database straight into w.
func Export(ctx context.Context, st *store.Store, chat store.Chat, options Options, format Format, w io.Writer) error {
	if !format.valid() {
		return ErrInvalidFormat
	}
	options.Locale = options.Locale.Normalize()
	wr := writerFor(format, options.Locale)

	if err := wr.header(w, chat); err != nil {
		return err
	}

	previousDate := ""
	err := st.IterateMessages(ctx, chat.ID, func(m store.Message) error {
		l := buildLine(m, options)
		if l.Date != "" && l.Date != previousDate {
			previousDate = l.Date
			if err := wr.dateDivider(w, l.Date); err != nil {
				return err
			}
		}
		return wr.line(w, l)
	})
	if err != nil {
		return err
	}
	return wr.footer(w, chat)
}

// The download name, without characters that would confuse the filesystem or the header.
func FileName(chat store.Chat, format Format) string {
	base := sanitizeFileName(chat.Name)
	if base == "" {
		base = fmt.Sprintf("conversa-%d", chat.ID)
	}
	return base + format.extension()
}

var forbiddenFileChars = strings.NewReplacer(
	"/", "-", "\\", "-", ":", "-", "*", "-", "?", "-",
	"\"", "'", "<", "-", ">", "-", "|", "-",
)

func sanitizeFileName(name string) string {
	return strings.TrimSpace(forbiddenFileChars.Replace(name))
}

func exportedOn(lang render.Locale) string {
	now := time.Now()
	return locale.T(lang, "export.footer", render.ShortDate(now, lang)+" "+render.ClockTime(now, lang))
}
