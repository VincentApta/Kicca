// GitHub integration handlers (T7): per-project PAT storage (encrypted at
// rest, never serialized) and task → issue creation. Visibility rides the
// existing project/task loaders (404 no-leak).
package handlers

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/VincentApta/Kicca/api/internal/github"
	"github.com/VincentApta/Kicca/api/internal/models"
)

// ghRepoRe: `owner/name` — GitHub allows [A-Za-z0-9_.-] in both halves.
var ghRepoRe = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

// ghLinkJSON is the wire `gh_link` shape (also the POST issue response).
type ghLinkJSON struct {
	Repo        string `json:"repo"`
	IssueNumber int64  `json:"issue_number"`
	IssueURL    string `json:"issue_url"`
}

type putProjectGithubReq struct {
	Repo  string `json:"repo"`
	Token string `json:"token"`
}

// PutProjectGithub: PUT /api/projects/:id/github — admin/project_admin.
// Stores repo + encrypted PAT. A blank token keeps the stored one (the
// token is write-only, so re-saving just the repo must not demand it back);
// it is required when none is stored yet.
func PutProjectGithub(gdb *gorm.DB, encKey *[32]byte) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		p, pm, ok := loadVisibleProject(c, gdb, u, c.Params("id"))
		if !ok {
			return nil
		}
		if u.GlobalRole != roleAdmin && (pm == nil || pm.Role != roleProjectAdmin) {
			return httpErr(c, fiber.StatusForbidden, "forbidden", "project admin role required")
		}
		var req putProjectGithubReq
		if err := c.BodyParser(&req); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		}
		repo := strings.TrimSpace(req.Repo)
		if !ghRepoRe.MatchString(repo) {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "repo must be owner/name")
		}
		token := strings.TrimSpace(req.Token)
		if token == "" && len(p.GhTokenEnc) == 0 {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "token is required")
		}
		updates := map[string]interface{}{"gh_repo": repo}
		if token != "" {
			enc, err := github.EncryptToken(encKey, token)
			if err != nil {
				return httpErr(c, fiber.StatusInternalServerError, "internal", "GitHub encryption key not configured")
			}
			updates["gh_token_enc"] = enc
		}
		if err := gdb.Model(&models.Project{}).Where("id = ?", p.ID).Updates(updates).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not save GitHub settings")
		}
		return c.JSON(fiber.Map{"repo": repo})
	}
}

// CreateTaskIssue: POST /api/tasks/:id/github/issue — member+. Creates the
// upstream issue (title, description + footer) and links it; one link per
// task. 409 already linked, 422 project unconfigured, 502 upstream failure
// after the single retry.
func CreateTaskIssue(gdb *gorm.DB, encKey *[32]byte) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		t, ok := loadVisibleTask(c, gdb, u, c.Params("id"))
		if !ok {
			return nil
		}
		var existing models.GitHubIssueLink
		if err := gdb.First(&existing, "task_id = ?", t.ID).Error; err == nil {
			return httpErr(c, fiber.StatusConflict, "task_already_linked", "task already has a GitHub issue")
		}
		var p models.Project
		if err := gdb.First(&p, "id = ?", t.ProjectID).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not load project")
		}
		if p.GhRepo == nil || len(p.GhTokenEnc) == 0 {
			return httpErr(c, fiber.StatusUnprocessableEntity, "gh_unconfigured", "project has no GitHub configuration")
		}
		token, err := github.DecryptToken(encKey, p.GhTokenEnc)
		if err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not decrypt GitHub token")
		}
		issue, err := github.NewClient().CreateIssue(*p.GhRepo, token, t.Title, issueBody(p.Key, t))
		if err != nil {
			return httpErr(c, fiber.StatusBadGateway, "gh_upstream_error", err.Error())
		}
		link := &models.GitHubIssueLink{
			TaskID: t.ID, Repo: *p.GhRepo, IssueNumber: issue.Number, IssueURL: issue.URL,
		}
		if err := gdb.Create(link).Error; err != nil {
			if isUniqueViolation(err) { // concurrent create lost the race
				return httpErr(c, fiber.StatusConflict, "task_already_linked", "task already has a GitHub issue")
			}
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not store GitHub link")
		}
		return c.Status(fiber.StatusCreated).JSON(ghLinkJSON{Repo: link.Repo, IssueNumber: link.IssueNumber, IssueURL: link.IssueURL})
	}
}

// issueBody: task description plus a small footer pointing back at the task.
func issueBody(projectKey string, t *models.Task) string {
	return t.Description + "\n\n---\n\nCreated from kicca task " + projectKey + "-" + strconv.FormatInt(t.Number, 10)
}
