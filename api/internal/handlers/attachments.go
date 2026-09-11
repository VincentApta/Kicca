// Task attachment handlers (#34): multipart upload, list, stream, delete —
// team via /api/tasks/:id/attachments, clients via the /api/client/tickets/
// :id/attachments mirrors (own tickets only). Streaming (GET /api/attachments/
// :id) admits team users through project visibility and clients through
// created_by. Content type is sniffed server-side (http.DetectContentType),
// never trusted from the client.
package handlers

import (
	"context"
	"io"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/VincentApta/Kica/api/internal/models"
	"github.com/VincentApta/Kica/api/internal/storage"
)

// allowedAttachmentTypes: the sniffed-type allowlist (images + browser-safe
// videos). key → object-key extension.
var allowedAttachmentTypes = map[string]string{
	"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp", "image/gif": ".gif",
	"video/mp4": ".mp4", "video/quicktime": ".mov", "video/webm": ".webm",
}

// attStore + maxAttachmentBytes are wired in Register (env-driven, same
// pattern as GITHUB_API_BASE).
var (
	attStore           storage.Store
	maxAttachmentBytes int64
)

// attachmentJSON is the wire `attachment` shape (team). Clients get the
// minimal clientAttachmentJSON — no creator identity beyond what they sent.
type attachmentJSON struct {
	ID          string    `json:"id"`
	TaskID      string    `json:"task_id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

type clientAttachmentJSON struct {
	ID          string    `json:"id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	CreatedAt   time.Time `json:"created_at"`
}

// sanitizeFilename strips path + control/markup-significant characters and
// truncates. Display-only (the object key is a uuid), but it round-trips
// through markdown alt text and Content-Disposition, so keep it boring.
func sanitizeFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	var b strings.Builder
	for _, r := range name {
		switch {
		case unicode.IsControl(r), strings.ContainsRune(`[]()!`+"`<>|", r):
			b.WriteRune('_')
		case unicode.IsSpace(r):
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" || out == "." || out == ".." {
		return "attachment"
	}
	if runes := []rune(out); len(runes) > 200 {
		out = string(runes[:200])
	}
	return out
}

// saveAttachment is the shared team/client upload path: parse the `file`
// part, enforce size + sniffed-type allowlist, store the blob, insert the
// row. The returned error is always an httpErr response ready to return.
func saveAttachment(c *fiber.Ctx, gdb *gorm.DB, taskID string, u *models.User) (*models.TaskAttachment, error) {
	fh, err := c.FormFile("file")
	if err != nil {
		return nil, httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "multipart `file` part is required")
	}
	if fh.Size > maxAttachmentBytes {
		return nil, httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed",
			"file exceeds the "+strconv.FormatInt(maxAttachmentBytes>>20, 10)+"MB limit")
	}
	f, err := fh.Open()
	if err != nil {
		return nil, httpErr(c, fiber.StatusBadRequest, "invalid_multipart", "could not read the uploaded file")
	}
	defer f.Close()
	buf, err := io.ReadAll(f)
	if err != nil {
		return nil, httpErr(c, fiber.StatusBadRequest, "invalid_multipart", "could not read the uploaded file")
	}
	if int64(len(buf)) > maxAttachmentBytes { // fh.Size lies → re-check
		return nil, httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "file exceeds the size limit")
	}
	sniff := buf
	if len(sniff) > 512 {
		sniff = sniff[:512]
	}
	contentType := http.DetectContentType(sniff)
	ext, ok := allowedAttachmentTypes[contentType]
	if !ok {
		return nil, httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed",
			"unsupported file type — allowed: png, jpeg, webp, gif, mp4, mov, webm")
	}
	a := &models.TaskAttachment{
		TaskID:      taskID,
		Filename:    sanitizeFilename(fh.Filename),
		ContentType: contentType,
		SizeBytes:   int64(len(buf)),
		Storage:     attStore.Kind(),
		ObjectKey:   uuid.NewString() + ext,
		CreatedBy:   u.ID,
	}
	ctx := context.Background()
	if err := attStore.Put(ctx, a.ObjectKey, a.ContentType, a.SizeBytes, strings.NewReader(string(buf))); err != nil {
		log.Printf("attachments: put %s: %v", a.ObjectKey, err)
		return nil, httpErr(c, fiber.StatusInternalServerError, "internal", "could not store attachment")
	}
	if err := gdb.Create(a).Error; err != nil {
		attStore.Delete(ctx, a.ObjectKey) // best-effort; a stray blob is harmless
		return nil, httpErr(c, fiber.StatusInternalServerError, "internal", "could not save attachment")
	}
	return a, nil
}

// UploadTaskAttachment: POST /api/tasks/:id/attachments (multipart `file`) —
// member+, any visible task (trashed included, same as comments).
func UploadTaskAttachment(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		t, ok := loadVisibleTask(c, gdb, u, c.Params("id"))
		if !ok {
			return nil
		}
		a, err := saveAttachment(c, gdb, t.ID, u)
		if a == nil {
			return err // rejection already written (httpErr's return is always nil)
		}
		return c.Status(fiber.StatusCreated).JSON(attachmentJSON{
			ID: a.ID, TaskID: a.TaskID, Filename: a.Filename, ContentType: a.ContentType,
			SizeBytes: a.SizeBytes, CreatedBy: a.CreatedBy, CreatedAt: a.CreatedAt,
		})
	}
}

