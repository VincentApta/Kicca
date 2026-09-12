// Project CRUD handlers. Visibility (domain rule 1): a user sees a project
// iff global admin or project member — enforced in loadVisibleProject, and
// denial is a 404 so project existence never leaks.
package handlers

import (
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/VincentApta/Kica/api/internal/models"
)

const roleProjectAdmin = "project_admin"

// projectKeyRe per api-contract: uppercase letter first, then uppercase
// letters or digits, 2-10 chars total.
var projectKeyRe = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,9}$`)

// projectJSON is the wire `project` shape. gh_repo/gh_token_enc are never
// serialized (api-contract: GH token never returned).
type projectJSON struct {
	ID          string `json:"id"`
	TeamID      string `json:"team_id"`
	TeamName    string `json:"team_name,omitempty"`
	Name        string `json:"name"`
	Key         string `json:"key"`
	Description string `json:"description"`
}

// toProjectJSON fills TeamName when the caller has it (list/detail do one
// batched lookup); empty otherwise. ponytail: team rename mid-session keeps
// serving the stale name until refetch — fine at this scale.
func toProjectJSON(p *models.Project, teamName string) projectJSON {
	return projectJSON{ID: p.ID, TeamID: p.TeamID, TeamName: teamName, Name: p.Name, Key: p.Key, Description: p.Description}
}

// teamNamesFor batched team-name lookup for a page of projects.
func teamNamesFor(gdb *gorm.DB, projects []models.Project) map[string]string {
	if len(projects) == 0 {
		return map[string]string{}
	}
	ids := make(map[string]bool, len(projects))
	list := make([]string, 0, len(projects))
	for i := range projects {
		if !ids[projects[i].TeamID] {
			ids[projects[i].TeamID] = true
			list = append(list, projects[i].TeamID)
		}
	}
	var teams []models.Team
	names := map[string]string{}
	if err := gdb.Select("id, name").Where("id IN ?", list).Find(&teams).Error; err == nil {
		for i := range teams {
			names[teams[i].ID] = teams[i].Name
		}
	}
	return names
}

type projectMemberJSON struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	Name   string `json:"name"`
	Email  string `json:"email"`
}

type createProjectReq struct {
	TeamID      string   `json:"team_id"`
	TeamIDs     []string `json:"team_ids"`
	Name        string   `json:"name"`
	Key         string   `json:"key"`
	Description *string  `json:"description"`
}

type patchProjectReq struct {
	Name        *string `json:"name"`
	Key         *string `json:"key"`
	Description *string `json:"description"`
}

type projectMemberReq struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

type replaceProjectMembersReq struct {
	Members []projectMemberReq `json:"members"`
}

// loadVisibleProject fetches the project and applies domain rule 1. Denial
// writes a 404 (visible-deny, no leak) and returns ok=false. pm is nil for
// global admins (they need no membership row).
func loadVisibleProject(c *fiber.Ctx, gdb *gorm.DB, u *models.User, id string) (*models.Project, *models.ProjectMember, bool) {
	if _, err := uuid.Parse(id); err != nil {
		httpErr(c, fiber.StatusBadRequest, "invalid_uuid", "project id must be a uuid")
		return nil, nil, false
	}
	var p models.Project
	if err := gdb.First(&p, "id = ?", id).Error; err != nil {
		httpErr(c, fiber.StatusNotFound, "not_found", "no such project")
		return nil, nil, false
	}
	if u.GlobalRole == roleAdmin {
		return &p, nil, true
	}
	var pm models.ProjectMember
	if err := gdb.First(&pm, "project_id = ? AND user_id = ?", p.ID, u.ID).Error; err != nil {
		// no explicit row: grant plain-member access via contributing team
		var viaTeam int64
		if err := gdb.Table("project_teams pt").
			Joins("JOIN team_members tm ON tm.team_id = pt.team_id").
			Where("pt.project_id = ? AND tm.user_id = ?", p.ID, u.ID).
			Count(&viaTeam).Error; err != nil || viaTeam == 0 {
			httpErr(c, fiber.StatusNotFound, "not_found", "no such project")
			return nil, nil, false
		}
		return &p, nil, true
	}
	return &p, &pm, true
}

// ListProjects: GET /api/projects — visible projects only (rule 1).
func ListProjects(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		page := queryInt(c, "page", 1)
		perPage := queryInt(c, "per_page", defaultPerPage)
		scope := func() *gorm.DB {
			q := gdb.Model(&models.Project{})
			if u.GlobalRole != roleAdmin {
				q = q.Where("id IN (?)", visibleProjectIDs(gdb, u.ID))
			}
			return q
		}
		var total int64
		if err := scope().Count(&total).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not count projects")
		}
		var projects []models.Project
		if err := scope().Order("key").Limit(perPage).Offset((page - 1) * perPage).Find(&projects).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not list projects")
		}
		names := teamNamesFor(gdb, projects)
		data := make([]projectJSON, len(projects))
		for i := range projects {
			data[i] = toProjectJSON(&projects[i], names[projects[i].TeamID])
		}
		return c.JSON(fiber.Map{"data": data, "page": page, "per_page": perPage, "total": total})
	}
}

// CreateProject: POST /api/projects — global admin only (403 otherwise);
// key unique per projectKeyRe; team must exist.
func CreateProject(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		if u.GlobalRole != roleAdmin {
			return httpErr(c, fiber.StatusForbidden, "forbidden", "global admin role required")
		}
		var req createProjectReq
		if err := c.BodyParser(&req); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		}
		req.Name = strings.TrimSpace(req.Name)
		req.Key = strings.TrimSpace(req.Key)
		if req.TeamID == "" || req.Name == "" || req.Key == "" {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "team_id, name and key are required")
		}
		if !projectKeyRe.MatchString(req.Key) {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "key must be 2-10 chars: uppercase letter first, then uppercase letters or digits")
		}
		if _, err := uuid.Parse(req.TeamID); err != nil {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "team_id must be a uuid")
		}
		var team models.Team
		if err := gdb.First(&team, "id = ?", req.TeamID).Error; err != nil {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "team_id must reference an existing team")
		}
		// contributing teams: team_ids if given, else just the owner
		teamIDs := req.TeamIDs
		if len(teamIDs) == 0 {
			teamIDs = []string{req.TeamID}
		}
		seen := map[string]bool{}
		uniq := make([]string, 0, len(teamIDs))
		owns := false
		for _, id := range teamIDs {
			if id == "" || seen[id] {
				continue
			}
			if _, err := uuid.Parse(id); err != nil {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "team_ids must be uuids")
			}
			seen[id] = true
			uniq = append(uniq, id)
			if id == req.TeamID {
				owns = true
			}
		}
		if !owns {
			uniq = append(uniq, req.TeamID) // owner always contributes
		}
		var cnt int64
		if err := gdb.Model(&models.Team{}).Where("id IN ?", uniq).Count(&cnt).Error; err != nil || cnt != int64(len(uniq)) {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "team_ids must reference existing teams")
		}
		p := &models.Project{TeamID: req.TeamID, Name: req.Name, Key: req.Key}
		if req.Description != nil {
			p.Description = *req.Description
		}
		// Same transaction: creator becomes project_admin (domain rule) so
		// fresh projects have members populated. Contributing teams seeded.
		err := gdb.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(p).Error; err != nil {
				return err
			}
			rows := make([]models.ProjectTeam, len(uniq))
			for i, id := range uniq {
				rows[i] = models.ProjectTeam{ProjectID: p.ID, TeamID: id}
			}
			if err := tx.Create(&rows).Error; err != nil {
				return err
			}
			return tx.Create(&models.ProjectMember{ProjectID: p.ID, UserID: u.ID, Role: roleProjectAdmin}).Error
		})
		if err != nil {
			if isUniqueViolation(err) {
				return httpErr(c, fiber.StatusConflict, "key_exists", "a project with this key already exists")
			}
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not create project")
		}
		return c.Status(fiber.StatusCreated).JSON(toProjectJSON(p, team.Name))
	}
}

// teamsForProjects returns contributing teams [{id,name}] per project id.
func teamsForProjects(gdb *gorm.DB, projectID string) []map[string]string {
	var rows []struct {
		ProjectID string
		TeamID    string
		Name      string
	}
	if err := gdb.Table("project_teams").
		Select("project_teams.project_id, project_teams.team_id, teams.name").
		Joins("JOIN teams ON teams.id = project_teams.team_id").
		Where("project_teams.project_id = ?", projectID).
		Order("teams.name").Scan(&rows).Error; err != nil {
		return []map[string]string{}
	}
	out := make([]map[string]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]string{"id": r.TeamID, "name": r.Name})
	}
	return out
}

// GetProject: GET /api/projects/:id — detail + my_role + members. 404 for
// non-members (no leak). my_role is the project role, or "admin" for a
// global admin without membership.
func GetProject(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		p, pm, ok := loadVisibleProject(c, gdb, u, c.Params("id"))
		if !ok {
			return nil
		}
		// global admins and explicit rows carry their role; team-derived
		// access (pm == nil, non-admin) is plain member.
		myRole := roleMember
		if u.GlobalRole == roleAdmin {
			myRole = roleAdmin
		} else if pm != nil {
			myRole = pm.Role
		}
		members, err := projectMembersJSON(gdb, p.ID)
		if err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not list members")
		}
		var owner models.Team
		teamName := ""
		if err := gdb.Select("name").First(&owner, "id = ?", p.TeamID).Error; err == nil {
			teamName = owner.Name
		}
		return c.JSON(fiber.Map{
			"id": p.ID, "team_id": p.TeamID, "team_name": teamName,
			"teams": teamsForProjects(gdb, p.ID),
			"name":  p.Name, "key": p.Key,
			"description": p.Description, "my_role": myRole, "members": members,
			"gh_repo": p.GhRepo, // repo is not secret; the token never leaves the db
		})
	}
}

// projectMembersJSON lists members with user identity for display.
func projectMembersJSON(gdb *gorm.DB, projectID string) ([]projectMemberJSON, error) {
	var out []projectMemberJSON
	err := gdb.Table("project_members").
		Select("project_members.user_id, project_members.role, users.name, users.email").
		Joins("JOIN users ON users.id = project_members.user_id").
		Where("project_members.project_id = ?", projectID).
		Order("users.name").
		Scan(&out).Error
	return out, err
}

// PatchProject: PATCH /api/projects/:id — global admin only. Members see
// the project, so they get 403 (not 404) here.
func PatchProject(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		p, _, ok := loadVisibleProject(c, gdb, u, c.Params("id"))
		if !ok {
			return nil
		}
		if u.GlobalRole != roleAdmin {
			return httpErr(c, fiber.StatusForbidden, "forbidden", "global admin role required")
		}
		var req patchProjectReq
		if err := c.BodyParser(&req); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		}
		updates := map[string]interface{}{}
		if req.Name != nil {
			if strings.TrimSpace(*req.Name) == "" {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "name must not be empty")
			}
			updates["name"] = strings.TrimSpace(*req.Name)
		}
		if req.Key != nil {
			k := strings.TrimSpace(*req.Key)
			if !projectKeyRe.MatchString(k) {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "key must be 2-10 chars: uppercase letter first, then uppercase letters or digits")
			}
			updates["key"] = k
		}
		if req.Description != nil {
			updates["description"] = *req.Description
		}
		if len(updates) > 0 {
			if err := gdb.Model(p).Updates(updates).Error; err != nil {
				if isUniqueViolation(err) {
					return httpErr(c, fiber.StatusConflict, "key_exists", "a project with this key already exists")
				}
				return httpErr(c, fiber.StatusInternalServerError, "internal", "could not update project")
			}
			// map Updates doesn't sync the struct; reload for the response.
			if err := gdb.First(p, "id = ?", p.ID).Error; err != nil {
				return httpErr(c, fiber.StatusInternalServerError, "internal", "could not reload project")
			}
		}
		var team models.Team
		teamName := ""
		if err := gdb.Select("name").First(&team, "id = ?", p.TeamID).Error; err == nil {
			teamName = team.Name
		}
		return c.JSON(toProjectJSON(p, teamName))
	}
}

// DeleteProject: DELETE /api/projects/:id — global admin; soft delete
// (sets deleted_at per api-contract).
func DeleteProject(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		p, _, ok := loadVisibleProject(c, gdb, u, c.Params("id"))
		if !ok {
			return nil
		}
		if u.GlobalRole != roleAdmin {
			return httpErr(c, fiber.StatusForbidden, "forbidden", "global admin role required")
		}
		if err := gdb.Delete(p).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not delete project")
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}

// ReplaceProjectMembers: PUT /api/projects/:id/members — global admin or
// project_admin. Full replace; at least one project_admin must remain.
func ReplaceProjectMembers(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		p, pm, ok := loadVisibleProject(c, gdb, u, c.Params("id"))
		if !ok {
			return nil
		}
		if u.GlobalRole != roleAdmin && (pm == nil || pm.Role != roleProjectAdmin) {
			return httpErr(c, fiber.StatusForbidden, "forbidden", "project admin role required")
		}
		var req replaceProjectMembersReq
		if err := c.BodyParser(&req); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		}
		ids := make([]string, len(req.Members))
		admins := 0
		for i, m := range req.Members {
			if m.Role != roleProjectAdmin && m.Role != roleMember {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "role must be project_admin or member")
			}
			if m.Role == roleProjectAdmin {
				admins++
			}
			ids[i] = m.UserID
		}
		if admins == 0 {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "at least one project_admin must remain")
		}
		if msg := validateUserIDs(gdb, ids); msg != "" {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", msg)
		}
		err := gdb.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("project_id = ?", p.ID).Delete(&models.ProjectMember{}).Error; err != nil {
				return err
			}
			rows := make([]models.ProjectMember, len(req.Members))
			for i, m := range req.Members {
				rows[i] = models.ProjectMember{ProjectID: p.ID, UserID: m.UserID, Role: m.Role}
			}
			return tx.Create(&rows).Error
		})
		if err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not replace members")
		}
		members, err := projectMembersJSON(gdb, p.ID)
		if err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not list members")
		}
		return c.JSON(fiber.Map{"members": members})
	}
}

// PatchProjectTeams: PATCH /api/projects/:id/teams — replace the set of
// contributing teams (global admin or project_admin). team_ids must be
// uuids referencing existing teams and non-empty; the owning team
// (projects.team_id) is always kept in the set (removal attempts 422).
func PatchProjectTeams(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		p, pm, ok := loadVisibleProject(c, gdb, u, c.Params("id"))
		if !ok {
			return nil
		}
		if u.GlobalRole != roleAdmin && (pm == nil || pm.Role != roleProjectAdmin) {
			return httpErr(c, fiber.StatusForbidden, "forbidden", "project admin role required")
		}
		var req struct {
			TeamIDs []string `json:"team_ids"`
		}
		if err := c.BodyParser(&req); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		}
		seen := map[string]bool{}
		uniq := make([]string, 0, len(req.TeamIDs))
		owns := false
		for _, id := range req.TeamIDs {
			if id == "" || seen[id] {
				continue
			}
			if _, err := uuid.Parse(id); err != nil {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "team_ids must be uuids")
			}
			seen[id] = true
			uniq = append(uniq, id)
			if id == p.TeamID {
				owns = true
			}
		}
		if len(uniq) == 0 {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "team_ids must not be empty")
		}
		if !owns {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "the owning team must stay in the set")
		}
		var cnt int64
		if err := gdb.Model(&models.Team{}).Where("id IN ?", uniq).Count(&cnt).Error; err != nil || cnt != int64(len(uniq)) {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "team_ids must reference existing teams")
		}
		err := gdb.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("project_id = ?", p.ID).Delete(&models.ProjectTeam{}).Error; err != nil {
				return err
			}
			rows := make([]models.ProjectTeam, len(uniq))
			for i, id := range uniq {
				rows[i] = models.ProjectTeam{ProjectID: p.ID, TeamID: id}
			}
			return tx.Create(&rows).Error
		})
		if err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not update teams")
		}
		return c.JSON(fiber.Map{"data": teamsForProjects(gdb, p.ID)})
	}
}
