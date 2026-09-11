// #34 task attachment tests: team upload/list/stream/delete, client mirrors
// with own-ticket isolation, validation (sniffed type, size cap), GitHub
// issue embedding incl. the skip-don't-fail rule. LocalStore under a temp
// ATTACHMENTS_DIR; same harness as routes_test.go.
package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/VincentApta/Kica/api/internal/models"
)

// pngBytes: minimal payload whose magic sniffs as image/png.
var pngBytes = append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, bytes.Repeat([]byte("pngdata"), 8)...)

// doStream fires an arbitrary request and returns status, parsed JSON (when
// the response is JSON) and the raw body.
func doStream(t *testing.T, app *fiber.App, method, path, cookie string) (int, map[string]interface{}, []byte, http.Header) {
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
	var out map[string]interface{}
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		json.Unmarshal(raw, &out)
	}
	return resp.StatusCode, out, raw, resp.Header
}

// uploadPng: happy-path image upload helper, returns the created row JSON.
func uploadPng(t *testing.T, app *fiber.App, path, cookie, name string) map[string]interface{} {
	t.Helper()
	status, body, _ := doUpload(t, app, path, cookie, name, pngBytes)
	if status != http.StatusCreated {
		t.Fatalf("upload %s: got %d %v", path, status, body)
	}
	return body
}

// doUpload fires a multipart POST with one `file` part. The part's declared
// content type is octet-stream on purpose — the handler must sniff, not trust.
func doUpload(t *testing.T, app *fiber.App, path, cookie, filename string, content []byte) (int, map[string]interface{}, []byte) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("multipart part: %v", err)
	}
	fw.Write(content)
	w.Close()
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	return doReq(t, app, req)
}

func doReq(t *testing.T, app *fiber.App, req *http.Request) (int, map[string]interface{}, []byte) {
	t.Helper()
	resp, err := app.Test(req, 10_000)
	if err != nil {
		t.Fatalf("%s %s: %v", req.Method, req.URL.Path, err)
	}
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		json.Unmarshal(raw, &out)
	}
	return resp.StatusCode, out, raw
}

// TestAttachmentTeamLifecycle: member uploads on a visible task, lists, the
// blob streams back with the sniffed content type, delete removes row+blob.
// Outsider (no project visibility) gets 404 no-leak.
func TestAttachmentTeamLifecycle(t *testing.T) {
	t.Setenv("ATTACHMENTS_DIR", t.TempDir())
	app, gdb := newTestApp(t)
	f := newProjectFixture(t, app)
	taskID := mkTask(t, app, f.pm, f.projectID, "With attachments")

	// outsider → 404 no-leak
	if status, body, _ := doUpload(t, app, "/api/tasks/"+taskID+"/attachments", f.out, "shot.png", pngBytes); status != http.StatusNotFound {
		t.Fatalf("outsider upload: got %d %v, want 404", status, body)
	}

	created := uploadPng(t, app, "/api/tasks/"+taskID+"/attachments", f.pm, "shot.png")
	attID := created["id"].(string)
	if created["content_type"] != "image/png" || created["filename"] != "shot.png" ||
		created["size_bytes"] != float64(len(pngBytes)) || created["task_id"] != taskID ||
		created["created_by"] != f.pmID {
		t.Fatalf("upload payload: %v", created)
	}

	// sniffing wins over the declared part content type
	if status, body, _ := doUpload(t, app, "/api/tasks/"+taskID+"/attachments", f.pm,
		"evil.png", []byte("<html>not an image</html>")); status != http.StatusUnprocessableEntity || errCode(t, body) != "validation_failed" {
		t.Fatalf("text-as-png: got %d %v, want 422", status, body)
	}

	status, body, _, _ := doStream(t, app, http.MethodGet, "/api/tasks/"+taskID+"/attachments", f.pa)
	if status != http.StatusOK {
		t.Fatalf("list: got %d", status)
	}
	data, _ := body["data"].([]interface{})
	if len(data) != 1 || rowMap(t, data[0])["id"] != attID {
		t.Fatalf("list rows: %v", body["data"])
	}

	// stream: bytes round-trip, sniffed type
	status, _, raw, h := doStream(t, app, http.MethodGet, "/api/attachments/"+attID, f.pm)
	if status != http.StatusOK {
		t.Fatalf("stream: got %d", status)
	}
	if !bytes.Equal(raw, pngBytes) {
		t.Fatalf("stream bytes: %d bytes, want %d", len(raw), len(pngBytes))
	}
	if h.Get("Content-Type") != "image/png" {
		t.Fatalf("stream content-type: %q", h.Get("Content-Type"))
	}

	// delete (any project member) → 204, then gone
	if status, body, _ := do(t, app, http.MethodDelete, "/api/attachments/"+attID, "", f.pm); status != http.StatusNoContent {
		t.Fatalf("delete: got %d %v", status, body)
	}
	if status, _, _, _ := doStream(t, app, http.MethodGet, "/api/attachments/"+attID, f.pm); status != http.StatusNotFound {
		t.Fatalf("stream after delete: got %d", status)
	}
	var n int64
	gdb.Model(&models.TaskAttachment{}).Where("task_id = ?", taskID).Count(&n)
	if n != 0 {
		t.Fatalf("rows after delete: %d", n)
	}
}

