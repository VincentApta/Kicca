// #47 client-portal search + filter tests: ?q= over title/description, a
// linked-project filter and the open/closed status split — always over own
// tickets only, same search predicate as the team list.
package handlers

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/VincentApta/Kica/api/internal/models"
)

// twoProjectsFixture: KIC + OTH in one team, a member on both, and a client
// linked to both.
func twoProjectsFixture(t *testing.T, app *fiber.App) (projectFixture, string, string) {
	t.Helper()
	f := newProjectFixture(t, app)
	status, p2, _ := do(t, app, http.MethodPost, "/api/projects",
		`{"team_id":"`+f.teamID+`","name":"Other","key":"OTH"}`, f.admin)
	if status != http.StatusCreated {
		t.Fatalf("create project 2: got %d %v", status, p2)
	}
	otherID := p2["id"].(string)
	members := fmt.Sprintf(`{"members":[{"user_id":%q,"role":"project_admin"}]}`, f.pmID) // keep the ≥1 project_admin guard happy
	if status, _, _ := do(t, app, http.MethodPut, "/api/projects/"+otherID+"/members", members, f.admin); status != http.StatusOK {
		t.Fatalf("add pm to project 2: got %d", status)
	}
	client, _ := mkClient(t, app, f, "client@example.com", "client-pass-1", f.projectID, otherID)
	return f, otherID, client
}

// TestClientTicketSearchAndFilters: q matches title OR description (case-
// insensitive); project_id narrows within the linked set; status=open|closed
// splits on done.
func TestClientTicketSearchAndFilters(t *testing.T) {
	app, gdb := newTestApp(t)
	f, otherID, client := twoProjectsFixture(t, app)

	_, t1 := clientTicketViaAPI(t, app, client, f.projectID, "Printer on fire")      // KIC, inbox
	clientTicketViaAPI(t, app, client, otherID, "Slow VPN")                          // OTH, inbox
	_, t3 := clientTicketViaAPI(t, app, client, f.projectID, "Fixed already thanks") // KIC → done below
	if err := gdb.Model(&models.Task{}).Where("id = ?", t3["id"]).Update("status", "done").Error; err != nil {
		t.Fatalf("set done: %v", err)
	}
	// another client's ticket with a matching title — must never surface
	c2, _ := mkClient(t, app, f, "c2@example.com", "c2-pass-1")
	clientTicketViaAPI(t, app, c2, f.projectID, "Printer on fire TOO")
	// team task with a matching title — invisible to clients
	mkTask(t, app, f.pm, f.projectID, "Printer on fire TEAM")

	totalOf := func(query string) float64 {
		t.Helper()
		status, body, _ := do(t, app, http.MethodGet, "/api/client/tickets"+query, "", client)
		if status != http.StatusOK {
			t.Fatalf("list %s: got %d %v", query, status, body)
		}
		if msg := errCode(t, body); msg != "" {
			t.Fatalf("list %s: %v", query, body)
		}
		return body["total"].(float64)
	}

	if got := totalOf(""); got != 3 {
		t.Fatalf("unfiltered total: got %v, want 3", got)
	}
	if got := totalOf("?q=printer"); got != 1 { // own title match only
		t.Fatalf("q=printer total: got %v, want 1", got)
	}
	if got := totalOf("?q=printer&status=closed"); got != 0 {
		t.Fatalf("q+closed total: got %v, want 0", got)
	}
	if got := totalOf("?status=open"); got != 2 {
		t.Fatalf("open total: got %v, want 2", got)
	}
	if got := totalOf("?status=closed"); got != 1 {
		t.Fatalf("closed total: got %v, want 1", got)
	}
	if got := totalOf("?project_id=" + otherID); got != 1 {
		t.Fatalf("project filter total: got %v, want 1", got)
	}
	if got := totalOf("?q=slow+vpn&project_id=" + otherID); got != 1 {
		t.Fatalf("q+project total: got %v, want 1", got)
	}
	// description match, case-insensitive
	if got := totalOf("?q=CLIENT+WORDS"); got != 3 { // clientTicketViaAPI seeds "Client words"
		t.Fatalf("description match total: got %v, want 3", got)
	}

	// rejections + no-leak
	for _, tc := range []struct{ query, wantCode string }{
		{"?status=wip", "validation_failed"},
		{"?project_id=not-a-uuid", "validation_failed"},
	} {
		if status, body, _ := do(t, app, http.MethodGet, "/api/client/tickets"+tc.query, "", client); status != http.StatusUnprocessableEntity || errCode(t, body) != tc.wantCode {
			t.Fatalf("list %s: got %d %v, want 422 %s", tc.query, status, body, tc.wantCode)
		}
	}
	// an unlinked project id: valid uuid, but the linked-scope intersection is empty — no leak
	if got := totalOf("?project_id=" + mkUnlinkedProject(t, app, f)); got != 0 {
		t.Fatalf("unlinked project total: got %v, want 0", got)
	}
	_ = t1
}

// mkUnlinkedProject creates a project the client has no link to.
func mkUnlinkedProject(t *testing.T, app *fiber.App, f projectFixture) string {
	t.Helper()
	status, p, _ := do(t, app, http.MethodPost, "/api/projects",
		`{"team_id":"`+f.teamID+`","name":"Secret","key":"SEC"}`, f.admin)
	if status != http.StatusCreated {
		t.Fatalf("create secret project: got %d %v", status, p)
	}
	return p["id"].(string)
}
