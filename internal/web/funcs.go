package web

import (
	"fmt"
	"html/template"
	"path/filepath"
	"strings"
	"time"

	"github.com/wallacepnts/vaultzap/internal/locale"
	"github.com/wallacepnts/vaultzap/internal/render"
	"github.com/wallacepnts/vaultzap/internal/store"
)

var baseTemplateFuncs = template.FuncMap{
	"dict":                dict,
	"format":              render.Format,
	"formatSnippet":       render.FormatSnippet,
	"isOnlyEmoji":         render.IsOnlyEmoji,
	"initials":            initials,
	"avatarClass":         avatarClass,
	"dateTimeDisplay":     dateTimeDisplayFunc(render.LocalePTBR),
	"shortDate":           shortDateFunc(render.LocalePTBR),
	"fileSize":            fileSize,
	"fileExtension":       fileExtension,
	"percentage":          percentage,
	"messageActions":      messageActionsFor,
	"progressPlaceholder": progressPlaceholder,
	"asset":               asset,
	"pinnedLimit":         func() int { return store.MaxPinnedMessages },
	"t":                   translateFunc(render.LocalePTBR),
}

// Templates have no struct literal, and this fragment is rendered both inside a message
// and alone as a toggle's response.
func messageActionsFor(chatID, messageID int64, favorite, pinned bool, shortTime, wrapClass string) MessageActionsData {
	return MessageActionsData{
		ChatID: chatID, MessageID: messageID,
		Favorite: favorite, Pinned: pinned,
		ShortTime: shortTime, WrapClass: wrapClass,
	}
}

func localeTemplateFuncs(lang render.Locale) template.FuncMap {
	return template.FuncMap{
		"t":               translateFunc(lang),
		"dateTimeDisplay": dateTimeDisplayFunc(lang),
		"shortDate":       shortDateFunc(lang),
	}
}

func translateFunc(lang render.Locale) func(string, ...any) string {
	return func(key string, args ...any) string {
		return locale.T(lang, key, args...)
	}
}

// An invalid timestamp returns an empty string.
func shortDateFunc(lang render.Locale) func(string) string {
	return func(sentAt string) string {
		t, err := time.Parse("2006-01-02 15:04:05", sentAt)
		if err != nil {
			return ""
		}
		return render.ShortDate(t, lang)
	}
}

// Zero or negative returns an empty string.
func fileSize(bytes int64) string {
	if bytes <= 0 {
		return ""
	}
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.0f %cB", float64(bytes)/float64(div), "KMGT"[exp])
}

func fileExtension(name string) string {
	ext := strings.TrimPrefix(filepath.Ext(name), ".")
	if ext == "" {
		return "ARQ"
	}
	return strings.ToUpper(ext)
}

func percentage(v float64) string {
	return fmt.Sprintf("%.0f%%", v*100)
}

// The only way to hand more than one value to a {{template}} call. An odd argument count
// is a template bug, so it fails loudly.
func dict(pairs ...any) (map[string]any, error) {
	if len(pairs)%2 != 0 {
		return nil, fmt.Errorf("dict: número ímpar de argumentos (%d)", len(pairs))
	}
	m := make(map[string]any, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		key, ok := pairs[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict: chave %v não é string", pairs[i])
		}
		m[key] = pairs[i+1]
	}
	return m, nil
}

// The bar's first frame, or the panel shows an empty gap until the first poll answers.
func progressPlaceholder(file string) ProgressView {
	return ProgressView{File: file, Percent: 0}
}

// The build fingerprint, so new HTML never pairs with the previous binary's cached CSS.
func asset(path string) string {
	return "/static/" + path + "?v=" + assetVersion
}
