// Route registration. Static routes are registered before any parametric
// one so Fiber matches /api/health, /api/auth/* and /api/users before
// /api/users/:id.
package handlers

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/VincentApta/Kicca/api/internal/middleware"
)

// Register mounts all v1 routes. jwtSecret signs/verifies session cookies.
func Register(app *fiber.App, gdb *gorm.DB, jwtSecret string) {
	api := app.Group("/api")

	// static
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})
	authG := api.Group("/auth")
	authG.Post("/login", Login(jwtSecret, gdb))
	authG.Post("/logout", Logout)
	authG.Get("/me", middleware.RequireAuth(jwtSecret, gdb), Me)

	// user management — global admin only
	users := api.Group("/users", middleware.RequireAdmin(jwtSecret, gdb))
	users.Get("/", ListUsers(gdb))
	users.Post("/", CreateUser(gdb))

	// parametric — after the static /api/users routes above
	users.Patch("/:id", PatchUser(gdb))
}
