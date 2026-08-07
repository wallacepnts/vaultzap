package export

import (
	"html/template"
	"io"

	"github.com/wallacepnts/vaultzap/internal/render"
	"github.com/wallacepnts/vaultzap/internal/store"
)

// Self-contained: embedded CSS, no CDN, no external file. print=true swaps the stylesheet
// for one that survives a printer, and the body goes through the same render.Format the
// live conversation uses.
type htmlWriter struct {
	lang  render.Locale
	print bool
}

func (e htmlWriter) header(w io.Writer, chat store.Chat) error {
	style := screenStyle
	if e.print {
		style = printStyle
	}
	// Style needs the template.CSS type, or html/template replaces the <style> content with
	// the "ZgotmplZ" placeholder — silently, with no error anywhere.
	return htmlHeaderTemplate.Execute(w, struct {
		Lang  string
		Name  string
		Style template.CSS
	}{Lang: string(e.lang), Name: chat.Name, Style: template.CSS(style)})
}

func (e htmlWriter) dateDivider(w io.Writer, isoDate string) error {
	return htmlDividerTemplate.Execute(w, render.LongDate(isoDate, e.lang))
}

func (htmlWriter) line(w io.Writer, l line) error {
	return htmlLineTemplate.Execute(w, struct {
		Sender string
		Own    bool
		Time   string
		Body   template.HTML
	}{
		Sender: l.Sender,
		Own:    l.Own,
		Time:   l.Time,
		Body:   render.Format(l.Body),
	})
}

func (e htmlWriter) footer(w io.Writer, _ store.Chat) error {
	return htmlFooterTemplate.Execute(w, exportedOn(e.lang))
}

var htmlFooterTemplate = template.Must(template.New("footer").Parse(
	"</div>\n<p class=\"footer\">{{.}}</p>\n</body>\n</html>\n"))

var htmlHeaderTemplate = template.Must(template.New("header").Parse(`<!DOCTYPE html>
<html lang="{{.Lang}}">
<head>
<meta charset="utf-8">
<title>{{.Name}}</title>
<style>{{.Style}}</style>
</head>
<body>
<h1>{{.Name}}</h1>
<div class="conversation">
`))

var htmlDividerTemplate = template.Must(template.New("divider").Parse(`<div class="date">{{.}}</div>
`))

var htmlLineTemplate = template.Must(template.New("line").Parse(
	`<div class="msg{{if .Own}} own{{end}}{{if not .Sender}} system{{end}}">
{{if .Sender}}<div class="sender">{{.Sender}}</div>{{end}}
<div class="body">{{.Body}}</div>
<div class="time">{{.Time}}</div>
</div>
`))

const screenStyle = `
body { background:#0b141a; color:#e9edef; font-family:-apple-system,"Segoe UI",Roboto,sans-serif; max-width:760px; margin:0 auto; padding:24px 16px 60px; }
h1 { font-size:20px; margin-bottom:4px; }
.conversation { display:flex; flex-direction:column; gap:2px; }
.date { align-self:center; background:#182229; color:#8696a0; font-size:12px; padding:5px 10px; border-radius:8px; margin:14px 0 8px; }
.msg { max-width:75%; background:#202c33; border-radius:8px; padding:6px 9px; align-self:flex-start; }
.msg.own { align-self:flex-end; background:#005c4b; }
.msg.system { align-self:center; background:transparent; color:#8696a0; font-size:12.5px; text-align:center; max-width:90%; }
.sender { font-size:12.5px; font-weight:600; color:#6ecf9a; margin-bottom:2px; }
.body { white-space:pre-wrap; word-wrap:break-word; font-size:14.2px; line-height:1.4; }
.body a { color:#53bdeb; }
.time { font-size:11px; color:#8696a0; text-align:right; margin-top:2px; }
.footer { text-align:center; color:#667781; font-size:12px; margin-top:24px; }
`

// The bubbles get a border on top of the fill, because browsers print without background
// colours by default; print-color-adjust brings the fill back for whoever enables
// "background graphics" in the print dialog.
const printStyle = `
* { print-color-adjust:exact; -webkit-print-color-adjust:exact; }
body { background:#fff; color:#111b21; font-family:-apple-system,"Segoe UI",Roboto,sans-serif; max-width:720px; margin:0 auto; padding:24px 12px 40px; }
h1 { font-size:18px; margin-bottom:4px; text-align:center; }
.conversation { display:flex; flex-direction:column; gap:4px; }
.date { align-self:center; color:#667781; font-size:10.5px; border:1px solid #e2e2e2; border-radius:7px; padding:3px 9px; margin:12px 0 6px; }
.msg { max-width:72%; align-self:flex-start; background:#fff; border:1px solid #d9dbdc; border-radius:7px; padding:5px 8px; break-inside:avoid; }
.msg.own { align-self:flex-end; background:#e9fbe4; border-color:#bfe3b4; }
.msg.system { align-self:center; max-width:85%; background:#f6f6f6; border-color:#e6e6e6; color:#667781; font-size:11px; text-align:center; font-style:italic; }
.sender { font-size:11.5px; font-weight:700; color:#0f7a5b; margin-bottom:1px; }
.body { white-space:pre-wrap; word-wrap:break-word; font-size:12.5px; line-height:1.35; }
.time { font-size:10px; color:#8696a0; text-align:right; }
.footer { text-align:center; color:#888; font-size:10.5px; margin-top:20px; }
@media print { body { padding:0; } }
@page { margin: 1.5cm; }
`
