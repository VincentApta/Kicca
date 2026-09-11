// Integration tests (issue #44): per-task activity timeline — team endpoint
// visibility + actor shape, client endpoint own-ticket scope + name-only
// actor. Same harness as routes_test.go.
package handlers

import (
	"net/http"
	"testing"
)

// activityRows pulls the data array as []map.
func activityRows(t *testing.T, body map[string]interface{}) []map[string]interface{} {
	t.Helper()
	data, _ := body["data"].([]interface{})
	out := make([]map[string]interface{}, len(data))
	for i, row := range data {
		out[i] = rowMap(t, row)
	}
	return out
}

// TestTaskActivityTimeline: events newest-first, creation event has
// from_status null, team actor carries the full user shape, outsider 404s.
func TestTaskActivityTimeline(t *testing.T) {
	app, _ := newTestApp(t)
	f := newProjectFixture(t, app)
	id := mkTask(t, app, f.pm, f.projectID, "Timeline")
	for _, col := range []string{"in_progress", "done"} {
		if status, body := moveTaskViaAPI(t, app, f.pm, id, col); status != http.StatusOK {
			t.Fatalf("move %s: got %d %v", col, status, body)
		}
	}
	// pa moves it once so two distinct actors appear
	if status, body := moveTaskViaAPI(t, app, f.pa, id, "backlog"); status != http.StatusOK {
		t.Fatalf("pa move: got %d %v", status, body)
	}

	status, body, _ := do(t, app, http.MethodGet, "/api/tasks/"+id+"/events", "", f.pm)
	if status != http.StatusOK {
		t.Fatalf("activity: got %d %v", status, body)
	}
	rows := activityRows(t, body)
	if len(rows) != 4 { // create + 3 moves
		t.Fatalf("want 4 events, got %d (%v)", len(rows), rows)
	}
	// newest first: backlog←done is the latest transition
	if rows[0]["from_status"] != "done" || rows[0]["to_status"] != "backlog" {
		t.Fatalf("newest-first order broken: %+v", rows[0])
	}
	actor, _ := rows[0]["actor"].(map[string]interface{})
	if actor["id"] != f.paID || actor["name"] != "pa" || actor["email"] == "" {
		t.Fatalf("team actor shape: %+v", actor)
	}
	// creation event is last (oldest) with from_status null
	created := rows[len(rows)-1]
	if created["from_status"] != nil || created["to_status"] != "backlog" {
		t.Fatalf("creation event: %+v", created)
	}

	// outsider (non-member) → 404 no-leak
	if status, _, _ = do(t, app, http.MethodGet, "/api/tasks/"+id+"/events", "", f.out); status != http.StatusNotFound {
		t.Fatalf("outsider activity: got %d, want 404", status)
	}
	// unauthenticated → 401
	if status, _, _ = do(t, app, http.MethodGet, "/api/tasks/"+id+"/events", "", ""); status != http.StatusUnauthorized {
		t.Fatalf("unauth activity: got %d, want 401", status)
	}

	// trashed task keeps its timeline (drawer opens from trash)
	if status, _, _ = do(t, app, http.MethodDelete, "/api/tasks/"+id, "", f.pm); status != http.StatusNoContent {
		t.Fatalf("trash: got %d", status)
	}
	if status, body, _ = do(t, app, http.MethodGet, "/api/tasks/"+id+"/events", "", f.pm); status != http.StatusOK || len(activityRows(t, body)) != 4 {
		t.Fatalf("trashed activity: got %d, want 200 with 4 rows", status)
	}
}

// TestClientTicketActivity: own ticket events with name-only actor; another
// client's ticket and team tasks 404.
func TestClientTicketActivity(t *testing.T) {
	app, _ := newTestApp(t)
	f := newProjectFixture(t, app)
	client, _ := mkClient(t, app, f, "client@example.com", "client-pass-1")

	status, ticket := clientTicketViaAPI(t, app, client, f.projectID, "On fire")
	if status != http.StatusCreated {
		t.Fatalf("ticket: got %d %v", status, ticket)
	}
	id := ticket["id"].(string)
	// team triages it — a second actor appears in the client's timeline
	if status, body, _ := do(t, app, http.MethodPatch, "/api/tasks/"+id, `{"status":"in_progress"}`, f.pm); status != http.StatusOK {
		t.Fatalf("triage: got %d %v", status, body)
	}

	status, body, _ := do(t, app, http.MethodGet, "/api/client/tickets/"+id+"/events", "", client)
	if status != http.StatusOK {
		t.Fatalf("client activity: got %d %v", status, body)
	}
	rows := activityRows(t, body)
	if len(rows) != 2 { // client creation + pm triage
		t.Fatalf("want 2 events, got %d", len(rows))
	}
	actor, _ := rows[0]["actor"].(map[string]interface{})
	if actor["name"] != "pm" {
		t.Fatalf("client actor name: %+v", actor)
	}
	if _, hasEmail := actor["email"]; hasEmail {
		t.Fatalf("client actor leaked email: %+v", actor)
	}
	if _, extra := actor["global_role"]; extra {
		t.Fatalf("client actor leaked role: %+v", actor)
	}

	// another client's ticket → 404 no-leak
	other, _ := mkClient(t, app, f, "other@example.com", "other-pass-1")
	if status, _, _ = do(t, app, http.MethodGet, "/api/client/tickets/"+id+"/events", "", other); status != http.StatusNotFound {
		t.Fatalf("other client activity: got %d, want 404", status)
	}
	// a team task the client never created → 404
	teamTask := mkTask(t, app, f.pm, f.projectID, "Internal")
	if status, _, _ = do(t, app, http.MethodGet, "/api/client/tickets/"+teamTask+"/events", "", client); status != http.StatusNotFound {
		t.Fatalf("team task via client surface: got %d, want 404", status)
	}
}
