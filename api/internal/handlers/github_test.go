// T7 GitHub integration tests. Upstream is an httptest mock wired through
// GITHUB_API_BASE (t.Setenv), per docs/architecture.md testing stance.
package handlers

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/VincentApta/Kica/api/internal/github"
	"github.com/VincentApta/Kica/api/internal/models"
)

// ghMock is a programmable GitHub upstream: sequential status codes per
// request, capturing what we sent (token header, path, payload).
type ghMock struct {
	mu       sync.Mutex
	statuses []int // consumed in order; last repeats
	calls    int
	auths    []string
	paths    []string
	bodies   []string
}

func (m *ghMock) handler(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	m.mu.Lock()
	m.calls++
	m.auths = append(m.auths, r.Header.Get("Authorization"))
	m.paths = append(m.paths, r.URL.Path)
	m.bodies = append(m.bodies, string(b))
	i := m.calls - 1
	if i >= len(m.statuses) {
		i = len(m.statuses) - 1
	}
	s := m.statuses[i]
	m.mu.Unlock()
	if s >= 300 {
		w.WriteHeader(s)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(s)
	if strings.HasPrefix(r.URL.Path, "/user/attachments") { // attachment upload (#34)
		fmt.Fprintf(w, `{"browser_download_url":"https://github.com/user-attachments/assets/att123"}`)
		return
	}
	fmt.Fprintf(w, `{"number":42,"html_url":"https://github.com/acme/app/issues/42"}`)
}

func (m *ghMock) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

func (m *ghMock) last(t *testing.T) (auth, path, body string) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.calls == 0 {
		t.Fatal("no upstream calls captured")
	}
	return m.auths[m.calls-1], m.paths[m.calls-1], m.bodies[m.calls-1]
}

