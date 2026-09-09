// Route registration. Static routes are registered before any parametric
// one so Fiber matches /api/health, /api/auth/*, /api/users, /api/teams and
// /api/projects before their /:id subpaths.
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

	// teams — reads for any authed user (members see own), writes global admin
	teams := api.Group("/teams", middleware.RequireAuth(jwtSecret, gdb))
	teams.Get("/", ListTeams(gdb))
	teamsAdmin := api.Group("/teams", middleware.RequireAdmin(jwtSecret, gdb))
	teamsAdmin.Post("/", CreateTeam(gdb))
	// parametric — after the static /api/teams route above
	teamsAdmin.Patch("/:id", PatchTeam(gdb))
	teamsAdmin.Delete("/:id", DeleteTeam(gdb))
	teamsAdmin.Put("/:id/members", ReplaceTeamMembers(gdb))

	// projects — visibility scoping (admin or member, else 404 no-leak) lives
	// in the handlers
	projects := api.Group("/projects", middleware.RequireAuth(jwtSecret, gdb))
	projects.Get("/", ListProjects(gdb))
	projects.Post("/", CreateProject(gdb))
	// parametric — after the static /api/projects routes above
	projects.Get("/:id", GetProject(gdb))
	projects.Patch("/:id", PatchProject(gdb))
	projects.Delete("/:id", DeleteProject(gdb))
	projects.Put("/:id/members", ReplaceProjectMembers(gdb))
}
