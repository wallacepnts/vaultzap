package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wallacepnts/vaultzap/internal/auth"
	"github.com/wallacepnts/vaultzap/internal/config"
	"github.com/wallacepnts/vaultzap/internal/store"
)

// Builds a handler in login mode with the password already set, plus the store so a test
// can look at what the handlers wrote.
const testUser = "ana"

func loginHandler(t *testing.T, password string) (*Handler, *store.Store) {
	t.Helper()
	s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "vaultzap.db"))
	if err != nil {
		t.Fatalf("abrir banco: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	credential, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	credential.Username = testUser
	if err := s.SetCredential(context.Background(), store.Credential(credential)); err != nil {
		t.Fatalf("gravar credencial: %v", err)
	}
	return NewHandler(s, config.Config{Auth: config.AuthLogin}, nil, ""), s
}

func postForm(path string, values url.Values) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	return req
}

// Logging in has to hand back a cookie that then opens the archive, and the cookie must
// not be readable by scripts — it is the whole credential once issued.
func TestLogin_correctPasswordOpensTheArchive(t *testing.T) {
	h, _ := loginHandler(t, "senha-de-teste")
	routes := h.Routes()

	rec := httptest.NewRecorder()
	routes.ServeHTTP(rec, postForm("/login", url.Values{"username": {testUser}, "password": {"senha-de-teste"}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("login correto: esperava 303, veio %d", rec.Code)
	}

	var session *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			session = c
		}
	}
	if session == nil || session.Value == "" {
		t.Fatal("login correto não devolveu cookie de sessão")
	}
	if !session.HttpOnly {
		t.Error("cookie de sessão precisa ser HttpOnly")
	}
	if session.SameSite != http.SameSiteLaxMode {
		t.Error("cookie de sessão precisa ser SameSite=Lax")
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(session)
	routes.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("com sessão válida esperava 200, veio %d", rec.Code)
	}
}

func TestLogin_wrongPasswordGivesNoSession(t *testing.T) {
	h, _ := loginHandler(t, "senha-de-teste")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, postForm("/login", url.Values{"username": {testUser}, "password": {"outra-senha"}}))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("senha errada: esperava 401, veio %d", rec.Code)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			t.Fatal("senha errada não pode emitir cookie de sessão")
		}
	}
}

// Without a session every page redirects to the login screen — except the login screen
// and the assets it needs to render.
func TestLogin_withoutSessionRedirects(t *testing.T) {
	h, _ := loginHandler(t, "senha-de-teste")
	routes := h.Routes()

	for _, path := range []string{"/", "/chats", "/imports"} {
		rec := httptest.NewRecorder()
		routes.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
			t.Errorf("%s sem sessão: esperava 303 para /login, veio %d %q",
				path, rec.Code, rec.Header().Get("Location"))
		}
	}

	for _, path := range []string{"/login", "/static/css/app.css"} {
		rec := httptest.NewRecorder()
		routes.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s deveria abrir sem sessão, veio %d", path, rec.Code)
		}
	}
}

// An htmx request cannot follow a redirect usefully: it would swap the login page into a
// fragment target. HX-Redirect is what makes the browser navigate instead.
func TestLogin_htmxGetsHXRedirect(t *testing.T) {
	h, _ := loginHandler(t, "senha-de-teste")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/chats", nil)
	req.Header.Set("HX-Request", "true")
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperava 401 para htmx sem sessão, veio %d", rec.Code)
	}
	if rec.Header().Get("HX-Redirect") != "/login" {
		t.Errorf("esperava HX-Redirect: /login, veio %q", rec.Header().Get("HX-Redirect"))
	}
}

func TestLogout_invalidatesTheSession(t *testing.T) {
	h, _ := loginHandler(t, "senha-de-teste")
	routes := h.Routes()

	rec := httptest.NewRecorder()
	routes.ServeHTTP(rec, postForm("/login", url.Values{"username": {testUser}, "password": {"senha-de-teste"}}))
	session := rec.Result().Cookies()[0]

	rec = httptest.NewRecorder()
	req := postForm("/logout", nil)
	req.AddCookie(session)
	routes.ServeHTTP(rec, req)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(session)
	routes.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("cookie de sessão continuou valendo depois do logout (%d)", rec.Code)
	}
}

