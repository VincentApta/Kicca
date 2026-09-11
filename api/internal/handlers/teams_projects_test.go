// T3 integration tests: teams + projects CRUD, membership, visibility
// scoping. Same harness as routes_test.go.
package handlers

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// createTeamViaAPI: admin POST /api/teams.
func createTeamViaAPI(t *testing.T, app *fiber.App, admin, name string) (int, map[string]interface{}) {
	t.Helper()
	status, body, _ := do(t, app, http.MethodPost, "/api/teams", `{"name":"`+name+`"}`, admin)
	return status, body
}

// projectFixture: admin + pa (project_admin) + pm (member) + out (outsider),
// one team, one project KIC with pa+pm as project members.
type projectFixture struct {
	admin, pa, pm, out            string // session cookies
	paID, pmID, teamID, projectID string
}

func newProjectFixture(t *testing.T, app *fiber.App) projectFixture {
	t.Helper()
	f := projectFixture{}
	f.admin = loginAndGet(t, app, adminEmail, adminPass)
	mk := func(email, pass string) (cookie, id string) {
		status, body := createUserViaAPI(t, app, f.admin, email, pass, "member")
		if status != http.StatusCreated {
			t.Fatalf("create %s: got %d %v", email, status, body)
		}
		return loginAndGet(t, app, email, pass), body["id"].(string)
	}
	f.pa, f.paID = mk("pa@example.com", "pa-pass-1")
	f.pm, f.pmID = mk("pm@example.com", "pm-pass-1")
	f.out, _ = mk("out@example.com", "out-pass-1")

	status, team := createTeamViaAPI(t, app, f.admin, "Fixture Team")
	if status != http.StatusCreated {
		t.Fatalf("create team: got %d %v", status, team)
	}
	f.teamID = team["id"].(string)

	status, body, _ := do(t, app, http.MethodPost, "/api/projects",
		`{"team_id":"`+f.teamID+`","name":"Kica","key":"KIC"}`, f.admin)
	if status != http.StatusCreated {
		t.Fatalf("create project: got %d %v", status, body)
	}
	f.projectID = body["id"].(string)

	members := fmt.Sprintf(`{"members":[{"user_id":%q,"role":"project_admin"},{"user_id":%q,"role":"member"}]}`, f.paID, f.pmID)
	if status, _, _ = do(t, app, http.MethodPut, "/api/projects/"+f.projectID+"/members", members, f.admin); status != http.StatusOK {
		t.Fatalf("set project members: got %d", status)
	}
	return f
}

func TestTeamSlugAndCRUD(t *testing.T) {
	app, _ := newTestApp(t)
	admin := loginAndGet(t, app, adminEmail, adminPass)

	status, body := createTeamViaAPI(t, app, admin, "Acme Corp!")
	if status != http.StatusCreated || body["slug"] != "acme-corp" {
		t.Fatalf("create: got %d %v", status, body)
	}

	// duplicate exact name → 409
	if status, body := createTeamViaAPI(t, app, admin, "Acme Corp!"); status != http.StatusConflict || errCode(t, body) != "name_exists" {
		t.Fatalf("dup name: got %d %v, want 409 name_exists", status, body)
	}

	// different name, colliding slug → deduped suffix
	status, body = createTeamViaAPI(t, app, admin, "acme corp!")
	if status != http.StatusCreated || body["slug"] != "acme-corp-2" {
		t.Fatalf("slug dedup: got %d %v", status, body)
	}
	id2 := body["id"].(string)

	// punctuation-only name has no slug → 422
	if status, _ := createTeamViaAPI(t, app, admin, "!!!"); status != http.StatusUnprocessableEntity {
		t.Fatalf("punct name: got %d, want 422", status)
	}

	// rename keeps slug stable
	status, body, _ = do(t, app, http.MethodPatch, "/api/teams/"+id2, `{"name":"Renamed"}`, admin)
	if status != http.StatusOK || body["name"] != "Renamed" || body["slug"] != "acme-corp-2" {
		t.Fatalf("rename: got %d %v", status, body)
	}

	// rename onto existing name → 409
	if status, _, _ := do(t, app, http.MethodPatch, "/api/teams/"+id2, `{"name":"Acme Corp!"}`, admin); status != http.StatusConflict {
		t.Fatalf("rename conflict: got %d, want 409", status)
	}
}

