// Route registration. Static routes are registered before any parametric
// one so Fiber matches /api/health, /api/auth/*, /api/users, /api/teams and
// /api/projects before their /:id subpaths.
package handlers

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/VincentApta/Kicca/api/internal/middleware"
)

// Register mounts all v1 routes. jwtSecret signs/verifies session cookies;
// ghEncKey (from GH_ENC_KEY) seals GitHub PATs — nil disables the feature.
func Register(app *fiber.App, gdb *gorm.DB, jwtSecret string, ghEncKey *[32]byte) {
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
	teams.Get("/:id/members", ListTeamMembers(gdb))
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
	projects.Get("/:id/tasks", ListTasks(gdb))
	projects.Post("/:id/tasks", CreateTask(gdb))
	projects.Get("/:id/labels", ListLabels(gdb))
	projects.Post("/:id/labels", CreateLabel(gdb))
	projects.Put("/:id/github", PutProjectGithub(gdb, ghEncKey))

	// tasks — GET / must be registered BEFORE the parametric /:id route
	tasks := api.Group("/tasks", middleware.RequireAuth(jwtSecret, gdb))
	tasks.Get("/", MyTasks(gdb))
	tasks.Get("/:id", GetTask(gdb))
	tasks.Patch("/:id", PatchTask(gdb))
	tasks.Delete("/:id", DeleteTask(gdb))
	tasks.Post("/:id/restore", RestoreTask(gdb))
	tasks.Post("/:id/move", MoveTask(gdb))
	tasks.Get("/:id/comments", ListComments(gdb))
	tasks.Post("/:id/comments", CreateComment(gdb))
	tasks.Post("/:id/github/issue", CreateTaskIssue(gdb, ghEncKey))

	// labels — DELETE only (creation is per-project above)
	labels := api.Group("/labels", middleware.RequireAuth(jwtSecret, gdb))
	labels.Delete("/:id", DeleteLabel(gdb))
}
