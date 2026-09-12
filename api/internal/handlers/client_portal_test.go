// T9 integration tests (client portal, #32): ticket submit lands in the
// project Inbox with shared per-project numbering, own-ticket visibility
// without assessment leak, role isolation both directions, unlinked-project
// 404 no-leak. Same harness as routes_test.go.
package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/VincentApta/Kica/api/internal/models"
)

// mkClient creates a client user linked to the given projects, returns its
// session cookie + id.
func mkClient(t *testing.T, app *fiber.App, f projectFixture, email, pass string, projectIDs ...string) (string, string) {
	t.Helper()
	if len(projectIDs) == 0 {
		projectIDs = []string{f.projectID}
	}
	quoted := make([]string, len(projectIDs))
	for i, id := range projectIDs {
		quoted[i] = fmt.Sprintf("%q", id)
	}
	payload := fmt.Sprintf(`{"email":%q,"name":"Client","password":%q,"global_role":"client","project_ids":[%s]}`,
		email, pass, strings.Join(quoted, ","))
	status, body, _ := do(t, app, http.MethodPost, "/api/users", payload, f.admin)
	if status != http.StatusCreated {
		t.Fatalf("create client %s: got %d %v", email, status, body)
	}
	return loginAndGet(t, app, email, pass), body["id"].(string)
}

// clientTicketViaAPI: cookie POST /api/client/tickets.
func clientTicketViaAPI(t *testing.T, app *fiber.App, cookie, projectID, title string) (int, map[string]interface{}) {
	t.Helper()
	payload := fmt.Sprintf(`{"project_id":%q,"title":%q,"description":"Client words"}`, projectID, title)
	status, body, _ := do(t, app, http.MethodPost, "/api/client/tickets", payload, cookie)
	return status, body
}

// TestClientTicketCreateLandsInboxNumbered: submit → status=inbox,
// created_by=client, number continues the project-wide sequence, creation
// event logged from NULL (rule 8), team sees it as a normal inbox task.
func TestClientTicketCreateLandsInboxNumbered(t *testing.T) {
	app, gdb := newTestApp(t)
	f := newProjectFixture(t, app)
	mkTask(t, app, f.pm, f.projectID, "Team task one") // number 1
	client, clientID := mkClient(t, app, f, "client@example.com", "client-pass-1")

	status, body := clientTicketViaAPI(t, app, client, f.projectID, "Printer on fire")
	if status != http.StatusCreated {
		t.Fatalf("create ticket: got %d %v", status, body)
	}
	if body["status"] != "inbox" || body["number"] != float64(2) || body["project_key"] != "KIC" {
		t.Fatalf("ticket payload: %v", body)
	}
	if body["description"] != "Client words" || body["title"] != "Printer on fire" {
		t.Fatalf("ticket content: %v", body)
	}

	var task models.Task
	if err := gdb.First(&task, "id = ?", body["id"]).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if task.CreatedBy != clientID || task.Assessment != "" {
		t.Fatalf("task row: created_by=%s assessment=%q", task.CreatedBy, task.Assessment)
	}
	evs := taskEventsFor(t, gdb, task.ID)
	if len(evs) != 1 || evs[0].FromStatus != nil || evs[0].ToStatus != "inbox" || evs[0].ActorID != clientID {
		t.Fatalf("creation event: %+v", evs)
	}

	// team reads it through the normal board API
	status, _, _ = do(t, app, http.MethodGet, "/api/projects/"+f.projectID+"/tasks?status=inbox", "", f.pm)
	if status != http.StatusOK {
		t.Fatalf("team list inbox: got %d", status)
	}

	// second ticket shares the sequence
	_, body = clientTicketViaAPI(t, app, client, f.projectID, "Second")
	if body["number"] != float64(3) {
		t.Fatalf("second ticket number: %v", body["number"])
	}
}

