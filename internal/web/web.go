// Package web holds the reading UI's HTTP handlers, templates and static assets.
package web

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wallacepnts/vaultzap/internal/config"
	"github.com/wallacepnts/vaultzap/internal/ingest"
	"github.com/wallacepnts/vaultzap/internal/store"
)

// Scanner is what the "scan now" button needs from the watched folder's scanner.
type Scanner interface {
	Scan(ctx context.Context)
	ImportInto(ctx context.Context, path, record string, chatID int64) (ingest.Report, error)
}

type Handler struct {
	store   *store.Store
	cfg     config.Config
	tz      *time.Location
	scanner Scanner
	version string
}

// The configured time zone is used to decide "today/yesterday" in date dividers.
func NewHandler(s *store.Store, cfg config.Config, scanner Scanner, version string) *Handler {
	tz, err := time.LoadLocation(cfg.TimeZone)
	if err != nil {
		tz = time.UTC
	}
	return &Handler{store: s, cfg: cfg, tz: tz, scanner: scanner, version: version}
}

// Routes builds the reading UI's mux, with Basic Auth applied when configured.
// /healthz is registered separately in main.go, outside the auth.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.home)
	mux.HandleFunc("GET /chats", h.chatList)
	mux.HandleFunc("GET /chats/{id}", h.conversation)
	mux.HandleFunc("GET /chats/{id}/messages", h.messages)
	mux.HandleFunc("GET /chats/{id}/media", h.gallery)
	mux.HandleFunc("GET /midia", h.allMedia)
	mux.HandleFunc("GET /eu", h.myProfile)
	mux.HandleFunc("POST /eu", h.saveMyProfile)
	mux.HandleFunc("GET /eu/foto", h.myPhoto)
	mux.HandleFunc("POST /eu/foto", h.setMyPhoto)
	mux.HandleFunc("GET /chats/{id}/search", h.search)
	mux.HandleFunc("GET /chats/{id}/calendario", h.calendar)
	mux.HandleFunc("GET /chats/{id}/perfil", h.contactProfile)
	mux.HandleFunc("GET /chats/{id}/favoritas", h.favoriteMessages)
	mux.HandleFunc("POST /chats/{id}/messages/{messageID}/favoritar", h.toggleMessageFavorite)
	mux.HandleFunc("POST /chats/{id}/messages/{messageID}/fixar", h.toggleMessagePinned)
	mux.HandleFunc("GET /chats/{id}/remetentes", h.senders)
	mux.HandleFunc("GET /chats/{id}/membros", h.members)
	mux.HandleFunc("GET /chats/{id}/participante/foto", h.participantPhotoPicker)
	mux.HandleFunc("POST /chats/{id}/participante/foto", h.linkParticipantPhoto)
	mux.HandleFunc("POST /chats/{id}/owner", h.setOwner)
	mux.HandleFunc("POST /chats/{id}/renomear", h.renameChat)
	mux.HandleFunc("POST /chats/{id}/participante", h.renameParticipant)
	mux.HandleFunc("GET /chats/{id}/atualizar", h.updateConversation)
	mux.HandleFunc("POST /chats/{id}/atualizar", h.applyUpdate)
	mux.HandleFunc("GET /chats/{id}/exportar", h.exportConversation)
	mux.HandleFunc("POST /chats/{id}/avatar", h.setAvatar)
	mux.HandleFunc("DELETE /chats/{id}/avatar", h.removeAvatar)
	mux.HandleFunc("GET /chats/{id}/avatar", h.chatAvatar)
	mux.HandleFunc("POST /chats/{id}/arquivar", h.archiveChat)
	mux.HandleFunc("POST /chats/{id}/fixar", h.pinChat)
	mux.HandleFunc("POST /chats/{id}/favoritar", h.favoriteChat)
	mux.HandleFunc("POST /chats/{id}/listas", h.chatInList)
	mux.HandleFunc("GET /setup", h.setupPage)
	mux.HandleFunc("POST /setup", h.setup)
	mux.HandleFunc("GET /login", h.loginPage)
	mux.HandleFunc("POST /login", h.login)
	mux.HandleFunc("POST /logout", h.logout)
	mux.HandleFunc("POST /eu/senha", h.changePassword)
	mux.HandleFunc("GET /sidebar", h.sidebar)
	mux.HandleFunc("GET /listas/nova", h.newListPanel)
	mux.HandleFunc("POST /listas", h.createList)
	mux.HandleFunc("DELETE /listas/{id}", h.deleteList)
	mux.HandleFunc("DELETE /chats/{id}", h.deleteChat)
	mux.HandleFunc("GET /media/{id}", h.media)
	mux.HandleFunc("GET /chats/{id}/mesclar", h.mergePicker)
	mux.HandleFunc("POST /chats/{id}/mesclar", h.mergeChats)
	mux.HandleFunc("GET /imports", h.imports)
	mux.HandleFunc("GET /imports/progress", h.importsProgress)
	mux.HandleFunc("GET /imports/badge", h.importsBadge)
	mux.HandleFunc("POST /imports/rescan", h.importsRescan)
	mux.Handle("GET /static/", http.StripPrefix("/static/",
		withStaticCache(http.FileServerFS(staticFileSystem()))))
	return h.guard(withOriginProtection(withSecurityHeaders(mux)))
}