// Changing the password has to sign out the other browsers, and keep the one that did it
// signed in — otherwise the user is thrown back to the login screen by their own action.
func TestChangePassword_rotatesSessions(t *testing.T) {
	h, s := loginHandler(t, "senha-antiga")
	routes := h.Routes()

	login := func() *http.Cookie {
		rec := httptest.NewRecorder()
		routes.ServeHTTP(rec, postForm("/login", url.Values{"username": {testUser}, "password": {"senha-antiga"}}))
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("login: veio %d", rec.Code)
		}
		return rec.Result().Cookies()[0]
	}
	outroNavegador := login()
	esteNavegador := login()

	rec := httptest.NewRecorder()
	req := postForm("/eu/senha", url.Values{"current": {"senha-antiga"}, "new": {"senha-nova-longa"}})
	req.AddCookie(esteNavegador)
	routes.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("trocar senha: esperava 200, veio %d", rec.Code)
	}

	credential, err := s.Credential(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !auth.Verify(auth.Credential(credential), "senha-nova-longa") {
		t.Error("a senha nova não foi gravada")
	}
	if auth.Verify(auth.Credential(credential), "senha-antiga") {
		t.Error("a senha antiga continua valendo")
	}

	var novo *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			novo = c
		}
	}
	if novo == nil || novo.Value == "" {
		t.Fatal("quem trocou a senha precisa sair com uma sessão nova")
	}

	check := func(c *http.Cookie) int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(c)
		routes.ServeHTTP(rec, req)
		return rec.Code
	}
	if got := check(novo); got != http.StatusOK {
		t.Errorf("a sessão nova deveria valer, veio %d", got)
	}
	if got := check(outroNavegador); got != http.StatusSeeOther {
		t.Errorf("o outro navegador deveria ter caído, veio %d", got)
	}
}

func TestChangePassword_rejectsWrongCurrentAndShortNew(t *testing.T) {
	h, s := loginHandler(t, "senha-antiga")
	routes := h.Routes()

	rec := httptest.NewRecorder()
	routes.ServeHTTP(rec, postForm("/login", url.Values{"username": {testUser}, "password": {"senha-antiga"}}))
	session := rec.Result().Cookies()[0]

	for _, c := range []struct {
		nome    string
		current string
		next    string
	}{
		{"senha atual errada", "chutando", "senha-nova-longa"},
		{"senha nova curta", "senha-antiga", "abc"},
	} {
		t.Run(c.nome, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := postForm("/eu/senha", url.Values{"current": {c.current}, "new": {c.next}})
			req.AddCookie(session)
			routes.ServeHTTP(rec, req)

			credential, err := s.Credential(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if !auth.Verify(auth.Credential(credential), "senha-antiga") {
				t.Error("a senha foi trocada quando não deveria")
			}
		})
	}
}

// Basic Auth keeps winning, so an existing deployment never gets a login screen it did
// not ask for.
func TestAuthMode_basicAuthWinsOverLogin(t *testing.T) {
	s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "vaultzap.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	h := NewHandler(s, config.Config{}.WithBasicAuth("ana", "segredo"), nil, "")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("ana", "segredo")
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Basic Auth deveria abrir direto, veio %d", rec.Code)
	}
}

// Before anyone has finished the setup screen, every path leads to it — the login form
// included, since no credential could satisfy it.
func TestSetup_everythingLeadsToTheSetupScreen(t *testing.T) {
	s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "vaultzap.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	routes := NewHandler(s, config.Config{Auth: config.AuthLogin}, nil, "").Routes()

	for _, path := range []string{"/", "/chats", "/login"} {
		rec := httptest.NewRecorder()
		routes.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/setup" {
			t.Errorf("%s: esperava 303 para /setup, veio %d %q",
				path, rec.Code, rec.Header().Get("Location"))
		}
	}

	rec := httptest.NewRecorder()
	routes.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/setup", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/setup deveria abrir, veio %d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, `name="confirm"`) {
		t.Error("a tela de cadastro precisa pedir a senha duas vezes")
	}
}

