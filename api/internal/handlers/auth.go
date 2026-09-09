// Package handlers: kicca HTTP handlers. Error shape per api-contract:
// { "error": { "code": "...", "message": "..." } }.
package handlers

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/VincentApta/Kicca/api/internal/auth"
	"github.com/VincentApta/Kicca/api/internal/middleware"
	"github.com/VincentApta/Kicca/api/internal/models"
)

// userJSON is the wire `user` shape: {id, email, name, global_role}. The
// password hash is never serialized.
type userJSON struct {
	ID         string `json:"id"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	GlobalRole string `json:"global_role"`
}

func toUserJSON(u *models.User) userJSON {
	return userJSON{ID: u.ID, Email: u.Email, Name: u.Name, GlobalRole: u.GlobalRole}
}

// httpErr emits the contract error shape.
func httpErr(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": fiber.Map{"code": code, "message": message}})
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login: POST /api/auth/login → 200 {user} + session cookie. 401 on bad
// credentials or disabled account.
func Login(jwtSecret string, gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req loginReq
		if err := c.BodyParser(&req); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		}
		var user models.User
		err := gdb.First(&user, "email = ?", strings.ToLower(strings.TrimSpace(req.Email))).Error
		if err != nil || !auth.CheckPassword(user.PasswordHash, req.Password) {
			return httpErr(c, fiber.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		}
		if user.Disabled() {
			return httpErr(c, fiber.StatusUnauthorized, "account_disabled", "account is disabled")
		}
		token, err := auth.SignToken(jwtSecret, user.ID)
		if err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not create session")
		}
		c.Cookie(&fiber.Cookie{
			Name:     auth.CookieName,
			Value:    token,
			Path:     "/",
			Expires:  time.Now().Add(auth.SessionTTL),
			HTTPOnly: true,
			SameSite: fiber.CookieSameSiteLaxMode,
		})
		return c.JSON(fiber.Map{"user": toUserJSON(&user)})
	}
}

// Logout: POST /api/auth/logout → 204, clears the session cookie.
func Logout(c *fiber.Ctx) error {
	c.ClearCookie(auth.CookieName)
	return c.SendStatus(fiber.StatusNoContent)
}

// Me: GET /api/auth/me → 200 {user}. Behind RequireAuth.
func Me(c *fiber.Ctx) error {
	u := c.Locals(middleware.UserKey).(*models.User)
	return c.JSON(fiber.Map{"user": toUserJSON(u)})
}
