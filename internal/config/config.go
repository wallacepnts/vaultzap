// Package config loads vaultzap's configuration from environment variables.
package config

import (
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
}

func (c Config) AuthEnabled() bool {
	return c.BasicAuthUser != ""
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
		TimeZone:         envOrDefault("TZ", "America/Sao_Paulo"),
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
		cfg.BasicAuthUser = user
		cfg.BasicAuthPassword = password
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
	direct := os.Getenv("VAULTZAP_BASIC_AUTH")
	path := os.Getenv("VAULTZAP_BASIC_AUTH_FILE")

	switch {
	case direct != "" && path != "":
		return "", fmt.Errorf("defina VAULTZAP_BASIC_AUTH ou VAULTZAP_BASIC_AUTH_FILE, não as duas")
	case path == "":
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
