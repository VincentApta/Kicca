// T4 integration tests: task CRUD, numbering, filters, trash/restore/purge,
// move midpoint + rebalance, comments, labels, permission scoping. Same
// harness as routes_test.go.
package handlers

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// createTaskViaAPI: cookie POST /api/projects/:id/tasks.
func createTaskViaAPI(t *testing.T, app *fiber.App, cookie, projectID, payload string) (int, map[string]interface{}) {
	t.Helper()
	status, body, _ := do(t, app, http.MethodPost, "/api/projects/"+projectID+"/tasks", payload, cookie)
	return status, body
}

// createLabelViaAPI: cookie POST /api/projects/:id/labels.
func createLabelViaAPI(t *testing.T, app *fiber.App, cookie, projectID, name, color string) (int, map[string]interface{}) {
	t.Helper()
	status, body, _ := do(t, app, http.MethodPost, "/api/projects/"+projectID+"/labels",
		`{"name":"`+name+`","color":"`+color+`"}`, cookie)
	return status, body
}

// mkTask creates one task titled tN and returns its id.
func mkTask(t *testing.T, app *fiber.App, cookie, projectID, title string) string {
	t.Helper()
	status, body := createTaskViaAPI(t, app, cookie, projectID, `{"title":"`+title+`"}`)
	if status != http.StatusCreated {
		t.Fatalf("create %s: got %d %v", title, status, body)
	}
	return body["id"].(string)
}

// taskByID lists the project's tasks and returns data rows (map by id).
func listTaskRows(t *testing.T, app *fiber.App, cookie, projectID, query string) []interface{} {
	t.Helper()
	_, body, _ := do(t, app, http.MethodGet, "/api/projects/"+projectID+"/tasks"+query, "", cookie)
	data, _ := body["data"].([]interface{})
	return data
}

func rowMap(t *testing.T, row interface{}) map[string]interface{} {
	t.Helper()
	m, _ := row.(map[string]interface{})
	return m
}

