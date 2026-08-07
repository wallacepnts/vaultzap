package locale

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
)

// The subset of .po this project uses: msgid/msgstr pairs, each a string on one or more
// contiguous quoted lines. No plurals (msgid_plural), no context (msgctxt).
func decodeCatalog(data []byte) (map[string]string, error) {
	catalog := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	const (
		modeNone = iota
		modeID
		modeStr
	)
	mode := modeNone
	var msgid, msgstr strings.Builder
	lineNumber := 0

	closeEntry := func() {
		if mode == modeStr {
			catalog[msgid.String()] = msgstr.String()
		}
		mode = modeNone
		msgid.Reset()
		msgstr.Reset()
	}

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
			continue
		case strings.HasPrefix(line, "msgid "):
			closeEntry()
			mode = modeID
			s, err := decodePOString(strings.TrimPrefix(line, "msgid "))
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			msgid.WriteString(s)
		case strings.HasPrefix(line, "msgstr "):
			if mode != modeID {
				return nil, fmt.Errorf("line %d: msgstr without a preceding msgid", lineNumber)
			}
			mode = modeStr
			s, err := decodePOString(strings.TrimPrefix(line, "msgstr "))
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			msgstr.WriteString(s)
		case strings.HasPrefix(line, `"`):
			// String continuation across several quoted lines.
			s, err := decodePOString(line)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			switch mode {
			case modeID:
				msgid.WriteString(s)
			case modeStr:
				msgstr.WriteString(s)
			default:
				return nil, fmt.Errorf("line %d: loose string outside msgid/msgstr", lineNumber)
			}
		default:
			return nil, fmt.Errorf("line %d: %q not recognized in a .po file", lineNumber, line)
		}
	}
	closeEntry()
	return catalog, scanner.Err()
}

// Strips the quotes and resolves the C-style escapes the format uses.
func decodePOString(field string) (string, error) {
	field = strings.TrimSpace(field)
	if len(field) < 2 || field[0] != '"' || field[len(field)-1] != '"' {
		return "", fmt.Errorf("malformed .po string: %q", field)
	}
	inner := field[1 : len(field)-1]

	var sb strings.Builder
	sb.Grow(len(inner))
	for i := 0; i < len(inner); i++ {
		c := inner[i]
		if c != '\\' || i+1 >= len(inner) {
			sb.WriteByte(c)
			continue
		}
		i++
		switch inner[i] {
		case 'n':
			sb.WriteByte('\n')
		case 't':
			sb.WriteByte('\t')
		case '"':
			sb.WriteByte('"')
		case '\\':
			sb.WriteByte('\\')
		default:
			sb.WriteByte('\\')
			sb.WriteByte(inner[i])
		}
	}
	return sb.String(), nil
}