func TestTeamMembershipVisibility(t *testing.T) {
	app, _ := newTestApp(t)
	admin := loginAndGet(t, app, adminEmail, adminPass)

	mk := func(email string) (cookie, id string) {
		status, body := createUserViaAPI(t, app, admin, email, email+"-pass", "member")
		if status != http.StatusCreated {
			t.Fatalf("create %s: %d", email, status)
		}
		return loginAndGet(t, app, email, email+"-pass"), body["id"].(string)
	}
	m1, m1ID := mk("m1@example.com")
	m2, m2ID := mk("m2@example.com")

	status, team := createTeamViaAPI(t, app, admin, "Eng")
	if status != http.StatusCreated {
		t.Fatalf("create team: got %d %v", status, team)
	}
	teamID := team["id"].(string)

	if status, _, _ := do(t, app, http.MethodPut, "/api/teams/"+teamID+"/members",
		`{"user_ids":["`+m1ID+`"]}`, admin); status != http.StatusOK {
		t.Fatalf("add m1: got %d, want 200", status)
	}

	// m1 sees the team w/ count; m2 sees nothing
	_, body, _ := do(t, app, http.MethodGet, "/api/teams", "", m1)
	if body["total"] != float64(1) {
		t.Fatalf("m1 teams: %v", body)
	}
	if data := body["data"].([]interface{}); len(data) != 1 || data[0].(map[string]interface{})["member_count"] != float64(1) {
		t.Fatalf("m1 team row: %v", body["data"])
	}
	_, body, _ = do(t, app, http.MethodGet, "/api/teams", "", m2)
	if body["total"] != float64(0) {
		t.Fatalf("m2 teams: %v, want 0", body["total"])
	}

	// member mutations forbidden
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/teams", `{"name":"X"}`},
		{http.MethodPatch, "/api/teams/" + teamID, `{"name":"X"}`},
		{http.MethodDelete, "/api/teams/" + teamID, ""},
		{http.MethodPut, "/api/teams/" + teamID + "/members", `{"user_ids":[]}`},
	} {
		if status, _, _ := do(t, app, tc.method, tc.path, tc.body, m1); status != http.StatusForbidden {
			t.Fatalf("member %s %s: got %d, want 403", tc.method, tc.path, status)
		}
	}

	// swap membership m1 → m2
	if status, _, _ := do(t, app, http.MethodPut, "/api/teams/"+teamID+"/members",
		`{"user_ids":["`+m2ID+`"]}`, admin); status != http.StatusOK {
		t.Fatalf("swap: got %d, want 200", status)
	}
	_, body, _ = do(t, app, http.MethodGet, "/api/teams", "", m1)
	if body["total"] != float64(0) {
		t.Fatalf("m1 after swap: %v, want 0", body["total"])
	}
	_, body, _ = do(t, app, http.MethodGet, "/api/teams", "", m2)
	if body["total"] != float64(1) {
		t.Fatalf("m2 after swap: %v, want 1", body["total"])
	}

	// bad payloads → 422
	stranger := uuid.NewString()
	for _, payload := range []string{
		`{"user_ids":["not-a-uuid"]}`,
		`{"user_ids":["` + stranger + `"]}`,
		`{"user_ids":["` + m1ID + `","` + m1ID + `"]}`,
	} {
		if status, body, _ := do(t, app, http.MethodPut, "/api/teams/"+teamID+"/members", payload, admin); status != http.StatusUnprocessableEntity {
			t.Fatalf("bad payload %s: got %d %v, want 422", payload, status, body)
		}
	}

	// clear membership → 200, count 0
	_, body, _ = do(t, app, http.MethodPut, "/api/teams/"+teamID+"/members", `{"user_ids":[]}`, admin)
	if body["member_count"] != float64(0) {
		t.Fatalf("clear: %v", body)
	}
}