// ListTaskAttachments: GET /api/tasks/:id/attachments — oldest first.
func ListTaskAttachments(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		t, ok := loadVisibleTask(c, gdb, u, c.Params("id"))
		if !ok {
			return nil
		}
		var atts []models.TaskAttachment
		if err := gdb.Where("task_id = ?", t.ID).Order("created_at").Find(&atts).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not list attachments")
		}
		out := make([]attachmentJSON, len(atts))
		for i := range atts {
			a := &atts[i]
			out[i] = attachmentJSON{ID: a.ID, TaskID: a.TaskID, Filename: a.Filename,
				ContentType: a.ContentType, SizeBytes: a.SizeBytes, CreatedBy: a.CreatedBy, CreatedAt: a.CreatedAt}
		}
		return c.JSON(fiber.Map{"data": out})
	}
}

// loadAttachment fetches by uuid, 404 no-leak on anything else.
func loadAttachment(c *fiber.Ctx, gdb *gorm.DB, id string) (*models.TaskAttachment, bool) {
	if _, err := uuid.Parse(id); err != nil {
		httpErr(c, fiber.StatusBadRequest, "invalid_uuid", "attachment id must be a uuid")
		return nil, false
	}
	var a models.TaskAttachment
	if err := gdb.First(&a, "id = ?", id).Error; err != nil {
		httpErr(c, fiber.StatusNotFound, "not_found", "no such attachment")
		return nil, false
	}
	return &a, true
}

// GetAttachment: GET /api/attachments/:id — streams the blob. Team users via
// project visibility (loadVisibleTask); clients iff they created it. Mounted
// behind RequireAuth so both roles reach it.
func GetAttachment(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		a, ok := loadAttachment(c, gdb, c.Params("id"))
		if !ok {
			return nil
		}
		if u.GlobalRole == roleClient {
			if a.CreatedBy != u.ID {
				return httpErr(c, fiber.StatusNotFound, "not_found", "no such attachment")
			}
		} else if _, ok := loadVisibleTask(c, gdb, u, a.TaskID); !ok {
			return nil
		}
		rc, err := attStore.Open(context.Background(), a.ObjectKey)
		if err != nil {
			return httpErr(c, fiber.StatusNotFound, "not_found", "no such attachment")
		}
		// rc is closed by fasthttp after the body streams out (SetBodyStream
		// marks the response must-close).
		c.Set(fiber.HeaderContentType, a.ContentType)
		c.Set(fiber.HeaderContentDisposition,
			mime.FormatMediaType("inline", map[string]string{"filename": a.Filename}))
		return c.SendStream(rc, int(a.SizeBytes))
	}
}

// DeleteAttachment: DELETE /api/attachments/:id — team only (route guard),
// project visibility via the parent task.
func DeleteAttachment(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		a, ok := loadAttachment(c, gdb, c.Params("id"))
		if !ok {
			return nil
		}
		if _, ok := loadVisibleTask(c, gdb, u, a.TaskID); !ok {
			return nil
		}
		if err := gdb.Delete(&models.TaskAttachment{}, "id = ?", a.ID).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not delete attachment")
		}
		if err := attStore.Delete(context.Background(), a.ObjectKey); err != nil {
			log.Printf("attachments: orphaned blob %s: %v", a.ObjectKey, err) // row is gone; blob is dead weight only
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}

// loadOwnTicket: the client-portal task loader — ticket exists, not trashed,
// created by this client, in a still-linked project. Any miss is a 404
// no-leak (same convention as loadVisibleProject).
func loadOwnTicket(c *fiber.Ctx, gdb *gorm.DB, u *models.User, id string) (*models.Task, bool) {
	if _, err := uuid.Parse(id); err != nil {
		httpErr(c, fiber.StatusBadRequest, "invalid_uuid", "ticket id must be a uuid")
		return nil, false
	}
	var t models.Task
	if err := gdb.First(&t, "id = ?", id).Error; err != nil || t.CreatedBy != u.ID {
		httpErr(c, fiber.StatusNotFound, "not_found", "no such ticket")
		return nil, false
	}
	linked, err := isLinkedClientProject(gdb, u.ID, t.ProjectID)
	if err != nil || !linked {
		httpErr(c, fiber.StatusNotFound, "not_found", "no such ticket")
		return nil, false
	}
	return &t, true
}

// ClientUploadTicketAttachment: POST /api/client/tickets/:id/attachments —
// own tickets only.
func ClientUploadTicketAttachment(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		t, ok := loadOwnTicket(c, gdb, u, c.Params("id"))
		if !ok {
			return nil
		}
		a, err := saveAttachment(c, gdb, t.ID, u)
		if a == nil {
			return err // rejection already written (httpErr's return is always nil)
		}
		return c.Status(fiber.StatusCreated).JSON(clientAttachmentJSON{
			ID: a.ID, Filename: a.Filename, ContentType: a.ContentType,
			SizeBytes: a.SizeBytes, CreatedAt: a.CreatedAt,
		})
	}
}

// ClientListTicketAttachments: GET /api/client/tickets/:id/attachments —
// minimal fields only.
func ClientListTicketAttachments(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		t, ok := loadOwnTicket(c, gdb, u, c.Params("id"))
		if !ok {
			return nil
		}
		var atts []models.TaskAttachment
		// own-created only: mirrors the GET /api/attachments/:id auth (client
		// via created_by), so every listed row is also streamable.
		if err := gdb.Where("task_id = ? AND created_by = ?", t.ID, u.ID).Order("created_at").Find(&atts).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not list attachments")
		}
		out := make([]clientAttachmentJSON, len(atts))
		for i := range atts {
			a := &atts[i]
			out[i] = clientAttachmentJSON{ID: a.ID, Filename: a.Filename,
				ContentType: a.ContentType, SizeBytes: a.SizeBytes, CreatedAt: a.CreatedAt}
		}
		return c.JSON(fiber.Map{"data": out})
	}
}
