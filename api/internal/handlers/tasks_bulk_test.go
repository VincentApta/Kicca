// T-bulk integration tests: PATCH /api/tasks/bulk (issue #46) — move,
// assign/unassign, trash, all-or-nothing validation, visibility scoping,
// analytics stamps. Same harness as tasks_test.go.
package handlers

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func bulk(t *testing.T, app *fiber.App, cookie, payload string) (int, map[string]interface{}) {
	t.Helper()
	status, body, _ := do(t, app, http.MethodPatch, "/api/tasks/bulk", payload, cookie)
	return status, body
}

func bulkDetails(t *testing.T, body map[string]interface{}) map[string]string {
	t.Helper()
	e, _ := body["error"].(map[string]interface{})
	out := map[string]string{}
	if ds, ok := e["details"].([]interface{}); ok {
		for _, d := range ds {
			m, _ := d.(map[string]interface{})
			out[m["task_id"].(string)] = m["message"].(string)
		}
	}
	return out
}

func TestBulkMoveAssignTrash(t *testing.T) {
	app, _ := newTestApp(t)
	f := newProjectFixture(t, app)

	a := mkTask(t, app, f.pm, f.projectID, "A") // backlog 1024
	b := mkTask(t, app, f.pm, f.projectID, "B") // backlog 2048
	c := mkTask(t, app, f.pm, f.projectID, "C") // backlog 3072

	get := func(id string) map[string]interface{} {
		t.Helper()
		status, body, _ := do(t, app, http.MethodGet, "/api/tasks/"+id, "", f.pm)
		if status != http.StatusOK {
			t.Fatalf("get %s: %d", id, status)
		}
		return body
	}

	// bulk move to in_progress: appended at column end, 1024-spaced, in the
	// order given; started_at stamped + one task_event row each (rule 8)
	status, body := bulk(t, app, f.pm, fmt.Sprintf(`{"ids":[%q,%q],"status":"in_progress"}`, a, b))
	if status != http.StatusOK || body["updated"] != float64(2) {
		t.Fatalf("bulk move: got %d %v", status, body)
	}
	if get(a)["position"] != float64(1024) || get(b)["position"] != float64(2048) {
		t.Fatalf("bulk move positions: a=%v b=%v", get(a)["position"], get(b)["position"])
	}
	if get(a)["started_at"] == nil || get(b)["started_at"] == nil {
		t.Fatalf("started_at not stamped: %v %v", get(a)["started_at"], get(b)["started_at"])
	}

	// bulk assign + unassign (null)
	status, body = bulk(t, app, f.pm, fmt.Sprintf(`{"ids":[%q,%q,%q],"assignee_id":%q}`, a, b, c, f.paID))
	if status != http.StatusOK || body["updated"] != float64(3) {
		t.Fatalf("bulk assign: got %d %v", status, body)
	}
	if get(c)["assignee"].(map[string]interface{})["id"] != f.paID {
		t.Fatalf("assignee not set: %v", get(c)["assignee"])
	}
	if status, _ = bulk(t, app, f.pm, fmt.Sprintf(`{"ids":[%q],"assignee_id":null}`, c)); status != http.StatusOK {
		t.Fatalf("bulk unassign: got %d", status)
	}
	if get(c)["assignee"] != nil {
		t.Fatalf("assignee not cleared: %v", get(c)["assignee"])
	}
	// combined move+assign in one call
	if status, _ = bulk(t, app, f.pm, fmt.Sprintf(`{"ids":[%q],"status":"done","assignee_id":%q}`, c, f.paID)); status != http.StatusOK {
		t.Fatalf("combined: got %d", status)
	}
	row := get(c)
	if row["status"] != "done" || row["done_at"] == nil || row["assignee"].(map[string]interface{})["id"] != f.paID {
		t.Fatalf("combined fields: %v", row)
	}

	// bulk trash: soft delete semantics (status trash + deleted_at), reachable
	// via status=trash, no row in default list
	if status, _ = bulk(t, app, f.pm, fmt.Sprintf(`{"ids":[%q,%q],"status":"trash"}`, a, b)); status != http.StatusOK {
		t.Fatalf("bulk trash: got %d", status)
	}
	if rows := listTaskRows(t, app, f.pm, f.projectID, ""); len(rows) != 1 || rowMap(t, rows[0])["id"] != c {
		t.Fatalf("default list after bulk trash: %v", rows)
	}
	if rows := listTaskRows(t, app, f.pm, f.projectID, "?status=trash"); len(rows) != 2 {
		t.Fatalf("trash list: %v", rows)
	}
	// restore keeps working after bulk trash
	if status, _, _ := do(t, app, http.MethodPost, "/api/tasks/"+a+"/restore", "", f.pm); status != http.StatusOK {
		t.Fatalf("restore after bulk trash: got %d", status)
	}
}

