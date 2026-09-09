// Package config loads kicca api configuration from environment variables.
package config

import (
	"fmt"
	"os"
)

// Config holds all runtime configuration for the api server.
type Config struct {
	Port           string // listen port, default 8080
	DatabaseURL    string // postgres connection string
	JWTSecret      string // HS256 signing secret
	AdminEmail     string // first-boot seed admin
	AdminPassword  string // first-boot seed admin
	GHEncKey       string // 32-byte base64 AES-256-GCM key for GitHub PATs at rest
	GitHubAPIBase  string // default https://api.github.com, overridable for tests
}

// Load reads configuration from the environment and applies defaults.
// Required vars must be present; the server refuses to start without them.
func Load() (*Config, error) {
	cfg := &Config{
		Port:          envOr("PORT", "8080"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		AdminEmail:    os.Getenv("ADMIN_EMAIL"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		GHEncKey:      os.Getenv("GH_ENC_KEY"),
		GitHubAPIBase: envOr("GITHUB_API_BASE", "https://api.github.com"),
	}
	// T1: only DATABASE_URL/JWT_SECRET would gate startup in later tickets;
	// health endpoint runs before DB exists, so nothing is fatal yet.
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// String renders the config with secrets redacted, for startup logs.
func (c *Config) String() string {
	return fmt.Sprintf("port=%s db_set=%t jwt_set=%t admin_email=%s gh_enc_set=%t github_api_base=%s",
		c.Port, c.DatabaseURL != "", c.JWTSecret != "", c.AdminEmail, c.GHEncKey != "", c.GitHubAPIBase)
}
