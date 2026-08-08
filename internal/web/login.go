package web

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/wallacepnts/vaultzap/internal/auth"
	"github.com/wallacepnts/vaultzap/internal/locale"
	"github.com/wallacepnts/vaultzap/internal/store"
)

const (
	sessionCookie = "vaultzap_session"
	sessionTTL    = 30 * 24 * time.Hour
)

// withLogin guards everything behind a session cookie. The login page and the static
// assets are the exceptions: the first is where you get a session, and the second is what
// makes that page render at all — none of it is secret, it is in the public repository.
func (h *Handler) withLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/static/") {
			next.ServeHTTP(w, r)
			return
		}
		if h.hasSession(r) {
			next.ServeHTTP(w, r)
			return
		}

		// Nobody has finished the setup screen yet, so that is the only place to be —
		// including instead of the login form, which no credential could satisfy.
		credential, err := h.store.Credential(r.Context())
		if err != nil {
			h.internalError(w, "read credential", err)
			return
		}
		if !credential.Complete() {
			if r.URL.Path == "/setup" {
				next.ServeHTTP(w, r)
				return
			}
			h.sendToLogin(w, r, "/setup")
			return
		}
		if r.URL.Path == "/login" {
			next.ServeHTTP(w, r)
			return
		}
		h.sendToLogin(w, r, "/login")
	})
}

// An htmx request cannot follow a redirect usefully — the page would be swapped into
// whatever fragment target it had. HX-Redirect navigates instead.
func (h *Handler) sendToLogin(w http.ResponseWriter, r *http.Request, path string) {
	switch {
	case isHTMXRequest(r):
		w.Header().Set("HX-Redirect", path)
		w.WriteHeader(http.StatusUnauthorized)
	case r.Method == http.MethodGet:
		http.Redirect(w, r, path, http.StatusSeeOther)
	default:
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}
}

func (h *Handler) hasSession(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return false
	}
	valid, err := h.store.SessionValid(r.Context(), auth.TokenDigest(cookie.Value))
	if err != nil {
		slog.Error("check session", "error", err)
		return false
	}
	return valid
}

func (h *Handler) loginPage(w http.ResponseWriter, r *http.Request) {
	if h.hasSession(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	h.renderLogin(w, r, "")
}

func (h *Handler) renderLogin(w http.ResponseWriter, r *http.Request, failure string) {
	if failure != "" {
		w.WriteHeader(http.StatusUnauthorized)
	}
	h.render(w, r, "login.html", LoginData{Locale: requestLocale(r), Error: failure})
}

// setupPage is the first run: nobody has a credential yet, so this is where the username
// and password are chosen. It stops being reachable the moment one exists.
func (h *Handler) setupPage(w http.ResponseWriter, r *http.Request) {
	h.renderSetup(w, r, "", "")
}

func (h *Handler) renderSetup(w http.ResponseWriter, r *http.Request, username, failure string) {
	if failure != "" {
		w.WriteHeader(http.StatusBadRequest)
	}
	h.render(w, r, "login.html", LoginData{
		Locale: requestLocale(r), Setup: true, Username: username, Error: failure,
	})
}

func (h *Handler) setup(w http.ResponseWriter, r *http.Request) {
	// Re-checked here and not only in the middleware: without it a second POST could
	// overwrite the credential of an archive that is already set up.
	credential, err := h.store.Credential(r.Context())
	if err != nil {
		h.internalError(w, "read credential", err)
		return
	}
	if credential.Complete() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	lang := requestLocale(r)
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	switch {
	case username == "":
		h.renderSetup(w, r, username, locale.T(lang, "login.username_required"))
		return
	case len(password) < minPasswordLength:
		h.renderSetup(w, r, username, locale.T(lang, "login.too_short", minPasswordLength))
		return
	case password != r.FormValue("confirm"):
		h.renderSetup(w, r, username, locale.T(lang, "login.no_match"))
		return
	}

	hashed, err := auth.HashPassword(password)
	if err != nil {
		h.internalError(w, "hash password", err)
		return
	}
	hashed.Username = username
	if err := h.store.SetCredential(r.Context(), store.Credential(hashed)); err != nil {
		h.internalError(w, "save credential", err)
		return
	}
	h.startSession(w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) startSession(w http.ResponseWriter, r *http.Request) {
	token, digest := auth.NewSessionToken()
	if err := h.store.CreateSession(r.Context(), digest, time.Now().Add(sessionTTL)); err != nil {
		h.internalError(w, "create session", err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// Only when the request itself came over HTTPS: setting it unconditionally would
		// drop the cookie on a plain-http LAN install, locking the user out of their own
		// archive.
		Secure: r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
	})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	credential, err := h.store.Credential(r.Context())
	if err != nil {
		h.internalError(w, "read credential", err)
		return
	}
	// Both checks always run: "&&" instead of "||" so a wrong username costs the same as
	// a wrong password and does not give away which usernames exist.
	okUser := auth.SameUsername(r.FormValue("username"), credential.Username)
	okPass := auth.Verify(toAuthCredential(credential), r.FormValue("password"))
	if !(okUser && okPass) {
		h.renderLogin(w, r, locale.T(requestLocale(r), "login.wrong_password"))
		return
	}

	h.startSession(w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil && cookie.Value != "" {
		if err := h.store.DeleteSession(r.Context(), auth.TokenDigest(cookie.Value)); err != nil {
			slog.Error("delete session", "error", err)
		}
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	if isHTMXRequest(r) {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// changePassword answers with the profile panel re-rendered, carrying the outcome — the
// same shape every other action in that panel uses.
func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	lang := requestLocale(r)
	current := r.FormValue("current")
	next := r.FormValue("new")

	credential, err := h.store.Credential(r.Context())
	if err != nil {
		h.internalError(w, "read credential", err)
		return
	}
	switch {
	case !auth.Verify(toAuthCredential(credential), current):
		h.respondProfile(w, r, "", locale.T(lang, "login.wrong_current"))
		return
	case len(next) < minPasswordLength:
		h.respondProfile(w, r, "", locale.T(lang, "login.too_short", minPasswordLength))
		return
	}

	hashed, err := auth.HashPassword(next)
	if err != nil {
		h.internalError(w, "hash password", err)
		return
	}
	// The username is not part of this form; carrying it over keeps SetCredential from
	// blanking it and sending the user back to the setup screen.
	hashed.Username = credential.Username
	if err := h.store.SetCredential(r.Context(), store.Credential(hashed)); err != nil {
		h.internalError(w, "save password", err)
		return
	}

	// SetCredential dropped every session, this one included: mint a fresh one so the
	// user who just changed their own password is not thrown back to the login screen.
	h.startSession(w, r)
	h.respondProfile(w, r, locale.T(lang, "login.password_changed"), "")
}

const minPasswordLength = 8

func toAuthCredential(c store.Credential) auth.Credential {
	return auth.Credential(c)
}