func TestSetup_createsTheLoginAndSignsIn(t *testing.T) {
	s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "vaultzap.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	routes := NewHandler(s, config.Config{Auth: config.AuthLogin}, nil, "").Routes()

	rec := httptest.NewRecorder()
	routes.ServeHTTP(rec, postForm("/setup", url.Values{
		"username": {"wallace"}, "password": {"senha-escolhida"}, "confirm": {"senha-escolhida"},
	}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("cadastro: esperava 303, veio %d — %s", rec.Code, rec.Body.String())
	}

	credential, err := s.Credential(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if credential.Username != "wallace" {
		t.Errorf("usuário = %q, esperado wallace", credential.Username)
	}
	if !auth.Verify(auth.Credential(credential), "senha-escolhida") {
		t.Error("a senha escolhida não foi gravada")
	}

	// Whoever just set it up is already in — no second trip through the login form.
	var session *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			session = c
		}
	}
	if session == nil {
		t.Fatal("o cadastro deveria já abrir uma sessão")
	}
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(session)
	routes.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("sessão do cadastro não vale (%d)", rec.Code)
	}
}

func TestSetup_rejectsIncompleteForms(t *testing.T) {
	for _, c := range []struct {
		nome   string
		values url.Values
	}{
		{"sem usuário", url.Values{"username": {"  "}, "password": {"senha-longa"}, "confirm": {"senha-longa"}}},
		{"senha curta", url.Values{"username": {"ana"}, "password": {"abc"}, "confirm": {"abc"}}},
		{"senhas diferentes", url.Values{"username": {"ana"}, "password": {"senha-longa"}, "confirm": {"outra-longa"}}},
	} {
		t.Run(c.nome, func(t *testing.T) {
			s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "vaultzap.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { s.Close() })
			routes := NewHandler(s, config.Config{Auth: config.AuthLogin}, nil, "").Routes()

			rec := httptest.NewRecorder()
			routes.ServeHTTP(rec, postForm("/setup", c.values))
			if rec.Code == http.StatusSeeOther {
				t.Fatal("cadastro inválido não deveria ser aceito")
			}
			credential, err := s.Credential(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if credential.Complete() {
				t.Error("gravou credencial a partir de um formulário inválido")
			}
		})
	}
}

// The screen must close behind itself: a second POST cannot overwrite the credential of an
// archive that is already claimed.
func TestSetup_cannotReclaimAnArchive(t *testing.T) {
	h, s := loginHandler(t, "senha-de-teste")

	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, postForm("/setup", url.Values{
		"username": {"invasor"}, "password": {"senha-do-invasor"}, "confirm": {"senha-do-invasor"},
	}))

	credential, err := s.Credential(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if credential.Username != testUser {
		t.Errorf("o usuário foi trocado para %q", credential.Username)
	}
	if !auth.Verify(auth.Credential(credential), "senha-de-teste") {
		t.Error("a senha original deixou de valer")
	}
}

// A row left by the previous version has a password but no username, and no login form
// could satisfy it — it has to land on setup instead of locking the archive shut.
func TestSetup_incompleteCredentialFallsBackToSetup(t *testing.T) {
	s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "vaultzap.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	credential, err := auth.HashPassword("gerada-pela-versao-antiga")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetCredential(context.Background(), store.Credential(credential)); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	NewHandler(s, config.Config{Auth: config.AuthLogin}, nil, "").Routes().
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Header().Get("Location") != "/setup" {
		t.Errorf("esperava /setup, veio %q", rec.Header().Get("Location"))
	}
}

// A wrong username has to fail exactly like a wrong password: same status, no session.
func TestLogin_wrongUsernameIsRejected(t *testing.T) {
	h, _ := loginHandler(t, "senha-de-teste")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, postForm("/login", url.Values{
		"username": {"outro"}, "password": {"senha-de-teste"},
	}))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("usuário errado: esperava 401, veio %d", rec.Code)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			t.Fatal("usuário errado não pode emitir sessão")
		}
	}
}

// Changing the password must not blank the username, or the next start would send the user
// back to the setup screen.
func TestChangePassword_keepsTheUsername(t *testing.T) {
	h, s := loginHandler(t, "senha-antiga")
	routes := h.Routes()

	rec := httptest.NewRecorder()
	routes.ServeHTTP(rec, postForm("/login", url.Values{"username": {testUser}, "password": {"senha-antiga"}}))
	session := rec.Result().Cookies()[0]

	rec = httptest.NewRecorder()
	req := postForm("/eu/senha", url.Values{"current": {"senha-antiga"}, "new": {"senha-nova-longa"}})
	req.AddCookie(session)
	routes.ServeHTTP(rec, req)

	credential, err := s.Credential(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if credential.Username != testUser {
		t.Errorf("usuário virou %q depois de trocar a senha", credential.Username)
	}
	if !credential.Complete() {
		t.Error("a credencial ficou incompleta e o app voltaria pro cadastro")
	}
}
