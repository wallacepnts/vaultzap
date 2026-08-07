package parser

import (
	"fmt"
	"strconv"
	"strings"
)

// Dialect identifies the export's header format.
type Dialect int

const (
	DialectUnknown Dialect = iota
	DialectAndroid
	DialectIOS
)

func (d Dialect) String() string {
	switch d {
	case DialectAndroid:
		return "android"
	case DialectIOS:
		return "ios"
	default:
		return "unknown"
	}
}

const linesForDetection = 50

// Tests the file's first lines against both known header patterns, picks the winner.
func detectDialect(lines []string) (Dialect, error) {
	iosCount, androidCount, tested := 0, 0, 0

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if iosHeaderRe.MatchString(line) {
			iosCount++
		}
		if androidHeaderRe.MatchString(line) {
			androidCount++
		}
		tested++
		if tested >= linesForDetection {
			break
		}
	}

	switch {
	case iosCount == 0 && androidCount == 0:
		return DialectUnknown, fmt.Errorf("could not detect the file's format (neither iOS nor Android)")
	case iosCount >= androidCount:
		return DialectIOS, nil
	default:
		return DialectAndroid, nil
	}
}

// dateComponents holds the three numbers of a date exactly as they appear in the file.
// Which is day, month and year is decided later, once the order is inferred for the
// whole export (see inferOrder).
type dateComponents struct {
	c1, c2, c3 int
}

// yearFirst reports whether the first component can only be a year, as in the
// year-first exports (sv/lt/hu/ja/zh): no day or month goes past 31.
func (c dateComponents) yearFirst() bool { return c.c1 > 31 }

type parsedTime struct {
	hour, minute, second int
}

// Rejects dates that matched structurally but are semantically impossible.
func plausibleComponents(c dateComponents) bool {
	// Year-first: the remaining two are month and day, in that order.
	if c.yearFirst() {
		return c.c1 >= 1000 && c.c2 >= 1 && c.c2 <= 12 && c.c3 >= 1 && c.c3 <= 31
	}
	if c.c1 < 1 || c.c2 < 1 || c.c2 > 31 {
		return false
	}
	if c.c1 > 12 && c.c2 > 12 {
		return false
	}
	return true
}

func parseDateComponents(s string) (dateComponents, bool) {
	m := dateComponentsRe.FindStringSubmatch(s)
	if m == nil {
		return dateComponents{}, false
	}
	c1, _ := strconv.Atoi(m[1])
	c2, _ := strconv.Atoi(m[2])
	c3, _ := strconv.Atoi(m[3])
	c := dateComponents{c1: c1, c2: c2, c3: c3}
	return c, plausibleComponents(c)
}

func parseTime(s string) (parsedTime, bool) {
	m := timeRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return parsedTime{}, false
	}
	hour, _ := strconv.Atoi(m[1])
	minute, _ := strconv.Atoi(m[2])
	second := 0
	if m[3] != "" {
		second, _ = strconv.Atoi(m[3])
	}
	if m[4] != "" {
		suffix := strings.ToUpper(strings.ReplaceAll(m[4], ".", ""))
		suffix = strings.TrimSpace(suffix)
		switch suffix {
		case "PM":
			if hour != 12 {
				hour += 12
			}
		case "AM":
			if hour == 12 {
				hour = 0
			}
		}
	}
	if hour > 23 || minute > 59 || second > 59 {
		return parsedTime{}, false
	}
	return parsedTime{hour: hour, minute: minute, second: second}, true
}

type headerResult int

const (
	headerNone headerResult = iota
	headerValid
	headerInvalid
)

type matchedHeader struct {
	date dateComponents
	time parsedTime
	rest string
}

// Three outcomes: not a header (a continuation), a valid header, or a structurally
// recognizable header with an impossible date/time.
func matchHeader(d Dialect, line string) (matchedHeader, headerResult) {
	re := androidHeaderRe
	if d == DialectIOS {
		re = iosHeaderRe
	}

	m := re.FindStringSubmatch(line)
	if m == nil {
		return matchedHeader{}, headerNone
	}

	date, dateOk := parseDateComponents(m[1])
	time, timeOk := parseTime(m[2])
	if !dateOk || !timeOk {
		return matchedHeader{rest: line}, headerInvalid
	}

	return matchedHeader{date: date, time: time, rest: m[3]}, headerValid
}
