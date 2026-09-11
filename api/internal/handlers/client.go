// Client portal handlers — mounted behind RequireClient. A client submits
// tickets into the Inbox of projects they're linked to via client_projects;
// they see only their own tickets, and the wire shape deliberately omits
// team-only fields (assessment, assignee, labels, position…).
package handlers

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/VincentApta/Kica/api/internal/models"
)

type clientTicketReq struct {
	ProjectID   string `json:"project_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// clientTicketJSON is the wire `client ticket` shape — no assessment, no
// assignment, no positioning: read-only tracking for the submitter.
type clientTicketJSON struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	ProjectKey  string    `json:"project_key"`
	Number      int64     `json:"number"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// linkedProjectIDs returns the client's linked project ids.
func linkedProjectIDs(gdb *gorm.DB, clientID string) ([]string, error) {
	var ids []string
	err := gdb.Model(&models.ClientProject{}).
		Where("client_id = ?", clientID).
		Pluck("project_id", &ids).Error
	return ids, err
}

// isLinkedClientProject reports a client_projects row for (client, project).
func isLinkedClientProject(gdb *gorm.DB, clientID, projectID string) (bool, error) {
	var n int64
	err := gdb.Model(&models.ClientProject{}).
		Where("client_id = ? AND project_id = ?", clientID, projectID).
		Count(&n).Error
	return n > 0, err
}

// ClientListProjects: GET /api/client/projects — the submit form's project
// picker source: linked projects only, {id, key, name}.
func ClientListProjects(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		var rows []struct {
			ID   string
			Key  string
			Name string
		}
		err := gdb.Table("client_projects").
			Select("projects.id, projects.key, projects.name").
			Joins("JOIN projects ON projects.id = client_projects.project_id").
			Where("client_projects.client_id = ?", u.ID).
			Order("projects.key").
			Scan(&rows).Error
		if err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not list projects")
		}
		data := make([]fiber.Map, len(rows))
		for i, r := range rows {
			data[i] = fiber.Map{"id": r.ID, "key": r.Key, "name": r.Name}
		}
		return c.JSON(fiber.Map{"data": data})
	}
}

// ClientCreateTicket: POST /api/client/tickets {project_id, title,
// description} — creates an inbox task (status=inbox, created_by=client,
// per-project numbering, rule 8 creation event). Unlinked project → 404
// no-leak, same convention as loadVisibleProject.
func ClientCreateTicket(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		var req clientTicketReq
		if err := c.BodyParser(&req); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		}
		req.Title = strings.TrimSpace(req.Title)
		if req.Title == "" {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "title is required")
		}
		if _, err := uuid.Parse(req.ProjectID); err != nil {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "project_id must be a uuid")
		}
		linked, err := isLinkedClientProject(gdb, u.ID, req.ProjectID)
		if err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not verify project link")
		}
		var p models.Project
		if !linked || gdb.First(&p, "id = ?", req.ProjectID).Error != nil {
			return httpErr(c, fiber.StatusNotFound, "not_found", "no such project")
		}
		task := &models.Task{
			ProjectID: p.ID, Title: req.Title, Description: req.Description,
			Status: "inbox", CreatedBy: u.ID,
		}
		err = gdb.Transaction(func(tx *gorm.DB) error {
			// atomic per-project number (rule 2) + inbox column append
			// (rule 3), then the creation event (rule 8).
			if err := tx.Raw("UPDATE projects SET task_seq = task_seq + 1 WHERE id = ? RETURNING task_seq", p.ID).Scan(&task.Number).Error; err != nil {
				return err
			}
			if err := tx.Raw("SELECT COALESCE(MAX(position), 0) FROM tasks WHERE project_id = ? AND status = 'inbox' AND deleted_at IS NULL", p.ID).Scan(&task.Position).Error; err != nil {
				return err
			}
			task.Position += positionGap
			if err := tx.Create(task).Error; err != nil {
				return err
			}
			return tx.Create(&models.TaskEvent{
				TaskID: task.ID, ActorID: u.ID, FromStatus: nil, ToStatus: "inbox",
			}).Error
		})
		if err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not create ticket")
		}
		return c.Status(fiber.StatusCreated).JSON(clientTicketJSON{
			ID: task.ID, ProjectID: p.ID, ProjectKey: p.Key, Number: task.Number,
			Title: task.Title, Description: task.Description, Status: task.Status,
			CreatedAt: task.CreatedAt, UpdatedAt: task.UpdatedAt,
		})
	}
}

// clientCommentAuthorJSON: name only — the client surface never sees emails,
// roles or any other user field (#43).
type clientCommentAuthorJSON struct {
	Name string `json:"name"`
}

// clientCommentJSON is the wire `client comment` shape — {id, body,
// created_at, user:{name}}; team comments on the ticket are visible to the
// client with the author name only.
type clientCommentJSON struct {
	ID        string                 `json:"id"`
	Body      string                 `json:"body"`
	CreatedAt time.Time              `json:"created_at"`
	User      clientCommentAuthorJSON `json:"user"`
}

