package web

import (
	"testing"

	"github.com/wallacepnts/vaultzap/internal/render"
)

func TestTimeOrShortDate(t *testing.T) {
	hoje := "2026-07-27"
	cases := []struct {
		sentAt, want string
		locale       render.Locale
	}{
		{"2026-07-27 14:32:00", "14:32", render.LocalePTBR},
		{"2026-07-26 09:00:00", "26/07/2026", render.LocalePTBR},
		{"", "", render.LocalePTBR},
		{"2026-07-27 14:32:00", "2:32 PM", render.LocaleEN},
		{"2026-07-26 09:00:00", "07/26/2026", render.LocaleEN},
	}
	for _, c := range cases {
		if got := timeOrShortDate(c.sentAt, hoje, c.locale); got != c.want {
			t.Errorf("timeOrShortDate(%q, %q) = %q, esperado %q", c.sentAt, c.locale, got, c.want)
		}
	}
}