// TestAttachmentSizeCap: ATTACHMENTS_MAX_MB bounds uploads (re-checked on
// the read body, not the declared size).
func TestAttachmentSizeCap(t *testing.T) {
	t.Setenv("ATTACHMENTS_DIR", t.TempDir())
	t.Setenv("ATTACHMENTS_MAX_MB", "1")
	app, _ := newTestApp(t)
	f := newProjectFixture(t, app)
	taskID := mkTask(t, app, f.pm, f.projectID, "Big files")

	big := bytes.Repeat(pngBytes[:8], 200_000) // >1MB of png-magic-prefixed bytes
	if status, body, _ := doUpload(t, app, "/api/tasks/"+taskID+"/attachments", f.pm, "big.png", big); status != http.StatusUnprocessableEntity || errCode(t, body) != "validation_failed" {
		t.Fatalf("oversize: got %d %v, want 422", status, body)
	}
}

// TestClientTicketAttachmentsIsolation: client uploads/lists on own tickets
// (minimal wire shape), cannot touch others' tickets or team-created
// attachments; team surfaces stay 403.
func TestClientTicketAttachmentsIsolation(t *testing.T) {
	t.Setenv("ATTACHMENTS_DIR", t.TempDir())
	app, gdb := newTestApp(t)
	f := newProjectFixture(t, app)
	c1, _ := mkClient(t, app, f, "c1@example.com", "c1-pass-1")
	c2, _ := mkClient(t, app, f, "c2@example.com", "c2-pass-1")

	status, mine := clientTicketViaAPI(t, app, c1, f.projectID, "Mine")
	if status != http.StatusCreated {
		t.Fatalf("c1 ticket: %d", status)
	}
	mineID := mine["id"].(string)
	_, theirs := clientTicketViaAPI(t, app, c2, f.projectID, "Theirs")
	theirsID := theirs["id"].(string)

	// own ticket → 201 with the minimal client shape
	created := uploadPng(t, app, "/api/client/tickets/"+mineID+"/attachments", c1, "evidence.png")
	for _, leaked := range []string{"created_by", "task_id", "storage", "object_key"} {
		if _, ok := created[leaked]; ok {
			t.Fatalf("client attachment JSON leaks %q: %v", leaked, created)
		}
	}

	// someone else's ticket → 404 no-leak
	if status, body, _ := doUpload(t, app, "/api/client/tickets/"+theirsID+"/attachments", c1, "x.png", pngBytes); status != http.StatusNotFound {
		t.Fatalf("other client's ticket: got %d %v, want 404", status, body)
	}

	// team-uploaded attachment on the same ticket is invisible to the client
	teamAtt := uploadPng(t, app, "/api/tasks/"+mineID+"/attachments", f.pm, "internal.png")
	if status, _, _, _ := doStream(t, app, http.MethodGet, "/api/attachments/"+teamAtt["id"].(string), c1); status != http.StatusNotFound {
		t.Fatalf("client reads team attachment: got %d, want 404", status)
	}
	// but their own streams fine
	if status, _, _, _ := doStream(t, app, http.MethodGet, "/api/attachments/"+created["id"].(string), c1); status != http.StatusOK {
		t.Fatalf("client streams own attachment: got %d", status)
	}

	// client list: own row present
	status, body, _, _ := doStream(t, app, http.MethodGet, "/api/client/tickets/"+mineID+"/attachments", c1)
	if status != http.StatusOK {
		t.Fatalf("client list: got %d", status)
	}
	data, _ := body["data"].([]interface{})
	if len(data) != 1 || rowMap(t, data[0])["filename"] != "evidence.png" {
		t.Fatalf("client list rows: %v", body["data"])
	}

	// client delete → 403 (route is team-only)
	if status, body, _ := do(t, app, http.MethodDelete, "/api/attachments/"+created["id"].(string), "", c1); status != http.StatusForbidden {
		t.Fatalf("client delete: got %d %v, want 403", status, body)
	}
	// trashed ticket disappears from the client surface (soft-delete scope)
	if err := gdb.Delete(&models.Task{}, "id = ?", mineID).Error; err != nil {
		t.Fatalf("trash ticket: %v", err)
	}
	if status, _, _, _ := doStream(t, app, http.MethodGet, "/api/client/tickets/"+mineID+"/attachments", c1); status != http.StatusNotFound {
		t.Fatalf("trashed ticket attachments: got %d, want 404", status)
	}
}

// TestGithubIssueSkipsLocalAttachments: local storage has no presign, so
// attachments never land in the issue body (GitHub can't fetch them);
// the issue itself is still created.
func TestGithubIssueSkipsLocalAttachments(t *testing.T) {
	t.Setenv("ATTACHMENTS_DIR", t.TempDir())
	app, _, mock, admin, projectID, taskID := ghFixture(t, http.StatusCreated)
	uploadPng(t, app, "/api/tasks/"+taskID+"/attachments", admin, "shot.png")
	configureGithub(t, app, admin, projectID)

	status, body, _ := do(t, app, http.MethodPost, "/api/tasks/"+taskID+"/github/issue", "", admin)
	if status != http.StatusCreated {
		t.Fatalf("create issue: got %d %v", status, body)
	}
	// only the issue create is upstream — no attachment upload call (#34
	// removed: GitHub has no public upload API)
	if mock.count() != 1 {
		t.Fatalf("upstream calls: %d, want 1", mock.count())
	}
	_, path, upBody := mock.last(t)
	if path != "/repos/acme/app/issues" {
		t.Fatalf("last call path: %q", path)
	}
	if strings.Contains(upBody, "user-attachments") || strings.Contains(upBody, "shot.png") {
		t.Fatalf("local attachment must not land in issue body: %s", upBody)
	}
}