func TestTeamDeleteConflictWithProjects(t *testing.T) {
	app, _ := newTestApp(t)
	admin := loginAndGet(t, app, adminEmail, adminPass)

	status, team := createTeamViaAPI(t, app, admin, "HasProjects")
	if status != http.StatusCreated {
		t.Fatalf("create team: got %d", status)
	}
	teamID := team["id"].(string)

	if status, _, _ := do(t, app, http.MethodPost, "/api/projects",
		`{"team_id":"`+teamID+`","name":"Blocked","key":"BLK"}`, admin); status != http.StatusCreated {
		t.Fatalf("create project: got %d", status)
	}
	if status, body, _ := do(t, app, http.MethodDelete, "/api/teams/"+teamID, "", admin); status != http.StatusConflict || errCode(t, body) != "team_has_projects" {
		t.Fatalf("delete w/ project: got %d %v, want 409 team_has_projects", status, body)
	}

	// soft-deleted project still blocks (row references remain)
	if status, _, _ := do(t, app, http.MethodGet, "/api/projects", "", admin); status != http.StatusOK {
		t.Fatalf("list: %d", status)
	}
	_, body, _ := do(t, app, http.MethodGet, "/api/projects", "", admin)
	data := body["data"].([]interface{})
	projID := data[0].(map[string]interface{})["id"].(string)
	if status, _, _ := do(t, app, http.MethodDelete, "/api/projects/"+projID, "", admin); status != http.StatusNoContent {
		t.Fatalf("delete project: got %d, want 204", status)
	}
	if status, _, _ := do(t, app, http.MethodDelete, "/api/teams/"+teamID, "", admin); status != http.StatusConflict {
		t.Fatalf("delete w/ soft-deleted project: got %d, want 409", status)
	}

	// empty team deletes clean
	status, empty := createTeamViaAPI(t, app, admin, "Empty")
	if status != http.StatusCreated {
		t.Fatalf("create empty team: got %d", status)
	}
	if status, _, _ := do(t, app, http.MethodDelete, "/api/teams/"+empty["id"].(string), "", admin); status != http.StatusNoContent {
		t.Fatalf("delete empty: got %d, want 204", status)
	}
}

func TestProjectKeyValidation(t *testing.T) {
	app, _ := newTestApp(t)
	admin := loginAndGet(t, app, adminEmail, adminPass)
	_, team := createTeamViaAPI(t, app, admin, "KT")
	teamID := team["id"].(string)

	for _, key := range []string{"kic", "K", "KICDEVELOP1", "1KIC", "K C", ""} {
		status, body, _ := do(t, app, http.MethodPost, "/api/projects",
			`{"team_id":"`+teamID+`","name":"X","key":"`+key+`"}`, admin)
		if status != http.StatusUnprocessableEntity {
			t.Fatalf("key %q: got %d %v, want 422", key, status, body)
		}
	}

	// bad team refs
	for _, teamID := range []string{"nope", uuid.NewString()} {
		if status, _, _ := do(t, app, http.MethodPost, "/api/projects",
			`{"team_id":"`+teamID+`","name":"X","key":"KIC"}`, admin); status != http.StatusUnprocessableEntity {
			t.Fatalf("team_id %q: got %d, want 422", teamID, status)
		}
	}

	status, body, _ := do(t, app, http.MethodPost, "/api/projects",
		`{"team_id":"`+teamID+`","name":"Kica","key":"KIC"}`, admin)
	if status != http.StatusCreated || body["key"] != "KIC" || body["team_id"] != teamID {
		t.Fatalf("create: got %d %v", status, body)
	}

	// fresh project: creator is the sole member, as project_admin
	_, detail, _ := do(t, app, http.MethodGet, "/api/projects/"+body["id"].(string), "", admin)
	members := detail["members"].([]interface{})
	if len(members) != 1 {
		t.Fatalf("fresh project members: got %v, want 1", members)
	}
	m := members[0].(map[string]interface{})
	if m["role"] != "project_admin" || m["email"] != adminEmail {
		t.Fatalf("fresh project member: got %v, want creator as project_admin", m)
	}

	// duplicate key → 409
	if status, body, _ := do(t, app, http.MethodPost, "/api/projects",
		`{"team_id":"`+teamID+`","name":"Other","key":"KIC"}`, admin); status != http.StatusConflict || errCode(t, body) != "key_exists" {
		t.Fatalf("dup key: got %d %v, want 409 key_exists", status, body)
	}
}

