// Label CRUD handlers. Reads for any user who can see the project; writes
// (create/delete) for project_admin or global admin (permission matrix row
// "Manage project members/labels/GH config").
package handlers

import (
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/VincentApta/Kica/api/internal/models"
)

// labelColorRe per domain: hex color.
var labelColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type createLabelReq struct {
	Name  string  `json:"name"`
	Color *string `json:"color"`
}

type patchLabelReq struct {
	Name  *string `json:"name"`
	Color *string `json:"color"`
}

// labelJSON is the wire `label` shape (task.labels and project label list).
type labelJSON struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

func toLabelJSON(l *models.Label) labelJSON {
	return labelJSON{ID: l.ID, Name: l.Name, Color: l.Color}
}

// ListLabels: GET /api/projects/:id/labels — any visible user.
func ListLabels(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		p, _, ok := loadVisibleProject(c, gdb, u, c.Params("id"))
		if !ok {
			return nil
		}
		var labels []models.Label
		if err := gdb.Where("project_id = ?", p.ID).Order("name").Find(&labels).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not list labels")
		}
		data := make([]labelJSON, len(labels))
		for i := range labels {
			data[i] = toLabelJSON(&labels[i])
		}
		return c.JSON(fiber.Map{"data": data})
	}
}

// CreateLabel: POST /api/projects/:id/labels — project_admin+. Members see
// the project, so they get 403 (not 404) here.
func CreateLabel(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		p, pm, ok := loadVisibleProject(c, gdb, u, c.Params("id"))
		if !ok {
			return nil
		}
		if u.GlobalRole != roleAdmin && (pm == nil || pm.Role != roleProjectAdmin) {
			return httpErr(c, fiber.StatusForbidden, "forbidden", "project admin role required")
		}
		var req createLabelReq
		if err := c.BodyParser(&req); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		}
		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "name is required")
		}
		color := "#94a3b8"
		if req.Color != nil && *req.Color != "" {
			if !labelColorRe.MatchString(*req.Color) {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "color must be a hex value like #0ea5e9")
			}
			color = *req.Color
		}
		l := &models.Label{ProjectID: p.ID, Name: req.Name, Color: color}
		if err := gdb.Create(l).Error; err != nil {
			if isUniqueViolation(err) {
				return httpErr(c, fiber.StatusConflict, "label_exists", "a label with this name already exists in the project")
			}
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not create label")
		}
		return c.Status(fiber.StatusCreated).JSON(toLabelJSON(l))
	}
}

// DeleteLabel: DELETE /api/labels/:id — project_admin+. Removes the label's
// task associations too (sqlite tests have no FK cascade).
func DeleteLabel(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		id := c.Params("id")
		if _, err := uuid.Parse(id); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_uuid", "label id must be a uuid")
		}
		var l models.Label
		if err := gdb.First(&l, "id = ?", id).Error; err != nil {
			return httpErr(c, fiber.StatusNotFound, "not_found", "no such label")
		}
		_, pm, ok := loadVisibleProject(c, gdb, u, l.ProjectID)
		if !ok {
			return nil
		}
		if u.GlobalRole != roleAdmin && (pm == nil || pm.Role != roleProjectAdmin) {
			return httpErr(c, fiber.StatusForbidden, "forbidden", "project admin role required")
		}
		err := gdb.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("label_id = ?", l.ID).Delete(&models.TaskLabel{}).Error; err != nil {
				return err
			}
			return tx.Delete(&l).Error
		})
		if err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not delete label")
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}

// PatchLabel: PATCH /api/labels/:id — project_admin+. Updates name and/or
// color; same visibility/role guard as DeleteLabel.
func PatchLabel(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		id := c.Params("id")
		if _, err := uuid.Parse(id); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_uuid", "label id must be a uuid")
		}
		var req patchLabelReq
		if err := c.BodyParser(&req); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		}
		if req.Name == nil && req.Color == nil {
			return httpErr(c, fiber.StatusBadRequest, "empty_patch", "nothing to update")
		}
		if req.Name != nil {
			name := strings.TrimSpace(*req.Name)
			if name == "" {
				return httpErr(c, fiber.StatusBadRequest, "invalid_name", "label name must not be empty")
			}
			req.Name = &name
		}
		if req.Color != nil && !labelColorRe.MatchString(strings.TrimSpace(*req.Color)) {
			return httpErr(c, fiber.StatusBadRequest, "invalid_color", "color must be a #RRGGBB hex value")
		}
		var l models.Label
		if err := gdb.First(&l, "id = ?", id).Error; err != nil {
			return httpErr(c, fiber.StatusNotFound, "not_found", "no such label")
		}
		_, pm, ok := loadVisibleProject(c, gdb, u, l.ProjectID)
		if !ok {
			return nil
		}
		if u.GlobalRole != roleAdmin && (pm == nil || pm.Role != roleProjectAdmin) {
			return httpErr(c, fiber.StatusForbidden, "forbidden", "project admin role required")
		}
		updates := map[string]any{}
		if req.Name != nil {
			updates["name"] = *req.Name
		}
		if req.Color != nil {
			updates["color"] = strings.TrimSpace(*req.Color)
		}
		if err := gdb.Model(&l).Updates(updates).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not update label")
		}
		if err := gdb.First(&l, "id = ?", id).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not reload label")
		}
		return c.JSON(toLabelJSON(&l))
	}
}