// ghFixture spins the mock + app with team/project/task.
func ghFixture(t *testing.T, statuses ...int) (*fiber.App, *gorm.DB, *ghMock, string, string, string) {
	t.Helper()
	mock := &ghMock{statuses: statuses}
	srv := httptest.NewServer(http.HandlerFunc(mock.handler))
	t.Cleanup(srv.Close)
	t.Setenv("GITHUB_API_BASE", srv.URL)
	t.Setenv("GITHUB_UPLOADS_BASE", srv.URL)

	app, gdb := newTestApp(t)
	admin := loginAndGet(t, app, adminEmail, adminPass)
	if status, _, _ := do(t, app, http.MethodPost, "/api/teams", `{"name":"GH Team"}`, admin); status != http.StatusCreated {
		t.Fatalf("create team: %d", status)
	}
	var team models.Team
	if err := gdb.First(&team, "name = ?", "GH Team").Error; err != nil {
		t.Fatalf("load team: %v", err)
	}
	if status, _, _ := do(t, app, http.MethodPost, "/api/projects",
		fmt.Sprintf(`{"team_id":"%s","name":"App","key":"APP"}`, team.ID), admin); status != http.StatusCreated {
		t.Fatalf("create project: %d", status)
	}
	var p models.Project
	if err := gdb.First(&p, "key = ?", "APP").Error; err != nil {
		t.Fatalf("load project: %v", err)
	}
	if status, _, _ := do(t, app, http.MethodPost, "/api/projects/"+p.ID+"/tasks",
		`{"title":"Fix login bug","description":"Steps to reproduce here."}`, admin); status != http.StatusCreated {
		t.Fatalf("create task: %d", status)
	}
	var task models.Task
	if err := gdb.First(&task, "project_id = ?", p.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	return app, gdb, mock, admin, p.ID, task.ID
}

func configureGithub(t *testing.T, app *fiber.App, admin, projectID string) {
	t.Helper()
	if status, _, _ := do(t, app, http.MethodPut, "/api/projects/"+projectID+"/github",
		`{"repo":"acme/app","token":"ghp_tok"}`, admin); status != http.StatusOK {
		t.Fatalf("configure github: %d", status)
	}
}

func TestGithubIssueCreateSuccess(t *testing.T) {
	app, _, mock, admin, projectID, taskID := ghFixture(t, http.StatusCreated)

	status, body, _ := do(t, app, http.MethodPut, "/api/projects/"+projectID+"/github",
		`{"repo":"acme/app","token":"ghp_supersecret123"}`, admin)
	if status != http.StatusOK || body["repo"] != "acme/app" {
		t.Fatalf("put github config: got %d %v", status, body)
	}

	status, body, _ = do(t, app, http.MethodPost, "/api/tasks/"+taskID+"/github/issue", "", admin)
	if status != http.StatusCreated {
		t.Fatalf("create issue: got %d %v, want 201", status, body)
	}
	if body["repo"] != "acme/app" || body["issue_number"] != float64(42) ||
		body["issue_url"] != "https://github.com/acme/app/issues/42" {
		t.Fatalf("issue payload: %+v", body)
	}

	// upstream saw the PAT, the right path, task title + description + footer
	auth, path, upBody := mock.last(t)
	if auth != "Bearer ghp_supersecret123" {
		t.Fatalf("upstream auth: %q", auth)
	}
	if path != "/repos/acme/app/issues" {
		t.Fatalf("upstream path: %q", path)
	}
	if !strings.Contains(upBody, `"title":"Fix login bug"`) ||
		!strings.Contains(upBody, "Steps to reproduce here.") ||
		!strings.Contains(upBody, "Created from kica task APP-1") {
		t.Fatalf("upstream body: %s", upBody)
	}
	if mock.count() != 1 {
		t.Fatalf("upstream calls: %d, want 1", mock.count())
	}

	// task JSON now carries gh_link (detail + list paths)
	status, body, _ = do(t, app, http.MethodGet, "/api/tasks/"+taskID, "", admin)
	if status != http.StatusOK {
		t.Fatalf("get task: %d", status)
	}
	link, _ := body["gh_link"].(map[string]interface{})
	if link == nil || link["issue_number"] != float64(42) || link["repo"] != "acme/app" {
		t.Fatalf("task gh_link: %+v", body["gh_link"])
	}
	status, body, _ = do(t, app, http.MethodGet, "/api/projects/"+projectID+"/tasks", "", admin)
	if status != http.StatusOK {
		t.Fatalf("list tasks: %d", status)
	}
	data, _ := body["data"].([]interface{})
	if len(data) != 1 {
		t.Fatalf("list len: %d", len(data))
	}
	if l, _ := data[0].(map[string]interface{})["gh_link"].(map[string]interface{}); l == nil {
		t.Fatalf("list gh_link missing: %+v", data[0])
	}
}

func TestGithubIssueDuplicateConflict(t *testing.T) {
	app, _, mock, admin, projectID, taskID := ghFixture(t, http.StatusCreated)
	configureGithub(t, app, admin, projectID)
	if status, _, _ := do(t, app, http.MethodPost, "/api/tasks/"+taskID+"/github/issue", "", admin); status != http.StatusCreated {
		t.Fatalf("first create failed")
	}
	status, body, _ := do(t, app, http.MethodPost, "/api/tasks/"+taskID+"/github/issue", "", admin)
	if status != http.StatusConflict || errCode(t, body) != "task_already_linked" {
		t.Fatalf("second create: got %d %v, want 409 task_already_linked", status, body)
	}
	if mock.count() != 1 { // 409 short-circuits before any upstream call
		t.Fatalf("upstream calls: %d, want 1", mock.count())
	}
}

func TestGithubIssueUnconfigured(t *testing.T) {
	app, _, mock, admin, _, taskID := ghFixture(t, http.StatusCreated)
	status, body, _ := do(t, app, http.MethodPost, "/api/tasks/"+taskID+"/github/issue", "", admin)
	if status != http.StatusUnprocessableEntity || errCode(t, body) != "gh_unconfigured" {
		t.Fatalf("unconfigured: got %d %v, want 422 gh_unconfigured", status, body)
	}
	if mock.count() != 0 {
		t.Fatalf("upstream calls: %d, want 0", mock.count())
	}
}

func TestGithubIssueUpstream5xx(t *testing.T) {
	// 500 then 500 → one retry then 502
	app, _, mock, admin, projectID, taskID := ghFixture(t, http.StatusInternalServerError, http.StatusInternalServerError)
	configureGithub(t, app, admin, projectID)
	status, body, _ := do(t, app, http.MethodPost, "/api/tasks/"+taskID+"/github/issue", "", admin)
	if status != http.StatusBadGateway || errCode(t, body) != "gh_upstream_error" {
		t.Fatalf("upstream 5xx: got %d %v, want 502 gh_upstream_error", status, body)
	}
	if mock.count() != 2 {
		t.Fatalf("upstream calls after 5xx-then-5xx: got %d, want 2 (one retry)", mock.count())
	}
	if strings.Contains(fmt.Sprint(body), "ghp_tok") {
		t.Fatalf("token leaked in 502 body: %v", body)
	}
}

func TestGithubIssueRetryRecovers(t *testing.T) {
	// 500 then 201 → the single retry lands the issue
	app, _, mock, admin, projectID, taskID := ghFixture(t, http.StatusInternalServerError, http.StatusCreated)
	configureGithub(t, app, admin, projectID)
	if status, _, _ := do(t, app, http.MethodPost, "/api/tasks/"+taskID+"/github/issue", "", admin); status != http.StatusCreated {
		t.Fatalf("retry-then-success create failed")
	}
	if mock.count() != 2 {
		t.Fatalf("upstream calls: got %d, want 2", mock.count())
	}
}

// TestGithubTokenNeverLeaked: the PAT appears in no response body (PUT
// config, GET project, POST issue, GET task) and is not stored in plaintext.
func TestGithubTokenNeverLeaked(t *testing.T) {
	app, gdb, _, admin, projectID, taskID := ghFixture(t, http.StatusCreated)
	const token = "ghp_topsecret_leakcanary"
	var responses []string

	status, body, _ := do(t, app, http.MethodPut, "/api/projects/"+projectID+"/github",
		`{"repo":"acme/app","token":"`+token+`"}`, admin)
	if status != http.StatusOK {
		t.Fatalf("configure: %d", status)
	}
	responses = append(responses, fmt.Sprint(body))
	status, body, _ = do(t, app, http.MethodGet, "/api/projects/"+projectID, "", admin)
	if status != http.StatusOK {
		t.Fatalf("get project: %d", status)
	}
	if _, leaked := body["gh_token_enc"]; leaked {
		t.Fatal("gh_token_enc key serialized in project detail")
	}
	responses = append(responses, fmt.Sprint(body))
	status, body, _ = do(t, app, http.MethodPost, "/api/tasks/"+taskID+"/github/issue", "", admin)
	if status != http.StatusCreated {
		t.Fatalf("create issue: %d", status)
	}
	responses = append(responses, fmt.Sprint(body))
	status, body, _ = do(t, app, http.MethodGet, "/api/tasks/"+taskID, "", admin)
	if status != http.StatusOK {
		t.Fatalf("get task: %d", status)
	}
	responses = append(responses, fmt.Sprint(body))
	for i, r := range responses {
		if strings.Contains(r, token) {
			t.Fatalf("token leaked in response %d: %s", i, r)
		}
	}

	var p models.Project
	if err := gdb.First(&p, "id = ?", projectID).Error; err != nil {
		t.Fatalf("load project: %v", err)
	}
	if len(p.GhTokenEnc) == 0 || strings.Contains(string(p.GhTokenEnc), token) {
		t.Fatalf("token not encrypted at rest: %q", string(p.GhTokenEnc))
	}
	// ciphertext round-trips, so encryption is lossless
	got, err := github.DecryptToken(&testGhEncKey, p.GhTokenEnc)
	if err != nil || got != token {
		t.Fatalf("decrypt round-trip: %q %v", got, err)
	}
}

func TestPutProjectGithubValidation(t *testing.T) {
	app, gdb, _, admin, projectID, _ := ghFixture(t, http.StatusCreated)

	// bad repo shape
	status, body, _ := do(t, app, http.MethodPut, "/api/projects/"+projectID+"/github",
		`{"repo":"not-a-slash-shape","token":"t"}`, admin)
	if status != http.StatusUnprocessableEntity || errCode(t, body) != "validation_failed" {
		t.Fatalf("bad repo: got %d %v, want 422", status, body)
	}
	// blank token on first save (nothing stored yet) → 422
	status, body, _ = do(t, app, http.MethodPut, "/api/projects/"+projectID+"/github",
		`{"repo":"acme/app","token":""}`, admin)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("blank token first save: got %d %v, want 422", status, body)
	}
	// plain project member (role member) → 403
	createUserViaAPI(t, app, admin, memberEmail, memberPass, "member")
	member := loginAndGet(t, app, memberEmail, memberPass)
	var mu, au models.User
	if err := gdb.First(&mu, "email = ?", memberEmail).Error; err != nil {
		t.Fatalf("load member: %v", err)
	}
	if err := gdb.First(&au, "email = ?", adminEmail).Error; err != nil {
		t.Fatalf("load admin: %v", err)
	}
	if status, _, _ := do(t, app, http.MethodPut, "/api/projects/"+projectID+"/members",
		fmt.Sprintf(`{"members":[{"user_id":%q,"role":"member"},{"user_id":%q,"role":"project_admin"}]}`, mu.ID, au.ID), admin); status != http.StatusOK {
		t.Fatalf("add member: %d", status)
	}
	status, body, _ = do(t, app, http.MethodPut, "/api/projects/"+projectID+"/github",
		`{"repo":"acme/app","token":"t"}`, member)
	if status != http.StatusForbidden {
		t.Fatalf("member put github: got %d %v, want 403", status, body)
	}
}
