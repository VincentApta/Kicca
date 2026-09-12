// #50 CSV export tests: shape (header row, RFC4180 quoting, UTF-8 BOM),
// filters riding the same builders as the list endpoints, and visibility —
// team export follows project visibility, client export is own-tickets-only.
package handlers

import (
	"encoding/csv"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// doRaw fires a request and returns status + raw body (CSV is not JSON).
func doRaw(t *testing.T, app *fiber.App, method, path, cookie string) (int, string, http.Header) {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := app.Test(req, 10_000)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw), resp.Header
}

// readCSV strips the BOM and parses rows with encoding/csv (round-trip of the
// writer's RFC4180 quoting).
func readCSV(t *testing.T, raw string) [][]string {
	t.Helper()
	raw = strings.TrimPrefix(raw, "\xEF\xBB\xBF")
	rows, err := csv.NewReader(strings.NewReader(raw)).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	return rows
}

// TestExportTasksCSVShapeAndEscaping: header row, BOM, comma/quote/newline
// escaping, assignee + created_by names, labels joined, markdown description
// as-is.
func TestExportTasksCSVShapeAndEscaping(t *testing.T) {
	app, _ := newTestApp(t)
	f := newProjectFixture(t, app)
	_, lbl := createLabelViaAPI(t, app, f.pa, f.projectID, "urgent-fix", "#ff0000")
	labelID := lbl["id"].(string)

	// nasty content: comma, double quote, newline, markdown
	payload := `{"title":"Fix ` + "`login`" + `, the \"quoted\" one` + `\nsecond line","description":"**bold** and, commas","assignee_id":"` + f.pmID + `","label_ids":["` + labelID + `"],"estimate":3,"due_date":"2026-10-01"}`
	if status, body := createTaskViaAPI(t, app, f.pm, f.projectID, payload); status != http.StatusCreated {
		t.Fatalf("create nasty task: got %d %v", status, body)
	}
	mkTask(t, app, f.pm, f.projectID, "Plain task")

	status, raw, h := doRaw(t, app, http.MethodGet, "/api/tasks/export?project_id="+f.projectID, f.pm)
	if status != http.StatusOK {
		t.Fatalf("export: got %d %s", status, raw)
	}
	if ct := h.Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Fatalf("content-type: %q", ct)
	}
	if cd := h.Get("Content-Disposition"); !strings.Contains(cd, "attachment") || !strings.Contains(cd, "kica-kic-tasks-") {
		t.Fatalf("content-disposition: %q", cd)
	}
	if !strings.HasPrefix(raw, "\xEF\xBB\xBF") {
		t.Fatal("missing UTF-8 BOM")
	}

	rows := readCSV(t, raw)
	if len(rows) != 3 { // header + 2 tasks
		t.Fatalf("rows: got %d, want 3", len(rows))
	}
	wantHeader := []string{"id", "number", "title", "status", "priority", "type", "assignee",
		"labels", "created_by", "created_at", "updated_at", "started_at", "done_at", "estimate", "due_date", "description"}
	if strings.Join(rows[0], "|") != strings.Join(wantHeader, "|") {
		t.Fatalf("header: %v", rows[0])
	}
	// rows are ordered created_at, number → nasty task first (number 1)
	nasty := rows[1]
	if nasty[1] != "1" || !strings.Contains(nasty[2], `"quoted"`) || !strings.Contains(nasty[2], "\nsecond line") {
		t.Fatalf("escaped title: %q", nasty[2])
	}
	if nasty[6] != "pm" { // assignee name
		t.Fatalf("assignee: %q", nasty[6])
	}
	if nasty[7] != "urgent-fix" {
		t.Fatalf("labels: %q", nasty[7])
	}
	if nasty[8] != "pm" { // created_by name
		t.Fatalf("created_by: %q", nasty[8])
	}
	if nasty[13] != "3" || nasty[14] != "2026-10-01" {
		t.Fatalf("estimate/due: %q %q", nasty[13], nasty[14])
	}
	if nasty[15] != "**bold** and, commas" {
		t.Fatalf("description: %q", nasty[15])
	}
	if rows[2][1] != "2" || rows[2][6] != "" || rows[2][7] != "" {
		t.Fatalf("plain task row: %v", rows[2])
	}
}