// TestClientSeesOwnTicketsOnlyNoAssessment: list scoped to created_by=this
// client; assessment (and other team-only fields) never serialized.
func TestClientSeesOwnTicketsOnlyNoAssessment(t *testing.T) {
	app, gdb := newTestApp(t)
	f := newProjectFixture(t, app)
	c1, _ := mkClient(t, app, f, "c1@example.com", "c1-pass-1")
	c2, _ := mkClient(t, app, f, "c2@example.com", "c2-pass-1")

	if status, body := clientTicketViaAPI(t, app, c1, f.projectID, "Mine one"); status != http.StatusCreated {
		t.Fatalf("c1 ticket: %d %v", status, body)
	} else if err := gdb.Model(&models.Task{}).Where("id = ?", body["id"]).
		Update("assessment", "triaged: hardware fault").Error; err != nil {
		t.Fatalf("seed assessment: %v", err)
	}
	if status, body := clientTicketViaAPI(t, app, c1, f.projectID, "Mine two"); status != http.StatusCreated {
		t.Fatalf("c1 ticket 2: %d %v", status, body)
	}
	mkTask(t, app, f.pm, f.projectID, "Team task") // invisible to clients
	if status, _ := clientTicketViaAPI(t, app, c2, f.projectID, "Theirs"); status != http.StatusCreated {
		t.Fatalf("c2 ticket: %d", status)
	}

	for _, tc := range []struct {
		cookie    string
		wantTotal float64
		wantTitle string
	}{
		{c1, 2, "Mine two"}, // newest-updated first
		{c2, 1, "Theirs"},
	} {
		status, body, _ := do(t, app, http.MethodGet, "/api/client/tickets", "", tc.cookie)
		if status != http.StatusOK {
			t.Fatalf("list tickets: got %d", status)
		}
		if body["total"] != tc.wantTotal {
			t.Fatalf("total: got %v, want %v", body["total"], tc.wantTotal)
		}
		data, _ := body["data"].([]interface{})
		if len(data) != int(tc.wantTotal) {
			t.Fatalf("rows: got %d, want %v", len(data), tc.wantTotal)
		}
		if rowMap(t, data[0])["title"] != tc.wantTitle {
			t.Fatalf("first row: %v", data[0])
		}
		for _, row := range data {
			m := rowMap(t, row)
			for _, leaked := range []string{"assessment", "assignee", "labels", "position", "priority"} {
				if _, ok := m[leaked]; ok {
					t.Fatalf("client ticket JSON leaks %q: %v", leaked, m)
				}
			}
		}
	}
}

// TestRoleIsolationClientVsTeam: clients get 403 on every team surface;
// team users (member AND admin) get 403 on /api/client/*.
func TestRoleIsolationClientVsTeam(t *testing.T) {
	app, _ := newTestApp(t)
	f := newProjectFixture(t, app)
	client, _ := mkClient(t, app, f, "client@example.com", "client-pass-1")
	taskID := mkTask(t, app, f.pm, f.projectID, "Target")

	teamRoutes := []struct{ method, path, body string }{
		{http.MethodGet, "/api/users", ""},
		{http.MethodGet, "/api/teams", ""},
		{http.MethodGet, "/api/projects", ""},
		{http.MethodGet, "/api/projects/" + f.projectID + "/tasks", ""},
		{http.MethodGet, "/api/tasks", ""},
		{http.MethodGet, "/api/tasks/" + taskID, ""},
		{http.MethodPatch, "/api/tasks/" + taskID, `{"title":"X"}`},
		{http.MethodPost, "/api/tasks/" + taskID + "/comments", `{"body":"X"}`},
		{http.MethodDelete, "/api/labels/not-a-uuid", ""},
	}
	for _, r := range teamRoutes {
		if status, body, _ := do(t, app, r.method, r.path, r.body, client); status != http.StatusForbidden || errCode(t, body) != "forbidden" {
			t.Fatalf("client %s %s: got %d %v, want 403 forbidden", r.method, r.path, status, body)
		}
	}

	// /auth/me stays reachable — the portal needs the session user
	if status, _, _ := do(t, app, http.MethodGet, "/api/auth/me", "", client); status != http.StatusOK {
		t.Fatalf("client me: got %d, want 200", status)
	}

	for _, cookie := range []string{f.pm, f.admin} {
		if status, body, _ := do(t, app, http.MethodGet, "/api/client/projects", "", cookie); status != http.StatusForbidden {
			t.Fatalf("team user client projects: got %d %v, want 403", status, body)
		}
		payload := fmt.Sprintf(`{"project_id":%q,"title":"X"}`, f.projectID)
		if status, body, _ := do(t, app, http.MethodPost, "/api/client/tickets", payload, cookie); status != http.StatusForbidden {
			t.Fatalf("team user client ticket: got %d %v, want 403", status, body)
		}
		if status, body, _ := do(t, app, http.MethodGet, "/api/client/tickets", "", cookie); status != http.StatusForbidden {
			t.Fatalf("team user client tickets list: got %d %v, want 403", status, body)
		}
	}
}

