// Package handlers: kica HTTP handlers. Error shape per api-contract:
// { "error": { "code": "...", "message": "..." } }.
package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/VincentApta/Kica/api/internal/auth"
	"github.com/VincentApta/Kica/api/internal/middleware"
	"github.com/VincentApta/Kica/api/internal/models"
)

// userJSON is the wire `user` shape: {id, email, name, global_role}. The
// password hash is never serialized. ProjectIDs carries a client's project
// links on the admin user-management responses only (omitempty elsewhere).
// Disabled flags a soft-disabled account (issue #45 — Users page toggle);
// omitted on active accounts.
type userJSON struct {
	ID         string   `json:"id"`
	Email      string   `json:"email"`
	Name       string   `json:"name"`
	GlobalRole string   `json:"global_role"`
	ProjectIDs []string `json:"project_ids,omitempty"`
	Disabled   bool     `json:"disabled,omitempty"`
}

func toUserJSON(u *models.User) userJSON {
	return userJSON{ID: u.ID, Email: u.Email, Name: u.Name, GlobalRole: u.GlobalRole, Disabled: u.Disabled()}
}

// httpErr emits the contract error shape.
func httpErr(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": fiber.Map{"code": code, "message": message}})
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// loginGuard: in-memory failed-login tracker. Key = lowercase(email) + "|" +
// client IP. 5 consecutive failures lock the key for 15 minutes; any
// successful login clears it. Process-local state (lost on restart —
// acceptable). `ponytail:` swap to a Redis-backed store if multiple API
// replicas ever run.
type loginGuard struct {
	mu        sync.Mutex
	maxFails  int
	lockout   time.Duration
	entries   map[string]loginEntry
	lastSweep time.Time
}

type loginEntry struct {
	fails       int
	lockedUntil time.Time
}

func newLoginGuard() *loginGuard {
	return &loginGuard{maxFails: 5, lockout: 15 * time.Minute, entries: map[string]loginEntry{}}
}

// remaining reports lockout time left for key (0 = not locked).
func (g *loginGuard) remaining(key string, now time.Time) time.Duration {
	g.mu.Lock()
	defer g.mu.Unlock()
	e, ok := g.entries[key]
	if !ok || !now.Before(e.lockedUntil) {
		return 0
	}
	return e.lockedUntil.Sub(now)
}

// fail records a failure; the maxFails-th consecutive one starts the lockout.
func (g *loginGuard) fail(key string, now time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	// occasional sweep so keys that go quiet don't accumulate forever
	if now.Sub(g.lastSweep) > g.lockout {
		for k, e := range g.entries {
			if !now.Before(e.lockedUntil) {
				delete(g.entries, k)
			}
		}
		g.lastSweep = now
	}
	e := g.entries[key]
	if !e.lockedUntil.IsZero() && !now.Before(e.lockedUntil) {
		e.fails = 0 // expired lockout — start a fresh streak
	}
	e.fails++
	if e.fails >= g.maxFails {
		e.lockedUntil = now.Add(g.lockout)
	}
	g.entries[key] = e
}

// clear drops the key — a successful login resets the streak.
func (g *loginGuard) clear(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.entries, key)
}

// Login: POST /api/auth/login → 200 {user} + session cookie. 401 on bad
// credentials or disabled account. 429 for 15 minutes after 5 consecutive
// failures per email+IP (Retry-After set, minutes left in the message);
// checked before the DB lookup — bad creds shouldn't be free either. Any
// successful login clears the streak.
func Login(jwtSecret string, gdb *gorm.DB) fiber.Handler {
	guard := newLoginGuard()
	return func(c *fiber.Ctx) error {
		var req loginReq
		if err := c.BodyParser(&req); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		}
		email := strings.ToLower(strings.TrimSpace(req.Email))
		key := email + "|" + c.IP()
		now := time.Now()
		if left := guard.remaining(key, now); left > 0 {
			minutes := int((left + time.Minute - 1) / time.Minute) // ceil
			c.Set("Retry-After", strconv.Itoa(int((left+time.Second-1)/time.Second)))
			return httpErr(c, fiber.StatusTooManyRequests, "rate_limited",
				fmt.Sprintf("too many failed login attempts, try again in %d minutes", minutes))
		}
		var user models.User
		err := gdb.First(&user, "email = ?", email).Error
		if err != nil || !auth.CheckPassword(user.PasswordHash, req.Password) {
			guard.fail(key, now)
			return httpErr(c, fiber.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		}
		if user.Disabled() {
			return httpErr(c, fiber.StatusUnauthorized, "account_disabled", "account is disabled")
		}
		guard.clear(key)
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

type patchMeReq struct {
	Name            *string `json:"name"`
	CurrentPassword *string `json:"current_password"`
	NewPassword     *string `json:"new_password"`
}

// PatchMe: PATCH /api/me (#49) — self-service name + password change for the
// CURRENT user only. Non-admin (admins keep the Users page). A password
// change verifies the current password first; email never changes here (it's
// the login identity — admin-only path). Behind RequireAuth.
func PatchMe(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		if u.GlobalRole == roleAdmin {
			return httpErr(c, fiber.StatusForbidden, "forbidden", "admins manage their own account on the Users page")
		}
		var req patchMeReq
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
		if req.NewPassword != nil {
			if *req.NewPassword == "" {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "new_password must not be empty")
			}
			if req.CurrentPassword == nil || !auth.CheckPassword(u.PasswordHash, *req.CurrentPassword) {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "current password is incorrect")
			}
			hash, err := auth.HashPassword(*req.NewPassword)
			if err != nil {
				return httpErr(c, fiber.StatusInternalServerError, "internal", "could not hash password")
			}
			updates["password_hash"] = hash
		}
		if len(updates) > 0 {
			if err := gdb.Model(u).Updates(updates).Error; err != nil {
				return httpErr(c, fiber.StatusInternalServerError, "internal", "could not update profile")
			}
		}
		var fresh models.User
		if err := gdb.First(&fresh, "id = ?", u.ID).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not reload user")
		}
		return c.JSON(toUserJSON(&fresh))
	}
}
