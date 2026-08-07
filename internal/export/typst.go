package export

import (
	"fmt"
	"io"
	"strings"

	"github.com/wallacepnts/vaultzap/internal/render"
	"github.com/wallacepnts/vaultzap/internal/store"
)

// A .typ source, to compile with `typst compile`. The drawing logic sits in three
// functions declared once in the preamble, so each message is one short `#bubble(...)` and
// the file stays readable at 30k messages. User text goes in as a string literal, never
// as markup.
type typstWriter struct{ lang render.Locale }

func (typstWriter) header(w io.Writer, chat store.Chat) error {
	if _, err := io.WriteString(w, typstPreamble); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "#align(center)[#text(size: 16pt, weight: \"bold\")[#%s]]\n",
		typstString(chat.Name))
	return err
}

func (e typstWriter) dateDivider(w io.Writer, isoDate string) error {
	_, err := fmt.Fprintf(w, "#day(%s)\n", typstString(render.LongDate(isoDate, e.lang)))
	return err
}

func (typstWriter) line(w io.Writer, l line) error {
	if l.Sender == "" {
		_, err := fmt.Fprintf(w, "#system(%s)\n", typstString(l.Body))
		return err
	}
	_, err := fmt.Fprintf(w, "#bubble(%t, %s, %s, %s)\n",
		l.Own, typstString(l.Sender), typstString(l.Time), typstString(l.Body))
	return err
}

func (e typstWriter) footer(w io.Writer, _ store.Chat) error {
	_, err := fmt.Fprintf(w, "\n#v(1em)\n#align(center)[#text(size: 8pt, fill: timeColor)[#%s]]\n",
		typstString(exportedOn(e.lang)))
	return err
}

// height: auto makes the whole conversation one continuous page instead of A4 sheets, so
// no bubble is ever cut across a page break. The width stays A4; only pagination is gone.
//
// The bubble width comes from `measure`: a short message shrinks to the text, a long one
// caps at 72% of the line. The font stack covers mac/Windows/Linux; with none of them
// the compiler falls back to its default.
const typstPreamble = `#set page(height: auto, margin: 1.6cm, fill: rgb("#efeae2"))
#set text(size: 10pt, fill: rgb("#111b21"),
          font: ("Helvetica", "Arial", "Liberation Sans", "DejaVu Sans", "Noto Sans"))
#set par(leading: 0.5em)

#let timeColor = rgb("#667781")

#let bubble(own, sender, time, body) = layout(disp => {
  let core = {
    text(size: 8.5pt, weight: "bold", fill: rgb("#0f7a5b"))[#sender]
    linebreak()
    body
    h(0.7em)
    text(size: 7pt, fill: timeColor)[#time]
  }
  let width = calc.min(measure(core).width + 22pt, disp.width * 0.72)
  align(if own { right } else { left },
    block(width: width, radius: 6pt, inset: (x: 8pt, y: 6pt),
          above: 0.35em, below: 0.35em,
          fill: if own { rgb("#d9fdd3") } else { white },
          stroke: 0.4pt + rgb("#0000001f"),
          align(left, core)))
})

#let system(body) = align(center, block(inset: (x: 8pt, y: 4pt), radius: 6pt,
  above: 0.5em, below: 0.5em, fill: rgb("#ffffffbb"),
  text(size: 8pt, fill: timeColor, style: "italic")[#body]))

#let day(label) = align(center, block(inset: (x: 9pt, y: 5pt), radius: 6pt,
  above: 1em, below: 0.6em, fill: rgb("#ffffffcc"),
  text(size: 8pt, fill: timeColor)[#label]))

`

func typstString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
