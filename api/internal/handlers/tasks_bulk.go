// BulkPatchTasks: PATCH /api/tasks/bulk — multi-select board/list actions
// (issue #46): {ids[], status?, assignee_id?}. All-or-nothing: per-task
// validation first (uuid, exists, project visibility — deny is a reported
// failure, not a 404, because the caller already sees the task), then one
// transaction applies everything; a failure anywhere changes nothing.
// status=trash is the bulk delete (same semantics as single DELETE: soft,
// no task_events row). A status move appends at the target column end per
// task project (rule 3 spacing) and stamps analytics (rule 8). Team-only —
// the /api/tasks group is behind RequireTeam.
package handlers

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/VincentApta/Kica/api/internal/models"
)

type bulkPatchReq struct {
	IDs        []string       `json:"ids"`
	Status     *string        `json:"status"`
	AssigneeID json.RawMessage `json:"assignee_id"` // uuid string | null (unassign)
}

// bulkFailure is one rejected id in the 422 details array.
type bulkFailure struct {
	TaskID  string `json:"task_id"`
	Message string `json:"message"`
}

func bulkErr(c *fiber.Ctx, failures []bulkFailure) error {
	return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
		"error": fiber.Map{
			"code":    "validation_failed",
			"message": fmt.Sprintf("%d of the selected tasks failed validation", len(failures)),
			"details": failures,
		},
	})
}

// BulkPatchTasks handler. See file comment for semantics.
func BulkPatchTasks(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		var req bulkPatchReq
		if err := c.BodyParser(&req); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		}
		if len(req.IDs) == 0 {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "ids must not be empty")
		}
		seen := make(map[string]bool, len(req.IDs))
		for _, id := range req.IDs {
			if seen[id] {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "ids must not contain duplicates")
			}
			seen[id] = true
		}
		hasStatus := req.Status != nil
		hasAssignee := len(req.AssigneeID) > 0
		if !hasStatus && !hasAssignee {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "provide at least one of status or assignee_id")
		}
		if hasStatus && !taskStatuses[*req.Status] {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "status must be a valid task status")
		}
		var assignee *string // nil = unassign when hasAssignee
		if hasAssignee && string(req.AssigneeID) != "null" {
			var a string
			if err := json.Unmarshal(req.AssigneeID, &a); err != nil {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "assignee_id must be a uuid")
			}
			if msg := validateUserIDs(gdb, []string{a}); msg != "" {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", msg)
			}
			assignee = &a
		}

		// load + per-task validation (all-or-nothing happens here)
		var tasks []models.Task
		if err := gdb.Unscoped().Where("id IN ?", req.IDs).Find(&tasks).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not load tasks")
		}
		byID := make(map[string]*models.Task, len(tasks))
		for i := range tasks {
			byID[tasks[i].ID] = &tasks[i]
		}
		var failures []bulkFailure
		for _, id := range req.IDs {
			t, ok := byID[id]
			if !ok {
				failures = append(failures, bulkFailure{id, "no such task"})
				continue
			}
			if u.GlobalRole != roleAdmin {
				var n int64
				gdb.Model(&models.ProjectMember{}).Where("project_id = ? AND user_id = ?", t.ProjectID, u.ID).Count(&n)
				if n == 0 {
					failures = append(failures, bulkFailure{id, "no such task"}) // deny, no leak
					continue
				}
			}
			if hasStatus && t.Status == "trash" && *req.Status != "trash" {
				failures = append(failures, bulkFailure{id, "task is in trash — restore it first"})
			}
		}
		if failures != nil {
			return bulkErr(c, failures)
		}

		// per (project, target column) max position, so appended tasks space
		// 1024 apart even when ids span projects
		maxPos := map[string]float64{}
		if hasStatus && *req.Status != "trash" {
			var rows []struct {
				ProjectID string
				Max       float64
			}
			projects := make([]string, 0, len(tasks))
			for i := range tasks {
				projects = append(projects, tasks[i].ProjectID)
			}
			if err := gdb.Model(&models.Task{}).
				Select("project_id, COALESCE(MAX(position), 0) AS max").
				Where("status = ? AND deleted_at IS NULL AND project_id IN ?", *req.Status, projects).
				Group("project_id").Scan(&rows).Error; err != nil {
				return httpErr(c, fiber.StatusInternalServerError, "internal", "could not load column positions")
			}
			for _, r := range rows {
				maxPos[r.ProjectID] = r.Max
			}
		}

		err := gdb.Transaction(func(tx *gorm.DB) error {
			for _, id := range req.IDs { // request order: appended positions match it
				t := byID[id]
				updates := map[string]interface{}{}
				if hasAssignee {
					updates["assignee_id"] = assignee
				}
				if hasStatus && *req.Status != t.Status {
					if *req.Status == "trash" {
						updates["status"] = "trash"
						updates["deleted_at"] = time.Now().UTC()
					} else {
						maxPos[t.ProjectID] += positionGap
						updates["status"] = *req.Status
						updates["position"] = maxPos[t.ProjectID]
						applyTransition(updates, t, *req.Status)
						from := t.Status
						if err := tx.Create(&models.TaskEvent{
							TaskID: t.ID, ActorID: u.ID, FromStatus: &from, ToStatus: *req.Status,
						}).Error; err != nil {
							return err
						}
					}
				}
				if len(updates) == 0 {
					continue
				}
				if err := tx.Unscoped().Model(&models.Task{}).Where("id = ?", t.ID).Updates(updates).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not update tasks")
		}

		var fresh []models.Task
		if err := gdb.Unscoped().Where("id IN ?", req.IDs).Find(&fresh).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not reload tasks")
		}
		return c.JSON(fiber.Map{"updated": len(fresh), "data": tasksJSON(gdb, fresh)})
	}
}
