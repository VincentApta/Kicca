// Integration tests over the real router + GORM on in-memory sqlite.
// Dialect quirks handled: uuid PKs come from the app (BeforeCreate), email
// lowercasing stands in for citext, unique-violation detection covers both
// dialects (see isUniqueViolation).
package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/VincentApta/Kicca/api/internal/db"
	"github.com/VincentApta/Kicca/api/internal/models"
)

const (
	testSecret  = "test-secret"
	adminEmail  = "admin@example.com"
	adminPass   = "admin-pass-1"
	memberEmail = "member@example.com"
	memberPass  = "member-pass-1"
)

func newTestApp(t *testing.T) (*fiber.App, *gorm.DB) {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	if err := gdb.AutoMigrate(&models.User{}, &models.Team{}, &models.TeamMember{}, &models.Project{}, &models.ProjectMember{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	if err := db.SeedAdmin(gdb, adminEmail, adminPass); err != nil {
		t.Fatalf("seed: %v", err)
	}
	app := fiber.New()
	Register(app, gdb, testSecret)
	return app, gdb
}

// do fires a JSON request with an optional session cookie.
func do(t *testing.T, app *fiber.App, method, path, body string, cookie string) (int, map[string]interface{}, http.Header) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := app.Test(req, 10_000)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	var out map[string]interface{}
	raw, _ := io.ReadAll(resp.Body)
	json.Unmarshal(raw, &out) // nil is fine for 204s
	return resp.StatusCode, out, resp.Header
}

// sessionCookie extracts the kicca_session value from Set-Cookie.
func sessionCookie(t *testing.T, h http.Header) string {
	t.Helper()
	for _, c := range h.Values("Set-Cookie") {
		if strings.HasPrefix(c, "kicca_session=") {
			return strings.SplitN(c, ";", 2)[0]
		}
	}
	t.Fatal("no kicca_session cookie in response")
	return ""
}

// loginAndGet returns a valid session cookie header value.
func loginAndGet(t *testing.T, app *fiber.App, email, pass string) string {
	t.Helper()
	status, _, h := do(t, app, http.MethodPost, "/api/auth/login",
		`{"email":"`+email+`","password":"`+pass+`"}`, "")
	if status != http.StatusOK {
		t.Fatalf("login %s: got %d, want 200", email, status)
	}
	return sessionCookie(t, h)
}

// createUserViaAPI is the admin path POST /api/users.
func createUserViaAPI(t *testing.T, app *fiber.App, admin, email, pass, role string) (int, map[string]interface{}) {
	t.Helper()
	status, body, _ := do(t, app, http.MethodPost, "/api/users",
		`{"email":"`+email+`","name":"`+strings.SplitN(email, "@", 2)[0]+`","password":"`+pass+`","global_role":"`+role+`"}`, admin)
	return status, body
}

func errCode(t *testing.T, body map[string]interface{}) string {
	t.Helper()
	e, _ := body["error"].(map[string]interface{})
	code, _ := e["code"].(string)
	return code
}

func TestAuthFlow(t *testing.T) {
	app, _ := newTestApp(t)

	// login → 200 {user} + cookie
	status, body, h := do(t, app, http.MethodPost, "/api/auth/login",
		`{"email":"`+adminEmail+`","password":"`+adminPass+`"}`, "")
	if status != http.StatusOK {
		t.Fatalf("login: got %d, want 200", status)
	}
	user, _ := body["user"].(map[string]interface{})
	if user["email"] != adminEmail || user["global_role"] != "admin" {
		t.Fatalf("login user payload: %+v", body)
	}
	if _, leaked := user["password_hash"]; leaked {
		t.Fatal("password_hash serialized in response")
	}
	cookie := sessionCookie(t, h)
	for _, want := range []string{"HttpOnly", "SameSite=Lax"} {
		if !setCookieHas(t, h, want) {
			t.Fatalf("cookie missing %s", want)
		}
	}

	// me with cookie → 200 same user
	status, body, _ = do(t, app, http.MethodGet, "/api/auth/me", "", cookie)
	if status != http.StatusOK {
		t.Fatalf("me: got %d, want 200", status)
	}
	user, _ = body["user"].(map[string]interface{})
	if user["email"] != adminEmail {
		t.Fatalf("me user: %+v", body)
	}

	// bad password → 401 contract error shape
	status, body, _ = do(t, app, http.MethodPost, "/api/auth/login",
		`{"email":"`+adminEmail+`","password":"wrong"}`, "")
	if status != http.StatusUnauthorized || errCode(t, body) != "invalid_credentials" {
		t.Fatalf("bad login: got %d %v, want 401 invalid_credentials", status, body)
	}

	// me without cookie → 401
	status, _, _ = do(t, app, http.MethodGet, "/api/auth/me", "", "")
	if status != http.StatusUnauthorized {
		t.Fatalf("me unauth: got %d, want 401", status)
	}

	// logout → 204
	status, _, _ = do(t, app, http.MethodPost, "/api/auth/logout", "", cookie)
	if status != http.StatusNoContent {
		t.Fatalf("logout: got %d, want 204", status)
	}
}

func setCookieHas(t *testing.T, h http.Header, attr string) bool {
	t.Helper()
	for _, c := range h.Values("Set-Cookie") {
		if strings.HasPrefix(c, "kicca_session=") && strings.Contains(c, attr) {
			return true
		}
	}
	return false
}

func TestDisabledUserRejected(t *testing.T) {
	app, _ := newTestApp(t)
	admin := loginAndGet(t, app, adminEmail, adminPass)

	status, created := createUserViaAPI(t, app, admin, memberEmail, memberPass, "member")
	if status != http.StatusCreated {
		t.Fatalf("create member: got %d %v", status, created)
	}
	id, _ := created["id"].(string)

	// member can log in and use /me
	member := loginAndGet(t, app, memberEmail, memberPass)
	if status, _, _ = do(t, app, http.MethodGet, "/api/auth/me", "", member); status != http.StatusOK {
		t.Fatalf("member me before disable: got %d, want 200", status)
	}

	// admin disables → existing JWT is dead (middleware lookup)
	status, _, _ = do(t, app, http.MethodPatch, "/api/users/"+id, `{"disabled":true}`, admin)
	if status != http.StatusOK {
		t.Fatalf("disable: got %d, want 200", status)
	}
	if status, body, _ := do(t, app, http.MethodGet, "/api/auth/me", "", member); status != http.StatusUnauthorized || errCode(t, body) != "account_disabled" {
		t.Fatalf("member me after disable: got %d %v, want 401 account_disabled", status, body)
	}

	// fresh login also rejected
	if status, body, _ := do(t, app, http.MethodPost, "/api/auth/login",
		`{"email":"`+memberEmail+`","password":"`+memberPass+`"}`, ""); status != http.StatusUnauthorized || errCode(t, body) != "account_disabled" {
		t.Fatalf("disabled login: got %d %v, want 401 account_disabled", status, body)
	}
}

func TestMemberForbiddenOnUserManagement(t *testing.T) {
	app, _ := newTestApp(t)
	admin := loginAndGet(t, app, adminEmail, adminPass)
	if status, _ := createUserViaAPI(t, app, admin, memberEmail, memberPass, "member"); status != http.StatusCreated {
		t.Fatalf("create member: got %d", status)
	}
	member := loginAndGet(t, app, memberEmail, memberPass)

	if status, body, _ := do(t, app, http.MethodGet, "/api/users", "", member); status != http.StatusForbidden || errCode(t, body) != "forbidden" {
		t.Fatalf("member list users: got %d %v, want 403 forbidden", status, body)
	}
	if status, body, _ := do(t, app, http.MethodPost, "/api/users",
		`{"email":"x@example.com","name":"X","password":"pw123456","global_role":"member"}`, member); status != http.StatusForbidden {
		t.Fatalf("member create user: got %d %v, want 403", status, body)
	}

	// admin still can
	if status, _, _ := do(t, app, http.MethodGet, "/api/users", "", admin); status != http.StatusOK {
		t.Fatalf("admin list users: got %d, want 200", status)
	}
}

func TestDuplicateEmailConflict(t *testing.T) {
	app, _ := newTestApp(t)
	admin := loginAndGet(t, app, adminEmail, adminPass)

	if status, _ := createUserViaAPI(t, app, admin, memberEmail, memberPass, "member"); status != http.StatusCreated {
		t.Fatalf("first create: got %d", status)
	}
	status, body := createUserViaAPI(t, app, admin, memberEmail, "other-pass", "member")
	if status != http.StatusConflict || errCode(t, body) != "email_exists" {
		t.Fatalf("dup create: got %d %v, want 409 email_exists", status, body)
	}
	// case-insensitive (citext semantics via lowercasing)
	status, body = createUserViaAPI(t, app, admin, strings.ToUpper(memberEmail), "other-pass", "member")
	if status != http.StatusConflict {
		t.Fatalf("dup create uppercased: got %d %v, want 409", status, body)
	}
}

func TestListUsersPagination(t *testing.T) {
	app, _ := newTestApp(t)
	admin := loginAndGet(t, app, adminEmail, adminPass)
	for i := 0; i < 3; i++ {
		email := string(rune('a'+i)) + "@example.com"
		if status, _ := createUserViaAPI(t, app, admin, email, "pw-123456", "member"); status != http.StatusCreated {
			t.Fatalf("create %s: got %d", email, status)
		}
	}
	status, body, _ := do(t, app, http.MethodGet, "/api/users?page=2&per_page=2", "", admin)
	if status != http.StatusOK {
		t.Fatalf("list: got %d", status)
	}
	if body["total"] != float64(4) || body["page"] != float64(2) || body["per_page"] != float64(2) {
		t.Fatalf("pagination meta: %+v", body)
	}
	data, _ := body["data"].([]interface{})
	if len(data) != 2 {
		t.Fatalf("page data len: got %d, want 2", len(data))
	}
}
