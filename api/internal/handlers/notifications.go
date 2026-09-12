// Notifications: unread task_events relevant to the current user.
//
// Reuses task_events as the feed (this is the `ponytail:`-endorsed design —
// no per-user notification rows). An event is relevant to a user who is the
// task's creator OR current assignee, and is not the event actor. Read state
// is a single per-user watermark (notification_reads.last_read_at); unread =
// relevant events with At > watermark, mark-all-read = bump the watermark.
package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/VincentApta/Kica/api/internal/models"
)

// NotificationRead: per-user read watermark for the notification feed.
type NotificationRead struct {
	UserID     string    `gorm:"primaryKey;type:uuid"`
	LastReadAt time.Time `gorm:"not null"`
}

func (NotificationRead) TableName() string { return "notification_reads" }

type notifJSON struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	At         time.Time       `json:"at"`
	ActorName  string          `json:"actor_name"`
	TaskID     string          `json:"task_id"`
	TaskNumber int64           `json:"task_number"`
	TaskTitle  string          `json:"task_title"`
	ProjectKey string          `json:"project_key"`
	ProjectID string          `json:"project_id"`
	FromStatus *string         `json:"from_status,omitempty"`
	ToStatus   string          `json:"to_status"`
}

// ListNotifications: GET /api/notifications — team + client both. Returns the
// relevant unread events (after the user's watermark) newest-first.
func ListNotifications(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)

		var wm NotificationRead
		err := gdb.First(&wm, "user_id = ?", u.ID).Error
		after := time.Time{}
		if err == nil {
			after = wm.LastReadAt
		}

		var events []struct {
			models.TaskEvent
			ActorName  string
			TaskNumber int64
			TaskTitle  string
			ProjectKey string
			ProjectID  string
		}
		q := gdb.Table("task_events").
			Select(`task_events.id, task_events.task_id, task_events.actor_id,
				task_events.from_status, task_events.to_status, task_events.at,
				task_events.type, task_events.comment_id,
				users.name AS actor_name, tasks.number AS task_number,
				tasks.title AS task_title, projects.key AS project_key,
				projects.id AS project_id`).
			Joins("JOIN users ON users.id = task_events.actor_id").
			Joins("JOIN tasks ON tasks.id = task_events.task_id").
			Joins("JOIN projects ON projects.id = tasks.project_id").
			Where("task_events.at > ?", after).
			Where("task_events.actor_id <> ?", u.ID).
			// creation events (from_status NULL) aren't notifications — only
			// transitions, assignments, comments on tasks I now own
			Where("task_events.from_status IS NOT NULL OR task_events.type <> 'status'").
			Where("(tasks.created_by = ? OR tasks.assignee_id = ?)", u.ID, u.ID).
			Order("task_events.at DESC").
			Limit(50)

		if err := q.Scan(&events).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not load notifications")
		}

		out := make([]notifJSON, 0, len(events))
		for _, ev := range events {
			out = append(out, notifJSON{
				ID:         ev.ID,
				Type:       ev.Type,
				At:         ev.At,
				ActorName:  ev.ActorName,
				TaskID:     ev.TaskID,
				TaskNumber: ev.TaskNumber,
				TaskTitle:  ev.TaskTitle,
				ProjectKey: ev.ProjectKey,
				ProjectID: ev.ProjectID,
				FromStatus: ev.FromStatus,
				ToStatus:   ev.ToStatus,
			})
		}
		return c.JSON(fiber.Map{"data": out})
	}
}

// MarkNotificationsRead: POST /api/notifications/read — bumps the watermark to
// now. Idempotent.
func MarkNotificationsRead(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		now := time.Now().UTC()
		if err := gdb.Save(&NotificationRead{UserID: u.ID, LastReadAt: now}).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not mark read")
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}