func TestTaskNumberingDefaultsAndJSON(t *testing.T) {
	app, _ := newTestApp(t)
	f := newProjectFixture(t, app)

	status, lbl := createLabelViaAPI(t, app, f.pa, f.projectID, "bug", "#ff0000")
	if status != http.StatusCreated {
		t.Fatalf("create label: got %d %v", status, lbl)
	}
	labelID := lbl["id"].(string)

	// plain create: defaults backlog/medium, position 1024, number 1
	status, body := createTaskViaAPI(t, app, f.pm, f.projectID, `{"title":"First"}`)
	if status != http.StatusCreated {
		t.Fatalf("create: got %d %v", status, body)
	}
	for k, want := range map[string]interface{}{
		"number": float64(1), "status": "backlog", "priority": "medium",
		"position": float64(1024), "project_id": f.projectID, "created_by": f.pmID,
	} {
		if body[k] != want {
			t.Fatalf("create %s: got %v, want %v", k, body[k], want)
		}
	}
	if _, isNull := body["assignee"].(map[string]interface{}); isNull {
		t.Fatalf("assignee should be null: %v", body["assignee"])
	}
	if body["gh_link"] != nil {
		t.Fatalf("gh_link should be null placeholder: %v", body["gh_link"])
	}
	if labels := body["labels"].([]interface{}); len(labels) != 0 {
		t.Fatalf("labels should be empty array: %v", body["labels"])
	}

	// full create: explicit fields, labels, assignee, due date; number 2,
	// first task in the inbox column → position 1024 there
	payload := fmt.Sprintf(`{"title":"Second","description":"desc text","status":"inbox","priority":"urgent",`+
		`"assignee_id":%q,"due_date":"2030-01-02","label_ids":[%q]}`, f.paID, labelID)
	status, body = createTaskViaAPI(t, app, f.pm, f.projectID, payload)
	if status != http.StatusCreated {
		t.Fatalf("create full: got %d %v", status, body)
	}
	if body["number"] != float64(2) || body["position"] != float64(1024) || body["description"] != "desc text" {
		t.Fatalf("create full fields: %v", body)
	}
	if body["due_date"] != "2030-01-02" {
		t.Fatalf("due_date: got %v", body["due_date"])
	}
	assignee, _ := body["assignee"].(map[string]interface{})
	if assignee == nil || assignee["id"] != f.paID || assignee["email"] != "pa@example.com" {
		t.Fatalf("assignee object: %v", body["assignee"])
	}
	if labels := body["labels"].([]interface{}); len(labels) != 1 ||
		rowMap(t, labels[0])["name"] != "bug" || rowMap(t, labels[0])["color"] != "#ff0000" {
		t.Fatalf("labels: %v", body["labels"])
	}

	// third backlog task appends after the first
	if _, body = createTaskViaAPI(t, app, f.pm, f.projectID, `{"title":"Third"}`); body["number"] != float64(3) ||
		body["position"] != float64(2048) {
		t.Fatalf("third: %v", body)
	}

	// numbering is per project: second project restarts at 1
	status, p2, _ := do(t, app, http.MethodPost, "/api/projects",
		`{"team_id":"`+f.teamID+`","name":"Other","key":"OTH"}`, f.admin)
	if status != http.StatusCreated {
		t.Fatalf("create project 2: got %d %v", status, p2)
	}
	if _, body = createTaskViaAPI(t, app, f.admin, p2["id"].(string), `{"title":"Other proj task"}`); body["number"] != float64(1) {
		t.Fatalf("project 2 numbering: %v", body)
	}

	// validation → 422
	for _, payload := range []string{
		`{"title":""}`,
		`{"title":"X","status":"nope"}`,
		`{"title":"X","priority":"nope"}`,
		`{"title":"X","assignee_id":"not-a-uuid"}`,
		fmt.Sprintf(`{"title":"X","assignee_id":%q}`, "00000000-0000-0000-0000-000000000000"),
		`{"title":"X","due_date":"01-02-2030"}`,
		`{"title":"X","label_ids":["not-a-uuid"]}`,
		fmt.Sprintf(`{"title":"X","label_ids":[%q,%q]}`, labelID, labelID),
	} {
		if status, body := createTaskViaAPI(t, app, f.pm, f.projectID, payload); status != http.StatusUnprocessableEntity {
			t.Fatalf("payload %s: got %d %v, want 422", payload, status, body)
		}
	}

	// outsider: list + create 404 (no leak)
	if status, _, _ := do(t, app, http.MethodGet, "/api/projects/"+f.projectID+"/tasks", "", f.out); status != http.StatusNotFound {
		t.Fatalf("outsider list: got %d, want 404", status)
	}
	if status, _ := createTaskViaAPI(t, app, f.out, f.projectID, `{"title":"X"}`); status != http.StatusNotFound {
		t.Fatalf("outsider create: got %d, want 404", status)
	}
}

