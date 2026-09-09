// Boot validation tests: bad config fails fast with a clear error.
package config

import (
	"strings"
	"testing"
)

func TestLoadValidation(t *testing.T) {
	// 32 bytes base64, as GH_ENC_KEY expects (openssl rand -base64 32 shape).
	const goodKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	const goodSecret = "0123456789abcdef"

	cases := []struct {
		name    string
		setup   func(t *testing.T)
		wantErr string // substring of the error
	}{
		{
			name:    "DATABASE_URL required",
			setup:   func(t *testing.T) { t.Setenv("JWT_SECRET", goodSecret) },
			wantErr: "DATABASE_URL is required",
		},
		{
			name: "JWT_SECRET required",
			setup: func(t *testing.T) {
				t.Setenv("DATABASE_URL", "postgres://x")
			},
			wantErr: "JWT_SECRET is required",
		},
		{
			name: "JWT_SECRET too short",
			setup: func(t *testing.T) {
				t.Setenv("DATABASE_URL", "postgres://x")
				t.Setenv("JWT_SECRET", "short")
			},
			wantErr: "at least 16",
		},
		{
			name: "GH_ENC_KEY not base64",
			setup: func(t *testing.T) {
				t.Setenv("DATABASE_URL", "postgres://x")
				t.Setenv("JWT_SECRET", goodSecret)
				t.Setenv("GH_ENC_KEY", "!!!not-base64!!!")
			},
			wantErr: "GH_ENC_KEY",
		},
		{
			name: "GH_ENC_KEY wrong length",
			setup: func(t *testing.T) {
				t.Setenv("DATABASE_URL", "postgres://x")
				t.Setenv("JWT_SECRET", goodSecret)
				t.Setenv("GH_ENC_KEY", "AAAA") // decodes, but 3 bytes
			},
			wantErr: "want 32",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup(t)
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("got %v, want error containing %q", err, tc.wantErr)
			}
		})
	}

	t.Run("valid config loads", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://x")
		t.Setenv("JWT_SECRET", goodSecret)
		t.Setenv("GH_ENC_KEY", goodKey)
		cfg, err := Load()
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if cfg.Port != "8080" || cfg.GitHubAPIBase != "https://api.github.com" {
			t.Fatalf("defaults: %+v", cfg)
		}
	})
}