// TestClientUnlinkedProject404: a project the client has no link to is a 404
// (no leak), even though it exists; the project picker lists linked only.
func TestClientUnlinkedProject404(t *testing.T) {
	app, _ := newTestApp(t)
	f := newProjectFixture(t, app)
	status, p2, _ := do(t, app, http.MethodPost, "/api/projects",
		`{"team_id":"`+f.teamID+`","name":"Other","key":"OTH"}`, f.admin)
	if status != http.StatusCreated {
		t.Fatalf("create project 2: got %d %v", status, p2)
	}
	client, _ := mkClient(t, app, f, "client@example.com", "client-pass-1")

	if status, body := clientTicketViaAPI(t, app, client, p2["id"].(string), "Sneaky"); status != http.StatusNotFound || errCode(t, body) != "not_found" {
		t.Fatalf("unlinked ticket: got %d %v, want 404 not_found", status, body)
	}
	if status, body, _ := do(t, app, http.MethodGet, "/api/client/projects", "", client); status != http.StatusOK {
		t.Fatalf("client projects: got %d", status)
	} else {
		data, _ := body["data"].([]interface{})
		if len(data) != 1 || rowMap(t, data[0])["key"] != "KIC" {
			t.Fatalf("linked projects: %v", data)
		}
	}
}

// TestClientProjectLinksAdminManaged: create/patch wire round-trip of
// project_ids, plus the ≥1-link guard.
func TestClientProjectLinksAdminManaged(t *testing.T) {
	app, gdb := newTestApp(t)
	f := newProjectFixture(t, app)

	// client without links → 422
	status, body, _ := do(t, app, http.MethodPost, "/api/users",
		`{"email":"cl@example.com","name":"C","password":"cl-pass-123","global_role":"client"}`, f.admin)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("client no links: got %d %v, want 422", status, body)
	}

	// member→client patch without links → 422; with links → 200 + echo
	status, created := createUserViaAPI(t, app, f.admin, "cl2@example.com", "cl2-pass-1", "member")
	if status != http.StatusCreated {
		t.Fatalf("create member: %d", status)
	}
	id := created["id"].(string)
	status, body, _ = do(t, app, http.MethodPatch, "/api/users/"+id, `{"global_role":"client"}`, f.admin)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("patch to client no links: got %d %v, want 422", status, body)
	}
	payload := fmt.Sprintf(`{"global_role":"client","project_ids":[%q]}`, f.projectID)
	status, body, _ = do(t, app, http.MethodPatch, "/api/users/"+id, payload, f.admin)
	if status != http.StatusOK || body["global_role"] != "client" {
		t.Fatalf("patch to client: got %d %v", status, body)
	}
	if ids, _ := body["project_ids"].([]interface{}); len(ids) != 1 || ids[0] != f.projectID {
		t.Fatalf("patch echo project_ids: %v", body["project_ids"])
	}

	// client→member drops the links
	if status, _, _ := do(t, app, http.MethodPatch, "/api/users/"+id, `{"global_role":"member"}`, f.admin); status != http.StatusOK {
		t.Fatalf("patch back to member: got %d", status)
	}
	var links int64
	gdb.Model(&models.ClientProject{}).Where("client_id = ?", id).Count(&links)
	if links != 0 {
		t.Fatalf("links after demote: got %d, want 0", links)
	}

	// list response carries project_ids for clients only
	status, body, _ = do(t, app, http.MethodPost, "/api/users",
		fmt.Sprintf(`{"email":"cl3@example.com","name":"C3","password":"cl3-pass-123","global_role":"client","project_ids":[%q]}`, f.projectID), f.admin)
	if status != http.StatusCreated {
		t.Fatalf("create client: got %d %v", status, body)
	}
	if ids, _ := body["project_ids"].([]interface{}); len(ids) != 1 {
		t.Fatalf("create echo project_ids: %v", body["project_ids"])
	}
	status, body, _ = do(t, app, http.MethodGet, "/api/users?per_page=100", "", f.admin)
	if status != http.StatusOK {
		t.Fatalf("list users: %d", status)
	}
	data, _ := body["data"].([]interface{})
	clientsWithLinks := 0
	for _, row := range data {
		m := rowMap(t, row)
		if ids, ok := m["project_ids"].([]interface{}); ok {
			clientsWithLinks++
			if len(ids) != 1 {
				t.Fatalf("list project_ids: %v", m)
			}
		}
	}
	if clientsWithLinks != 1 {
		t.Fatalf("clients with links in list: got %d, want 1", clientsWithLinks)
	}
}