func TestTaskFilters(t *testing.T) {
	app, _ := newTestApp(t)
	f := newProjectFixture(t, app)

	_, lbl := createLabelViaAPI(t, app, f.pa, f.projectID, "bug", "#ff0000")
	labelID := lbl["id"].(string)

	mk := func(title, payload string) string {
		t.Helper()
		status, body := createTaskViaAPI(t, app, f.pm, f.projectID, payload)
		if status != http.StatusCreated {
			t.Fatalf("create %s: got %d %v", title, status, body)
		}
		return body["id"].(string)
	}
	mk("A", fmt.Sprintf(`{"title":"Fix login page","priority":"urgent","assignee_id":%q}`, f.paID))
	mk("B", fmt.Sprintf(`{"title":"Write docs","status":"in_progress","priority":"low","assignee_id":%q,"label_ids":[%q]}`, f.pmID, labelID))
	mk("C", `{"title":"Harden the api gateway","description":"login flow hardening notes"}`)

	list := func(query string) map[string]interface{} {
		t.Helper()
		status, body, _ := do(t, app, http.MethodGet, "/api/projects/"+f.projectID+"/tasks"+query, "", f.pm)
		if status != http.StatusOK {
			t.Fatalf("list %s: got %d %v", query, status, body)
		}
		return body
	}
	totalOf := func(body map[string]interface{}) float64 { return body["total"].(float64) }

	// no filter → all 3
	if got := totalOf(list("")); got != 3 {
		t.Fatalf("no filter: got %v", got)
	}
	// status
	if got := totalOf(list("?status=in_progress")); got != 1 {
		t.Fatalf("status filter: got %v", got)
	}
	// invalid filter values → 422
	for _, q := range []string{"?status=nope", "?priority=nope", "?assignee_id=nope", "?label=nope"} {
		if status, body, _ := do(t, app, http.MethodGet, "/api/projects/"+f.projectID+"/tasks"+q, "", f.pm); status != http.StatusUnprocessableEntity {
			t.Fatalf("bad filter %s: got %d %v, want 422", q, status, body)
		}
	}
	// priority
	if got := totalOf(list("?priority=urgent")); got != 1 {
		t.Fatalf("priority filter: got %v", got)
	}
	// assignee
	if got := totalOf(list("?assignee_id=" + f.pmID)); got != 1 {
		t.Fatalf("assignee filter: got %v", got)
	}
	// label
	if got := totalOf(list("?label=" + labelID)); got != 1 {
		t.Fatalf("label filter: got %v", got)
	}
	// q on title and description, case-insensitive
	if got := totalOf(list("?q=LOGIN")); got != 2 { // "Fix login page" + "login flow hardening notes"
		t.Fatalf("q filter: got %v", got)
	}
	if got := totalOf(list("?q=gateway")); got != 1 {
		t.Fatalf("q on description: got %v", got)
	}
	if got := totalOf(list("?q=nomatch")); got != 0 {
		t.Fatalf("q no match: got %v", got)
	}
	// combo: status + priority
	if got := totalOf(list("?status=in_progress&priority=low")); got != 1 {
		t.Fatalf("combo filter: got %v", got)
	}
	if got := totalOf(list("?status=in_progress&priority=urgent")); got != 0 {
		t.Fatalf("combo filter empty: got %v", got)
	}
	// pagination
	body := list("?per_page=2&page=2")
	if body["total"] != float64(3) || body["page"] != float64(2) || body["per_page"] != float64(2) {
		t.Fatalf("pagination meta: %v", body)
	}
	if data := body["data"].([]interface{}); len(data) != 1 || rowMap(t, data[0])["title"] != "Harden the api gateway" {
		t.Fatalf("page 2 rows: %v", body["data"])
	}
}

func TestTrashRestorePurge(t *testing.T) {
	app, _ := newTestApp(t)
	f := newProjectFixture(t, app)

	id1 := mkTask(t, app, f.pm, f.projectID, "Keep me")
	id2 := mkTask(t, app, f.pm, f.projectID, "Trash me")

	// member trashes (204); task keeps its number and is reachable with
	// status=trash
	if status, _, _ := do(t, app, http.MethodDelete, "/api/tasks/"+id2, "", f.pm); status != http.StatusNoContent {
		t.Fatalf("trash: got %d, want 204", status)
	}
	if rows := listTaskRows(t, app, f.pm, f.projectID, ""); len(rows) != 1 || rowMap(t, rows[0])["id"] != id1 {
		t.Fatalf("default list after trash: %v", rows)
	}
	rows := listTaskRows(t, app, f.pm, f.projectID, "?status=trash")
	if len(rows) != 1 || rowMap(t, rows[0])["id"] != id2 || rowMap(t, rows[0])["status"] != "trash" {
		t.Fatalf("trash list: %v", rows)
	}
	if status, body, _ := do(t, app, http.MethodGet, "/api/tasks/"+id2, "", f.pm); status != http.StatusOK || body["status"] != "trash" {
		t.Fatalf("get trashed: got %d %v", status, body)
	}

	// patch still works on a trashed task (e.g. rename from trash view)
	if status, _, _ := do(t, app, http.MethodPatch, "/api/tasks/"+id2, `{"title":"Trash me (edited)"}`, f.pm); status != http.StatusOK {
		t.Fatalf("patch trashed: got %d, want 200", status)
	}

	// restore → backlog (rule 4), appended at column end
	status, body, _ := do(t, app, http.MethodPost, "/api/tasks/"+id2+"/restore", "", f.pm)
	if status != http.StatusOK || body["status"] != "backlog" || body["position"] != float64(2048) {
		t.Fatalf("restore: got %d %v", status, body)
	}
	if rows := listTaskRows(t, app, f.pm, f.projectID, "?status=trash"); len(rows) != 0 {
		t.Fatalf("trash list after restore: %v", rows)
	}
	if rows := listTaskRows(t, app, f.pm, f.projectID, ""); len(rows) != 2 {
		t.Fatalf("default list after restore: %v", rows)
	}

	// purge: member/project_admin → 403, global admin → gone for good
	if status, _, _ := do(t, app, http.MethodDelete, "/api/tasks/"+id2+"?purge=1", "", f.pm); status != http.StatusForbidden {
		t.Fatalf("member purge: got %d, want 403", status)
	}
	if status, _, _ := do(t, app, http.MethodDelete, "/api/tasks/"+id2+"?purge=1", "", f.pa); status != http.StatusForbidden {
		t.Fatalf("project_admin purge: got %d, want 403", status)
	}
	// comments + labels go with it
	if status, _, _ := do(t, app, http.MethodPost, "/api/tasks/"+id2+"/comments", `{"body":"c"}`, f.pm); status != http.StatusCreated {
		t.Fatalf("comment: got %d", status)
	}
	if status, _, _ := do(t, app, http.MethodDelete, "/api/tasks/"+id2+"?purge=1", "", f.admin); status != http.StatusNoContent {
		t.Fatalf("admin purge: got %d, want 204", status)
	}
	if status, body, _ := do(t, app, http.MethodGet, "/api/tasks/"+id2, "", f.admin); status != http.StatusNotFound || errCode(t, body) != "not_found" {
		t.Fatalf("get purged: got %d %v, want 404 not_found", status, body)
	}
	if rows := listTaskRows(t, app, f.admin, f.projectID, "?status=trash"); len(rows) != 0 {
		t.Fatalf("trash list after purge: %v", rows)
	}

	// outsider trash/restore → 404
	if status, _, _ := do(t, app, http.MethodDelete, "/api/tasks/"+id1, "", f.out); status != http.StatusNotFound {
		t.Fatalf("outsider trash: got %d, want 404", status)
	}
	if status, _, _ := do(t, app, http.MethodPost, "/api/tasks/"+id1+"/restore", "", f.out); status != http.StatusNotFound {
		t.Fatalf("outsider restore: got %d, want 404", status)
	}
}

