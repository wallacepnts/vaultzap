package config

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"github.com/wallacepnts/vaultzap/internal/parser"
)

func TestLoadFromEnv_padroes(t *testing.T) {
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if cfg.Addr != ":8927" {
		t.Errorf("Addr = %q, esperado :8927", cfg.Addr)
	}
	if cfg.DBPath != "/data/vaultzap.db" {
		t.Errorf("DBPath = %q, esperado /data/vaultzap.db", cfg.DBPath)
	}
	if cfg.DefaultDateOrder != parser.OrderDMY {
		t.Errorf("DefaultDateOrder = %q, esperado DMY", cfg.DefaultDateOrder)
	}
	if cfg.PostImportPolicy != PolicyMove {
		t.Errorf("PostImportPolicy = %q, esperado move", cfg.PostImportPolicy)
	}
	if cfg.AuthEnabled() {
		t.Error("AuthEnabled() deveria ser false sem VAULTZAP_BASIC_AUTH")
	}
}

func TestLoadFromEnv_basicAuth(t *testing.T) {
	t.Setenv("VAULTZAP_BASIC_AUTH", "wallace:segredo")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !cfg.AuthEnabled() {
		t.Error("AuthEnabled() deveria ser true")
	}
	if cfg.BasicAuthUser != "wallace" || cfg.BasicAuthPassword != "segredo" {
		t.Errorf("usuario/senha = %q/%q, esperado wallace/segredo", cfg.BasicAuthUser, cfg.BasicAuthPassword)
	}
}

// A secret file almost always ends in a newline; without TrimSpace it becomes part of
// the password and the login fails with no explanation.
func TestLoadFromEnv_basicAuthFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "basic-auth.txt")
	if err := os.WriteFile(path, []byte("wallace:segredo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VAULTZAP_BASIC_AUTH_FILE", path)

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if cfg.BasicAuthUser != "wallace" || cfg.BasicAuthPassword != "segredo" {
		t.Errorf("usuario/senha = %q/%q, esperado wallace/segredo", cfg.BasicAuthUser, cfg.BasicAuthPassword)
	}
}

// Set-but-empty has to be an error, not a silent way to run without auth: a ".env" line
// left as "VAULTZAP_BASIC_AUTH=" would open the service with no warning. Unset is the
// supported way to run without it, and TestLoadFromEnv_padroes covers that.
func TestLoadFromEnv_basicAuthDefinidaEVazia(t *testing.T) {
	t.Setenv("VAULTZAP_BASIC_AUTH", "")
	if _, err := LoadFromEnv(); err == nil {
		t.Error("VAULTZAP_BASIC_AUTH definida e vazia precisa ser erro, senão o app sobe sem auth nenhuma")
	}
}

func TestLoadFromEnv_basicAuthFileDefinidaEVazia(t *testing.T) {
	t.Setenv("VAULTZAP_BASIC_AUTH_FILE", "")
	if _, err := LoadFromEnv(); err == nil {
		t.Error("VAULTZAP_BASIC_AUTH_FILE definida e vazia precisa ser erro, igual à variável direta")
	}
}

func TestLoadFromEnv_basicAuthFileMissing(t *testing.T) {
	t.Setenv("VAULTZAP_BASIC_AUTH_FILE", filepath.Join(t.TempDir(), "nao-existe.txt"))
	if _, err := LoadFromEnv(); err == nil {
		t.Error("esperava erro quando o arquivo do secret não existe")
	}
}

func TestLoadFromEnv_basicAuthFileEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vazio.txt")
	if err := os.WriteFile(path, []byte("   \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VAULTZAP_BASIC_AUTH_FILE", path)
	if _, err := LoadFromEnv(); err == nil {
		t.Error("arquivo vazio precisa ser erro, senão o app sobe achando que tem auth")
	}
}

// Both at once is an error, not silent precedence: two passwords in two places with one
// quietly ignored is a nasty hour to debug.
func TestLoadFromEnv_basicAuthAmbas(t *testing.T) {
	path := filepath.Join(t.TempDir(), "basic-auth.txt")
	if err := os.WriteFile(path, []byte("wallace:doArquivo"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VAULTZAP_BASIC_AUTH", "wallace:daVariavel")
	t.Setenv("VAULTZAP_BASIC_AUTH_FILE", path)
	if _, err := LoadFromEnv(); err == nil {
		t.Error("esperava erro com VAULTZAP_BASIC_AUTH e _FILE definidas juntas")
	}
}

func TestLoadFromEnv_invalidBasicAuth(t *testing.T) {
	t.Setenv("VAULTZAP_BASIC_AUTH", "sem-dois-pontos")
	if _, err := LoadFromEnv(); err == nil {
		t.Error("esperava erro para VAULTZAP_BASIC_AUTH sem separador")
	}
}

func TestLoadFromEnv_invalidDateOrder(t *testing.T) {
	t.Setenv("VAULTZAP_DATE_ORDER", "YMD")
	if _, err := LoadFromEnv(); err == nil {
		t.Error("esperava erro para VAULTZAP_DATE_ORDER inválido")
	}
}

func TestLoadFromEnv_invalidAfterImport(t *testing.T) {
	t.Setenv("VAULTZAP_AFTER_IMPORT", "apagar-tudo")
	if _, err := LoadFromEnv(); err == nil {
		t.Error("esperava erro para VAULTZAP_AFTER_IMPORT inválido")
	}
}

// The digests are what withAuth compares, so a Config that carries a credential without
// them would reject every login. WithBasicAuth is the only way to set the pair.
func TestWithBasicAuth_precalculaOsDigests(t *testing.T) {
	cfg := Config{}.WithBasicAuth("wallace", "segredo")

	user, password := cfg.BasicAuthHashes()
	if user != sha256.Sum256([]byte("wallace")) {
		t.Error("digest do usuário não bate com sha256 do valor")
	}
	if password != sha256.Sum256([]byte("segredo")) {
		t.Error("digest da senha não bate com sha256 do valor")
	}

	if zero, _ := (Config{}).BasicAuthHashes(); zero != [sha256.Size]byte{} {
		t.Error("Config sem credencial deveria ter digests zerados")
	}
}

// Login is the default for a real run, Basic Auth still wins over it, and VAULTZAP_AUTH=off
// is the explicit opt-out. The zero value is off so a Config built by hand in a test never
// demands a session it has no password for.
func TestAuthMode(t *testing.T) {
	if got := (Config{}).AuthMode(); got != AuthOff {
		t.Errorf("Config vazia = %q, esperado off", got)
	}
	if got := (Config{}.WithBasicAuth("ana", "segredo")).AuthMode(); got != AuthBasic {
		t.Errorf("com Basic Auth = %q, esperado basic", got)
	}
	if got := (Config{Auth: AuthLogin}.WithBasicAuth("ana", "segredo")).AuthMode(); got != AuthBasic {
		t.Errorf("Basic Auth precisa vencer o login, veio %q", got)
	}

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AuthMode() != AuthLogin {
		t.Errorf("LoadFromEnv sem variável = %q, esperado login", cfg.AuthMode())
	}
}

func TestAuthMode_off(t *testing.T) {
	t.Setenv("VAULTZAP_AUTH", "off")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AuthMode() != AuthOff {
		t.Errorf("VAULTZAP_AUTH=off = %q, esperado off", cfg.AuthMode())
	}
}
