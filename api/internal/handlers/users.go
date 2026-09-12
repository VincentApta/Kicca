// User CRUD handlers — global admin only (mounted behind RequireAdmin).
package handlers

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/VincentApta/Kica/api/internal/auth"
	"github.com/VincentApta/Kica/api/internal/models"
)

const (
	defaultPerPage = 50
	roleAdmin      = "admin"
	roleMember     = "member"
	roleClient     = "client"
)

var validRoles = map[string]bool{roleAdmin: true, roleMember: true, roleClient: true}

type createUserReq struct {
	Email      string   `json:"email"`
	Name       string   `json:"name"`
	Password   string   `json:"password"`
	GlobalRole string   `json:"global_role"`
	ProjectIDs []string `json:"project_ids"` // client only: projects they may submit tickets into
}

type patchUserReq struct {
	Name       *string  `json:"name"`
	GlobalRole *string  `json:"global_role"`
	Password   *string  `json:"password"`
	Disabled   *bool    `json:"disabled"`
	ProjectIDs []string `json:"project_ids"` // client only: non-nil replaces the link set
}

// ListUsers: GET /api/users?page=&per_page= → {data, page, per_page, total}.
func ListUsers(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		page := queryInt(c, "page", 1)
		perPage := queryInt(c, "per_page", defaultPerPage)
		var total int64
		if err := gdb.Model(&models.User{}).Count(&total).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not count users")
		}
		var users []models.User
		if err := gdb.Order("email").Limit(perPage).Offset((page - 1) * perPage).Find(&users).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not list users")
		}
		data := make([]userJSON, len(users))
		for i := range users {
			data[i] = toUserJSON(&users[i])
		}
		attachClientProjectIDs(gdb, data)
		return c.JSON(fiber.Map{"data": data, "page": page, "per_page": perPage, "total": total})
	}
}

// CreateUser: POST /api/users → 201 user; 409 duplicate email; 422 validation.
func CreateUser(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req createUserReq
		if err := c.BodyParser(&req); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		}
		req.Email = strings.ToLower(strings.TrimSpace(req.Email))
		req.Name = strings.TrimSpace(req.Name)
		if req.GlobalRole == "" {
			req.GlobalRole = roleMember
		}
		if !strings.Contains(req.Email, "@") || req.Name == "" || req.Password == "" {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "email, name and password are required")
		}
		if !validRoles[req.GlobalRole] {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "global_role must be admin, member or client")
		}
		if req.GlobalRole == roleClient {
			req.ProjectIDs = dedupeIDs(req.ProjectIDs)
			if len(req.ProjectIDs) == 0 {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "a client must be linked to at least one project")
			}
		}
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not hash password")
		}
		user := &models.User{Email: req.Email, Name: req.Name, PasswordHash: hash, GlobalRole: req.GlobalRole}
		if err := gdb.Create(user).Error; err != nil {
			if isUniqueViolation(err) {
				return httpErr(c, fiber.StatusConflict, "email_exists", "a user with this email already exists")
			}
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not create user")
		}
		if req.GlobalRole == roleClient {
			if err := linkClientProjects(gdb, user.ID, req.ProjectIDs); err != nil {
				return httpErr(c, fiber.StatusInternalServerError, "internal", "could not link client projects")
			}
		}
		out := toUserJSON(user)
		if req.GlobalRole == roleClient {
			out.ProjectIDs = req.ProjectIDs
		}
		return c.Status(fiber.StatusCreated).JSON(out)
	}
}