func TestMoveMidpointAndRebalance(t *testing.T) {
	app, _ := newTestApp(t)
	f := newProjectFixture(t, app)

	// backlog column: A 1024, B 2048, C 3072
	aID := mkTask(t, app, f.pm, f.projectID, "A")
	bID := mkTask(t, app, f.pm, f.projectID, "B")
	cID := mkTask(t, app, f.pm, f.projectID, "C")

	getTask := func(id, cookie string) map[string]interface{} {
		t.Helper()
		status, body, _ := do(t, app, http.MethodGet, "/api/tasks/"+id, "", cookie)
		if status != http.StatusOK {
			t.Fatalf("get %s: got %d", id, status)
		}
		return body
	}
	posOf := func(id string) float64 { return getTask(id, f.pm)["position"].(float64) }

	// D before B → midpoint (1024+2048)/2 = 1536, no rebalance
	dID := mkTask(t, app, f.pm, f.projectID, "D")
	status, body, _ := do(t, app, http.MethodPost, "/api/tasks/"+dID+"/move",
		fmt.Sprintf(`{"status":"backlog","before_task_id":%q}`, bID), f.pm)
	if status != http.StatusOK || body["position"] != float64(1536) {
		t.Fatalf("midpoint move: got %d %v", status, body)
	}
	if posOf(aID) != 1024 || posOf(bID) != 2048 || posOf(cID) != 3072 {
		t.Fatalf("column disturbed by midpoint move")
	}

	// after_task_id: D after C → 3072+1024 = 4096
	if status, body, _ = do(t, app, http.MethodPost, "/api/tasks/"+dID+"/move",
		fmt.Sprintf(`{"status":"backlog","after_task_id":%q}`, cID), f.pm); status != http.StatusOK ||
		body["position"] != float64(4096) {
		t.Fatalf("append move: got %d %v", status, body)
	}

	// move to another column without anchor → appended at 1024 there
	if status, body, _ = do(t, app, http.MethodPost, "/api/tasks/"+dID+"/move",
		`{"status":"in_progress"}`, f.pm); status != http.StatusOK ||
		body["status"] != "in_progress" || body["position"] != float64(1024) {
		t.Fatalf("cross-column move: got %d %v", status, body)
	}

	// squeeze via position patch (contract allows patching position), then a
	// midpoint move whose gaps would drop under 1 → whole column rebalances
	for id, pos := range map[string]float64{aID: 100, bID: 101} {
		if status, _, _ := do(t, app, http.MethodPatch, "/api/tasks/"+id,
			fmt.Sprintf(`{"position":%v}`, pos), f.admin); status != http.StatusOK {
			t.Fatalf("patch position %s: got %d", id, status)
		}
	}
	if status, body, _ = do(t, app, http.MethodPost, "/api/tasks/"+cID+"/move",
		fmt.Sprintf(`{"status":"backlog","before_task_id":%q}`, bID), f.pm); status != http.StatusOK {
		t.Fatalf("rebalance move: got %d %v", status, body)
	}
	// resulting order A, C, B rewritten 1024-spaced; D (other column) untouched
	if pos := posOf(aID); pos != 1024 {
		t.Fatalf("rebalance A: got %v, want 1024", pos)
	}
	if pos := posOf(cID); pos != 2048 {
		t.Fatalf("rebalance C: got %v, want 2048", pos)
	}
	if pos := posOf(bID); pos != 3072 {
		t.Fatalf("rebalance B: got %v, want 3072", pos)
	}
	if pos := posOf(dID); pos != 1024 {
		t.Fatalf("other column disturbed: got %v", pos)
	}

	// validation → 422
	for _, tc := range []struct{ name, payload string }{
		{"bad status", `{"status":"nope"}`},
		{"trash via move", `{"status":"trash"}`},
		{"both anchors", fmt.Sprintf(`{"status":"backlog","before_task_id":%q,"after_task_id":%q}`, aID, bID)},
		{"anchor not uuid", `{"status":"backlog","before_task_id":"nope"}`},
		{"anchor other column", fmt.Sprintf(`{"status":"review","before_task_id":%q}`, aID)},
		{"anchor itself", fmt.Sprintf(`{"status":"backlog","before_task_id":%q}`, cID)},
	} {
		if status, body, _ := do(t, app, http.MethodPost, "/api/tasks/"+cID+"/move", tc.payload, f.pm); status != http.StatusUnprocessableEntity {
			t.Fatalf("%s: got %d %v, want 422", tc.name, status, body)
		}
	}
	// anchor from another project → 422
	status, p2, _ := do(t, app, http.MethodPost, "/api/projects",
		`{"team_id":"`+f.teamID+`","name":"Other","key":"OTH"}`, f.admin)
	otherTask := mkTask(t, app, f.admin, p2["id"].(string), "Other proj")
	if status, _, _ := do(t, app, http.MethodPost, "/api/tasks/"+cID+"/move",
		fmt.Sprintf(`{"status":"backlog","before_task_id":%q}`, otherTask), f.pm); status != http.StatusUnprocessableEntity {
		t.Fatalf("anchor other project: got %d, want 422", status)
	}

	// outsider move → 404
	if status, _, _ := do(t, app, http.MethodPost, "/api/tasks/"+aID+"/move", `{"status":"backlog"}`, f.out); status != http.StatusNotFound {
		t.Fatalf("outsider move: got %d, want 404", status)
	}
}

