// T6 integration tests (analytics foundation): started_at first-touch,
// done_at set/clear/re-stamp, task_events per transition incl. create,
// type/estimate validation. Same harness as routes_test.go.
package handlers

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/VincentApta/Kicca/api/internal/models"
)

// moveTaskViaAPI: cookie POST /api/tasks/:id/move to a column.
func moveTaskViaAPI(t *testing.T, app *fiber.App, cookie, taskID, status string) (int, map[string]interface{}) {
	t.Helper()
	s, body, _ := do(t, app, http.MethodPost, "/api/tasks/"+taskID+"/move", `{"status":"`+status+`"}`, cookie)
	return s, body
}

// taskEventsFor loads the transition log ordered oldest-first.
func taskEventsFor(t *testing.T, gdb *gorm.DB, taskID string) []models.TaskEvent {	t.Helper()
	var evs []models.TaskEvent
	if err := gdb.Where("task_id = ?", taskID).Order("at").Find(&evs).Error; err != nil {
		t.Fatalf("load task_events: %v", err)
	}
	return evs
}

// TestStartedAtFirstTouch: first move into in_progress/review stamps
// started_at; later moves (incl. back into in_progress) never overwrite it.
func TestStartedAtFirstTouch(t *testing.T) {
	app, _ := newTestApp(t)
	f := newProjectFixture(t, app)
	id := mkTask(t, app, f.pm, f.projectID, "Stamp me")

	status, body := moveTaskViaAPI(t, app, f.pm, id, "in_progress")
	if status != http.StatusOK {
		t.Fatalf("move in_progress: got %d", status)
	}
	first, _ := body["started_at"].(string)
	if first == "" {
		t.Fatalf("started_at should be stamped: %v", body["started_at"])
	}

	// review keeps the original stamp, and a later return to in_progress
	// does not re-stamp (first touch wins)
	for _, col := range []string{"review", "backlog", "in_progress"} {
		if status, _ := moveTaskViaAPI(t, app, f.pm, id, col); status != http.StatusOK {
			t.Fatalf("move %s: got %d", col, status)
		}
	}
	_, body, _ = do(t, app, http.MethodGet, "/api/tasks/"+id, "", f.pm)
	if s, _ := body["started_at"].(string); s != first {
		t.Fatalf("started_at re-stamped: got %v, want %v", s, first)
	}
}

// TestDoneAtSetClearRestamp: done stamps done_at, leaving done clears it,
// re-entering done re-stamps.
func TestDoneAtSetClearRestamp(t *testing.T) {
	app, _ := newTestApp(t)
	f := newProjectFixture(t, app)
	id := mkTask(t, app, f.pm, f.projectID, "Done cycle")

	if status, body := moveTaskViaAPI(t, app, f.pm, id, "done"); status != http.StatusOK || body["done_at"] == nil {
		t.Fatalf("first done: got %d done_at=%v", status, body["done_at"])
	}
	_, firstBody, _ := do(t, app, http.MethodGet, "/api/tasks/"+id, "", f.pm)
	first, _ := firstBody["done_at"].(string)

	if status, body := moveTaskViaAPI(t, app, f.pm, id, "backlog"); status != http.StatusOK || body["done_at"] != nil {
		t.Fatalf("reopen: got %d done_at=%v, want null", status, body["done_at"])
	}

	if status, body := moveTaskViaAPI(t, app, f.pm, id, "done"); status != http.StatusOK || body["done_at"] == nil {
		t.Fatalf("re-done: got %d done_at=%v", status, body["done_at"])
	} else if s, _ := body["done_at"].(string); s == first {
		t.Fatal("re-done must re-stamp done_at, got identical timestamp")
	}
}

