// Package config loads kica api configuration from environment variables.
package config

import (
	"fmt"
	"os"

	"github.com/VincentApta/Kica/api/internal/github"
)

// minJWTSecretLen: HS256 keys under 16 bytes are brute-forceable; refuse them.
const minJWTSecretLen = 16

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
	// The server now opens the DB and signs JWTs at startup; refuse to boot
	// without the essentials. ADMIN_* stays optional (seed skips).
	for k, v := range map[string]string{
		"DATABASE_URL": cfg.DatabaseURL,
		"JWT_SECRET":   cfg.JWTSecret,
	} {
		if v == "" {
			return nil, fmt.Errorf("%s is required", k)
		}
	}
	if len(cfg.JWTSecret) < minJWTSecretLen {
		return nil, fmt.Errorf("JWT_SECRET must be at least %d characters", minJWTSecretLen)
	}
	// GH_ENC_KEY is optional (nil disables GitHub), but a set value must be
	// usable — fail now, not on the first PAT save.
	if _, err := github.ParseEncKey(cfg.GHEncKey); err != nil {
		return nil, fmt.Errorf("GH_ENC_KEY: %w", err)
	}
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
