// CSV export (#50): the board/list toolbar and the client portal header get
// a text/csv attachment of the CURRENT filtered view — same filters and the
// same visibility scoping as the list endpoints (shared builders in
// tasks.go / client.go). encoding/csv does RFC4180 quoting (quote+double);
// the UTF-8 BOM makes Excel open it correctly. Optional ?from=&to=
// (YYYY-MM-DD, inclusive) filters by created_at. `ponytail:` no other
// format options v1.
package handlers

import (
	"bytes"
	"encoding/csv"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/VincentApta/Kica/api/internal/models"
)

// sendCSV writes buf as a downloadable text/csv attachment.
func sendCSV(c *fiber.Ctx, filename string, buf []byte) error {
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="`+filename+`"`)
	return c.Send(buf)
}

func csvTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func csvDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}

func csvInt(i *int) string {
	if i == nil {
		return ""
	}
	return strconv.Itoa(*i)
}

// parseDateRange reads ?from=&to= (YYYY-MM-DD) and returns the inclusive
// [start, end) time range. from → start of day; to → start of NEXT day
// (inclusive date). Either may be empty. Non-empty but malformed → error msg.
func parseDateRange(c *fiber.Ctx) (time.Time, time.Time, string) {
	var from, to time.Time
	if s := c.Query("from"); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return from, to, "from must be YYYY-MM-DD"
		}
		from = t
	}
	if s := c.Query("to"); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return from, to, "to must be YYYY-MM-DD"
		}
		to = t.AddDate(0, 0, 1) // inclusive day
	}
	return from, to, ""
}

// applyDateRange filters tasks by created_at within [from, to).
func applyDateRange(q *gorm.DB, from, to time.Time) *gorm.DB {
	if !from.IsZero() {
		q = q.Where("created_at >= ?", from)
	}
	if !to.IsZero() {
		q = q.Where("created_at < ?", to)
	}
	return q
}

// ExportTasks: GET /api/tasks/export?project_id=&status=&assignee_id=&
// priority=&label=&q=&from=&to= — team export. project_id optional: given → one
// visible project (404 no-leak otherwise, same as the list); omitted → all
// visible projects (MyTasks scope).
func ExportTasks(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		f, msg := parseTaskListFilters(c)
		if msg != "" {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", msg)
		}
		from, to, msg := parseDateRange(c)
		if msg != "" {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", msg)
		}
		q := gdb.Unscoped().Model(&models.Task{})
		name := "kica-tasks-" + time.Now().UTC().Format("20060102") + ".csv"
		if pid := c.Query("project_id"); pid != "" {
			if _, err := uuid.Parse(pid); err != nil {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "project_id must be a uuid")
			}
			p, _, ok := loadVisibleProject(c, gdb, u, pid)
			if !ok {
				return nil // 404 no-leak
			}
			q = q.Where("project_id = ?", p.ID)
			name = "kica-" + strings.ToLower(p.Key) + "-tasks-" + time.Now().UTC().Format("20060102") + ".csv"
		} else if u.GlobalRole != roleAdmin {
			visible := visibleProjectIDs(gdb, u.ID)
			q = q.Where("project_id IN (?)", visible)
		}
		var tasks []models.Task
		if err := applyDateRange(applyTaskListFilters(q, gdb, f), from, to).Order("created_at, number").Find(&tasks).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not export tasks")
		}
		buf, err := tasksCSV(gdb, tasks)
		if err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not export tasks")
		}
		return sendCSV(c, name, buf)
	}
}

// tasksCSV renders the team export columns: id, number, title, status,
// priority, type, assignee, labels, created_by, created_at, updated_at,
// started_at, done_at, estimate, due_date, description (markdown as-is).
// assignee/created_by are display names; labels are comma-joined.
func tasksCSV(gdb *gorm.DB, tasks []models.Task) ([]byte, error) {
	names := map[string]string{}
	labels := map[string][]string{}
	if len(tasks) > 0 {
		ids := make([]string, 0, len(tasks)*2)
		taskIDs := make([]string, len(tasks))
		for i := range tasks {
			ids = append(ids, tasks[i].CreatedBy)
			if tasks[i].AssigneeID != nil {
				ids = append(ids, *tasks[i].AssigneeID)
			}
			taskIDs[i] = tasks[i].ID
		}
		var users []models.User
		if err := gdb.Select("id, name").Where("id IN ?", ids).Find(&users).Error; err == nil {
			for i := range users {
				names[users[i].ID] = users[i].Name
			}
		}
		var rows []struct{ TaskID, Name string }
		if err := gdb.Table("task_labels").
			Select("task_labels.task_id, labels.name").
			Joins("JOIN labels ON labels.id = task_labels.label_id").
			Where("task_labels.task_id IN ?", taskIDs).
			Order("labels.name").Scan(&rows).Error; err == nil {
			for _, r := range rows {
				labels[r.TaskID] = append(labels[r.TaskID], r.Name)
			}
		}
	}
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM — Excel opens UTF-8 CSV correctly
	w := csv.NewWriter(&buf)
	if err := w.Write([]string{
		"id", "number", "title", "status", "priority", "type", "assignee",
		"labels", "created_by", "created_at", "updated_at", "started_at",
		"done_at", "estimate", "due_date", "description",
	}); err != nil {
		return nil, err
	}
	for i := range tasks {
		t := &tasks[i]
		assignee := ""
		if t.AssigneeID != nil {
			assignee = names[*t.AssigneeID]
		}
		if err := w.Write([]string{
			t.ID, strconv.FormatInt(t.Number, 10), t.Title, t.Status, t.Priority, t.Type,
			assignee, strings.Join(labels[t.ID], ","), names[t.CreatedBy],
			csvTime(&t.CreatedAt), csvTime(&t.UpdatedAt), csvTime(t.StartedAt), csvTime(t.DoneAt),
			csvInt(t.Estimate), csvDate(t.DueDate), t.Description,
		}); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

// ClientExportTickets: GET /api/client/tickets/export?q=&project_id=&status=
// — client mirror: own tickets in linked projects, same filters as the list
// (#47), client-safe columns only (team-only fields never serialize).
func ClientExportTickets(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		ids, err := linkedProjectIDs(gdb, u.ID)
		if err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not list projects")
		}
		f, msg := parseClientTicketFilters(c)
		if msg != "" {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", msg)
		}
		from, to, msg := parseDateRange(c)
		if msg != "" {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", msg)
		}
		var tasks []models.Task
		if err := applyDateRange(applyClientTicketFilters(
			gdb.Model(&models.Task{}).Where("created_by = ? AND project_id IN ?", u.ID, ids), f,
		), from, to).Order("updated_at DESC").Find(&tasks).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not export tickets")
		}
		// project keys for the rows (same batching as ClientListTickets)
		keys := map[string]string{}
		if len(tasks) > 0 {
			pids := map[string]bool{}
			for i := range tasks {
				pids[tasks[i].ProjectID] = true
			}
			list := make([]string, 0, len(pids))
			for id := range pids {
				list = append(list, id)
			}
			var projs []models.Project
			if err := gdb.Where("id IN ?", list).Find(&projs).Error; err == nil {
				for i := range projs {
					keys[projs[i].ID] = projs[i].Key
				}
			}
		}
		var buf bytes.Buffer
		buf.WriteString("\xEF\xBB\xBF")
		w := csv.NewWriter(&buf)
		if err := w.Write([]string{
			"id", "project", "number", "title", "status",
			"created_at", "updated_at", "description",
		}); err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not export tickets")
		}
		for i := range tasks {
			t := &tasks[i]
			if err := w.Write([]string{
				t.ID, keys[t.ProjectID], strconv.FormatInt(t.Number, 10), t.Title, t.Status,
				csvTime(&t.CreatedAt), csvTime(&t.UpdatedAt), t.Description,
			}); err != nil {
				return httpErr(c, fiber.StatusInternalServerError, "internal", "could not export tickets")
			}
		}
		w.Flush()
		if w.Error() != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not export tickets")
		}
		name := "kica-my-tickets-" + time.Now().UTC().Format("20060102") + ".csv"
		return sendCSV(c, name, buf.Bytes())
	}
}
