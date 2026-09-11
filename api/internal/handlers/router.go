// Route registration. Static routes are registered before any parametric
// one so Fiber matches /api/health, /api/auth/*, /api/users, /api/teams and
// /api/projects before their /:id subpaths.
package handlers

import (
	"os"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/VincentApta/Kica/api/internal/middleware"
	"github.com/VincentApta/Kica/api/internal/storage"
)

// Register mounts all v1 routes. jwtSecret signs/verifies session cookies;
// ghEncKey (from GH_ENC_KEY) seals GitHub PATs — nil disables the feature.
func Register(app *fiber.App, gdb *gorm.DB, jwtSecret string, ghEncKey *[32]byte) {
	attStore = storage.NewFromEnv()
	maxAttachmentBytes = 25 << 20
	if mb, err := strconv.ParseInt(os.Getenv("ATTACHMENTS_MAX_MB"), 10, 64); err == nil && mb > 0 {
		maxAttachmentBytes = mb << 20
	}

	api := app.Group("/api")

	// static
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})
	authG := api.Group("/auth")
	authG.Post("/login", Login(jwtSecret, gdb))
	authG.Post("/logout", Logout)
	authG.Get("/me", middleware.RequireAuth(jwtSecret, gdb), Me)

	// user management — global admin only (implies non-client)
	users := api.Group("/users", middleware.RequireAdmin(jwtSecret, gdb))
	users.Get("/", ListUsers(gdb))
	users.Post("/", CreateUser(gdb))

	// parametric — after the static /api/users routes above
	users.Patch("/:id", PatchUser(gdb))

	// teams — reads for any team user (members see own), writes global admin.
	// RequireTeam: clients get 403 on every team surface.
	teams := api.Group("/teams", middleware.RequireTeam(jwtSecret, gdb))
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
	projects := api.Group("/projects", middleware.RequireTeam(jwtSecret, gdb))
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

	// tasks — GET / and /events must be registered BEFORE the parametric
	// /:id route (static-before-parametric, same as /api/users above)
	tasks := api.Group("/tasks", middleware.RequireTeam(jwtSecret, gdb))
	tasks.Get("/", MyTasks(gdb))
	tasks.Get("/events", TaskEvents(gdb))
	tasks.Get("/:id", GetTask(gdb))
	tasks.Patch("/:id", PatchTask(gdb))
	tasks.Delete("/:id", DeleteTask(gdb))
	tasks.Post("/:id/restore", RestoreTask(gdb))
	tasks.Post("/:id/move", MoveTask(gdb))
	tasks.Get("/:id/comments", ListComments(gdb))
	tasks.Post("/:id/comments", CreateComment(gdb))
	tasks.Get("/:id/attachments", ListTaskAttachments(gdb))
	tasks.Post("/:id/attachments", UploadTaskAttachment(gdb))
	tasks.Post("/:id/github/issue", CreateTaskIssue(gdb, ghEncKey))

	// attachment blobs — GET is open to both roles (team via project
	// visibility, client via created_by, checked in the handler); DELETE is
	// team-only.
	blobs := api.Group("/attachments", middleware.RequireAuth(jwtSecret, gdb))
	blobs.Get("/:id", GetAttachment(gdb))
	blobs.Delete("/:id", middleware.RequireTeam(jwtSecret, gdb), DeleteAttachment(gdb))

	// labels — DELETE only (creation is per-project above)
	labels := api.Group("/labels", middleware.RequireTeam(jwtSecret, gdb))
	labels.Delete("/:id", DeleteLabel(gdb))

	// client portal — clients only (team users get 403). Static group before
	// nothing parametric; registered last per the static-first convention.
	client := api.Group("/client", middleware.RequireClient(jwtSecret, gdb))
	client.Get("/projects", ClientListProjects(gdb))
	client.Post("/tickets", ClientCreateTicket(gdb))
	client.Get("/tickets", ClientListTickets(gdb))
	client.Post("/tickets/:id/attachments", ClientUploadTicketAttachment(gdb))
	client.Get("/tickets/:id/attachments", ClientListTicketAttachments(gdb))
	client.Get("/tickets/:id/comments", ClientListTicketComments(gdb))
	client.Post("/tickets/:id/comments", ClientCreateTicketComment(gdb))
}