func TestPatchTask(t *testing.T) {
	app, _ := newTestApp(t)
	f := newProjectFixture(t, app)

	_, l1 := createLabelViaAPI(t, app, f.pa, f.projectID, "one", "#111111")
	_, l2 := createLabelViaAPI(t, app, f.pa, f.projectID, "two", "#222222")

	payload := fmt.Sprintf(`{"title":"T","assignee_id":%q,"due_date":"2030-06-01","label_ids":[%q]}`,
		f.paID, l1["id"].(string))
	status, body := createTaskViaAPI(t, app, f.pm, f.projectID, payload)
	if status != http.StatusCreated {
		t.Fatalf("create: got %d %v", status, body)
	}
	id := body["id"].(string)

	// partial patch: title, priority, description
	if status, body, _ = do(t, app, http.MethodPatch, "/api/tasks/"+id,
		`{"title":"T2","priority":"high","description":"d"}`, f.pa); status != http.StatusOK ||
		body["title"] != "T2" || body["priority"] != "high" || body["description"] != "d" ||
		body["status"] != "backlog" || body["assignee"].(map[string]interface{})["id"] != f.paID {
		t.Fatalf("partial patch: got %d %v", status, body)
	}

	// null clears assignee + due_date; empty label_ids replaces the set
	patch := fmt.Sprintf(`{"assignee_id":null,"due_date":null,"label_ids":[%q]}`, l2["id"].(string))
	if status, body, _ = do(t, app, http.MethodPatch, "/api/tasks/"+id, patch, f.pm); status != http.StatusOK ||
		body["assignee"] != nil || body["due_date"] != nil {
		t.Fatalf("null clear: got %d %v", status, body)
	}
	if labels := body["labels"].([]interface{}); len(labels) != 1 || rowMap(t, labels[0])["name"] != "two" {
		t.Fatalf("label replace: %v", body["labels"])
	}
	if status, body, _ = do(t, app, http.MethodPatch, "/api/tasks/"+id, `{"label_ids":[]}`, f.pm); status != http.StatusOK ||
		len(body["labels"].([]interface{})) != 0 {
		t.Fatalf("label clear: got %d %v", status, body)
	}

	// status via patch (simple state flip; repositioning is /move)
	if status, body, _ = do(t, app, http.MethodPatch, "/api/tasks/"+id, `{"status":"done"}`, f.pm); status != http.StatusOK ||
		body["status"] != "done" {
		t.Fatalf("status patch: got %d %v", status, body)
	}

	// bad values → 422
	for _, p := range []string{
		`{"title":""}`, `{"status":"nope"}`, `{"priority":"nope"}`,
		`{"assignee_id":"nope"}`, `{"due_date":"2030/01/02"}`,
		fmt.Sprintf(`{"label_ids":[%q]}`, "00000000-0000-0000-0000-000000000000"),
	} {
		if status, body, _ := do(t, app, http.MethodPatch, "/api/tasks/"+id, p, f.pm); status != http.StatusUnprocessableEntity {
			t.Fatalf("patch %s: got %d %v, want 422", p, status, body)
		}
	}

	// outsider patch → 404; unknown task → 404
	if status, _, _ := do(t, app, http.MethodPatch, "/api/tasks/"+id, `{"title":"X"}`, f.out); status != http.StatusNotFound {
		t.Fatalf("outsider patch: got %d, want 404", status)
	}
	if status, _, _ := do(t, app, http.MethodGet, "/api/tasks/"+id, "", f.out); status != http.StatusNotFound {
		t.Fatalf("outsider get: got %d, want 404", status)
	}
	if status, _, _ := do(t, app, http.MethodGet, "/api/tasks/not-a-uuid", "", f.pm); status != http.StatusBadRequest {
		t.Fatalf("bad uuid: got %d, want 400", status)
	}
	if status, _, _ := do(t, app, http.MethodGet, "/api/tasks/"+uuid.NewString(), "", f.pm); status != http.StatusNotFound {
		t.Fatalf("unknown task: got %d, want 404", status)
	}
}