// Framing matters even for a local app: any page on the web can put 127.0.0.1:8927 in an
// iframe and steer clicks onto destructive actions the user never meant to press.
//
// No Content-Security-Policy on purpose: Alpine evaluates its expressions at runtime, so a
// useful CSP here would need 'unsafe-eval', and one that allows eval buys little.
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// Without Cache-Control the file server only sends Last-Modified, and browsers fall back to
// heuristic caching — an edited CSS or image simply doesn't show up.
//
// The wallpapers are the exception: they are referenced from inside app.css, which cannot
// carry the "?v=" fingerprint the templates add, so a cached copy would survive a new
// build. They revalidate on every load instead — a 304 for a file nobody fetches twice per
// page. It stopped being cosmetic when the old file was the one with third-party logos.
func withStaticCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		control := staticCacheControl
		if strings.HasPrefix(r.URL.Path, "img/wallpaper-") {
			control = "no-cache"
		}
		w.Header().Set("Cache-Control", control)
		next.ServeHTTP(w, r)
	})
}

// guard applies whichever authentication the configuration asks for.
func (h *Handler) guard(next http.Handler) http.Handler {
	switch h.cfg.AuthMode() {
	case config.AuthBasic:
		return h.withAuth(next)
	case config.AuthLogin:
		return h.withLogin(next)
	default:
		return next
	}
}

// Compares digests, not the strings themselves, for two reasons that compound:
// ConstantTimeCompare returns early when the lengths differ, which lets the password's
// length be probed, and digests are always 32 bytes. And the two results are combined
// with "&&" inside the negation instead of short-circuiting "||", so a wrong username
// still pays for the password comparison — otherwise a wrong user answers measurably
// faster than a wrong password and gives away which usernames exist.
func (h *Handler) withAuth(next http.Handler) http.Handler {
	wantUser, wantPass := h.cfg.BasicAuthHashes()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		gotUser := sha256.Sum256([]byte(username))
		gotPass := sha256.Sum256([]byte(password))
		okUser := subtle.ConstantTimeCompare(gotUser[:], wantUser[:]) == 1
		okPass := subtle.ConstantTimeCompare(gotPass[:], wantPass[:]) == 1
		if !ok || !(okUser && okPass) {
			w.Header().Set("WWW-Authenticate", `Basic realm="vaultzap"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Rejects mutations that don't look like they came from this site: Sec-Fetch-Site first,
// falling back to Origin against the request's Host.
func withOriginProtection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutation := r.Method == http.MethodPost || r.Method == http.MethodDelete
		if mutation && !trustedOrigin(r) {
			http.Error(w, "untrusted origin", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func trustedOrigin(r *http.Request) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" {
		return site == "same-origin" || site == "none"
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	originURL, err := url.Parse(origin)
	return err == nil && originURL.Host == r.Host
}
