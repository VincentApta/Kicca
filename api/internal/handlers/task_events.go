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

	"github.com/VincentApta/Kica/api/internal/models"
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

// activityEventJSON is one timeline row (issue #44). Team scope carries the
// full actor user shape; the client surface swaps in clientActorJSON (name
// only — an event row has no assessment/team fields, but an actor email is
// still team data).
type activityEventJSON struct {
	ID         string      `json:"id"`
	FromStatus *string     `json:"from_status"`
	ToStatus   string      `json:"to_status"`
	Actor      interface{} `json:"actor"`
	At         time.Time   `json:"at"`
}

// clientActorJSON: actor name only on the client surface.
type clientActorJSON struct {
	Name string `json:"name"`
}

// activityEvents loads a task's transition log newest-first and resolves
// actors in one batch. Missing actor rows fall back to "Unknown" (defensive
// — actors are users, and users are never hard-deleted).
func activityEvents(gdb *gorm.DB, taskID string, clientScope bool) ([]activityEventJSON, error) {
	var evs []models.TaskEvent
	if err := gdb.Where("task_id = ?", taskID).Order("at DESC, id DESC").Find(&evs).Error; err != nil {
		return nil, err
	}
	actorIDs := make([]string, 0, len(evs))
	for i := range evs {
		actorIDs = append(actorIDs, evs[i].ActorID)
	}
	users := map[string]models.User{}
	if len(actorIDs) > 0 {
		var rows []models.User
		if err := gdb.Where("id IN ?", actorIDs).Find(&rows).Error; err != nil {
			return nil, err
		}
		for i := range rows {
			users[rows[i].ID] = rows[i]
		}
	}
	out := make([]activityEventJSON, len(evs))
	for i := range evs {
		out[i] = activityEventJSON{
			ID: evs[i].ID, FromStatus: evs[i].FromStatus, ToStatus: evs[i].ToStatus, At: evs[i].At,
		}
		if a, ok := users[evs[i].ActorID]; ok {
			if clientScope {
				out[i].Actor = clientActorJSON{Name: a.Name}
			} else {
				u := a
				out[i].Actor = toUserJSON(&u)
			}
		} else {
			out[i].Actor = clientActorJSON{Name: "Unknown"}
		}
	}
	return out, nil
}

// TaskActivity: GET /api/tasks/:id/events (issue #44) — the drawer's
// activity timeline. Visibility-scoped like the task itself (loadVisibleTask:
// trashed tasks stay reachable).
func TaskActivity(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		t, ok := loadVisibleTask(c, gdb, u, c.Params("id"))
		if !ok {
			return nil
		}
		out, err := activityEvents(gdb, t.ID, false)
		if err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not load activity")
		}
		return c.JSON(fiber.Map{"data": out})
	}
}

// ClientTicketEvents: GET /api/client/tickets/:id/events (issue #44) — same
// timeline through the portal: own tickets only (loadOwnTicket 404 no-leak),
// actor name only.
func ClientTicketEvents(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		t, ok := loadOwnTicket(c, gdb, u, c.Params("id"))
		if !ok {
			return nil
		}
		out, err := activityEvents(gdb, t.ID, true)
		if err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not load activity")
		}
		return c.JSON(fiber.Map{"data": out})
	}
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
