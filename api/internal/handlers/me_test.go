// Integration tests (issue #49): PATCH /api/me — self-service name +
// password for the current user; current-password verification; admin 403.
// Same harness as routes_test.go.
package handlers

import (
	"net/http"
	"testing"
)

func TestPatchMe(t *testing.T) {
	app, _ := newTestApp(t)
	admin := loginAndGet(t, app, adminEmail, adminPass)
	if status, _ := createUserViaAPI(t, app, admin, memberEmail, memberPass, "member"); status != http.StatusCreated {
		t.Fatalf("create member: got %d", status)
	}

	// rename → 200, reflected on /auth/me
	status, body, _ := do(t, app, http.MethodPatch, "/api/me", `{"name":"Renamed Self"}`, loginAndGet(t, app, memberEmail, memberPass))
	if status != http.StatusOK || body["name"] != "Renamed Self" {
		t.Fatalf("rename: got %d %v", status, body)
	}

	// empty name → 422
	if status, body, _ := do(t, app, http.MethodPatch, "/api/me", `{"name":"  "}`, loginAndGet(t, app, memberEmail, memberPass)); status != http.StatusUnprocessableEntity || errCode(t, body) != "validation_failed" {
		t.Fatalf("empty name: got %d %v, want 422", status, body)
	}
}

func TestPatchMePassword(t *testing.T) {
	app, _ := newTestApp(t)
	admin := loginAndGet(t, app, adminEmail, adminPass)
	if status, _ := createUserViaAPI(t, app, admin, memberEmail, memberPass, "member"); status != http.StatusCreated {
		t.Fatalf("create member: got %d", status)
	}
	member := loginAndGet(t, app, memberEmail, memberPass)

	// missing / wrong current password → 422, hash unchanged
	for _, payload := range []string{
		`{"new_password":"new-pass-123"}`,
		`{"current_password":"wrong","new_password":"new-pass-123"}`,
	} {
		if status, body, _ := do(t, app, http.MethodPatch, "/api/me", payload, member); status != http.StatusUnprocessableEntity {
			t.Fatalf("%s: got %d %v, want 422", payload, status, body)
		}
	}
	if status, body, _ := do(t, app, http.MethodPost, "/api/auth/login",
		`{"email":"`+memberEmail+`","password":"`+memberPass+`"}`, ""); status != http.StatusOK {
		t.Fatalf("old password must still work: got %d %v", status, body)
	}

	// correct current password → change lands, old fails / new works
	if status, _, _ := do(t, app, http.MethodPatch, "/api/me",
		`{"current_password":"`+memberPass+`","new_password":"new-pass-123"}`, member); status != http.StatusOK {
		t.Fatalf("password change: got %d", status)
	}
	if status, _, _ := do(t, app, http.MethodPost, "/api/auth/login",
		`{"email":"`+memberEmail+`","password":"`+memberPass+`"}`, ""); status != http.StatusUnauthorized {
		t.Fatalf("old password after change: got %d, want 401", status)
	}
	if status, _, _ := do(t, app, http.MethodPost, "/api/auth/login",
		`{"email":"`+memberEmail+`","password":"new-pass-123"}`, ""); status != http.StatusOK {
		t.Fatal("new password must work")
	}

	// unauthenticated → 401
	if status, _, _ := do(t, app, http.MethodPatch, "/api/me", `{"name":"X"}`, ""); status != http.StatusUnauthorized {
		t.Fatalf("unauth: got %d, want 401", status)
	}
}

func TestPatchMeAdminForbidden(t *testing.T) {
	app, _ := newTestApp(t)
	admin := loginAndGet(t, app, adminEmail, adminPass)

	// admins keep the Users page (#49)
	if status, body, _ := do(t, app, http.MethodPatch, "/api/me", `{"name":"Nope"}`, admin); status != http.StatusForbidden || errCode(t, body) != "forbidden" {
		t.Fatalf("admin patch me: got %d %v, want 403 forbidden", status, body)
	}

	// clients (also non-admin) may manage their own profile
	client, _ := mkClient(t, app, newProjectFixture(t, app), "me-client@example.com", "client-pass-1")
	if status, body, _ := do(t, app, http.MethodPatch, "/api/me", `{"name":"Client Renamed"}`, client); status != http.StatusOK || body["name"] != "Client Renamed" {
		t.Fatalf("client patch me: got %d %v", status, body)
	}
}
