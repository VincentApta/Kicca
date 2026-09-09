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

	"github.com/VincentApta/Kicca/api/internal/auth"
	"github.com/VincentApta/Kicca/api/internal/models"
)

const (
	defaultPerPage = 50
	roleAdmin      = "admin"
	roleMember     = "member"
)

type createUserReq struct {
	Email      string `json:"email"`
	Name       string `json:"name"`
	Password   string `json:"password"`
	GlobalRole string `json:"global_role"`
}

type patchUserReq struct {
	Name       *string `json:"name"`
	GlobalRole *string `json:"global_role"`
	Password   *string `json:"password"`
	Disabled   *bool   `json:"disabled"`
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
		if req.GlobalRole != roleAdmin && req.GlobalRole != roleMember {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "global_role must be admin or member")
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
		return c.Status(fiber.StatusCreated).JSON(toUserJSON(user))
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
		if req.GlobalRole != nil {
			if *req.GlobalRole != roleAdmin && *req.GlobalRole != roleMember {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "global_role must be admin or member")
			}
			updates["global_role"] = *req.GlobalRole
		}
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
		return c.JSON(toUserJSON(&user))
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
