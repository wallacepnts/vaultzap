package render

import (
	"fmt"
	"time"
)

// Locale picks the month and weekday names, "today/yesterday", the date order and
// separator, and the 12h/24h clock.
type Locale string

const (
	LocalePTBR Locale = "pt-BR"
	LocalePT   Locale = "pt"
	LocaleEN   Locale = "en"
	LocaleES   Locale = "es"
	LocaleIT   Locale = "it"
	LocaleFR   Locale = "fr"
	LocaleDE   Locale = "de"
	LocaleNL   Locale = "nl"
)

var Locales = []Locale{LocalePTBR, LocalePT, LocaleEN, LocaleES, LocaleIT, LocaleFR, LocaleDE, LocaleNL}

var localeLabels = map[Locale]string{
	LocalePTBR: "Português (Brasil)",
	LocalePT:   "Português",
	LocaleEN:   "English",
	LocaleES:   "Español",
	LocaleIT:   "Italiano",
	LocaleFR:   "Français",
	LocaleDE:   "Deutsch",
	LocaleNL:   "Nederlands",
}

// months/weekdays: index 0 unused (time.Month is 1-based); weekdays start on Sunday.
var monthNames = map[Locale][13]string{
	LocalePTBR: {"", "janeiro", "fevereiro", "março", "abril", "maio", "junho",
		"julho", "agosto", "setembro", "outubro", "novembro", "dezembro"},
	LocalePT: {"", "janeiro", "fevereiro", "março", "abril", "maio", "junho",
		"julho", "agosto", "setembro", "outubro", "novembro", "dezembro"},
	LocaleEN: {"", "January", "February", "March", "April", "May", "June",
		"July", "August", "September", "October", "November", "December"},
	LocaleES: {"", "enero", "febrero", "marzo", "abril", "mayo", "junio",
		"julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"},
	LocaleIT: {"", "gennaio", "febbraio", "marzo", "aprile", "maggio", "giugno",
		"luglio", "agosto", "settembre", "ottobre", "novembre", "dicembre"},
	LocaleFR: {"", "janvier", "février", "mars", "avril", "mai", "juin",
		"juillet", "août", "septembre", "octobre", "novembre", "décembre"},
	LocaleDE: {"", "Januar", "Februar", "März", "April", "Mai", "Juni",
		"Juli", "August", "September", "Oktober", "November", "Dezember"},
	LocaleNL: {"", "januari", "februari", "maart", "april", "mei", "juni",
		"juli", "augustus", "september", "oktober", "november", "december"},
}

