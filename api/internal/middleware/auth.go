// Package middleware: Fiber auth guards. RequiresAuth parses the session
// cookie, loads the user, and rejects disabled accounts (domain rule 7).
package middleware

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/VincentApta/Kicca/api/internal/auth"
	"github.com/VincentApta/Kicca/api/internal/models"
)

const (
	// UserKey is the c.Locals key holding the loaded *models.User.
	UserKey = "user"
)

// loadUser authenticates the session cookie. A nil return means rejected:
// the 401 response is already written, the caller must return nil (not
// c.Next) so the chain stops without tripping Fiber's error handler.
func loadUser(c *fiber.Ctx, jwtSecret string, gdb *gorm.DB) *models.User {
	token := c.Cookies(auth.CookieName)
	if token == "" {
		unauthorized(c, "missing_session")
		return nil
	}
	userID, err := auth.ParseToken(jwtSecret, token)
	if err != nil {
		unauthorized(c, "invalid_session")
		return nil
	}
	var user models.User
	if err := gdb.First(&user, "id = ?", userID).Error; err != nil {
		unauthorized(c, "invalid_session")
		return nil
	}
	if user.Disabled() {
		unauthorized(c, "account_disabled")
		return nil
	}
	return &user
}

// RequireAuth: 401 on missing/invalid token or disabled user.
func RequireAuth(jwtSecret string, gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		user := loadUser(c, jwtSecret, gdb)
		if user == nil {
			return nil
		}
		c.Locals(UserKey, user)
		return c.Next()
	}
}

// RequireAdmin: auth first, then 403 unless global_role=admin. Does NOT call
// RequireAuth directly — that would run its c.Next() inline and execute the
// route handler twice (the second c.Next overwrites the response with 404).
func RequireAdmin(jwtSecret string, gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		user := loadUser(c, jwtSecret, gdb)
		if user == nil {
			return nil
		}
		if user.GlobalRole != "admin" {
			return forbidden(c, "global admin role required")
		}
		return c.Next()
	}
}

// RequireTeam: auth, then 403 for clients — every team surface (users/teams/
// projects/tasks/labels) is off-limits to the client role; they live on
// /api/client/* only. Same loadUser-not-RequireAuth rule as RequireAdmin.
func RequireTeam(jwtSecret string, gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		user := loadUser(c, jwtSecret, gdb)
		if user == nil {
			return nil
		}
		if user.GlobalRole == "client" {
			return forbidden(c, "team access required")
		}
		c.Locals(UserKey, user)
		return c.Next()
	}
}

// RequireClient: auth, then 403 unless global_role=client — the portal routes.
func RequireClient(jwtSecret string, gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		user := loadUser(c, jwtSecret, gdb)
		if user == nil {
			return nil
		}
		if user.GlobalRole != "client" {
			return forbidden(c, "client role required")
		}
		c.Locals(UserKey, user)
		return c.Next()
	}
}

func forbidden(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
		"error": fiber.Map{"code": "forbidden", "message": message},
	})
}

func unauthorized(c *fiber.Ctx, code string) {
	c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"error": fiber.Map{"code": code, "message": "authentication required"},
	})
}
