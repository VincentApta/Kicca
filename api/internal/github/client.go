// Minimal GitHub REST v3 client — just enough to create issues.
package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
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

// uploadsBase mirrors base for the user-attachments host (tests point it at
// their mock the same way as GITHUB_API_BASE).
func uploadsBase() string {
	if b := os.Getenv("GITHUB_UPLOADS_BASE"); b != "" {
		return strings.TrimSuffix(b, "/")
	}
	return "https://uploads.github.com"
}

// UploadAttachment POSTs the file as multipart/form-data to
// /user/attachments?name=<filename> and returns the permanent
// browser_download_url (renders inline in issue bodies). GitHub requires
// multipart + the name query param (raw body gets 400 "Multipart form data
// required") and answers 202 Accepted. No retry: the caller skips the
// attachment on failure rather than failing the issue.
func (c *Client) UploadAttachment(token, filename, contentType string, body io.Reader) (string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("build multipart: %w", err)
	}
	if _, err := io.Copy(fw, body); err != nil {
		return "", fmt.Errorf("copy attachment: %w", err)
	}
	if err := mw.Close(); err != nil {
		return "", fmt.Errorf("close multipart: %w", err)
	}

	name := url.QueryEscape(filename)
	// GitHub ignores the multipart part's Content-Type and sniffs bytes
	// server-side; CreateFormFile sends application/octet-stream, which the
	// endpoint accepts.
	req, err := http.NewRequest(http.MethodPost, uploadsBase()+"/user/attachments?name="+name, &buf)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("github uploads unreachable: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read github response: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("github uploads returned %d", resp.StatusCode)
	}
	var out struct {
		URL string `json:"browser_download_url"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.URL == "" {
		return "", fmt.Errorf("unexpected github uploads response shape")
	}
	return out.URL, nil
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
