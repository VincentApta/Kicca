// Global search: GET /api/search?q= — cross-project for team users.
// Tasks matched on title/description ILIKE, scoped to visible projects
// (admin: all; others: project_member rows — same predicate as
// loadVisibleProject/ListProjects). Projects matched on name/key ILIKE.
// q < 2 chars → empty result. Limits: 20 tasks, 10 projects.
package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/VincentApta/Kica/api/internal/models"
)

type searchTaskJSON struct {
	ID         string `json:"id"`
	ProjectID  string `json:"project_id"`
	ProjectKey string `json:"project_key"`
	Number     int64  `json:"number"`
	Title      string `json:"title"`
	Status     string `json:"status"`
}

type searchProjectJSON struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

type searchOut struct {
	Tasks    []searchTaskJSON    `json:"tasks"`
	Projects []searchProjectJSON `json:"projects"`
}

func Search(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		q := strings.TrimSpace(c.Query("q"))
		out := searchOut{Tasks: []searchTaskJSON{}, Projects: []searchProjectJSON{}}
		if len(q) < 2 {
			return c.JSON(fiber.Map{"data": out})
		}
		like := "%" + strings.ToLower(q) + "%"

		// member scoping — same predicate shape as ListProjects
		scope := gdb.Model(&models.Project{})
		if u.GlobalRole != roleAdmin {
			visible := visibleProjectIDs(gdb, u.ID)
			scope = scope.Where("id IN (?)", visible)
		}

		var projects []models.Project
		if err := scope.Where("LOWER(name) LIKE ? OR LOWER(key) LIKE ?", like, like).
			Order("name").Limit(10).Find(&projects).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not search")
		}
		for i := range projects {
			out.Projects = append(out.Projects, searchProjectJSON{ID: projects[i].ID, Key: projects[i].Key, Name: projects[i].Name})
		}

		// tasks join projects for key + scoping; trash excluded
		tq := gdb.Model(&models.Task{}).
			Select("tasks.id, tasks.project_id, projects.key AS project_key, tasks.number, tasks.title, tasks.status").
			Joins("JOIN projects ON projects.id = tasks.project_id").
			Where("tasks.deleted_at IS NULL").
			Where("(LOWER(tasks.title) LIKE ? OR LOWER(tasks.description) LIKE ?)", like, like)
		if u.GlobalRole != roleAdmin {
			visible := visibleProjectIDs(gdb, u.ID)
			tq = tq.Where("tasks.project_id IN (?)", visible)
		}
		var tasks []searchTaskJSON
		if err := tq.Order("tasks.updated_at DESC").Limit(20).Find(&tasks).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not search")
		}
		if tasks != nil {
			out.Tasks = tasks
		}
		return c.JSON(fiber.Map{"data": out})
	}
}
