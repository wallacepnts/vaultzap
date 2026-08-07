package parser

import "fmt"

// DateOrder is the day/month order assumed when building the final timestamp.
type DateOrder string

const (
	OrderDMY DateOrder = "DMY"
	OrderMDY DateOrder = "MDY"
	// OrderYMD covers year-first exports (sv/lt/hu/ja/zh). Never a fallback: it is only used
	// when a component makes the reading unambiguous.
	OrderYMD DateOrder = "YMD"
)

func formatDate(d dateComponents, h parsedTime, order DateOrder) string {
	var year, month, day int
	switch order {
	case OrderYMD:
		year, month, day = d.c1, d.c2, d.c3
	case OrderMDY:
		year, month, day = d.c3, d.c1, d.c2
	default: // OrderDMY
		year, month, day = d.c3, d.c2, d.c1
	}
	if year < 100 {
		year += 2000
	}
	return fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d", year, month, day, h.hour, h.minute, h.second)
}

// inferOrder resolves the DD/MM vs MM/DD ambiguity in four steps:
//  1. a first component > 31 can only be a year, so the file is year-first;
//  2. a component > 12 in any message decides on its own;
//  3. tie: try both orders, keep the one with a non-decreasing timestamp sequence;
//  4. still tied: use the configured default.
func inferOrder(messages []rawMessage, defaultOrder DateOrder) DateOrder {
	for _, m := range messages {
		if m.noDate {
			continue
		}
		if m.date.yearFirst() {
			return OrderYMD
		}
	}
	for _, m := range messages {
		if m.noDate {
			continue
		}
		if m.date.c1 > 12 {
			return OrderDMY
		}
	}
	for _, m := range messages {
		if m.noDate {
			continue
		}
		if m.date.c2 > 12 {
			return OrderMDY
		}
	}

	dmyOk := nonDecreasingSequence(messages, OrderDMY)
	mdyOk := nonDecreasingSequence(messages, OrderMDY)
	switch {
	case dmyOk && !mdyOk:
		return OrderDMY
	case mdyOk && !dmyOk:
		return OrderMDY
	default:
		return defaultOrder
	}
}

func nonDecreasingSequence(messages []rawMessage, order DateOrder) bool {
	previous := ""
	for _, m := range messages {
		if m.noDate {
			continue
		}
		current := formatDate(m.date, m.time, order)
		if current < previous {
			return false
		}
		previous = current
	}
	return true
}
