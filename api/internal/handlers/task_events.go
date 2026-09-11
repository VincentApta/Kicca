// TaskEvents: GET /api/tasks/events?days=30&project_id=... — per-day count
// of tasks in each status over the window (issue #26). Computed by walking
// task_events chronologically per task: the day a transition lands on owns
// the resulting status; days before a task's first event (creation) don't
// count it. Visibility-scoped like MyTasks (admin all, member visible
// projects); an explicit project_id goes through loadVisibleProject (404
// no-leak for outsiders).
package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/VincentApta/Kicca/api/internal/models"
)

// maxEventsDays caps the window.
const maxEventsDays = 90

// eventsDay is one day's row: date + count per status at end of day.
type eventsDay struct {
	Date   string         `json:"date"`
	Counts map[string]int `json:"counts"`
}

func eventsRows(now time.Time, days int) []eventsDay {
	start := now.AddDate(0, 0, -(days - 1)).Truncate(24 * time.Hour)
	dayEnds := make([]time.Time, days)
	for d := range dayEnds {
		dayEnds[d] = start.AddDate(0, 0, d+1)
	}
	rows := make([]eventsDay, days)
	for i := range rows {
		rows[i] = eventsDay{Date: start.AddDate(0, 0, i).Format("2006-01-02"), Counts: map[string]int{}}
	}
	return rows
}

// TaskEvents handler. See file comment for the walk.
func TaskEvents(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		days := queryInt(c, "days", 30)
		if days > maxEventsDays {
			days = maxEventsDays
		}
		now := time.Now().UTC()
		rows := eventsRows(now, days)
		projectID := c.Query("project_id")
		if projectID != "" {
			if _, _, ok := loadVisibleProject(c, gdb, u, projectID); !ok {
				return nil
			}
		}

		// live (non-trashed) tasks in scope
		scope := gdb.Unscoped().Model(&models.Task{}).Where("deleted_at IS NULL")
		if projectID != "" {
			scope = scope.Where("project_id = ?", projectID)
		} else if u.GlobalRole != roleAdmin {
			visible := gdb.Model(&models.ProjectMember{}).Select("project_id").Where("user_id = ?", u.ID)
			scope = scope.Where("project_id IN (?)", visible)
		}
		var ids []string
		if err := scope.Pluck("id", &ids).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not load tasks")
		}
		if len(ids) == 0 {
			return c.JSON(fiber.Map{"days": days, "data": rows})
		}

		var events []models.TaskEvent
		if err := gdb.Where("task_id IN ?", ids).Order("task_id, at").Find(&events).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not load task events")
		}
		byTask := make(map[string][]models.TaskEvent, len(ids))
		for i := range events {
			if events[i].At.IsZero() {
				continue // unusable row (should not happen; At is stamped)
			}
			byTask[events[i].TaskID] = append(byTask[events[i].TaskID], events[i])
		}

		start := now.AddDate(0, 0, -(days - 1)).Truncate(24 * time.Hour)
		for _, id := range ids {
			trs := byTask[id]
			cur := 0
			for d := 0; d < days; d++ {
				dayEnd := start.AddDate(0, 0, d+1)
				for cur < len(trs) && !trs[cur].At.After(dayEnd) {
					cur++
				}
				if cur > 0 {
					rows[d].Counts[trs[cur-1].ToStatus]++
				}
			}
		}
		return c.JSON(fiber.Map{"days": days, "data": rows})
	}
}