// PatchUser: PATCH /api/users/:id → 200 user. Partial: name, global_role,
// password, disabled.
func PatchUser(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if _, err := uuid.Parse(id); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_uuid", "user id must be a uuid")
		}
		var req patchUserReq
		if err := c.BodyParser(&req); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		}
		var user models.User
		if err := gdb.First(&user, "id = ?", id).Error; err != nil {
			return httpErr(c, fiber.StatusNotFound, "user_not_found", "no such user")
		}
		updates := map[string]interface{}{}
		if req.Name != nil {
			if strings.TrimSpace(*req.Name) == "" {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "name must not be empty")
			}
			updates["name"] = strings.TrimSpace(*req.Name)
		}
		finalRole := user.GlobalRole
		if req.GlobalRole != nil {
			if !validRoles[*req.GlobalRole] {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "global_role must be admin, member or client")
			}
			updates["global_role"] = *req.GlobalRole
			finalRole = *req.GlobalRole
		}
		// client link management: non-nil project_ids replaces the set; a
		// client must always keep >= 1 link; leaving the client role drops
		// stale links.
		replaceLinks := false
		if req.ProjectIDs != nil {
			if finalRole != roleClient {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "project_ids only applies to the client role")
			}
			req.ProjectIDs = dedupeIDs(req.ProjectIDs)
			if len(req.ProjectIDs) == 0 {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "a client must be linked to at least one project")
			}
			if err := validateProjectIDs(gdb, req.ProjectIDs); err != nil {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "project_ids must reference existing projects")
			}
			replaceLinks = true
		} else if finalRole == roleClient && user.GlobalRole != roleClient {
			var links int64
			if err := gdb.Model(&models.ClientProject{}).Where("client_id = ?", user.ID).Count(&links).Error; err != nil {
				return httpErr(c, fiber.StatusInternalServerError, "internal", "could not count client projects")
			}
			if links == 0 {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "a client must be linked to at least one project")
			}
		}
		dropLinks := user.GlobalRole == roleClient && finalRole != roleClient
		if req.Password != nil {
			if *req.Password == "" {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "password must not be empty")
			}
			hash, err := auth.HashPassword(*req.Password)
			if err != nil {
				return httpErr(c, fiber.StatusInternalServerError, "internal", "could not hash password")
			}
			updates["password_hash"] = hash
		}
		if req.Disabled != nil {
			// self-disable guard (#45): an admin locking their own account
			// out mid-session; enabling yourself is a no-op (you can't be
			// disabled and still hold a session).
			if *req.Disabled && user.ID == currentUser(c).ID {
				return httpErr(c, fiber.StatusConflict, "self_disable", "you cannot disable your own account")
			}
			if *req.Disabled {
				updates["disabled_at"] = time.Now().UTC()
			} else {
				updates["disabled_at"] = nil
			}
		}
		// last-admin guard: demoting or disabling the only enabled global
		// admin would lock everyone out of user management
		demotes := req.GlobalRole != nil && *req.GlobalRole != roleAdmin
		disables := req.Disabled != nil && *req.Disabled
		if user.GlobalRole == roleAdmin && user.DisabledAt == nil && (demotes || disables) {
			var others int64
			if err := gdb.Model(&models.User{}).
				Where("global_role = ? AND disabled_at IS NULL AND id <> ?", roleAdmin, user.ID).
				Count(&others).Error; err != nil {
				return httpErr(c, fiber.StatusInternalServerError, "internal", "could not count admins")
			}
			if others == 0 {
				return httpErr(c, fiber.StatusConflict, "last_admin", "cannot demote or disable the last enabled admin")
			}
		}
		if len(updates) > 0 {
			if err := gdb.Model(&user).Updates(updates).Error; err != nil {
				return httpErr(c, fiber.StatusInternalServerError, "internal", "could not update user")
			}
			// map Updates doesn't sync the struct; reload for the response.
			if err := gdb.First(&user, "id = ?", id).Error; err != nil {
				return httpErr(c, fiber.StatusInternalServerError, "internal", "could not reload user")
			}
		}
		if replaceLinks || dropLinks {
			err := gdb.Transaction(func(tx *gorm.DB) error {
				if err := tx.Where("client_id = ?", user.ID).Delete(&models.ClientProject{}).Error; err != nil {
					return err
				}
				if replaceLinks {
					return linkClientProjects(tx, user.ID, req.ProjectIDs)
				}
				return nil
			})
			if err != nil {
				return httpErr(c, fiber.StatusInternalServerError, "internal", "could not update client projects")
			}
		}
		out := toUserJSON(&user)
		if finalRole == roleClient {
			if ids, err := linkedProjectIDs(gdb, user.ID); err == nil {
				out.ProjectIDs = ids
			}
		}
		return c.JSON(out)
	}
}

func queryInt(c *fiber.Ctx, key string, fallback int) int {
	if v, err := strconv.Atoi(c.Query(key)); err == nil && v > 0 {
		return v
	}
	return fallback
}

// isUniqueViolation covers both dialects: gorm.ErrDuplicatedKey (TranslateError)
// and raw driver strings for dialects without translation (sqlite in tests).
func isUniqueViolation(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "duplicate key") || strings.Contains(s, "unique constraint")
}

// dedupeIDs preserves order, drops repeats (client_projects PK safety).
func dedupeIDs(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// validateProjectIDs: uuids referencing existing projects.
func validateProjectIDs(gdb *gorm.DB, ids []string) error {
	for _, pid := range ids {
		if _, err := uuid.Parse(pid); err != nil {
			return err
		}
	}
	var count int64
	if err := gdb.Model(&models.Project{}).Where("id IN ?", ids).Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(ids)) {
		return errors.New("one or more projects not found")
	}
	return nil
}

// attachClientProjectIDs fills ProjectIDs on any client-role rows (one
// batched query) so the admin Users page can preselect the link editor.
func attachClientProjectIDs(gdb *gorm.DB, users []userJSON) {
	clientIDs := make([]string, 0, len(users))
	for i := range users {
		if users[i].GlobalRole == roleClient {
			clientIDs = append(clientIDs, users[i].ID)
		}
	}
	if len(clientIDs) == 0 {
		return
	}
	var links []models.ClientProject
	if err := gdb.Where("client_id IN ?", clientIDs).Find(&links).Error; err != nil {
		return
	}
	byClient := map[string][]string{}
	for _, l := range links {
		byClient[l.ClientID] = append(byClient[l.ClientID], l.ProjectID)
	}
	for i := range users {
		users[i].ProjectIDs = byClient[users[i].ID] // nil for non-clients → omitted
	}
}

// linkClientProjects inserts the client_projects rows. Validates each project
// exists; inside a caller transaction any failure rolls the whole thing back.
func linkClientProjects(gdb *gorm.DB, clientID string, projectIDs []string) error {
	if err := validateProjectIDs(gdb, projectIDs); err != nil {
		return err
	}
	rows := make([]models.ClientProject, 0, len(projectIDs))
	for _, pid := range projectIDs {
		rows = append(rows, models.ClientProject{ClientID: clientID, ProjectID: pid})
	}
	return gdb.Create(&rows).Error
}
