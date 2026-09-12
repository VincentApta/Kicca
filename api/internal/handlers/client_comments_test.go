// #43 client-portal comment tests: clients read the thread on their own
// tickets (author name only — no email/user_id leak), reply through the
// client endpoint, and cannot reach anyone else's tickets (404 no-leak).
// The team endpoint stays the drawer's full view: client comments surface
// there with the client's name.
package handlers

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/VincentApta/Kica/api/internal/models"
)

// TestClientTicketCommentsReadAndReply: team comment → visible to the ticket
// owner as {id, body, created_at, user:{name}}; client reply lands in the same
// comments table authored by the client; the team list shows it with the
// client's name.
func TestClientTicketCommentsReadAndReply(t *testing.T) {
	app, gdb := newTestApp(t)
	f := newProjectFixture(t, app)
	client, clientID := mkClient(t, app, f, "client@example.com", "client-pass-1")
	_, body := clientTicketViaAPI(t, app, client, f.projectID, "Printer on fire")
	ticketID := body["id"].(string)

	// team member asks a question in the drawer
	if status, body, _ := do(t, app, http.MethodPost, "/api/tasks/"+ticketID+"/comments",
		`{"body":"Which floor is the printer on?"}`, f.pm); status != http.StatusCreated {
		t.Fatalf("team comment: got %d %v", status, body)
	}

	status, list, _ := do(t, app, http.MethodGet, "/api/client/tickets/"+ticketID+"/comments", "", client)
	if status != http.StatusOK {
		t.Fatalf("client list comments: got %d %v", status, list)
	}
	data, _ := list["data"].([]interface{})
	if len(data) != 1 {
		t.Fatalf("client comment rows: got %d, want 1", len(data))
	}
	row := rowMap(t, data[0])
	if row["body"] != "Which floor is the printer on?" {
		t.Fatalf("team comment body: %v", row)
	}
	user, _ := row["user"].(map[string]interface{})
	if user["name"] != "pm" { // projectFixture pm — name defaults to the email prefix
		t.Fatalf("author name: %v", user)
	}
	for _, leaked := range []string{"email", "user_id", "task_id", "global_role"} {
		if _, ok := row[leaked]; ok {
			t.Fatalf("client comment JSON leaks %q: %v", leaked, row)
		}
		if _, ok := user[leaked]; ok {
			t.Fatalf("client comment user leaks %q: %v", leaked, user)
		}
	}

	// client replies through the portal endpoint
	payload := `{"body":"Second floor, by the kitchen"}`
	status, created, _ := do(t, app, http.MethodPost, "/api/client/tickets/"+ticketID+"/comments", payload, client)
	if status != http.StatusCreated {
		t.Fatalf("client reply: got %d %v", status, created)
	}
	if created["body"] != "Second floor, by the kitchen" {
		t.Fatalf("client reply body: %v", created)
	}
	if user, _ := created["user"].(map[string]interface{}); user["name"] != "Client" {
		t.Fatalf("client reply author: %v", created)
	}
	// empty body → 422, same as the team endpoint
	if status, body, _ := do(t, app, http.MethodPost, "/api/client/tickets/"+ticketID+"/comments", `{"body":"  "}`, client); status != http.StatusUnprocessableEntity {
		t.Fatalf("blank reply: got %d %v, want 422", status, body)
	}

	// the row is authored by the client user
	var n int64
	gdb.Model(&models.Comment{}).Where("task_id = ? AND user_id = ?", ticketID, clientID).Count(&n)
	if n != 1 {
		t.Fatalf("client-authored comment rows: got %d, want 1", n)
	}

	// team drawer: both comments, the client's one carries their name
	status, list, _ = do(t, app, http.MethodGet, "/api/tasks/"+ticketID+"/comments", "", f.pm)
	if status != http.StatusOK {
		t.Fatalf("team list comments: got %d", status)
	}
	data, _ = list["data"].([]interface{})
	if len(data) != 2 {
		t.Fatalf("team comment rows: got %d, want 2", len(data))
	}
	last := rowMap(t, data[1])
	if last["author"] != "Client" || last["user_id"] != clientID {
		t.Fatalf("team view of client comment: %v", last)
	}
}

// TestClientCommentsOnlyOwnTicket: listing or posting on a ticket the client
// did not create (or an unlinked one) is a 404 no-leak; team users get 403 on
// the client comment routes.
func TestClientCommentsOnlyOwnTicket(t *testing.T) {
	app, _ := newTestApp(t)
	f := newProjectFixture(t, app)
	c1, _ := mkClient(t, app, f, "c1@example.com", "c1-pass-1")
	c2, _ := mkClient(t, app, f, "c2@example.com", "c2-pass-1")
	_, mine := clientTicketViaAPI(t, app, c1, f.projectID, "Mine")
	mineID := mine["id"].(string)
	if status, body, _ := do(t, app, http.MethodPost, "/api/tasks/"+mineID+"/comments", `{"body":"hi"}`, f.pm); status != http.StatusCreated {
		t.Fatalf("seed comment: got %d %v", status, body)
	}

	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/client/tickets/" + mineID + "/comments", ""},
		{http.MethodPost, "/api/client/tickets/" + mineID + "/comments", `{"body":"hi"}`},
	} {
		if status, body, _ := do(t, app, tc.method, tc.path, tc.body, c2); status != http.StatusNotFound || errCode(t, body) != "not_found" {
			t.Fatalf("c2 %s %s: got %d %v, want 404 not_found", tc.method, tc.path, status, body)
		}
	}
	// not a uuid → 400
	if status, body, _ := do(t, app, http.MethodGet, "/api/client/tickets/not-a-uuid/comments", "", c1); status != http.StatusBadRequest {
		t.Fatalf("bad uuid: got %d %v, want 400", status, body)
	}
	// team users never reach the client surface
	for _, cookie := range []string{f.pm, f.admin} {
		path := fmt.Sprintf("/api/client/tickets/%s/comments", mineID)
		if status, body, _ := do(t, app, http.MethodGet, path, "", cookie); status != http.StatusForbidden {
			t.Fatalf("team user client comments: got %d %v, want 403", status, body)
		}
		if status, body, _ := do(t, app, http.MethodPost, path, `{"body":"x"}`, cookie); status != http.StatusForbidden {
			t.Fatalf("team user client comment post: got %d %v, want 403", status, body)
		}
	}
}