func TestBulkValidationAllOrNothing(t *testing.T) {
	app, _ := newTestApp(t)
	f := newProjectFixture(t, app)

	a := mkTask(t, app, f.pm, f.projectID, "A")
	// a task in another project the caller cannot see
	status, p2, _ := do(t, app, http.MethodPost, "/api/projects",
		`{"team_id":"`+f.teamID+`","name":"Other","key":"OTH"}`, f.admin)
	otherTask := mkTask(t, app, f.admin, p2["id"].(string), "Invisible")

	// mixed visible + invisible → 422 with per-task details, nothing changed
	status, body := bulk(t, app, f.pm, fmt.Sprintf(`{"ids":[%q,%q],"status":"done"}`, a, otherTask))
	if status != http.StatusUnprocessableEntity || errCode(t, body) != "validation_failed" {
		t.Fatalf("mixed visibility: got %d %v, want 422", status, body)
	}
	details := bulkDetails(t, body)
	if len(details) != 1 || details[otherTask] != "no such task" {
		t.Fatalf("details: %v", details)
	}
	if get := listTaskRows(t, app, f.pm, f.projectID, "?status=done"); len(get) != 0 {
		t.Fatalf("all-or-nothing violated: %v", get)
	}

	// unknown id and trashed task are reported per-task
	trashed := mkTask(t, app, f.pm, f.projectID, "Trash me")
	if status, _, _ = do(t, app, http.MethodDelete, "/api/tasks/"+trashed, "", f.pm); status != http.StatusNoContent {
		t.Fatalf("trash: got %d", status)
	}
	status, body = bulk(t, app, f.pm, fmt.Sprintf(`{"ids":[%q,%q],"status":"backlog"}`,
		"00000000-0000-0000-0000-0000000000ff", trashed))
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown+trashed: got %d %v", status, body)
	}
	details = bulkDetails(t, body)
	if details["00000000-0000-0000-0000-0000000000ff"] != "no such task" ||
		details[trashed] != "task is in trash — restore it first" {
		t.Fatalf("details: %v", details)
	}

	// request-level validation → 422
	for name, payload := range map[string]string{
		"empty ids":        `{"ids":[],"status":"done"}`,
		"dup ids":          fmt.Sprintf(`{"ids":[%q,%q],"status":"done"}`, a, a),
		"no action":        fmt.Sprintf(`{"ids":[%q]}`, a),
		"bad status":       fmt.Sprintf(`{"ids":[%q],"status":"nope"}`, a),
		"bad assignee":     fmt.Sprintf(`{"ids":[%q],"assignee_id":"not-a-uuid"}`, a),
		"unknown assignee": fmt.Sprintf(`{"ids":[%q],"assignee_id":%q}`, a, "00000000-0000-0000-0000-000000000000"),
		"bad json":         `{`,
	} {
		if status, _ := bulk(t, app, f.pm, payload); status != http.StatusUnprocessableEntity && status != http.StatusBadRequest {
			t.Fatalf("%s: got %d, want 422/400", name, status)
		}
	}

	// outsider with a fully invisible set → 422 (reported failures, no leak)
	if status, body = bulk(t, app, f.out, fmt.Sprintf(`{"ids":[%q],"status":"done"}`, a)); status != http.StatusUnprocessableEntity ||
		bulkDetails(t, body)[a] != "no such task" {
		t.Fatalf("outsider: got %d %v", status, body)
	}

	// member of the project can bulk-act; ids may span visible projects
	if status, _ = bulk(t, app, f.admin, fmt.Sprintf(`{"ids":[%q,%q],"status":"review"}`, a, otherTask)); status != http.StatusOK {
		t.Fatalf("admin cross-project: got %d", status)
	}
}
