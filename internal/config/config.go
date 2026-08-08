// Package config loads vaultzap's configuration from environment variables.
package config

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wallacepnts/vaultzap/internal/parser"
)

type PostImportPolicy string

const (
	PolicyKeep   PostImportPolicy = "keep"
	PolicyMove   PostImportPolicy = "move"
	PolicyDelete PostImportPolicy = "delete"
)

type Config struct {
	Addr              string
	DBPath            string
	MediaDir          string
	BasicAuthUser     string
	BasicAuthPassword string
	DefaultDateOrder  parser.DateOrder
	Inbox             string
	PostImportPolicy  PostImportPolicy
	// Destination of the "move" policy; can live outside the inbox.
	ImportedDir string
	Me          string
	LogLevel    string
	TimeZone    string

	// Digests of the credential, so the comparison at request time is over a fixed
	// length. Computed once by WithBasicAuth, which is the only way to set them —
	// assigning the two fields above by hand would leave the hashes zeroed and no
	// credential would ever match.
	basicAuthUserHash [sha256.Size]byte
	basicAuthPassHash [sha256.Size]byte

	// Auth is how a request authenticates. The zero value means off, which is what a
	// Config assembled by hand gets; LoadFromEnv always sets it explicitly, so the
	// default for a real run is decided in one place and cannot be reached by accident.
	Auth AuthMode
}

// AuthMode is how a request proves it may read the archive.
type AuthMode string

const (
	// AuthBasic: VAULTZAP_BASIC_AUTH is set and wins over everything else, so an existing
	// deployment keeps working exactly as before.
	AuthBasic AuthMode = "basic"
	// AuthLogin: the default — a password stored in the database, generated on the first
	// start, entered on a login screen.
	AuthLogin AuthMode = "login"
	// AuthOff: explicit opt-out, for a machine where something else already guards the
	// port. Never the default: the archive is the user's whole message history.
	AuthOff AuthMode = "off"
)

func (c Config) AuthEnabled() bool {
	return c.BasicAuthUser != ""
}

func (c Config) AuthMode() AuthMode {
	if c.BasicAuthUser != "" {
		return AuthBasic
	}
	if c.Auth == "" {
		return AuthOff
	}
	return c.Auth
}

// WithBasicAuth returns a copy carrying the credential and its digests.
func (c Config) WithBasicAuth(user, password string) Config {
	c.BasicAuthUser = user
	c.BasicAuthPassword = password
	c.basicAuthUserHash = sha256.Sum256([]byte(user))
	c.basicAuthPassHash = sha256.Sum256([]byte(password))
	return c
}

// BasicAuthHashes exposes the precomputed digests to the middleware that compares them.
func (c Config) BasicAuthHashes() (user, password [sha256.Size]byte) {
	return c.basicAuthUserHash, c.basicAuthPassHash
}

func LoadFromEnv() (Config, error) {
	cfg := Config{
		Addr:             envOrDefault("VAULTZAP_ADDR", ":8927"),
		DBPath:           envOrDefault("VAULTZAP_DB", "/data/vaultzap.db"),
		MediaDir:         envOrDefault("VAULTZAP_MEDIA_DIR", "/data/media"),
		DefaultDateOrder: parser.DateOrder(envOrDefault("VAULTZAP_DATE_ORDER", string(parser.OrderDMY))),
		Inbox:            envOrDefault("VAULTZAP_INBOX", "/inbox"),
		PostImportPolicy: PostImportPolicy(envOrDefault("VAULTZAP_AFTER_IMPORT", string(PolicyMove))),
		Me:               os.Getenv("VAULTZAP_ME"),
		LogLevel:         envOrDefault("VAULTZAP_LOG_LEVEL", "info"),
		Auth:             AuthLogin,
		TimeZone:         envOrDefault("TZ", "America/Sao_Paulo"),
	}

	if strings.EqualFold(os.Getenv("VAULTZAP_AUTH"), string(AuthOff)) {
		cfg.Auth = AuthOff
	}

	basicAuth, err := readBasicAuth()
	if err != nil {
		return Config{}, err
	}
	if basicAuth != "" {
		user, password, ok := strings.Cut(basicAuth, ":")
		if !ok || user == "" || password == "" {
			return Config{}, fmt.Errorf("VAULTZAP_BASIC_AUTH inválido: formato esperado usuario:senha")
		}
		cfg = cfg.WithBasicAuth(user, password)
	}

	switch cfg.DefaultDateOrder {
	case parser.OrderDMY, parser.OrderMDY:
	default:
		return Config{}, fmt.Errorf("VAULTZAP_DATE_ORDER inválido: %q (use DMY ou MDY)", cfg.DefaultDateOrder)
	}

	switch cfg.PostImportPolicy {
	case PolicyKeep, PolicyMove, PolicyDelete:
	default:
		return Config{}, fmt.Errorf("VAULTZAP_AFTER_IMPORT inválido: %q (use keep, move ou delete)", cfg.PostImportPolicy)
	}

	cfg.ImportedDir = envOrDefault("VAULTZAP_IMPORTED_DIR", filepath.Join(cfg.Inbox, ".imported"))

	return cfg, nil
}

// The _FILE form follows the "*_FILE" convention of the Postgres/MySQL/Nextcloud images,
// and is what lets the password come from a Compose or Podman secret — both mount secrets
// as files. Setting both is an error rather than silent precedence: two passwords in two
// places with one quietly ignored is a nasty hour to debug.
func readBasicAuth() (string, error) {
	direct, directSet := os.LookupEnv("VAULTZAP_BASIC_AUTH")
	path, pathSet := os.LookupEnv("VAULTZAP_BASIC_AUTH_FILE")

	// Set-but-empty is a mistake, not a way to switch auth off — a ".env" line left as
	// "VAULTZAP_BASIC_AUTH=" by someone who cleared the value instead of commenting the
	// line would otherwise open the service with no warning at all. Leaving the variable
	// unset stays the supported way to run without auth, and an empty _FILE was already
	// an error, so the two paths now agree.
	if directSet && direct == "" {
		return "", fmt.Errorf("VAULTZAP_BASIC_AUTH está definida e vazia: remova a variável para rodar sem autenticação")
	}
	if pathSet && path == "" {
		return "", fmt.Errorf("VAULTZAP_BASIC_AUTH_FILE está definida e vazia: remova a variável para rodar sem autenticação")
	}

	switch {
	case directSet && pathSet:
		return "", fmt.Errorf("defina VAULTZAP_BASIC_AUTH ou VAULTZAP_BASIC_AUTH_FILE, não as duas")
	case !pathSet:
		return direct, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("VAULTZAP_BASIC_AUTH_FILE: %w", err)
	}
	// A secret file almost always ends in a newline, which would become part of the password.
	credential := strings.TrimSpace(string(content))
	if credential == "" {
		return "", fmt.Errorf("VAULTZAP_BASIC_AUTH_FILE: arquivo %s está vazio", path)
	}
	return credential, nil
}

func envOrDefault(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}