// TestTaskEventsPerTransition: one row per status change, the create event
// has from_status NULL, PATCH status changes log too.
func TestTaskEventsPerTransition(t *testing.T) {
	app, gdb := newTestApp(t)
	f := newProjectFixture(t, app)
	id := mkTask(t, app, f.pm, f.projectID, "Events")
	for _, col := range []string{"in_progress", "review", "done", "backlog"} {
		if status, body := moveTaskViaAPI(t, app, f.pm, id, col); status != http.StatusOK {
			t.Fatalf("move %s: got %d %v", col, status, body)
		}
	}
	if status, body, _ := do(t, app, http.MethodPatch, "/api/tasks/"+id, `{"status":"blocked"}`, f.pm); status != http.StatusOK {
		t.Fatalf("patch status: got %d %v", status, body)
	}

	evs := taskEventsFor(t, gdb, id)
	want := [][2]*string{
		{nil, sp("backlog")}, // create = first event from NULL
		{sp("backlog"), sp("in_progress")},
		{sp("in_progress"), sp("review")},
		{sp("review"), sp("done")},
		{sp("done"), sp("backlog")}, // reopen
		{sp("backlog"), sp("blocked")},
	}
	if len(evs) != len(want) {
		t.Fatalf("events: got %d rows, want %d (%+v)", len(evs), len(want), evs)
	}
	for i, w := range want {
		if !eqStrPtr(evs[i].FromStatus, w[0]) || !eqStrPtr(&evs[i].ToStatus, w[1]) {
			t.Fatalf("event %d: got %+v → %+v, want %v → %v", i, deref(evs[i].FromStatus), evs[i].ToStatus, deref(w[0]), deref(w[1]))
		}
		if evs[i].ActorID != f.pmID || evs[i].TaskID != id {
			t.Fatalf("event %d: actor/task mismatch: %+v", i, evs[i])
		}
	}

	// same-status move writes nothing extra
	if status, _ := moveTaskViaAPI(t, app, f.pm, id, "blocked"); status != http.StatusOK {
		t.Fatalf("same-status move: got %d", status)
	}
	if n := len(taskEventsFor(t, gdb, id)); n != len(want) {
		t.Fatalf("same-status move should not log: got %d events", n)
	}
}

// TestTaskTypeEstimate: create/patch validation (type enum, estimate >= 0),
// wire fields, MyTasks feed payload.
func TestTaskTypeEstimate(t *testing.T) {
	app, _ := newTestApp(t)
	f := newProjectFixture(t, app)

	// defaults: type task, estimate null
	status, body := createTaskViaAPI(t, app, f.pm, f.projectID, `{"title":"Plain"}`)
	if status != http.StatusCreated || body["type"] != "task" || body["estimate"] != nil {
		t.Fatalf("defaults: got %d type=%v estimate=%v", status, body["type"], body["estimate"])
	}

	// explicit values round-trip (assigned to pm so the feed finds it)
	status, body = createTaskViaAPI(t, app, f.pm, f.projectID, `{"title":"Bug","type":"bug","estimate":3,"assignee_id":"`+f.pmID+`"}`)
	if status != http.StatusCreated || body["type"] != "bug" || body["estimate"] != float64(3) {
		t.Fatalf("explicit: got %d %v", status, body)
	}
	id := body["id"].(string)

	// invalid create → 422
	for _, payload := range []string{
		`{"title":"X","type":"epic"}`,
		`{"title":"X","estimate":-1}`,
	} {
		if status, body := createTaskViaAPI(t, app, f.pm, f.projectID, payload); status != http.StatusUnprocessableEntity {
			t.Fatalf("create %s: got %d %v, want 422", payload, status, body)
		}
	}
	// invalid patch → 422
	for _, payload := range []string{`{"type":"epic"}`, `{"estimate":-5}`, `{"estimate":"lots"}`} {
		if status, body, _ := do(t, app, http.MethodPatch, "/api/tasks/"+id, payload, f.pm); status != http.StatusUnprocessableEntity {
			t.Fatalf("patch %s: got %d %v, want 422", payload, status, body)
		}
	}

	// valid patch + null clear
	status, body, _ = do(t, app, http.MethodPatch, "/api/tasks/"+id, `{"type":"chore","estimate":8}`, f.pm)
	if status != http.StatusOK || body["type"] != "chore" || body["estimate"] != float64(8) {
		t.Fatalf("patch: got %d %v", status, body)
	}
	status, body, _ = do(t, app, http.MethodPatch, "/api/tasks/"+id, `{"estimate":null}`, f.pm)
	if status != http.StatusOK || body["estimate"] != nil {
		t.Fatalf("estimate null clear: got %d %v", status, body)
	}

	// MyTasks feed carries the new fields
	status, body, _ = do(t, app, http.MethodGet, "/api/tasks?assignee_id="+f.pmID, "", f.pm)
	if status != http.StatusOK {
		t.Fatalf("feed: got %d", status)
	}
	data, _ := body["data"].([]interface{})
	if len(data) == 0 {
		t.Fatal("feed empty")
	}
	fed := rowMap(t, data[0])
	for _, k := range []string{"type", "estimate", "started_at", "done_at"} {
		if _, ok := fed[k]; !ok {
			t.Fatalf("feed row missing %s: %v", k, fed)
		}
	}
}

func sp(s string) *string { return &s }

func deref(p *string) string {
	if p == nil {
		return "<null>"
	}
	return *p
}

func eqStrPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
