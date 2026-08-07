// Package locale loads the UI's fixed text from one gettext .po file per language. msgid
// is the key used in templates ("chats.archive"), not the source string. Date and time
// formatting lives in internal/render instead.
//
// loadCatalogs is build-tag-split: disk in development, embed.FS in production.
package locale

import (
	"fmt"

	"github.com/wallacepnts/vaultzap/internal/render"
)

// A key missing in the language falls back to pt-BR; missing there too, it returns the key
// itself — never empty, never a panic.
func T(lang render.Locale, key string, args ...any) string {
	catalogs, err := loadCatalogs()
	if err != nil {
		return key
	}
	text, ok := catalogs[lang.Normalize()][key]
	if !ok {
		text, ok = catalogs[render.LocalePTBR][key]
	}
	if !ok || text == "" {
		return key
	}
	if len(args) == 0 {
		return text
	}
	return fmt.Sprintf(text, args...)
}