// ClientListTicketComments: GET /api/client/tickets/:id/comments — own tickets
// only (loadOwnTicket); every comment on the ticket (team + client), oldest
// first.
func ClientListTicketComments(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		t, ok := loadOwnTicket(c, gdb, u, c.Params("id"))
		if !ok {
			return nil
		}
		var comments []models.Comment
		if err := gdb.Where("task_id = ?", t.ID).Order("created_at").Find(&comments).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not list comments")
		}
		names := map[string]string{}
		if len(comments) > 0 {
			ids := make([]string, 0, len(comments))
			for i := range comments {
				ids = append(ids, comments[i].UserID)
			}
			var rows []struct{ ID, Name string }
			if err := gdb.Model(&models.User{}).Select("id, name").Where("id IN ?", ids).Scan(&rows).Error; err == nil {
				for _, r := range rows {
					names[r.ID] = r.Name
				}
			}
		}
		out := make([]clientCommentJSON, len(comments))
		for i := range comments {
			cm := &comments[i]
			out[i] = clientCommentJSON{
				ID: cm.ID, Body: cm.Body, CreatedAt: cm.CreatedAt,
				User: clientCommentAuthorJSON{Name: names[cm.UserID]},
			}
		}
		return c.JSON(fiber.Map{"data": out})
	}
}

// ClientCreateTicketComment: POST /api/client/tickets/:id/comments {body} —
// same request shape as the team endpoint; author = the client user id (same
// comments table, so the team drawer shows it with the client's name).
func ClientCreateTicketComment(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		t, ok := loadOwnTicket(c, gdb, u, c.Params("id"))
		if !ok {
			return nil
		}
		var req createCommentReq
		if err := c.BodyParser(&req); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		}
		if strings.TrimSpace(req.Body) == "" {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "body is required")
		}
		cm := &models.Comment{TaskID: t.ID, UserID: u.ID, Body: req.Body}
		if err := gdb.Create(cm).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not create comment")
		}
		return c.Status(fiber.StatusCreated).JSON(clientCommentJSON{
			ID: cm.ID, Body: cm.Body, CreatedAt: cm.CreatedAt,
			User: clientCommentAuthorJSON{Name: u.Name},
		})
	}
}

// clientTicketFilters: the /client/tickets query params shared by list +
// export (#47/#50).
type clientTicketFilters struct {
	Q         string
	ProjectID string
	Status    string // open (not done) | closed (done)
}

// parseClientTicketFilters validates the filters. Returns the rejection
// message, "" when valid. The linked-project scope already hides unlinked
// project_ids — an empty result, no leak.
func parseClientTicketFilters(c *fiber.Ctx) (clientTicketFilters, string) {
	f := clientTicketFilters{
		Q:         strings.TrimSpace(c.Query("q")),
		ProjectID: c.Query("project_id"),
		Status:    c.Query("status"),
	}
	if f.ProjectID != "" {
		if _, err := uuid.Parse(f.ProjectID); err != nil {
			return f, "project_id must be a uuid"
		}
	}
	switch f.Status {
	case "", "open", "closed":
	default:
		return f, "status must be open or closed"
	}
	return f, ""
}

// applyClientTicketFilters: the client-ticket predicates — same search as the
// team list (LOWER…LIKE works on PG and sqlite), open/closed split, project.
func applyClientTicketFilters(q *gorm.DB, f clientTicketFilters) *gorm.DB {
	if f.ProjectID != "" {
		q = q.Where("project_id = ?", f.ProjectID)
	}
	switch f.Status {
	case "open":
		q = q.Where("status <> ?", "done")
	case "closed":
		q = q.Where("status = ?", "done")
	}
	if f.Q != "" {
		like := "%" + strings.ToLower(f.Q) + "%"
		q = q.Where("(LOWER(title) LIKE ? OR LOWER(description) LIKE ?)", like, like)
	}
	return q
}

// ClientListTickets: GET /api/client/tickets?q=&project_id=&status= — own
// tickets across all linked projects, newest-updated first (#47 search +
// filters, same pattern as the team list). Soft-deleted (trashed) tickets
// disappear — clients track live work only.
func ClientListTickets(gdb *gorm.DB) fiber.Handler {
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
		page := queryInt(c, "page", 1)
		perPage := queryInt(c, "per_page", defaultPerPage)
		scope := func() *gorm.DB {
			return applyClientTicketFilters(
				gdb.Model(&models.Task{}).Where("created_by = ? AND project_id IN ?", u.ID, ids),
				f,
			)
		}
		var total int64
		if err := scope().Count(&total).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not count tickets")
		}
		var tasks []models.Task
		if err := scope().Order("updated_at DESC").Limit(perPage).Offset((page - 1) * perPage).Find(&tasks).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not list tickets")
		}
		out := make([]clientTicketJSON, len(tasks))
		if len(tasks) > 0 {
			projIDs := map[string]bool{}
			for i := range tasks {
				projIDs[tasks[i].ProjectID] = true
			}
			keys := make([]string, 0, len(projIDs))
			for id := range projIDs {
				keys = append(keys, id)
			}
			byID := map[string]string{}
			var projs []models.Project
			if err := gdb.Where("id IN ?", keys).Find(&projs).Error; err == nil {
				for i := range projs {
					byID[projs[i].ID] = projs[i].Key
				}
			}
			for i := range tasks {
				t := &tasks[i]
				out[i] = clientTicketJSON{
					ID: t.ID, ProjectID: t.ProjectID, ProjectKey: byID[t.ProjectID],
					Number: t.Number, Title: t.Title, Description: t.Description,
					Status: t.Status, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
				}
			}
		}
		return c.JSON(fiber.Map{"data": out, "page": page, "per_page": perPage, "total": total})
	}
}
