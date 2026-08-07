package parser

import "strings"

// normalize strips invisible marks common in exports (mostly iOS), turns non-breaking
// spaces into regular ones, and unifies line breaks. Skipping this is the number one
// cause of "the parser doesn't recognize anything in the iPhone file".
func normalize(input []byte) string {
	var sb strings.Builder
	sb.Grow(len(input))

	for _, r := range string(input) {
		switch {
		case r == 0xFEFF: // BOM
			continue
		case r == 0x200E || r == 0x200F: // directionality marks
			continue
		case r >= 0x202A && r <= 0x202E: // bidi embedding/override
			continue
		case r >= 0x2066 && r <= 0x2069: // bidi isolation
			continue
		// U+202F (narrow no-break space) is what current WhatsApp builds put before AM/PM in
		// 12-hour exports; Go's regexp \s is ASCII-only, so leaving it makes the whole file fail
		// dialect detection, not just that line.
		case r == 0x00A0 || r == 0x2007 || r == 0x2009 || r == 0x202F: // NBSP, figure/thin/narrow spaces
			sb.WriteRune(' ')
		default:
			sb.WriteRune(r)
		}
	}

	text := sb.String()
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return text
}