// TestExportTasksFiltersAndVisibility: export rides the same filters as the
// list (q, status, trash semantics) and the same visibility (member ok,
// outsider 404 no-leak, bad project 422, client 403, unauth 401).
func TestExportTasksFiltersAndVisibility(t *testing.T) {
	app, _ := newTestApp(t)
	f := newProjectFixture(t, app)
	id1 := mkTask(t, app, f.pm, f.projectID, "Alpha login broken")
	mkTask(t, app, f.pm, f.projectID, "Beta css glitch")

	// q narrows
	status, raw, _ := doRaw(t, app, http.MethodGet, "/api/tasks/export?project_id="+f.projectID+"&q=login", f.pm)
	if status != http.StatusOK || len(readCSV(t, raw)) != 2 { // header + Alpha
		t.Fatalf("q filter export: got %d, rows %d", status, len(readCSV(t, raw)))
	}

	// trashed tasks excluded by default, present with status=trash
	if status, _, _ := do(t, app, http.MethodDelete, "/api/tasks/"+id1, "", f.pm); status != http.StatusNoContent {
		t.Fatalf("trash: got %d", status)
	}
	_, raw, _ = doRaw(t, app, http.MethodGet, "/api/tasks/export?project_id="+f.projectID, f.pm)
	if rows := readCSV(t, raw); len(rows) != 2 {
		t.Fatalf("default export rows: got %d, want 2 (header + Beta)", len(rows))
	}
	_, raw, _ = doRaw(t, app, http.MethodGet, "/api/tasks/export?project_id="+f.projectID+"&status=trash", f.pm)
	if rows := readCSV(t, raw); len(rows) != 2 || rows[1][1] != "1" {
		t.Fatalf("trash export rows: %v", rows)
	}

	// bad filters → 422
	for _, q := range []string{"project_id=not-a-uuid", "status=wip", "assignee_id=zzz"} {
		if status, body, _ := do(t, app, http.MethodGet, "/api/tasks/export?"+q, "", f.pm); status != http.StatusUnprocessableEntity {
			t.Fatalf("export ?%s: got %d %v, want 422", q, status, body)
		}
	}
	// outsider on someone else's project → 404 no-leak (never rows)
	if status, _, _ := doRaw(t, app, http.MethodGet, "/api/tasks/export?project_id="+f.projectID, f.out); status != http.StatusNotFound {
		t.Fatalf("outsider export: got %d, want 404", status)
	}
	// unauth → 401; client → 403 (team surface)
	if status, _, _ := doRaw(t, app, http.MethodGet, "/api/tasks/export?project_id="+f.projectID, ""); status != http.StatusUnauthorized {
		t.Fatalf("unauth export: got %d", status)
	}
	client, _ := mkClient(t, app, f, "c@example.com", "c-pass-1")
	if status, _, _ := doRaw(t, app, http.MethodGet, "/api/tasks/export?project_id="+f.projectID, client); status != http.StatusForbidden {
		t.Fatalf("client team export: got %d, want 403", status)
	}
	// no project_id → global export spans the member's visible projects only
	status, raw, _ = doRaw(t, app, http.MethodGet, "/api/tasks/export", f.pm)
	if status != http.StatusOK || len(readCSV(t, raw)) != 2 { // header + Beta (Alpha is trashed)
		t.Fatalf("global export: got %d rows %d", status, len(readCSV(t, raw)))
	}
}

// TestClientExportTicketsOwnScope: client columns (no team-only fields),
// own tickets only, same filters as the portal list, team users 403.
func TestClientExportTicketsOwnScope(t *testing.T) {
	app, _ := newTestApp(t)
	f := newProjectFixture(t, app)
	client, _ := mkClient(t, app, f, "client@example.com", "client-pass-1")
	clientTicketViaAPI(t, app, client, f.projectID, "Mine, \"important\"\nline two")
	clientTicketViaAPI(t, app, client, f.projectID, "Mine other")
	c2, _ := mkClient(t, app, f, "c2@example.com", "c2-pass-1")
	clientTicketViaAPI(t, app, c2, f.projectID, "Theirs")
	mkTask(t, app, f.pm, f.projectID, "Team task") // invisible

	status, raw, h := doRaw(t, app, http.MethodGet, "/api/client/tickets/export", client)
	if status != http.StatusOK {
		t.Fatalf("client export: got %d %s", status, raw)
	}
	if ct := h.Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Fatalf("content-type: %q", ct)
	}
	if cd := h.Get("Content-Disposition"); !strings.Contains(cd, "kica-my-tickets-") {
		t.Fatalf("content-disposition: %q", cd)
	}
	rows := readCSV(t, raw)
	if len(rows) != 3 { // header + 2 own
		t.Fatalf("rows: got %d, want 3", len(rows))
	}
	wantHeader := "id|project|number|title|status|created_at|updated_at|description"
	if strings.Join(rows[0], "|") != wantHeader {
		t.Fatalf("header: %v", rows[0])
	}
	for _, r := range rows[1:] {
		if r[1] != "KIC" {
			t.Fatalf("project column: %v", r)
		}
		if strings.Contains(r[3], "Theirs") || strings.Contains(r[3], "Team task") {
			t.Fatalf("isolation leak: %v", r)
		}
	}
	// nasty title survives the round-trip
	if !strings.Contains(rows[2][3], "Mine, \"important\"\nline two") && !strings.Contains(rows[1][3], "Mine, \"important\"\nline two") {
		t.Fatalf("escaped title: %q / %q", rows[1][3], rows[2][3])
	}

	// filters mirror the list
	_, raw, _ = doRaw(t, app, http.MethodGet, "/api/client/tickets/export?q=other", client)
	if rows := readCSV(t, raw); len(rows) != 2 {
		t.Fatalf("q filter rows: got %d, want 2", len(rows))
	}
	// team users never reach the client surface
	if status, _, _ := doRaw(t, app, http.MethodGet, "/api/client/tickets/export", f.pm); status != http.StatusForbidden {
		t.Fatalf("team client export: got %d, want 403", status)
	}
}
