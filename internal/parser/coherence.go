package parser

import (
	"regexp"
	"strings"
	"time"
)

// Check is one thing about the export that does not look like what WhatsApp writes. None of
// these proves tampering and their absence proves nothing: an export carries no signature,
// so this is triage for careless edits, not authentication. Codes are symbolic because the
// UI translates them; the parser has no notion of locale.
type Check struct {
	Code  string `json:"code"`
	Count int    `json:"count"`
	Line  int    `json:"line,omitempty"` // first occurrence, 1-based
}

const (
	// CheckMarksMissing: the file does use the invisible marks WhatsApp puts before
	// attachment and system lines, but some of those lines lack them. An editor drops them.
	CheckMarksMissing = "marks_missing"
	// CheckOutOfOrder: a message dated well before the one above it. See orderTolerance.
	CheckOutOfOrder = "out_of_order"
	// CheckMediaNaming: an attachment whose name does not follow WhatsApp's convention.
	CheckMediaNaming = "media_naming"
	// CheckMediaDate: the date inside an attachment's name is not the day of the message.
	CheckMediaDate = "media_date"
	// CheckMediaMissing: files cited by the .txt that did not come in the unit, while others
	// did. Filled by the ingest side, which is what sees the media folder.
	CheckMediaMissing = "media_missing"
)

// Measured on a real 7.5k-message export: seven inversions, all between 1 and 20 seconds —
// WhatsApp lists messages in display order, and one that arrived late keeps its own clock.
// Flagging those puts eight alarms on an untouched file, which is how a check teaches
// people to ignore it. An edit that changes what was said moves things by hours or days.
const orderTolerance = 5 * time.Minute

// Documents and contact cards keep the name they arrived with, so their names say nothing.
var renamedByWhatsApp = map[string]bool{
	"image": true, "video": true, "audio": true, "voice": true, "sticker": true, "gif": true,
}

// Android: IMG-20260726-WA0001.jpg, PTT-..., VID-, AUD-, STK-, DOC-.
var reAndroidMedia = regexp.MustCompile(`^(?:IMG|VID|AUD|PTT|STK|DOC|GIF)-(\d{8})-WA\d{4,}\.\w+$`)

// iOS: 00000042-PHOTO-2026-07-26-14-32-15.jpg, and the sticker/audio variants.
var reIOSMedia = regexp.MustCompile(`^\d{8}-(?:PHOTO|VIDEO|AUDIO|STICKER|GIF|DOCUMENT)-(\d{4}-\d{2}-\d{2})-\d{2}-\d{2}-\d{2}\.\w+$`)

// Whether each raw line carried one of the invisible marks normalize() strips. It applies
// the same newline rules, so the indexes of the two line slices line up.
func marksByLine(input []byte) []bool {
	var marks []bool
	current := false
	skipLF := false

	for _, r := range string(input) {
		switch {
		case r == '\r':
			marks = append(marks, current)
			current = false
			skipLF = true
		case r == '\n':
			if skipLF {
				skipLF = false
				continue
			}
			marks = append(marks, current)
			current = false
		default:
			skipLF = false
			if r == 0x200E || r == 0x200F || (r >= 0x202A && r <= 0x202E) || (r >= 0x2066 && r <= 0x2069) {
				current = true
			}
		}
	}
	marks = append(marks, current)
	return marks
}

func coherenceChecks(messages []Message, lines []string, hadMark []bool) []Check {
	var checks []Check

	if c, ok := checkMarks(lines, hadMark); ok {
		checks = append(checks, c)
	}
	if c, ok := checkOrder(messages); ok {
		checks = append(checks, c)
	}
	naming, date := checkMediaNames(messages)
	if naming.Count > 0 {
		checks = append(checks, naming)
	}
	if date.Count > 0 {
		checks = append(checks, date)
	}
	return checks
}

// Only speaks when the file proves it uses the marks: an Android export has none at all,
// and calling that suspicious would flag every Android export ever made.
func checkMarks(lines []string, hadMark []bool) (Check, bool) {
	if len(hadMark) < len(lines) {
		return Check{}, false
	}

	withMark, without, first := 0, 0, 0
	for i, line := range lines {
		if !structuralLine(line) {
			continue
		}
		if hadMark[i] {
			withMark++
			continue
		}
		without++
		if first == 0 {
			first = i + 1
		}
	}

	if withMark == 0 || without == 0 {
		return Check{}, false
	}
	return Check{Code: CheckMarksMissing, Count: without, Line: first}, true
}

// A line WhatsApp itself writes, which is what carries the invisible mark.
func structuralLine(line string) bool {
	// The header is still on the line here, so the test is on what follows the last "] " or
	// " - "; falling back to the whole line costs nothing when neither is present.
	body := line
	if i := strings.LastIndex(body, "] "); i >= 0 {
		body = body[i+2:]
	} else if i := strings.Index(body, " - "); i >= 0 {
		body = body[i+3:]
	}
	if i := strings.Index(body, ": "); i >= 0 {
		body = body[i+2:]
	}
	return iosAttachmentRe.MatchString(body) ||
		androidAttachmentRe.MatchString(body) ||
		matchesAny(omittedMediaPatterns, body) ||
		matchesAny(deletedPatterns, body) ||
		matchesAny(generalSystemPatterns, body)
}

func checkOrder(messages []Message) (Check, bool) {
	count, first := 0, 0
	var previous time.Time
	for _, m := range messages {
		at, err := time.Parse("2006-01-02 15:04:05", m.SentAt)
		if err != nil {
			continue
		}
		if !previous.IsZero() && previous.Sub(at) > orderTolerance {
			count++
			if first == 0 {
				first = m.Seq
			}
		}
		previous = at
	}
	if count == 0 {
		return Check{}, false
	}
	return Check{Code: CheckOutOfOrder, Count: count, Line: first}, true
}

func checkMediaNames(messages []Message) (naming, date Check) {
	naming.Code, date.Code = CheckMediaNaming, CheckMediaDate

	for _, m := range messages {
		if m.AttachmentName == "" || !renamedByWhatsApp[m.MediaKind] {
			continue
		}
		day, known := mediaNameDay(m.AttachmentName)
		if !known {
			naming.Count++
			if naming.Line == 0 {
				naming.Line = m.Seq
			}
			continue
		}
		if day != "" && len(m.SentAt) >= 10 && day != m.SentAt[:10] {
			date.Count++
			if date.Line == 0 {
				date.Line = m.Seq
			}
		}
	}
	return naming, date
}

// The day encoded in the name, and whether the name follows a convention at all. A name
// that matches but carries no date returns ("", true).
func mediaNameDay(name string) (string, bool) {
	if m := reAndroidMedia.FindStringSubmatch(name); m != nil {
		d := m[1]
		return d[:4] + "-" + d[4:6] + "-" + d[6:], true
	}
	if m := reIOSMedia.FindStringSubmatch(name); m != nil {
		return m[1], true
	}
	return "", false
}
