// Package render converts WhatsApp message text (the app's simplified markdown) into
// safe HTML. The text is always escaped before any markup is applied, never the reverse.
package render

import (
	"html"
	"html/template"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	reMono   = regexp.MustCompile("(?s)```(.+?)```")
	reBold   = regexp.MustCompile(`\*(\S(?:[^*\n]*\S)?)\*`)
	reItalic = regexp.MustCompile(`_(\S(?:[^_\n]*\S)?)_`)
	reStrike = regexp.MustCompile(`~(\S(?:[^~\n]*\S)?)~`)
	// \x00 is excluded so a URL glued to a protected block doesn't swallow the placeholder
	// into the href.
	reURL         = regexp.MustCompile("https?://[^\\s<\x00]+")
	rePlaceholder = regexp.MustCompile("\x00(\\d+)\x00")
	reEmojiInline = regexp.MustCompile(`[\x{1F300}-\x{1FAFF}\x{2600}-\x{27BF}\x{1F1E6}-\x{1F1FF}\x{2B00}-\x{2BFF}\x{FE0F}\x{200D}]+`)
)

// Format applies WhatsApp markup (*bold*, _italic_, ~strike~, ```mono```) and URL
// autolinking on top of already-escaped text. Line breaks don't become <br>: the bubble
// is rendered with white-space:pre-wrap, which preserves "\n" without touching the HTML.
func Format(text string) template.HTML {
	// NUL is the sentinel used below to protect mono blocks. A body carrying one of its own
	// would forge a placeholder resolving to an index no block produced — a panic on the
	// whole conversation. It is a control byte with nothing to display, so dropping it is safe.
	escaped := html.EscapeString(strings.ReplaceAll(text, "\x00", ""))

	var blocks []string
	protect := func(html string) string {
		blocks = append(blocks, html)
		return "\x00" + strconv.Itoa(len(blocks)-1) + "\x00"
	}

	protected := reMono.ReplaceAllStringFunc(escaped, func(m string) string {
		return protect("<code>" + reMono.FindStringSubmatch(m)[1] + "</code>")
	})

	// Autolink runs BEFORE the markup regexes, and its output is protected. URLs routinely
	// carry "_", "*" and "~" (query strings, Wikipedia article names, S3 keys); marking
	// first injects tags into the middle of the address, and the link is then truncated at
	// the injected "<", because reURL stops there.
	protected = autolink(protected, protect)

	protected = reBold.ReplaceAllString(protected, `<b>$1</b>`)
	protected = reItalic.ReplaceAllString(protected, `<i>$1</i>`)
	protected = reStrike.ReplaceAllString(protected, `<s>$1</s>`)

	protected = reEmojiInline.ReplaceAllStringFunc(protected, func(m string) string {
		return `<span class="emoji-inline">` + m + `</span>`
	})

	result := rePlaceholder.ReplaceAllStringFunc(protected, func(m string) string {
		idx, _ := strconv.Atoi(rePlaceholder.FindStringSubmatch(m)[1])
		if idx < 0 || idx >= len(blocks) {
			return "" // unreachable once NUL is stripped above; belt and braces
		}
		return blocks[idx]
	})

	return template.HTML(result)
}

// Trailing sentence punctuation stays outside the link, and each link goes through protect
// so the markup regexes can't reach inside it.
func autolink(text string, protect func(string) string) string {
	return reURL.ReplaceAllStringFunc(text, func(u string) string {
		clean, tail := trimTrailingPunctuation(u)
		return protect(`<a href="`+clean+`" target="_blank" rel="noopener noreferrer">`+clean+`</a>`) + tail
	})
}

func trimTrailingPunctuation(u string) (string, string) {
	tail := ""
	for len(u) > 0 && strings.ContainsRune(".,;:!?)]}", rune(u[len(u)-1])) {
		tail = string(u[len(u)-1]) + tail
		u = u[:len(u)-1]
	}
	return u, tail
}

// The http(s) URLs in a raw text, in order. Same regex and cleanup as autolink.
func ExtractURLs(text string) []string {
	var urls []string
	for _, raw := range reURL.FindAllString(text, -1) {
		if clean, _ := trimTrailingPunctuation(raw); clean != "" {
			urls = append(urls, clean)
		}
	}
	return urls
}

// Control bytes used as highlight markers in FTS5 snippets, replaced with <mark> below.
const (
	snippetMarkStart = "\x01"
	snippetMarkEnd   = "\x02"
)

// FormatSnippet escapes an FTS5 snippet and turns its highlight marks into <mark>.
func FormatSnippet(snippet string) template.HTML {
	parts := strings.Split(snippet, snippetMarkStart)
	var sb strings.Builder
	sb.WriteString(html.EscapeString(parts[0]))
	for _, part := range parts[1:] {
		highlight, rest, _ := strings.Cut(part, snippetMarkEnd)
		sb.WriteString("<mark>")
		sb.WriteString(html.EscapeString(highlight))
		sb.WriteString("</mark>")
		sb.WriteString(html.EscapeString(rest))
	}
	return template.HTML(sb.String())
}

func IsOnlyEmoji(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return false
	}
	hasEmoji := false
	for _, r := range t {
		switch {
		case unicode.IsSpace(r):
			continue
		case r == 0xFE0F || r == 0x200D: // variation selector, zero-width joiner
			continue
		case reEmojiInline.MatchString(string(r)):
			hasEmoji = true
		default:
			return false
		}
	}
	return hasEmoji
}
