// T8 permission e2e sweep: every project-scoped route denies a non-member
// with 404 (no leak) and every unauthenticated call with 401, then the
// member / project_admin / global admin boundaries per docs/domain.md rule 6.
package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestNonMemberAndUnauthOnEveryProjectScopedRoute walks the full route table
// from router.go against a real project + task + label + GitHub config.
func TestNonMemberAndUnauthOnEveryProjectScopedRoute(t *testing.T) {
	app, _ := newTestApp(t)
	f := newProjectFixture(t, app)

	taskID := mkTask(t, app, f.pm, f.projectID, "Sweep target")
	_, lbl := createLabelViaAPI(t, app, f.pa, f.projectID, "sweep", "#123456")
	labelID := lbl["id"].(string)
	if status, _, _ := do(t, app, http.MethodPut, "/api/projects/"+f.projectID+"/github",
		`{"repo":"acme/app","token":"ghp_sweep"}`, f.admin); status != http.StatusOK {
		t.Fatalf("configure github: %d", status)
	}

	memberBody := fmt.Sprintf(`{"members":[{"user_id":%q,"role":"project_admin"},{"user_id":%q,"role":"member"}]}`, f.paID, f.pmID)
	move := `{"status":"backlog"}`

	routes := []struct{ method, path, body string }{
		{http.MethodGet, "/api/projects/" + f.projectID, ""},
		{http.MethodPatch, "/api/projects/" + f.projectID, `{"name":"X"}`},
		{http.MethodDelete, "/api/projects/" + f.projectID, ""},
		{http.MethodPut, "/api/projects/" + f.projectID + "/members", memberBody},
		{http.MethodGet, "/api/projects/" + f.projectID + "/tasks", ""},
		{http.MethodPost, "/api/projects/" + f.projectID + "/tasks", `{"title":"X"}`},
		{http.MethodGet, "/api/projects/" + f.projectID + "/labels", ""},
		{http.MethodPost, "/api/projects/" + f.projectID + "/labels", `{"name":"X","color":"#000000"}`},
		{http.MethodPut, "/api/projects/" + f.projectID + "/github", `{"repo":"a/b","token":"t"}`},
		{http.MethodGet, "/api/tasks/" + taskID, ""},
		{http.MethodPatch, "/api/tasks/" + taskID, `{"title":"X"}`},
		{http.MethodDelete, "/api/tasks/" + taskID, ""},
		{http.MethodPost, "/api/tasks/" + taskID + "/restore", ""},
		{http.MethodPost, "/api/tasks/" + taskID + "/move", move},
		{http.MethodGet, "/api/tasks/" + taskID + "/comments", ""},
		{http.MethodPost, "/api/tasks/" + taskID + "/comments", `{"body":"X"}`},
		{http.MethodPost, "/api/tasks/" + taskID + "/github/issue", ""},
		{http.MethodDelete, "/api/labels/" + labelID, ""},
	}

	// non-member (valid session, no membership row): 404 on every route —
	// never 403, the project's existence must not leak (contract "Status
	// codes locked")
	for _, r := range routes {
		if status, body, _ := do(t, app, r.method, r.path, r.body, f.out); status != http.StatusNotFound || errCode(t, body) != "not_found" {
			t.Fatalf("outsider %s %s: got %d %v, want 404 not_found", r.method, r.path, status, body)
		}
	}

	// no session at all: 401 on every route
	for _, r := range routes {
		if status, body, _ := do(t, app, r.method, r.path, r.body, ""); status != http.StatusUnauthorized {
			t.Fatalf("unauth %s %s: got %d %v, want 401", r.method, r.path, status, body)
		}
	}
}

// TestRoleBoundariesGithubConfigAndIssue: matrix row "Manage … GH config" is
// admin/project_admin only; issue creation is a task action (member+).
func TestRoleBoundariesGithubConfigAndIssue(t *testing.T) {
	app, _ := newTestApp(t)
	f := newProjectFixture(t, app)

	// member: sees the project → 403 (not 404) on GH config write
	if status, body, _ := do(t, app, http.MethodPut, "/api/projects/"+f.projectID+"/github",
		`{"repo":"acme/app","token":"t"}`, f.pm); status != http.StatusForbidden || errCode(t, body) != "forbidden" {
		t.Fatalf("member put github: got %d %v, want 403 forbidden", status, body)
	}

	// project_admin (no global role) can configure; response has no token
	status, body, _ := do(t, app, http.MethodPut, "/api/projects/"+f.projectID+"/github",
		`{"repo":"acme/app","token":"ghp_pa"}`, f.pa)
	if status != http.StatusOK || body["repo"] != "acme/app" {
		t.Fatalf("pa put github: got %d %v, want 200", status, body)
	}
	if fmt.Sprint(body) != `map[repo:acme/app]` {
		t.Fatalf("github config response leaks fields: %v", body)
	}

	// issue creation: member AND project_admin both may (member+); upstream
	// is the in-process mock from github_test.go
	mock := &ghMock{statuses: []int{http.StatusCreated}}
	srv := httptest.NewServer(http.HandlerFunc(mock.handler))
	t.Cleanup(srv.Close)
	t.Setenv("GITHUB_API_BASE", srv.URL)
	memberTask := mkTask(t, app, f.pm, f.projectID, "From member")
	paTask := mkTask(t, app, f.pa, f.projectID, "From pa")
	if status, body, _ := do(t, app, http.MethodPost, "/api/tasks/"+memberTask+"/github/issue", "", f.pm); status != http.StatusCreated {
		t.Fatalf("member create issue: got %d %v, want 201", status, body)
	}
	if status, body, _ := do(t, app, http.MethodPost, "/api/tasks/"+paTask+"/github/issue", "", f.pa); status != http.StatusCreated {
		t.Fatalf("pa create issue: got %d %v, want 201", status, body)
	}
}

// TestGlobalAdminNonMemberSweep: a global admin needs no membership row —
// every project-scoped action still works (rule 1, "iff global admin OR
// ProjectMember").
func TestGlobalAdminNonMemberSweep(t *testing.T) {
	app, _ := newTestApp(t)
	f := newProjectFixture(t, app)

	taskID := mkTask(t, app, f.pm, f.projectID, "Admin target")

	checks := []struct {
		name       string
		method, p  string
		body       string
		wantStatus int
	}{
		{"get project", http.MethodGet, "/api/projects/" + f.projectID, "", http.StatusOK},
		{"patch project", http.MethodPatch, "/api/projects/" + f.projectID, `{"description":"d"}`, http.StatusOK},
		{"replace members", http.MethodPut, "/api/projects/" + f.projectID + "/members",
			fmt.Sprintf(`{"members":[{"user_id":%q,"role":"project_admin"},{"user_id":%q,"role":"member"}]}`, f.paID, f.pmID), http.StatusOK},
		{"list tasks", http.MethodGet, "/api/projects/" + f.projectID + "/tasks", "", http.StatusOK},
		{"create label", http.MethodPost, "/api/projects/" + f.projectID + "/labels", `{"name":"adm","color":"#000001"}`, http.StatusCreated},
		{"patch task", http.MethodPatch, "/api/tasks/" + taskID, `{"title":"Renamed"}`, http.StatusOK},
		{"move task", http.MethodPost, "/api/tasks/" + taskID + "/move", `{"status":"in_progress"}`, http.StatusOK},
	}
	for _, c := range checks {
		if status, body, _ := do(t, app, c.method, c.p, c.body, f.admin); status != c.wantStatus {
			t.Fatalf("%s: got %d %v, want %d", c.name, status, body, c.wantStatus)
		}
	}
}