var weekdayNames = map[Locale][7]string{
	LocalePTBR: {"domingo", "segunda-feira", "terça-feira", "quarta-feira", "quinta-feira", "sexta-feira", "sábado"},
	LocalePT:   {"domingo", "segunda-feira", "terça-feira", "quarta-feira", "quinta-feira", "sexta-feira", "sábado"},
	LocaleEN:   {"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"},
	LocaleES:   {"domingo", "lunes", "martes", "miércoles", "jueves", "viernes", "sábado"},
	LocaleIT:   {"domenica", "lunedì", "martedì", "mercoledì", "giovedì", "venerdì", "sabato"},
	LocaleFR:   {"dimanche", "lundi", "mardi", "mercredi", "jeudi", "vendredi", "samedi"},
	LocaleDE:   {"Sonntag", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag"},
	LocaleNL:   {"zondag", "maandag", "dinsdag", "woensdag", "donderdag", "vrijdag", "zaterdag"},
}

var weekdayAbbrevs = map[Locale][7]string{
	LocalePTBR: {"dom", "seg", "ter", "qua", "qui", "sex", "sáb"},
	LocalePT:   {"dom", "seg", "ter", "qua", "qui", "sex", "sáb"},
	LocaleEN:   {"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"},
	LocaleES:   {"dom", "lun", "mar", "mié", "jue", "vie", "sáb"},
	LocaleIT:   {"dom", "lun", "mar", "mer", "gio", "ven", "sab"},
	LocaleFR:   {"dim", "lun", "mar", "mer", "jeu", "ven", "sam"},
	LocaleDE:   {"So", "Mo", "Di", "Mi", "Do", "Fr", "Sa"},
	LocaleNL:   {"zo", "ma", "di", "wo", "do", "vr", "za"},
}

// longDateFormat places day/month/year (Sprintf args 1/2/3) in each language's
// spelled-out order: "26 de julho de 2026", "26 luglio 2026", "26. Juli 2026",
// "July 26, 2026".
var longDateFormat = map[Locale]string{
	LocalePTBR: "%[1]d de %[2]s de %[3]d",
	LocalePT:   "%[1]d de %[2]s de %[3]d",
	LocaleEN:   "%[2]s %[1]d, %[3]d",
	LocaleES:   "%[1]d de %[2]s de %[3]d",
	LocaleIT:   "%[1]d %[2]s %[3]d",
	LocaleFR:   "%[1]d %[2]s %[3]d",
	LocaleDE:   "%[1]d. %[2]s %[3]d",
	LocaleNL:   "%[1]d %[2]s %[3]d",
}

var longMonthFormat = map[Locale]string{
	LocalePTBR: "%[1]s de %[2]d",
	LocalePT:   "%[1]s de %[2]d",
	LocaleEN:   "%[1]s %[2]d",
	LocaleES:   "%[1]s de %[2]d",
	LocaleIT:   "%[1]s %[2]d",
	LocaleFR:   "%[1]s %[2]d",
	LocaleDE:   "%[1]s %[2]d",
	LocaleNL:   "%[1]s %[2]d",
}

var todayLabels = map[Locale]string{
	LocalePTBR: "HOJE",
	LocalePT:   "HOJE",
	LocaleEN:   "TODAY",
	LocaleES:   "HOY",
	LocaleIT:   "OGGI",
	LocaleFR:   "AUJOURD'HUI",
	LocaleDE:   "HEUTE",
	LocaleNL:   "VANDAAG",
}

var yesterdayLabels = map[Locale]string{
	LocalePTBR: "ONTEM",
	LocalePT:   "ONTEM",
	LocaleEN:   "YESTERDAY",
	LocaleES:   "AYER",
	LocaleIT:   "IERI",
	LocaleFR:   "HIER",
	LocaleDE:   "GESTERN",
	LocaleNL:   "GISTEREN",
}

// A Go time layout: DD/MM/YYYY everywhere except en (MM/DD/YYYY) and de (DD.MM.YYYY).
var shortDateLayout = map[Locale]string{
	LocalePTBR: "02/01/2006",
	LocalePT:   "02/01/2006",
	LocaleEN:   "01/02/2006",
	LocaleES:   "02/01/2006",
	LocaleIT:   "02/01/2006",
	LocaleFR:   "02/01/2006",
	LocaleDE:   "02.01.2006",
	LocaleNL:   "02-01-2006",
}

// 24h everywhere except en (12h with AM/PM).
var clockLayout = map[Locale]string{
	LocalePTBR: "15:04",
	LocalePT:   "15:04",
	LocaleEN:   "3:04 PM",
	LocaleES:   "15:04",
	LocaleIT:   "15:04",
	LocaleFR:   "15:04",
	LocaleDE:   "15:04",
	LocaleNL:   "15:04",
}

func (l Locale) Label() string {
	return localeLabels[l.Normalize()]
}

// Normalize returns a valid locale — empty or unknown falls back to pt-BR.
func (l Locale) Normalize() Locale {
	if _, ok := localeLabels[l]; ok {
		return l
	}
	return LocalePTBR
}

func (l Locale) months() [13]string {
	return monthNames[l.Normalize()]
}

// Spells out an ISO date, with or without time. Invalid input comes back unchanged rather
// than inventing a date.
func LongDate(date string, locale Locale) string {
	if len(date) < 10 {
		return date
	}
	t, err := time.Parse("2006-01-02", date[:10])
	if err != nil {
		return date
	}
	locale = locale.Normalize()
	m := locale.months()
	return fmt.Sprintf(longDateFormat[locale], t.Day(), m[t.Month()], t.Year())
}

// Spells out "2022-09". Same invalid-input policy as LongDate.
func LongMonth(month string, locale Locale) string {
	t, err := time.Parse("2006-01", month)
	if err != nil {
		return month
	}
	locale = locale.Normalize()
	m := locale.months()
	return fmt.Sprintf(longMonthFormat[locale], m[t.Month()], t.Year())
}

func MonthNames(locale Locale) []string {
	m := locale.months()
	return m[1:]
}

// Used by the date divider for messages 2 to 6 days old.
func WeekdayName(day time.Weekday, locale Locale) string {
	return weekdayNames[locale.Normalize()][day]
}

func WeekdayAbbreviations(locale Locale) []string {
	abbrevs := weekdayAbbrevs[locale.Normalize()]
	return abbrevs[:]
}

func TodayLabel(locale Locale) string {
	return todayLabels[locale.Normalize()]
}

func YesterdayLabel(locale Locale) string {
	return yesterdayLabels[locale.Normalize()]
}

func ShortDate(t time.Time, locale Locale) string {
	return t.Format(shortDateLayout[locale.Normalize()])
}

func ClockTime(t time.Time, locale Locale) string {
	return t.Format(clockLayout[locale.Normalize()])
}
