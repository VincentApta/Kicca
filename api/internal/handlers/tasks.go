// Task handlers: CRUD, per-project numbering (rule 2), filters, trash/
// restore (rule 4), move position math (rule 3), comments. Visibility rides
// loadVisibleProject via the task's project; denial is 404 no-leak.
package handlers

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/VincentApta/Kicca/api/internal/models"
)

// positionGap is the spacing on create/append/rebalance (rule 3). Rebalance
// fires when the smallest adjacent gap in the column would drop under 1.
const positionGap = 1024

var taskStatuses = map[string]bool{
	"inbox": true, "backlog": true, "in_progress": true, "review": true,
	"done": true, "blocked": true, "trash": true,
}

var taskPriorities = map[string]bool{"urgent": true, "high": true, "medium": true, "low": true}

var taskTypes = map[string]bool{"task": true, "bug": true, "feature": true, "chore": true}

// startedStatuses: entering one of these stamps started_at (first touch wins).
var startedStatuses = map[string]bool{"in_progress": true, "review": true}

// errAnchorNotInColumn: move anchor is not a non-deleted task of the target
// column in the same project → 422.
var errAnchorNotInColumn = errors.New("anchor task not in target column")

type createTaskReq struct {
	Title       string   `json:"title"`
	Description *string  `json:"description"`
	Status      *string  `json:"status"`
	Priority    *string  `json:"priority"`
	Type        *string  `json:"type"`
	Estimate    *int     `json:"estimate"`
	AssigneeID  *string  `json:"assignee_id"`
	DueDate     *string  `json:"due_date"`
	LabelIDs    []string `json:"label_ids"`
}

type patchTaskReq struct {
	Title       *string         `json:"title"`
	Description *string         `json:"description"`
	Assessment  *string         `json:"assessment"`
	Status      *string         `json:"status"`
	Priority    *string         `json:"priority"`
	Type        *string         `json:"type"`
	Estimate    json.RawMessage `json:"estimate"`    // RawMessage: absent vs null (clear)
	AssigneeID  json.RawMessage `json:"assignee_id"` // RawMessage: absent vs null (clear)
	DueDate     json.RawMessage `json:"due_date"`    // ditto
	Position    *float64        `json:"position"`
	LabelIDs    []string        `json:"label_ids"` // non-nil slice (incl. empty) replaces
}

type moveTaskReq struct {
	Status       string  `json:"status"`
	BeforeTaskID *string `json:"before_task_id"`
	AfterTaskID  *string `json:"after_task_id"`
}

type createCommentReq struct {
	Body string `json:"body"`
}

