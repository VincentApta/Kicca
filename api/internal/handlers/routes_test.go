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
	"time"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/VincentApta/Kica/api/internal/db"
	"github.com/VincentApta/Kica/api/internal/models"
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
	if err := gdb.AutoMigrate(&models.User{}, &models.Team{}, &models.TeamMember{}, &models.Project{}, &models.ProjectTeam{}, &models.ProjectMember{},
		&models.Task{}, &models.Label{}, &models.TaskLabel{}, &models.Comment{}, &models.GitHubIssueLink{},
		&models.TaskEvent{}, &models.ClientProject{}, &models.TaskAttachment{}, &NotificationRead{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	if err := db.SeedAdmin(gdb, adminEmail, adminPass); err != nil {
		t.Fatalf("seed: %v", err)
	}
	app := fiber.New()
	Register(app, gdb, testSecret, &testGhEncKey)
	return app, gdb
}

// testGhEncKey is a fixed AES-256 key standing in for GH_ENC_KEY.
var testGhEncKey [32]byte

func init() {
	for i := range testGhEncKey {
		testGhEncKey[i] = byte(i + 1)
	}
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

// sessionCookie extracts the kica_session value from Set-Cookie.
func sessionCookie(t *testing.T, h http.Header) string {
	t.Helper()
	for _, c := range h.Values("Set-Cookie") {
		if strings.HasPrefix(c, "kica_session=") {
			return strings.SplitN(c, ";", 2)[0]
		}
	}
	t.Fatal("no kica_session cookie in response")
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
		if strings.HasPrefix(c, "kica_session=") && strings.Contains(c, attr) {
			return true
		}
	}
	return false
}

// TestLoginLockout: 5 consecutive failures per email+IP lock login for
// 15 minutes (fiber's test conn always reports the same remote addr, so the
// email part of the key is what varies here).
func TestLoginLockout(t *testing.T) {
	app, _ := newTestApp(t)

	badCreds := func(email string) (int, map[string]interface{}, string) {
		status, body, h := do(t, app, http.MethodPost, "/api/auth/login",
			`{"email":"`+email+`","password":"wrong"}`, "")
		return status, body, h.Get("Retry-After")
	}

	// failures 1–5 → 401; the 5th starts the lockout
	for i := 0; i < 5; i++ {
		if status, body, _ := badCreds(adminEmail); status != http.StatusUnauthorized {
			t.Fatalf("attempt %d: got %d %v, want 401", i+1, status, body)
		}
	}
	status, body, retryAfter := badCreds(adminEmail)
	if status != http.StatusTooManyRequests || errCode(t, body) != "rate_limited" {
		t.Fatalf("6th attempt: got %d %v, want 429 rate_limited", status, body)
	}
	if retryAfter != "900" {
		t.Fatalf("Retry-After: got %q, want 900", retryAfter)
	}
	e, _ := body["error"].(map[string]interface{})
	if msg, _ := e["message"].(string); !strings.Contains(msg, "15 minutes") {
		t.Fatalf("429 body missing retry info: %v", body)
	}

	// locked key guards the DB lookup: valid credentials also 429
	if status, _, _ := do(t, app, http.MethodPost, "/api/auth/login",
		`{"email":"`+adminEmail+`","password":"`+adminPass+`"}`, ""); status != http.StatusTooManyRequests {
		t.Fatalf("valid creds while locked: got %d, want 429", status)
	}

	// the key includes the email: a different email is a fresh bucket
	if status, body, _ := badCreds("other@example.com"); status != http.StatusUnauthorized {
		t.Fatalf("other email: got %d %v, want 401", status, body)
	}
}

// TestLoginLockoutResetBySuccess: any successful login clears the streak.
func TestLoginLockoutResetBySuccess(t *testing.T) {
	app, _ := newTestApp(t)
	bad := func() (int, map[string]interface{}) {
		status, body, _ := do(t, app, http.MethodPost, "/api/auth/login",
			`{"email":"`+adminEmail+`","password":"wrong"}`, "")
		return status, body
	}

	for i := 0; i < 4; i++ {
		if status, body := bad(); status != http.StatusUnauthorized {
			t.Fatalf("attempt %d: got %d %v, want 401", i+1, status, body)
		}
	}
	if status, _, _ := do(t, app, http.MethodPost, "/api/auth/login",
		`{"email":"`+adminEmail+`","password":"`+adminPass+`"}`, ""); status != http.StatusOK {
		t.Fatal("success after 4 failures: want 200")
	}
	// streak restarted: 5 more failures needed before the lockout
	for i := 0; i < 5; i++ {
		if status, body := bad(); status != http.StatusUnauthorized {
			t.Fatalf("post-reset attempt %d: got %d %v, want 401", i+1, status, body)
		}
	}
	if status, body := bad(); status != http.StatusTooManyRequests {
		t.Fatalf("after 5 post-reset failures: got %d %v, want 429", status, body)
	}
}

// TestLoginGuardExpiry: unit test on the guard itself (clock injected).
func TestLoginGuardExpiry(t *testing.T) {
	g := newLoginGuard()
	now := time.Now()
	key := "a@example.com|1.2.3.4"

	for i := 0; i < 4; i++ {
		g.fail(key, now)
	}
	if left := g.remaining(key, now); left != 0 {
		t.Fatalf("4 failures: locked for %v, want 0", left)
	}
	g.fail(key, now) // 5th — lockout starts
	if left := g.remaining(key, now); left <= 0 || left > 15*time.Minute {
		t.Fatalf("5 failures: left=%v, want (0, 15m]", left)
	}
	if left := g.remaining(key, now.Add(15*time.Minute)); left != 0 {
		t.Fatalf("after lockout: left=%v, want 0", left)
	}
	// a failure after expiry starts a fresh streak, not an instant re-lock
	later := now.Add(15*time.Minute + time.Second)
	g.fail(key, later)
	if left := g.remaining(key, later); left != 0 {
		t.Fatalf("fresh streak: left=%v, want 0", left)
	}
	// clear wipes even an active lockout
	for i := 0; i < 5; i++ {
		g.fail(key, later)
	}
	g.clear(key)
	if left := g.remaining(key, later.Add(time.Second)); left != 0 {
		t.Fatalf("after clear: left=%v, want 0", left)
	}
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

// TestLastAdminGuard: demoting/disabling the only enabled global admin → 409;
// allowed again once a second admin exists.
func TestLastAdminGuard(t *testing.T) {
	app, _ := newTestApp(t)
	admin := loginAndGet(t, app, adminEmail, adminPass)
	adminID := func() string {
		t.Helper()
		status, body, _ := do(t, app, http.MethodGet, "/api/users?per_page=100", "", admin)
		if status != http.StatusOK {
			t.Fatalf("list users: %d", status)
		}
		data, _ := body["data"].([]interface{})
		for _, row := range data {
			if rowMap(t, row)["global_role"] == "admin" {
				return rowMap(t, row)["id"].(string)
			}
		}
		t.Fatal("no admin found")
		return ""
	}()

	for _, payload := range []string{`{"global_role":"member"}`} {
		if status, body, _ := do(t, app, http.MethodPatch, "/api/users/"+adminID, payload, admin); status != http.StatusConflict || errCode(t, body) != "last_admin" {
			t.Fatalf("last admin %s: got %d %v, want 409 last_admin", payload, status, body)
		}
	}
	// disabling your own account trips the self guard first (#45), even as
	// the last admin
	if status, body, _ := do(t, app, http.MethodPatch, "/api/users/"+adminID, `{"disabled":true}`, admin); status != http.StatusConflict || errCode(t, body) != "self_disable" {
		t.Fatalf("self disable as last admin: got %d %v, want 409 self_disable", status, body)
	}

	// a second admin unblocks both mutations — and the last_admin disable
	// path itself: admin2 can now disable the seed admin
	if status, _ := createUserViaAPI(t, app, admin, "admin2@example.com", "admin2-pass-1", "admin"); status != http.StatusCreated {
		t.Fatalf("create second admin: %d", status)
	}
	admin2 := loginAndGet(t, app, "admin2@example.com", "admin2-pass-1")
	if status, body, _ := do(t, app, http.MethodPatch, "/api/users/"+adminID, `{"disabled":true}`, admin2); status != http.StatusOK {
		t.Fatalf("admin2 disables seed admin: got %d %v, want 200", status, body)
	}
}

// TestUserDisableToggleSerialization (#45): `disabled` serializes on the
// user-management responses so the Users page toggle reflects server state;
// self-disable is a 409 (admin lockout guard).
func TestUserDisableToggleSerialization(t *testing.T) {
	app, _ := newTestApp(t)
	admin := loginAndGet(t, app, adminEmail, adminPass)

	status, created := createUserViaAPI(t, app, admin, memberEmail, memberPass, "member")
	if status != http.StatusCreated {
		t.Fatalf("create member: got %d %v", status, created)
	}
	id, _ := created["id"].(string)
	if _, present := created["disabled"]; present {
		t.Fatalf("active user must omit disabled: %v", created)
	}

	// disable → response + list both carry disabled=true
	status, body, _ := do(t, app, http.MethodPatch, "/api/users/"+id, `{"disabled":true}`, admin)
	if status != http.StatusOK || body["disabled"] != true {
		t.Fatalf("disable: got %d %v, want 200 disabled=true", status, body)
	}
	status, body, _ = do(t, app, http.MethodGet, "/api/users?per_page=100", "", admin)
	if status != http.StatusOK {
		t.Fatalf("list: got %d", status)
	}
	data, _ := body["data"].([]interface{})
	var disabledRow map[string]interface{}
	var myID string
	for _, row := range data {
		if rowMap(t, row)["id"] == id {
			disabledRow = rowMap(t, row)
		}
		if rowMap(t, row)["global_role"] == "admin" && rowMap(t, row)["email"] == adminEmail {
			myID = rowMap(t, row)["id"].(string)
		}
	}
	if disabledRow == nil || disabledRow["disabled"] != true {
		t.Fatalf("list row missing disabled=true: %v", disabledRow)
	}

	// self-disable → 409 self_disable (needs a second enabled admin so the
	// last-admin guard isn't what trips it)
	if status, _ := createUserViaAPI(t, app, admin, "admin2@example.com", "admin2-pass-1", "admin"); status != http.StatusCreated {
		t.Fatalf("second admin: got %d", status)
	}
	status, body, _ = do(t, app, http.MethodPatch, "/api/users/"+myID, `{"disabled":true}`, admin)
	if status != http.StatusConflict || errCode(t, body) != "self_disable" {
		t.Fatalf("self disable: got %d %v, want 409 self_disable", status, body)
	}

	// enable → disabled omitted again (false)
	status, body, _ = do(t, app, http.MethodPatch, "/api/users/"+id, `{"disabled":false}`, admin)
	if status != http.StatusOK {
		t.Fatalf("enable: got %d %v", status, body)
	}
	if _, present := body["disabled"]; present {
		t.Fatalf("re-enabled user must omit disabled: %v", body)
	}
}

func TestListUsersPagination(t *testing.T) {	app, _ := newTestApp(t)
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

// TestNotifications: unread feed = relevant task_events after watermark;
// mark-read bumps it; assign + comment + status events all appear.
func TestNotifications(t *testing.T) {
	app, _ := newTestApp(t)
	_, adminBody, _ := do(t, app, http.MethodPost, "/api/auth/login",
		`{"email":"`+adminEmail+`","password":"`+adminPass+`"}`, "")
	adminUser, _ := adminBody["user"].(map[string]interface{})
	adminID, _ := adminUser["id"].(string)
	admin := loginAndGet(t, app, adminEmail, adminPass)

	// admin creates a team + project
	status, team, _ := do(t, app, http.MethodPost, "/api/teams",
		`{"name":"Notif Team"}`, admin)
	if status != http.StatusCreated {
		t.Fatalf("create team: got %d %v", status, team)
	}
	teamID, _ := team["id"].(string)
	status, proj, _ := do(t, app, http.MethodPost, "/api/projects",
		`{"team_id":"`+teamID+`","name":"Notif","key":"NTF"}`, admin)
	if status != http.StatusCreated {
		t.Fatalf("create project: got %d %v", status, proj)
	}
	projID, _ := proj["id"].(string)

	// make a member, create a task, assign the member
	status, created := createUserViaAPI(t, app, admin, memberEmail, memberPass, "member")
	if status != http.StatusCreated {
		t.Fatalf("create member: got %d %v", status, created)
	}
	member := loginAndGet(t, app, memberEmail, memberPass)
	memberID, _ := created["id"].(string)

	// member joins the team, then the project (whole replace keeps admin)
	status, _, _ = do(t, app, http.MethodPut, "/api/teams/"+teamID+"/members",
		`{"user_ids":["`+memberID+`"]}`, admin)
	if status != http.StatusOK {
		t.Fatalf("add member to team: got %d", status)
	}
	status, _, _ = do(t, app, http.MethodPut, "/api/projects/"+projID+"/members",
		`{"members":[{"user_id":"`+adminID+`","role":"project_admin"},{"user_id":"`+memberID+`","role":"member"}]}`, admin)
	if status != http.StatusOK {
		t.Fatalf("add member to project: got %d", status)
	}

	status, task, _ := do(t, app, http.MethodPost, "/api/projects/"+projID+"/tasks",
		`{"title":"notif task","priority":"medium"}`, admin)
	if status != http.StatusCreated {
		t.Fatalf("create task: got %d %v", status, task)
	}
	taskID, _ := task["id"].(string)

	// assign to member → member sees an 'assign' notification, admin (creator) doesn't
	status, _, _ = do(t, app, http.MethodPatch, "/api/tasks/"+taskID,
		`{"assignee_id":"`+memberID+`"}`, admin)
	if status != http.StatusOK {
		t.Fatalf("assign: got %d", status)
	}
	status, body, _ := do(t, app, http.MethodGet, "/api/notifications", "", member)
	if status != http.StatusOK {
		t.Fatalf("member notif list: got %d", status)
	}
	notifs, _ := body["data"].([]interface{})
	if len(notifs) != 1 {
		t.Fatalf("member unread after assign: got %d, want 1", len(notifs))
	}

	// member comments → admin (creator) gets a notif; member's own comment doesn't self-notify
	status, _, _ = do(t, app, http.MethodPost, "/api/tasks/"+taskID+"/comments",
		`{"body":"hello"}`, member)
	if status != http.StatusCreated {
		t.Fatalf("member comment: got %d", status)
	}
	status, body, _ = do(t, app, http.MethodGet, "/api/notifications", "", admin)
	if status != http.StatusOK {
		t.Fatalf("admin notif list: got %d", status)
	}
	notifs, _ = body["data"].([]interface{})
	if len(notifs) != 1 {
		t.Fatalf("admin unread after comment: got %d, want 1", len(notifs))
	}

	// status change by admin → member (assignee) notified
	status, _, _ = do(t, app, http.MethodPatch, "/api/tasks/"+taskID,
		`{"status":"in_progress"}`, admin)
	if status != http.StatusOK {
		t.Fatalf("status move: got %d", status)
	}
	status, body, _ = do(t, app, http.MethodGet, "/api/notifications", "", member)
	if status != http.StatusOK {
		t.Fatalf("member notif list 2: got %d", status)
	}
	notifs, _ = body["data"].([]interface{})
	if len(notifs) != 2 { // assign + status
		t.Fatalf("member unread after status: got %d, want 2", len(notifs))
	}

	// mark-read → empty
	status, _, _ = do(t, app, http.MethodPost, "/api/notifications/read", "", member)
	if status != http.StatusNoContent {
		t.Fatalf("mark read: got %d", status)
	}
	status, body, _ = do(t, app, http.MethodGet, "/api/notifications", "", member)
	if status != http.StatusOK {
		t.Fatalf("member notif list 3: got %d", status)
	}
	notifs, _ = body["data"].([]interface{})
	if len(notifs) != 0 {
		t.Fatalf("member unread after read: got %d, want 0", len(notifs))
	}
}

// TestSearch: cross-project search; member scoping; q<2 empty; client 403.
func TestSearch(t *testing.T) {
	app, _ := newTestApp(t)
	admin := loginAndGet(t, app, adminEmail, adminPass)

	// team + two projects, one task each
	_, tm, _ := do(t, app, http.MethodPost, "/api/teams", `{"name":"Search Team"}`, admin)
	teamID, _ := tm["id"].(string)
	_, p1, _ := do(t, app, http.MethodPost, "/api/projects", `{"team_id":"`+teamID+`","name":"Alpha Search","key":"ASRCH"}`, admin)
	_, p2, _ := do(t, app, http.MethodPost, "/api/projects", `{"team_id":"`+teamID+`","name":"Beta Zone","key":"BZON"}`, admin)
	p1ID, _ := p1["id"].(string)
	p2ID, _ := p2["id"].(string)
	do(t, app, http.MethodPost, "/api/projects/"+p1ID+"/tasks", `{"title":"needle in alpha"}`, admin)
	do(t, app, http.MethodPost, "/api/projects/"+p2ID+"/tasks", `{"title":"nothing here"}`, admin)

	// admin: finds task + project by fragment
	status, body, _ := do(t, app, http.MethodGet, "/api/search?q=needl", "", admin)
	if status != http.StatusOK {
		t.Fatalf("search: got %d", status)
	}
	data := body["data"].(map[string]interface{})
	tasks := data["tasks"].([]interface{})
	projs := data["projects"].([]interface{})
	if len(tasks) != 1 || len(projs) != 0 {
		t.Fatalf("admin search tasks=%d projs=%d, want 1/0", len(tasks), len(projs))
	}
	status, body, _ = do(t, app, http.MethodGet, "/api/search?q=alpha", "", admin)
	data = body["data"].(map[string]interface{})
	if projs := data["projects"].([]interface{}); len(projs) != 1 {
		t.Fatalf("project search: got %d, want 1", len(projs))
	}

	// member with no membership: sees nothing
	mEmail, mPass := "searchmember@example.com", "searchmember-pass-1"
	createUserViaAPI(t, app, admin, mEmail, mPass, "member")
	member := loginAndGet(t, app, mEmail, mPass)
	status, body, _ = do(t, app, http.MethodGet, "/api/search?q=needl", "", member)
	if status != http.StatusOK {
		t.Fatalf("member search: got %d", status)
	}
	data = body["data"].(map[string]interface{})
	if tasks := data["tasks"].([]interface{}); len(tasks) != 0 {
		t.Fatalf("member leak: got %d tasks, want 0", len(tasks))
	}

	// short q → empty
	status, body, _ = do(t, app, http.MethodGet, "/api/search?q=n", "", admin)
	if status != http.StatusOK {
		t.Fatalf("short q: got %d", status)
	}
	// client 403 (client linked to p1 so creation is valid)
	cEmail, cPass := "searchclient@example.com", "searchclient-pass-1"
	do(t, app, http.MethodPost, "/api/users",
		`{"email":"`+cEmail+`","name":"Client","password":"`+cPass+`","global_role":"client","project_ids":["`+p1ID+`"]}`, admin)
	client := loginAndGet(t, app, cEmail, cPass)
	if status, _, _ := do(t, app, http.MethodGet, "/api/search?q=needl", "", client); status != http.StatusForbidden {
		t.Fatalf("client search: got %d, want 403", status)
	}
}

// TestProjectTeams: create with team_ids seeds contributing teams; PATCH
// /:id/teams replaces the set; owner removal 422; detail returns teams.
func TestProjectTeams(t *testing.T) {
	app, _ := newTestApp(t)
	admin := loginAndGet(t, app, adminEmail, adminPass)

	_, t1, _ := do(t, app, http.MethodPost, "/api/teams", `{"name":"Team One"}`, admin)
	_, t2, _ := do(t, app, http.MethodPost, "/api/teams", `{"name":"Team Two"}`, admin)
	t1ID, _ := t1["id"].(string)
	t2ID, _ := t2["id"].(string)

	// create with two contributing teams
	status, body, _ := do(t, app, http.MethodPost, "/api/projects",
		`{"team_id":"`+t1ID+`","team_ids":["`+t1ID+`","`+t2ID+`"],"name":"Multi","key":"MULTI"}`, admin)
	if status != http.StatusCreated {
		t.Fatalf("create: got %d %v", status, body)
	}
	pid, _ := body["id"].(string)

	// detail carries team_name + teams
	_, detail, _ := do(t, app, http.MethodGet, "/api/projects/"+pid, "", admin)
	if detail["team_name"] != "Team One" {
		t.Fatalf("team_name: got %v", detail["team_name"])
	}
	teams, _ := detail["teams"].([]interface{})
	if len(teams) != 2 {
		t.Fatalf("teams: got %d, want 2", len(teams))
	}

	// PATCH: drop team two
	status, body, _ = do(t, app, http.MethodPatch, "/api/projects/"+pid+"/teams",
		`{"team_ids":["`+t1ID+`"]}`, admin)
	if status != http.StatusOK {
		t.Fatalf("patch teams: got %d %v", status, body)
	}
	teams, _ = body["data"].([]interface{})
	if len(teams) != 1 {
		t.Fatalf("after patch: got %d teams, want 1", len(teams))
	}

	// removing the owner 422
	if status, _, _ := do(t, app, http.MethodPatch, "/api/projects/"+pid+"/teams",
		`{"team_ids":["`+t2ID+`"]}`, admin); status != http.StatusUnprocessableEntity {
		t.Fatalf("owner removal: got %d, want 422", status)
	}
	// empty 422
	if status, _, _ := do(t, app, http.MethodPatch, "/api/projects/"+pid+"/teams",
		`{"team_ids":[]}`, admin); status != http.StatusUnprocessableEntity {
		t.Fatalf("empty: got %d, want 422", status)
	}
}
