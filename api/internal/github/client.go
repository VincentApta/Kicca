// Minimal GitHub REST v3 client — just enough to create issues.
package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Issue is the slice of the upstream response we keep.
type Issue struct {
	Number int64
	URL    string // html_url
}

// Client talks to GITHUB_API_BASE (default https://api.github.com — the env
// override is how tests point it at an httptest mock).
type Client struct {
	base string
	http *http.Client
}

func NewClient() *Client {
	base := os.Getenv("GITHUB_API_BASE")
	if base == "" {
		base = "https://api.github.com"
	}
	return &Client{
		base: strings.TrimSuffix(base, "/"),
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

// CreateIssue POSTs /repos/{owner}/{name}/issues — title + markdown body,
// PAT bearer auth. 10s timeout per attempt, exactly one retry on 5xx or
// transport error (architecture: GitHub integration). Errors carry the
// status only — never the token or the upstream body.
func (c *Client) CreateIssue(repo, token, title, body string) (Issue, error) {
	payload, err := json.Marshal(map[string]string{"title": title, "body": body})
	if err != nil {
		return Issue{}, fmt.Errorf("encode issue payload: %w", err)
	}
	url := c.base + "/repos/" + repo + "/issues"
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return Issue{}, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("github unreachable: %v", err)
			continue // transport error → retry once like a 5xx
		}
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("github returned %d", resp.StatusCode)
			continue
		}
		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			return Issue{}, fmt.Errorf("github returned %d", resp.StatusCode)
		}
		if readErr != nil {
			return Issue{}, fmt.Errorf("read github response: %v", readErr)
		}
		var out struct {
			Number  int64  `json:"number"`
			HTMLURL string `json:"html_url"`
		}
		if err := json.Unmarshal(raw, &out); err != nil || out.Number == 0 || out.HTMLURL == "" {
			return Issue{}, fmt.Errorf("unexpected github response shape")
		}
		return Issue{Number: out.Number, URL: out.HTMLURL}, nil
	}
	return Issue{}, lastErr
}
