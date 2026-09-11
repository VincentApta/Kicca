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
	Name        string `json:"name"`
	Key         string `json:"key"`
	Description string `json:"description"`
}

func toProjectJSON(p *models.Project) projectJSON {
	return projectJSON{ID: p.ID, TeamID: p.TeamID, Name: p.Name, Key: p.Key, Description: p.Description}
}

type projectMemberJSON struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	Name   string `json:"name"`
	Email  string `json:"email"`
}

type createProjectReq struct {
	TeamID      string  `json:"team_id"`
	Name        string  `json:"name"`
	Key         string  `json:"key"`
	Description *string `json:"description"`
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
		httpErr(c, fiber.StatusNotFound, "not_found", "no such project")
		return nil, nil, false
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
				memberOf := gdb.Model(&models.ProjectMember{}).Select("project_id").Where("user_id = ?", u.ID)
				q = q.Where("id IN (?)", memberOf)
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
		data := make([]projectJSON, len(projects))
		for i := range projects {
			data[i] = toProjectJSON(&projects[i])
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
		p := &models.Project{TeamID: req.TeamID, Name: req.Name, Key: req.Key}
		if req.Description != nil {
			p.Description = *req.Description
		}
		// Same transaction: creator becomes project_admin (domain rule) so
		// fresh projects have members populated.
		err := gdb.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(p).Error; err != nil {
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
		return c.Status(fiber.StatusCreated).JSON(toProjectJSON(p))
	}
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
		myRole := roleAdmin
		if pm != nil {
			myRole = pm.Role
		}
		members, err := projectMembersJSON(gdb, p.ID)
		if err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not list members")
		}
		return c.JSON(fiber.Map{
			"id": p.ID, "team_id": p.TeamID, "name": p.Name, "key": p.Key,
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
		return c.JSON(toProjectJSON(p))
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
