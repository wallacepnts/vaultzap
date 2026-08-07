package store

import (
	"context"
	"fmt"
	"time"
)

type DayWithMessages struct {
	Date    string // "2022-09-16"
	Total   int
	FirstID int64
}

// Counts each day's messages for a month ("2022-09"). A day with none doesn't come back.
func (s *Store) DaysWithMessages(ctx context.Context, chatID int64, month string) ([]DayWithMessages, error) {
	end, err := NextMonth(month)
	if err != nil {
		return nil, err
	}

	// The day's first message by display order (sent_at, seq), not MIN(id): reimporting an
	// old stretch inserts high ids in the middle of the conversation.
	rows, err := s.read.QueryContext(ctx, `
		SELECT substr(m.sent_at, 1, 10) AS day,
		       COUNT(*) AS total,
		       (SELECT p.id FROM messages p
		         WHERE p.chat_id = m.chat_id AND substr(p.sent_at, 1, 10) = substr(m.sent_at, 1, 10)
		         ORDER BY p.sent_at ASC, p.seq ASC LIMIT 1) AS first
		FROM messages m
		WHERE m.chat_id = ? AND m.sent_at >= ? AND m.sent_at < ?
		GROUP BY day
		ORDER BY day`, chatID, month+"-01", end+"-01")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var days []DayWithMessages
	for rows.Next() {
		var d DayWithMessages
		if err := rows.Scan(&d.Date, &d.Total, &d.FirstID); err != nil {
			return nil, err
		}
		days = append(days, d)
	}
	return days, rows.Err()
}

func NextMonth(month string) (string, error) {
	return shiftMonth(month, 1)
}

func PreviousMonth(month string) (string, error) {
	return shiftMonth(month, -1)
}

func shiftMonth(month string, step int) (string, error) {
	t, err := time.Parse("2006-01", month)
	if err != nil {
		return "", fmt.Errorf("mês inválido %q: %w", month, err)
	}
	return t.AddDate(0, step, 0).Format("2006-01"), nil
}