// commentJSON is the wire `comment` shape.
type commentJSON struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	UserID    string    `json:"user_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

func toCommentJSON(cm *models.Comment) commentJSON {
	return commentJSON{ID: cm.ID, TaskID: cm.TaskID, UserID: cm.UserID, Body: cm.Body, CreatedAt: cm.CreatedAt}
}

func toCommentJSONs(cms []models.Comment) []commentJSON {
	out := make([]commentJSON, len(cms))
	for i := range cms {
		out[i] = toCommentJSON(&cms[i])
	}
	return out
}

// taskJSON is the wire `task` shape. GhLink is the GitHub issue link when
// one exists, else null (populated in tasksJSON).
type taskJSON struct {
	ID          string      `json:"id"`
	ProjectID   string      `json:"project_id"`
	ProjectKey  string      `json:"project_key,omitempty"`
	ProjectName string      `json:"project_name,omitempty"`
	Number      int64       `json:"number"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Status      string      `json:"status"`
	Priority    string      `json:"priority"`
	Assignee    *userJSON   `json:"assignee"`
	Labels      []labelJSON `json:"labels"`
	DueDate     *string     `json:"due_date"`
	Position    float64     `json:"position"`
	StartedAt   *time.Time  `json:"started_at"`
	DoneAt      *time.Time  `json:"done_at"`
	Estimate    *int        `json:"estimate"`
	Type        string      `json:"type"`
	Assessment  string      `json:"assessment"`
	CreatedBy   string      `json:"created_by"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	GhLink      interface{} `json:"gh_link"`
}

// tasksJSON assembles wire tasks, batching assignee + labels + gh_link
// lookups so any list costs three extra queries regardless of page size.
func tasksJSON(gdb *gorm.DB, tasks []models.Task) []taskJSON {
	assignees := map[string]userJSON{}
	labels := map[string][]labelJSON{}
	ghLinks := map[string]ghLinkJSON{}
	if len(tasks) > 0 {
		assigneeIDs := make([]string, 0, len(tasks))
		taskIDs := make([]string, len(tasks))
		for i := range tasks {
			if tasks[i].AssigneeID != nil {
				assigneeIDs = append(assigneeIDs, *tasks[i].AssigneeID)
			}
			taskIDs[i] = tasks[i].ID
		}
		if len(assigneeIDs) > 0 {
			var users []models.User
			if err := gdb.Where("id IN ?", assigneeIDs).Find(&users).Error; err == nil {
				for i := range users {
					assignees[users[i].ID] = toUserJSON(&users[i])
				}
			}
		}
		var rows []struct {
			TaskID string
			ID     string
			Name   string
			Color  string
		}
		if err := gdb.Table("task_labels").
			Select("task_labels.task_id, labels.id, labels.name, labels.color").
			Joins("JOIN labels ON labels.id = task_labels.label_id").
			Where("task_labels.task_id IN ?", taskIDs).
			Order("labels.name").Scan(&rows).Error; err == nil {
			for _, r := range rows {
				labels[r.TaskID] = append(labels[r.TaskID], labelJSON{ID: r.ID, Name: r.Name, Color: r.Color})
			}
		}
		var links []models.GitHubIssueLink
		if err := gdb.Where("task_id IN ?", taskIDs).Find(&links).Error; err == nil {
			for i := range links {
				ghLinks[links[i].TaskID] = ghLinkJSON{
					Repo: links[i].Repo, IssueNumber: links[i].IssueNumber, IssueURL: links[i].IssueURL,
				}
			}
		}
	}
	out := make([]taskJSON, len(tasks))
	for i := range tasks {
		t := &tasks[i]
		tj := taskJSON{
			ID: t.ID, ProjectID: t.ProjectID, Number: t.Number, Title: t.Title,
			Description: t.Description, Status: t.Status, Priority: t.Priority,
			Position: t.Position, CreatedBy: t.CreatedBy,
			StartedAt: t.StartedAt, DoneAt: t.DoneAt, Estimate: t.Estimate,
			Type: t.Type, Assessment: t.Assessment,
			CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
			Labels: []labelJSON{},
		}
		if t.AssigneeID != nil {
			if a, ok := assignees[*t.AssigneeID]; ok {
				tj.Assignee = &a
			}
		}
		if t.DueDate != nil {
			s := t.DueDate.UTC().Format("2006-01-02")
			tj.DueDate = &s
		}
		if l := labels[t.ID]; l != nil {
			tj.Labels = l
		}
		if gl, ok := ghLinks[t.ID]; ok {
			tj.GhLink = gl
		}
		out[i] = tj
	}
	return out
}

// loadVisibleTask fetches the task unscoped (trashed tasks stay reachable
// for GET/restore/purge) then applies project visibility (rule 1).
func loadVisibleTask(c *fiber.Ctx, gdb *gorm.DB, u *models.User, id string) (*models.Task, bool) {
	if _, err := uuid.Parse(id); err != nil {
		httpErr(c, fiber.StatusBadRequest, "invalid_uuid", "task id must be a uuid")
		return nil, false
	}
	var t models.Task
	if err := gdb.Unscoped().First(&t, "id = ?", id).Error; err != nil {
		httpErr(c, fiber.StatusNotFound, "not_found", "no such task")
		return nil, false
	}
	if _, _, ok := loadVisibleProject(c, gdb, u, t.ProjectID); !ok {
		return nil, false
	}
	return &t, true
}

// parseDate validates the contract date format YYYY-MM-DD.
func parseDate(s string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", s)
	return t, err == nil
}

// applyTransition stamps the analytics columns for a status change (rule 8):
// started_at on first entry into in_progress/review (never overwritten),
// done_at on entry into done, cleared on leaving done (re-done re-stamps).
func applyTransition(updates map[string]interface{}, from *models.Task, to string) {
	now := time.Now().UTC()
	if startedStatuses[to] && from.StartedAt == nil {
		updates["started_at"] = now
	}
	if to == "done" {
		if from.DoneAt == nil {
			updates["done_at"] = now
		}
	} else if from.Status == "done" {
		updates["done_at"] = nil
	}
}

// validateLabelIDs: uuids, no duplicates, all referencing labels of this
// project. Returns the rejection message, "" when valid.
func validateLabelIDs(gdb *gorm.DB, projectID string, ids []string) string {
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if _, err := uuid.Parse(id); err != nil {
			return "label ids must be uuids"
		}
		if seen[id] {
			return "label ids must not contain duplicates"
		}
		seen[id] = true
	}
	if len(ids) == 0 {
		return ""
	}
	var count int64
	if err := gdb.Model(&models.Label{}).Where("id IN ? AND project_id = ?", ids, projectID).Count(&count).Error; err != nil {
		return "could not validate label ids"
	}
	if count != int64(len(ids)) {
		return "label ids must reference labels in this project"
	}
	return ""
}

// ListTasks: GET /api/projects/:id/tasks — filters status/assignee_id/
// priority/label/q + pagination. Trash excluded unless status=trash (rule 4).
func ListTasks(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		p, _, ok := loadVisibleProject(c, gdb, u, c.Params("id"))
		if !ok {
			return nil
		}
		page := queryInt(c, "page", 1)
		perPage := queryInt(c, "per_page", defaultPerPage)
		// fresh query per statement — reusing one after Count() drags the
		// count(*) select into the Find (same pattern as ListTeams).
		if status := c.Query("status"); status != "" && !taskStatuses[status] {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "status must be a valid task status")
		}
		if a := c.Query("assignee_id"); a != "" {
			if _, err := uuid.Parse(a); err != nil {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "assignee_id must be a uuid")
			}
		}
		if pr := c.Query("priority"); pr != "" && !taskPriorities[pr] {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "priority must be a valid task priority")
		}
		if labelID := c.Query("label"); labelID != "" {
			if _, err := uuid.Parse(labelID); err != nil {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "label must be a label uuid")
			}
		}
		scope := func() *gorm.DB {
			q := gdb.Unscoped().Model(&models.Task{}).Where("project_id = ?", p.ID)
			if status := c.Query("status"); status != "" {
				q = q.Where("status = ?", status)
				if status != "trash" {
					q = q.Where("deleted_at IS NULL")
				}
			} else {
				q = q.Where("deleted_at IS NULL")
			}
			if a := c.Query("assignee_id"); a != "" {
				q = q.Where("assignee_id = ?", a)
			}
			if pr := c.Query("priority"); pr != "" {
				q = q.Where("priority = ?", pr)
			}
			if labelID := c.Query("label"); labelID != "" {
				q = q.Where("id IN (?)", gdb.Model(&models.TaskLabel{}).Select("task_id").Where("label_id = ?", labelID))
			}
			if search := strings.TrimSpace(c.Query("q")); search != "" {
				// LOWER…LIKE behaves identically on PG and sqlite (tests);
				// PG ILIKE is the same for these two columns.
				like := "%" + strings.ToLower(search) + "%"
				q = q.Where("(LOWER(title) LIKE ? OR LOWER(description) LIKE ?)", like, like)
			}
			return q
		}
		var total int64
		if err := scope().Count(&total).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not count tasks")
		}
		var tasks []models.Task
		if err := scope().Order("number").Limit(perPage).Offset((page - 1) * perPage).Find(&tasks).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not list tasks")
		}
		return c.JSON(fiber.Map{"data": tasksJSON(gdb, tasks), "page": page, "per_page": perPage, "total": total})
	}
}

// MyTasks: GET /api/tasks[?assignee_id=...][?assignee_ids=a,b,c] — tasks
// across ALL visible projects (optionally one assignee's, or comma-separated
// list for team filtering), joined with project key + name.
// Powers the global Dashboard (all members, filterable) and My Tasks page.
// Visibility: only tasks whose project is visible to the caller.
func MyTasks(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		assignee := c.Query("assignee_id")
		if assignee != "" {
			if _, err := uuid.Parse(assignee); err != nil {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "assignee_id must be a uuid")
			}
		}
		assignees := strings.Split(c.Query("assignee_ids"), ",")
		for _, a := range assignees {
			if a == "" {
				continue
			}
			if _, err := uuid.Parse(a); err != nil {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "assignee_ids must be uuids")
			}
		}
		assignees = slices.DeleteFunc(assignees, func(s string) bool { return s == "" })
		page := queryInt(c, "page", 1)
		perPage := queryInt(c, "per_page", defaultPerPage)

		// visible project ids for this user
		visible := gdb.Model(&models.ProjectMember{}).Select("project_id").Where("user_id = ?", u.ID)
		scope := func() *gorm.DB {
			q := gdb.Unscoped().Model(&models.Task{}).
				Where("deleted_at IS NULL")
			if assignee != "" {
				q = q.Where("assignee_id = ?", assignee)
			} else if len(assignees) > 0 {
				q = q.Where("assignee_id IN ?", assignees)
			}
			if u.GlobalRole != roleAdmin {
				q = q.Where("project_id IN (?)", visible)
			}
			return q
		}
		var total int64
		if err := scope().Count(&total).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not count tasks")
		}
		var tasks []models.Task
		if err := scope().Order("created_at DESC").Limit(perPage).Offset((page - 1) * perPage).Find(&tasks).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not list tasks")
		}
		out := tasksJSON(gdb, tasks)

		// attach project key + name (batch by distinct project ids)
		if len(tasks) > 0 {
			projIDs := map[string]bool{}
			for i := range tasks {
				projIDs[tasks[i].ProjectID] = true
			}
			ids := make([]string, 0, len(projIDs))
			for id := range projIDs {
				ids = append(ids, id)
			}
			var projs []models.Project
			if err := gdb.Where("id IN ?", ids).Find(&projs).Error; err == nil {
				byID := map[string]models.Project{}
				for i := range projs {
					byID[projs[i].ID] = projs[i]
				}
				for i := range out {
					if p, ok := byID[out[i].ProjectID]; ok {
						out[i].ProjectKey = p.Key
						out[i].ProjectName = p.Name
					}
				}
			}
		}
		return c.JSON(fiber.Map{"data": out, "page": page, "per_page": perPage, "total": total})
	}
}

// CreateTask: POST /api/projects/:id/tasks — member+. Number comes from the
// atomic seq bump (rule 2), position from max+1024 in the target column
// (rule 3). Defaults: status backlog, priority medium.
func CreateTask(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		p, _, ok := loadVisibleProject(c, gdb, u, c.Params("id"))
		if !ok {
			return nil
		}
		var req createTaskReq
		if err := c.BodyParser(&req); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		}
		req.Title = strings.TrimSpace(req.Title)
		if req.Title == "" {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "title is required")
		}
		status := "backlog"
		if req.Status != nil && *req.Status != "" {
			if !taskStatuses[*req.Status] {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "status must be a valid task status")
			}
			status = *req.Status
		}
		priority := "medium"
		if req.Priority != nil && *req.Priority != "" {
			if !taskPriorities[*req.Priority] {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "priority must be a valid task priority")
			}
			priority = *req.Priority
		}
		taskType := "task"
		if req.Type != nil && *req.Type != "" {
			if !taskTypes[*req.Type] {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "type must be one of task|bug|feature|chore")
			}
			taskType = *req.Type
		}
		if req.Estimate != nil && *req.Estimate < 0 {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "estimate must be an integer >= 0")
		}
		var assignee *string
		if req.AssigneeID != nil && *req.AssigneeID != "" {
			if msg := validateUserIDs(gdb, []string{*req.AssigneeID}); msg != "" {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", msg)
			}
			assignee = req.AssigneeID
		}
		var due *time.Time
		if req.DueDate != nil && *req.DueDate != "" {
			t, ok := parseDate(*req.DueDate)
			if !ok {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "due_date must be YYYY-MM-DD")
			}
			due = &t
		}
		if msg := validateLabelIDs(gdb, p.ID, req.LabelIDs); msg != "" {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", msg)
		}
		task := &models.Task{
			ProjectID: p.ID, Title: req.Title, Status: status, Priority: priority,
			Type: taskType, Estimate: req.Estimate,
			AssigneeID: assignee, CreatedBy: u.ID, DueDate: due,
		}
		if req.Description != nil {
			task.Description = *req.Description
		}
		err := gdb.Transaction(func(tx *gorm.DB) error {
			// atomic per-project number (rule 2): bump-and-read in one
			// statement, row-locked by the UPDATE itself.
			if err := tx.Raw("UPDATE projects SET task_seq = task_seq + 1 WHERE id = ? RETURNING task_seq", p.ID).Scan(&task.Number).Error; err != nil {
				return err
			}
			if err := tx.Raw("SELECT COALESCE(MAX(position), 0) FROM tasks WHERE project_id = ? AND status = ? AND deleted_at IS NULL", p.ID, status).Scan(&task.Position).Error; err != nil {
				return err
			}
			task.Position += positionGap
			if err := tx.Create(task).Error; err != nil {
				return err
			}
			// creation is the first status event (from NULL) — rule 8.
			if err := tx.Create(&models.TaskEvent{
				TaskID: task.ID, ActorID: u.ID, FromStatus: nil, ToStatus: status,
			}).Error; err != nil {
				return err
			}
			if len(req.LabelIDs) > 0 {
				rows := make([]models.TaskLabel, len(req.LabelIDs))
				for i, id := range req.LabelIDs {
					rows[i] = models.TaskLabel{TaskID: task.ID, LabelID: id}
				}
				return tx.Create(&rows).Error
			}
			return nil
		})
		if err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not create task")
		}
		return c.Status(fiber.StatusCreated).JSON(tasksJSON(gdb, []models.Task{*task})[0])
	}
}

// GetTask: GET /api/tasks/:id — works on trashed tasks too (needed for the
// trash view + restore flow).
func GetTask(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		t, ok := loadVisibleTask(c, gdb, u, c.Params("id"))
		if !ok {
			return nil
		}
		return c.JSON(tasksJSON(gdb, []models.Task{*t})[0])
	}
}

// PatchTask: PATCH /api/tasks/:id — member+, any task field incl. status and
// position. label_ids (non-nil slice) replaces the label set. Precise
// repositioning is /move; this just writes what it's given.
func PatchTask(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		t, ok := loadVisibleTask(c, gdb, u, c.Params("id"))
		if !ok {
			return nil
		}
		var req patchTaskReq
		if err := c.BodyParser(&req); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		}
		updates := map[string]interface{}{}
		if req.Title != nil {
			if strings.TrimSpace(*req.Title) == "" {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "title must not be empty")
			}
			updates["title"] = strings.TrimSpace(*req.Title)
		}
		if req.Description != nil {
			updates["description"] = *req.Description
		}
		if req.Assessment != nil {
			updates["assessment"] = *req.Assessment
		}
		if req.Status != nil {
			if !taskStatuses[*req.Status] {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "status must be a valid task status")
			}
			if *req.Status != t.Status {
				applyTransition(updates, t, *req.Status)
			}
			updates["status"] = *req.Status
		}
		if req.Priority != nil {
			if !taskPriorities[*req.Priority] {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "priority must be a valid task priority")
			}
			updates["priority"] = *req.Priority
		}
		if req.Type != nil && *req.Type != "" {
			if !taskTypes[*req.Type] {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "type must be one of task|bug|feature|chore")
			}
			updates["type"] = *req.Type
		}
		if len(req.Estimate) > 0 {
			if string(req.Estimate) == "null" {
				updates["estimate"] = nil
			} else {
				var e int
				if err := json.Unmarshal(req.Estimate, &e); err != nil {
					return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "estimate must be an integer >= 0")
				}
				if e < 0 {
					return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "estimate must be an integer >= 0")
				}
				updates["estimate"] = e
			}
		}
		if len(req.AssigneeID) > 0 {
			if string(req.AssigneeID) == "null" {
				updates["assignee_id"] = nil
			} else {
				var a string
				if err := json.Unmarshal(req.AssigneeID, &a); err != nil {
					return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "assignee_id must be a uuid")
				}
				if msg := validateUserIDs(gdb, []string{a}); msg != "" {
					return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", msg)
				}
				updates["assignee_id"] = a
			}
		}
		if len(req.DueDate) > 0 {
			if string(req.DueDate) == "null" {
				updates["due_date"] = nil
			} else {
				var s string
				if err := json.Unmarshal(req.DueDate, &s); err != nil {
					return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "due_date must be YYYY-MM-DD")
				}
				d, ok := parseDate(s)
				if !ok {
					return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "due_date must be YYYY-MM-DD")
				}
				updates["due_date"] = d
			}
		}
		if req.Position != nil {
			updates["position"] = *req.Position
		}
		if req.LabelIDs != nil {
			if msg := validateLabelIDs(gdb, t.ProjectID, req.LabelIDs); msg != "" {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", msg)
			}
		}
		newStatus := t.Status
		if req.Status != nil {
			newStatus = *req.Status
		}
		err := gdb.Transaction(func(tx *gorm.DB) error {
			if len(updates) > 0 {
				// Unscoped: patching a trashed task must not gain a
				// deleted_at IS NULL predicate.
				if err := tx.Unscoped().Model(&models.Task{}).Where("id = ?", t.ID).Updates(updates).Error; err != nil {
					return err
				}
			}
			if newStatus != t.Status {
				from := t.Status
				if err := tx.Create(&models.TaskEvent{
					TaskID: t.ID, ActorID: u.ID, FromStatus: &from, ToStatus: newStatus,
				}).Error; err != nil {
					return err
				}
			}
			if req.LabelIDs != nil {
				if err := tx.Where("task_id = ?", t.ID).Delete(&models.TaskLabel{}).Error; err != nil {
					return err
				}
				if len(req.LabelIDs) > 0 {
					rows := make([]models.TaskLabel, len(req.LabelIDs))
					for i, id := range req.LabelIDs {
						rows[i] = models.TaskLabel{TaskID: t.ID, LabelID: id}
					}
					if err := tx.Create(&rows).Error; err != nil {
						return err
					}
				}
			}
			return nil
		})
		if err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not update task")
		}
		var fresh models.Task
		if err := gdb.Unscoped().First(&fresh, "id = ?", t.ID).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not reload task")
		}
		return c.JSON(tasksJSON(gdb, []models.Task{fresh})[0])
	}
}

// DeleteTask: DELETE /api/tasks/:id — soft (status=trash + deleted_at,
// rule 4) for any member; ?purge=1 hard-deletes, global admin only.
func DeleteTask(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		t, ok := loadVisibleTask(c, gdb, u, c.Params("id"))
		if !ok {
			return nil
		}
		if c.Query("purge") == "1" {
			if u.GlobalRole != roleAdmin {
				return httpErr(c, fiber.StatusForbidden, "forbidden", "global admin role required")
			}
			err := gdb.Transaction(func(tx *gorm.DB) error {
				if err := tx.Where("task_id = ?", t.ID).Delete(&models.TaskLabel{}).Error; err != nil {
					return err
				}
				if err := tx.Where("task_id = ?", t.ID).Delete(&models.Comment{}).Error; err != nil {
					return err
				}
				return tx.Unscoped().Delete(t).Error
			})
			if err != nil {
				return httpErr(c, fiber.StatusInternalServerError, "internal", "could not purge task")
			}
			return c.SendStatus(fiber.StatusNoContent)
		}
		if err := gdb.Unscoped().Model(&models.Task{}).Where("id = ?", t.ID).
			Updates(map[string]interface{}{"status": "trash", "deleted_at": time.Now().UTC()}).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not delete task")
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}

// RestoreTask: POST /api/tasks/:id/restore — back to backlog (rule 4),
// appended at the column end since the old position is stale.
func RestoreTask(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		t, ok := loadVisibleTask(c, gdb, u, c.Params("id"))
		if !ok {
			return nil
		}
		err := gdb.Transaction(func(tx *gorm.DB) error {
			var maxPos float64
			if err := tx.Raw("SELECT COALESCE(MAX(position), 0) FROM tasks WHERE project_id = ? AND status = 'backlog' AND deleted_at IS NULL AND id <> ?", t.ProjectID, t.ID).Scan(&maxPos).Error; err != nil {
				return err
			}
			return tx.Unscoped().Model(&models.Task{}).Where("id = ?", t.ID).Updates(map[string]interface{}{
				"status": "backlog", "deleted_at": nil, "position": maxPos + positionGap,
			}).Error
		})
		if err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not restore task")
		}
		var fresh models.Task
		if err := gdb.Unscoped().First(&fresh, "id = ?", t.ID).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not reload task")
		}
		return c.JSON(tasksJSON(gdb, []models.Task{fresh})[0])
	}
}

// MoveTask: POST /api/tasks/:id/move — server computes position (rule 3):
// midpoint between neighbors (±1024 at the ends, 1024 in an empty column);
// when the smallest adjacent gap in the resulting order would drop under 1,
// the whole column is rewritten 1024-spaced. Single transaction.
func MoveTask(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		t, ok := loadVisibleTask(c, gdb, u, c.Params("id"))
		if !ok {
			return nil
		}
		var req moveTaskReq
		if err := c.BodyParser(&req); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		}
		if !taskStatuses[req.Status] || req.Status == "trash" {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "status must be a board column (trash goes through DELETE)")
		}
		if req.BeforeTaskID != nil && req.AfterTaskID != nil {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "provide at most one of before_task_id or after_task_id")
		}
		var anchor string
		if req.BeforeTaskID != nil {
			anchor = *req.BeforeTaskID
		} else if req.AfterTaskID != nil {
			anchor = *req.AfterTaskID
		}
		if anchor != "" {
			if _, err := uuid.Parse(anchor); err != nil {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "anchor task id must be a uuid")
			}
		}
		err := gdb.Transaction(func(tx *gorm.DB) error {
			// target column without the moved task
			var col []models.Task
			if err := tx.Where("project_id = ? AND status = ? AND deleted_at IS NULL AND id <> ?", t.ProjectID, req.Status, t.ID).
				Order("position, number").Find(&col).Error; err != nil {
				return err
			}
			idx := len(col)
			if anchor != "" {
				idx = -1
				for i := range col {
					if col[i].ID == anchor {
						idx = i
						break
					}
				}
				if idx < 0 {
					return errAnchorNotInColumn
				}
				if req.AfterTaskID != nil {
					idx++
				}
			}
			var newPos float64
			switch {
			case len(col) == 0:
				newPos = positionGap
			case idx == 0:
				newPos = col[0].Position - positionGap
			case idx == len(col):
				newPos = col[idx-1].Position + positionGap
			default:
				newPos = (col[idx-1].Position + col[idx].Position) / 2
			}
			// resulting column order + gap check
			order := make([]models.Task, 0, len(col)+1)
			order = append(order, col[:idx]...)
			moved := *t
			moved.Status, moved.Position = req.Status, newPos
			order = append(order, moved)
			order = append(order, col[idx:]...)
			rebalance := false
			for i := 1; i < len(order); i++ {
				if order[i].Position-order[i-1].Position < 1 {
					rebalance = true
					break
				}
			}
			if rebalance {
				for i := range order {
					order[i].Position = float64(i+1) * positionGap
				}
				newPos = order[idx].Position // the moved task's slot after rebalancing
			}
			moveUpdates := map[string]interface{}{"status": req.Status, "position": newPos}
			if req.Status != t.Status {
				applyTransition(moveUpdates, t, req.Status)
				from := t.Status
				if err := tx.Create(&models.TaskEvent{
					TaskID: t.ID, ActorID: u.ID, FromStatus: &from, ToStatus: req.Status,
				}).Error; err != nil {
					return err
				}
			}
			if err := tx.Unscoped().Model(&models.Task{}).Where("id = ?", t.ID).
				Updates(moveUpdates).Error; err != nil {
				return err
			}
			if rebalance {
				for i := range order {
					if order[i].ID == t.ID {
						continue
					}
					if err := tx.Model(&models.Task{}).Where("id = ?", order[i].ID).Update("position", order[i].Position).Error; err != nil {
						return err
					}
				}
			}
			return nil
		})
		if err != nil {
			if errors.Is(err, errAnchorNotInColumn) {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "anchor task must be in the target column of the same project")
			}
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not move task")
		}
		var fresh models.Task
		if err := gdb.Unscoped().First(&fresh, "id = ?", t.ID).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not reload task")
		}
		return c.JSON(tasksJSON(gdb, []models.Task{fresh})[0])
	}
}

// ListComments: GET /api/tasks/:id/comments — oldest first.
// `ponytail:` pagination when threads grow.
func ListComments(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		t, ok := loadVisibleTask(c, gdb, u, c.Params("id"))
		if !ok {
			return nil
		}
		var comments []models.Comment
		if err := gdb.Where("task_id = ?", t.ID).Order("created_at").Find(&comments).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not list comments")
		}
		return c.JSON(fiber.Map{"data": toCommentJSONs(comments)})
	}
}

// CreateComment: POST /api/tasks/:id/comments — member+.
func CreateComment(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		t, ok := loadVisibleTask(c, gdb, u, c.Params("id"))
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
		return c.Status(fiber.StatusCreated).JSON(toCommentJSON(cm))
	}
}
