package export

import (
	"time"

	"github.com/wallacepnts/vaultzap/internal/locale"
	"github.com/wallacepnts/vaultzap/internal/render"
	"github.com/wallacepnts/vaultzap/internal/store"
)

// line is a message resolved to the text that will appear in the document — nickname
// applied, attachment turned into a type label — not yet escaped for the output syntax.
type line struct {
	Date      string // "2026-07-26"; empty if the message has no valid timestamp
	ShortDate string // localized: "26/07/2026", "07/26/2026", "26.07.2026"
	Time      string // localized: "14:32" or "2:32 PM"
	Sender    string // "" == system message, no bubble of its own
	Own       bool
	Body      string
}

func buildLine(m store.Message, options Options) line {
	l := line{Body: textBody(m, options.Locale)}

	if t, err := time.Parse("2006-01-02 15:04:05", m.SentAt); err == nil {
		l.Date = t.Format("2006-01-02")
		l.ShortDate = render.ShortDate(t, options.Locale)
		l.Time = render.ClockTime(t, options.Locale)
	}

	if m.Sender != nil {
		name := *m.Sender
		if nickname := options.Nicknames[name]; nickname != "" {
			name = nickname
		}
		l.Sender = name
		l.Own = options.Owner != "" && *m.Sender == options.Owner
	}
	return l
}

// An attachment carries only its type, never the original file name. The other variants
// arrive from the parser with a self-explanatory Body.
func textBody(m store.Message, lang render.Locale) string {
	if m.Kind == "media" || m.Kind == "contact" {
		return "[" + locale.T(lang, mediaKey(m.AttachmentMediaKind)) + "]"
	}
	return m.Body
}

var mediaKeys = map[string]string{
	"image":    "export.photo",
	"video":    "export.video",
	"gif":      "export.gif",
	"audio":    "export.audio",
	"voice":    "export.voice",
	"sticker":  "export.sticker",
	"document": "export.document",
	"contact":  "export.contact",
}

func mediaKey(kind string) string {
	if key, ok := mediaKeys[kind]; ok {
		return key
	}
	return "export.attachment"
}