func TestComments(t *testing.T) {
	app, _ := newTestApp(t)
	f := newProjectFixture(t, app)
	id := mkTask(t, app, f.pm, f.projectID, "Has comments")

	if status, _, _ := do(t, app, http.MethodPost, "/api/tasks/"+id+"/comments", `{"body":"first"}`, f.pm); status != http.StatusCreated {
		t.Fatalf("comment pm: got %d", status)
	}
	status, created, _ := do(t, app, http.MethodPost, "/api/tasks/"+id+"/comments", `{"body":"second"}`, f.pa)
	if status != http.StatusCreated || created["user_id"] != f.paID || created["task_id"] != id || created["body"] != "second" {
		t.Fatalf("comment pa: got %d %v", status, created)
	}

	status, body, _ := do(t, app, http.MethodGet, "/api/tasks/"+id+"/comments", "", f.pm)
	if status != http.StatusOK {
		t.Fatalf("list: got %d", status)
	}
	data := body["data"].([]interface{})
	if len(data) != 2 || rowMap(t, data[0])["body"] != "first" || rowMap(t, data[1])["body"] != "second" {
		t.Fatalf("comment order: %v", data)
	}

	// empty body → 422; outsider → 404
	if status, _, _ := do(t, app, http.MethodPost, "/api/tasks/"+id+"/comments", `{"body":"  "}`, f.pm); status != http.StatusUnprocessableEntity {
		t.Fatalf("empty body: got %d, want 422", status)
	}
	if status, _, _ := do(t, app, http.MethodGet, "/api/tasks/"+id+"/comments", "", f.out); status != http.StatusNotFound {
		t.Fatalf("outsider list: got %d, want 404", status)
	}
	if status, _, _ := do(t, app, http.MethodPost, "/api/tasks/"+id+"/comments", `{"body":"x"}`, f.out); status != http.StatusNotFound {
		t.Fatalf("outsider create: got %d, want 404", status)
	}
}

