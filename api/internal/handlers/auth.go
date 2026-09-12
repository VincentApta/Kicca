// Package handlers: kica HTTP handlers. Error shape per api-contract:
// { "error": { "code": "...", "message": "..." } }.
package handlers

import (
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

// loginLimiter: fixed-window rate limiter over arbitrary keys (the login
// handler uses "ip:<addr>" and "email:<addr>"). Simple in-memory — fine for
// one api container. `ponytail:` shared store + trusted-proxy config if the
// api ever scales beyond that.
type loginLimiter struct {
	mu        sync.Mutex
	window    time.Duration
	limit     int
	hits      map[string][]time.Time
	lastSweep time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{window: time.Minute, limit: 10, hits: map[string][]time.Time{}}
}

// allow records a hit on key unless the window already holds limit hits.
func (l *loginLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	// occasional sweep so keys that go quiet don't accumulate forever
	if now.Sub(l.lastSweep) > l.window {
		for k, v := range l.hits {
			l.prune(k, v, now)
		}
		l.lastSweep = now
	}
	hits := l.prune(key, l.hits[key], now)
	if len(hits) >= l.limit {
		l.hits[key] = hits
		return false
	}
	l.hits[key] = append(hits, now)
	return true
}

// prune drops expired timestamps; empty slices delete their key.
func (l *loginLimiter) prune(key string, hits []time.Time, now time.Time) []time.Time {
	kept := hits[:0]
	for _, h := range hits {
		if now.Sub(h) < l.window {
			kept = append(kept, h)
		}
	}
	if len(kept) == 0 {
		delete(l.hits, key)
	}
	return kept
}

// Login: POST /api/auth/login → 200 {user} + session cookie. 401 on bad
// credentials or disabled account. 429 after 10 attempts/min per IP and per
// email (checked before the DB lookup — bad creds shouldn't be free either).
func Login(jwtSecret string, gdb *gorm.DB) fiber.Handler {
	limiter := newLoginLimiter()
	return func(c *fiber.Ctx) error {
		var req loginReq
		if err := c.BodyParser(&req); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		}
		email := strings.ToLower(strings.TrimSpace(req.Email))
		now := time.Now()
		if !limiter.allow("ip:"+c.IP(), now) || !limiter.allow("email:"+email, now) {
			return httpErr(c, fiber.StatusTooManyRequests, "rate_limited", "too many login attempts, try again in a minute")
		}
		var user models.User
		err := gdb.First(&user, "email = ?", email).Error
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