func TestProjectVisibilityScoping(t *testing.T) {
	app, _ := newTestApp(t)
	f := newProjectFixture(t, app)

	// outsider: list excludes, detail + mutations 404 (never 403 — no leak)
	_, body, _ := do(t, app, http.MethodGet, "/api/projects", "", f.out)
	if body["total"] != float64(0) {
		t.Fatalf("outsider list: %v", body)
	}
	if status, body, _ := do(t, app, http.MethodGet, "/api/projects/"+f.projectID, "", f.out); status != http.StatusNotFound || errCode(t, body) != "not_found" {
		t.Fatalf("outsider detail: got %d %v, want 404 not_found", status, body)
	}
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPatch, "/api/projects/" + f.projectID, `{"name":"X"}`},
		{http.MethodDelete, "/api/projects/" + f.projectID, ""},
		{http.MethodPut, "/api/projects/" + f.projectID + "/members", `{"members":[{"user_id":"` + f.pmID + `","role":"project_admin"}]}`},
	} {
		if status, _, _ := do(t, app, tc.method, tc.path, tc.body, f.out); status != http.StatusNotFound {
			t.Fatalf("outsider %s: got %d, want 404", tc.method, status)
		}
	}

	// member sees exactly this project, my_role member
	_, body, _ = do(t, app, http.MethodGet, "/api/projects", "", f.pm)
	if body["total"] != float64(1) {
		t.Fatalf("pm list: %v", body)
	}
	if data := body["data"].([]interface{}); data[0].(map[string]interface{})["key"] != "KIC" {
		t.Fatalf("pm list row: %v", body["data"])
	}
	_, body, _ = do(t, app, http.MethodGet, "/api/projects/"+f.projectID, "", f.pm)
	if body["my_role"] != "member" {
		t.Fatalf("pm detail: %v", body)
	}
	if members := body["members"].([]interface{}); len(members) != 2 {
		t.Fatalf("pm detail members: %v", body["members"])
	}

	// member cannot create projects at all
	if status, body, _ := do(t, app, http.MethodPost, "/api/projects",
		`{"team_id":"`+f.teamID+`","name":"P","key":"NEW"}`, f.pm); status != http.StatusForbidden || errCode(t, body) != "forbidden" {
		t.Fatalf("pm create: got %d %v, want 403 forbidden", status, body)
	}

	// member sees the project, so mutating it is 403 (not 404); member cannot
	// manage members either
	if status, _, _ := do(t, app, http.MethodPatch, "/api/projects/"+f.projectID, `{"name":"X"}`, f.pm); status != http.StatusForbidden {
		t.Fatalf("pm patch: got %d, want 403", status)
	}
	if status, _, _ := do(t, app, http.MethodDelete, "/api/projects/"+f.projectID, "", f.pm); status != http.StatusForbidden {
		t.Fatalf("pm delete: got %d, want 403", status)
	}
	if status, _, _ := do(t, app, http.MethodPut, "/api/projects/"+f.projectID+"/members",
		`{"members":[{"user_id":"`+f.pmID+`","role":"project_admin"}]}`, f.pm); status != http.StatusForbidden {
		t.Fatalf("pm manage members: got %d, want 403", status)
	}

	// global admin sees everything
	_, body, _ = do(t, app, http.MethodGet, "/api/projects", "", f.admin)
	if body["total"] != float64(1) {
		t.Fatalf("admin list: %v", body)
	}
	_, body, _ = do(t, app, http.MethodGet, "/api/projects/"+f.projectID, "", f.admin)
	if body["my_role"] != "admin" {
		t.Fatalf("admin detail my_role: %v", body)
	}
}