func TestLabelsManagement(t *testing.T) {
	app, _ := newTestApp(t)
	f := newProjectFixture(t, app)

	// member reads but cannot write
	if status, _, _ := do(t, app, http.MethodGet, "/api/projects/"+f.projectID+"/labels", "", f.pm); status != http.StatusOK {
		t.Fatalf("member list labels: got %d", status)
	}
	if status, _ := createLabelViaAPI(t, app, f.pm, f.projectID, "nope", "#000000"); status != http.StatusForbidden {
		t.Fatalf("member create label: got %d, want 403", status)
	}

	// project_admin manages
	status, l1 := createLabelViaAPI(t, app, f.pa, f.projectID, "bug", "#ff0000")
	if status != http.StatusCreated || l1["name"] != "bug" || l1["color"] != "#ff0000" {
		t.Fatalf("pa create label: got %d %v", status, l1)
	}
	if status, _ := createLabelViaAPI(t, app, f.pa, f.projectID, "bug", "#00ff00"); status != http.StatusConflict {
		t.Fatalf("dup label: got %d, want 409", status)
	}
	for _, payload := range []string{
		`{"name":""}`, `{"name":"X","color":"red"}`, `{"name":"X","color":"123456"}`,
	} {
		if status, _, _ := do(t, app, http.MethodPost, "/api/projects/"+f.projectID+"/labels", payload, f.pa); status != http.StatusUnprocessableEntity {
			t.Fatalf("bad label %s: got %d, want 422", payload, status)
		}
	}

	// label delete detaches from tasks
	id := mkTask(t, app, f.pm, f.projectID, "Labeled")
	patch := fmt.Sprintf(`{"label_ids":[%q]}`, l1["id"].(string))
	if status, _, _ := do(t, app, http.MethodPatch, "/api/tasks/"+id, patch, f.pm); status != http.StatusOK {
		t.Fatalf("attach label: got %d", status)
	}
	if status, _, _ := do(t, app, http.MethodDelete, "/api/labels/"+l1["id"].(string), "", f.pm); status != http.StatusForbidden {
		t.Fatalf("member delete label: got %d, want 403", status)
	}
	if status, _, _ := do(t, app, http.MethodDelete, "/api/labels/"+l1["id"].(string), "", f.pa); status != http.StatusNoContent {
		t.Fatalf("pa delete label: got %d, want 204", status)
	}
	_, body, _ := do(t, app, http.MethodGet, "/api/tasks/"+id, "", f.pm)
	if labels := body["labels"].([]interface{}); len(labels) != 0 {
		t.Fatalf("labels after delete: %v", labels)
	}

	// label from another project: outsider to it → 404
	status, p2, _ := do(t, app, http.MethodPost, "/api/projects",
		`{"team_id":"`+f.teamID+`","name":"Other","key":"OTH"}`, f.admin)
	_, l2 := createLabelViaAPI(t, app, f.admin, p2["id"].(string), "private", "#000000")
	if status, _, _ := do(t, app, http.MethodDelete, "/api/labels/"+l2["id"].(string), "", f.pm); status != http.StatusNotFound {
		t.Fatalf("outsider delete label: got %d, want 404", status)
	}
	if status, _, _ := do(t, app, http.MethodGet, "/api/projects/"+p2["id"].(string)+"/labels", "", f.out); status != http.StatusNotFound {
		t.Fatalf("outsider list labels: got %d, want 404", status)
	}

	// label list is project-scoped
	_, body, _ = do(t, app, http.MethodGet, "/api/projects/"+f.projectID+"/labels", "", f.pm)
	if data := body["data"].([]interface{}); len(data) != 0 {
		t.Fatalf("project label list polluted: %v", data)
	}
}
