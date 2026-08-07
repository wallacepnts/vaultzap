package web

import (
	"fmt"
	"strconv"
	"time"

	"github.com/wallacepnts/vaultzap/internal/render"
	"github.com/wallacepnts/vaultzap/internal/store"
)

// CalendarDay is one grid cell. Day == 0 is the padding before the first and after the
// last day of the month.
type CalendarDay struct {
	Day     int
	Total   int
	FirstID int64
	Weight  int // 0 to 4: color intensity, by message volume
}

type MonthOption struct {
	Number string // "01".."12"
	Name   string
}

// DayListItem is one row of the day list below the grid, with the id of that day's first
// message — the same target as the matching grid cell.
type DayListItem struct {
	Label   string // "sexta-feira, 2 de setembro de 2022"
	Total   int
	FirstID int64
	Weight  int // 0 to 4, same scale as the grid cell's color intensity
}

type CalendarData struct {
	Chat             store.Chat
	Title            string // "setembro de 2022"
	Month            string // "2022-09"
	CurrentYear      string // "2022"
	CurrentMonth     string // "09"
	CurrentMonthName string // "setembro", for the month picker's closed button
	Years            []string
	Months           []MonthOption
	Weekdays         []string // grid header ("dom".."sáb" / "Sun".."Sat")
	Previous         string   // "" when there's no earlier month with a message
	Next             string
	Latest           string // month of the conversation's last message ("Recent" button)
	Weeks            [][]CalendarDay
	Days             []DayListItem
}

// Navigation between months stops at the conversation's first and last message.
func buildCalendar(chat store.Chat, month string, days []store.DayWithMessages, lang render.Locale) CalendarData {
	byDay := make(map[int]store.DayWithMessages, len(days))
	max := 0
	for _, d := range days {
		t, err := time.Parse("2006-01-02", d.Date)
		if err != nil {
			continue
		}
		byDay[t.Day()] = d
		if d.Total > max {
			max = d.Total
		}
	}

	dayItems := make([]DayListItem, 0, len(days))
	for _, d := range days {
		t, err := time.Parse("2006-01-02", d.Date)
		if err != nil {
			continue
		}
		dayItems = append(dayItems, DayListItem{
			Label:   render.WeekdayName(t.Weekday(), lang) + ", " + render.LongDate(d.Date, lang),
			Total:   d.Total,
			FirstID: d.FirstID,
			Weight:  dayWeight(d.Total, max),
		})
	}

	start, err := time.Parse("2006-01", month)
	if err != nil {
		start = time.Now()
	}
	totalDays := start.AddDate(0, 1, -1).Day()

	week := make([]CalendarDay, int(start.Weekday()))
	var weeks [][]CalendarDay
	for day := 1; day <= totalDays; day++ {
		cell := CalendarDay{Day: day}
		if d, ok := byDay[day]; ok {
			cell.Total, cell.FirstID, cell.Weight = d.Total, d.FirstID, dayWeight(d.Total, max)
		}
		week = append(week, cell)
		if len(week) == 7 {
			weeks = append(weeks, week)
			week = nil
		}
	}
	for len(week) > 0 && len(week) < 7 {
		week = append(week, CalendarDay{})
	}
	if len(week) == 7 {
		weeks = append(weeks, week)
	}

	data := CalendarData{
		Chat:             chat,
		Title:            render.LongMonth(month, lang),
		Month:            month,
		CurrentYear:      month[:4],
		CurrentMonth:     month[5:7],
		CurrentMonthName: render.MonthNames(lang)[start.Month()-1],
		Years:            yearsBetween(monthOf(chat.FirstMessageAt), monthOf(chat.LastMessageAt)),
		Months:           monthsOfYear(lang),
		Weekdays:         render.WeekdayAbbreviations(lang),
		Latest:           monthOf(chat.LastMessageAt),
		Weeks:            weeks,
		Days:             dayItems,
	}
	if previous, err := store.PreviousMonth(month); err == nil && previous >= monthOf(chat.FirstMessageAt) {
		data.Previous = previous
	}
	if next, err := store.NextMonth(month); err == nil && next <= data.Latest {
		data.Next = next
	}
	return data
}

func yearsBetween(firstMonth, lastMonth string) []string {
	if len(firstMonth) < 4 || len(lastMonth) < 4 {
		return nil
	}
	start, end := firstMonth[:4], lastMonth[:4]
	var years []string
	for year := start; year <= end; year = nextYear(year) {
		years = append(years, year)
	}
	return years
}

func nextYear(year string) string {
	n, err := strconv.Atoi(year)
	if err != nil {
		return year
	}
	return strconv.Itoa(n + 1)
}

func monthsOfYear(lang render.Locale) []MonthOption {
	names := render.MonthNames(lang)
	months := make([]MonthOption, len(names))
	for i, name := range names {
		months[i] = MonthOption{Number: fmt.Sprintf("%02d", i+1), Name: name}
	}
	return months
}

// Four bands relative to the month's busiest day: an absolute scale would paint a chat
// with 5 messages a day and one with 600 exactly the same.
func dayWeight(total, max int) int {
	if total == 0 || max == 0 {
		return 0
	}
	band := (total*4 + max - 1) / max // ceil(total/max*4)
	if band > 4 {
		band = 4
	}
	return band
}

func monthOf(timestamp string) string {
	if len(timestamp) < 7 {
		return ""
	}
	return timestamp[:7]
}