func TestProjectAdminManagesMembers(t *testing.T) {
	app, _ := newTestApp(t)
	f := newProjectFixture(t, app)

	// project_admin (not global admin) manages the roster: drops self while
	// promoting pm → still ≥1 project_admin, allowed
	payload := fmt.Sprintf(`{"members":[{"user_id":%q,"role":"project_admin"}]}`, f.pmID)
	status, body, _ := do(t, app, http.MethodPut, "/api/projects/"+f.projectID+"/members", payload, f.pa)
	if status != http.StatusOK {
		t.Fatalf("pa replace: got %d %v", status, body)
	}
	if members := body["members"].([]interface{}); len(members) != 1 {
		t.Fatalf("pa replace members: %v", body["members"])
	}

	// pm is now project_admin; roster with zero project_admins → 422
	payload = fmt.Sprintf(`{"members":[{"user_id":%q,"role":"member"}]}`, f.pmID)
	if status, body, _ := do(t, app, http.MethodPut, "/api/projects/"+f.projectID+"/members", payload, f.pm); status != http.StatusUnprocessableEntity {
		t.Fatalf("last admin removal: got %d %v, want 422", status, body)
	}
	if status, _, _ := do(t, app, http.MethodPut, "/api/projects/"+f.projectID+"/members", `{"members":[]}`, f.pm); status != http.StatusUnprocessableEntity {
		t.Fatalf("empty roster: got %d, want 422", status)
	}

	// bad payloads → 422
	for _, p := range []string{
		fmt.Sprintf(`{"members":[{"user_id":%q,"role":"owner"}]}`, f.pmID),
		fmt.Sprintf(`{"members":[{"user_id":%q,"role":"member"}]}`, uuid.NewString()),
		fmt.Sprintf(`{"members":[{"user_id":%q,"role":"member"},{"user_id":%q,"role":"project_admin"}]}`, f.pmID, f.pmID),
	} {
		if status, body, _ := do(t, app, http.MethodPut, "/api/projects/"+f.projectID+"/members", p, f.pm); status != http.StatusUnprocessableEntity {
			t.Fatalf("bad payload %s: got %d %v, want 422", p, status, body)
		}
	}

	// detail reflects the replace; pa demoted to nothing is gone
	_, body, _ = do(t, app, http.MethodGet, "/api/projects/"+f.projectID, "", f.pm)
	if body["my_role"] != "project_admin" {
		t.Fatalf("pm my_role after replace: %v", body)
	}
	if members := body["members"].([]interface{}); len(members) != 1 {
		t.Fatalf("members after replace: %v", body["members"])
	}
}

func TestProjectPatchDelete(t *testing.T) {
	app, _ := newTestApp(t)
	f := newProjectFixture(t, app)

	status, body, _ := do(t, app, http.MethodPatch, "/api/projects/"+f.projectID,
		`{"name":"Kica 2","description":"desc","key":"KIC2"}`, f.admin)
	if status != http.StatusOK || body["name"] != "Kica 2" || body["key"] != "KIC2" || body["description"] != "desc" {
		t.Fatalf("patch: got %d %v", status, body)
	}

	// second project guards key uniqueness on patch
	if status, _, _ := do(t, app, http.MethodPost, "/api/projects",
		`{"team_id":"`+f.teamID+`","name":"Other","key":"OTH"}`, f.admin); status != http.StatusCreated {
		t.Fatalf("create other: got %d", status)
	}
	if status, body, _ := do(t, app, http.MethodPatch, "/api/projects/"+f.projectID, `{"key":"OTH"}`, f.admin); status != http.StatusConflict || errCode(t, body) != "key_exists" {
		t.Fatalf("patch dup key: got %d %v, want 409 key_exists", status, body)
	}

	// soft delete removes it from every view
	if status, _, _ := do(t, app, http.MethodDelete, "/api/projects/"+f.projectID, "", f.admin); status != http.StatusNoContent {
		t.Fatalf("delete: got %d, want 204", status)
	}
	if status, body, _ := do(t, app, http.MethodGet, "/api/projects/"+f.projectID, "", f.admin); status != http.StatusNotFound {
		t.Fatalf("detail after delete: got %d %v, want 404", status, body)
	}
	_, body, _ = do(t, app, http.MethodGet, "/api/projects", "", f.pm)
	if body["total"] != float64(0) {
		t.Fatalf("pm list after delete: %v, want 0", body["total"])
	}
}
