// Team CRUD handlers. Writes are global-admin only (mounted behind
// RequireAdmin in the router); ListTeams mounts behind RequireAuth and
// scopes by role.
package handlers

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/VincentApta/Kica/api/internal/middleware"
	"github.com/VincentApta/Kica/api/internal/models"
)

type teamJSON struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	MemberCount int64  `json:"member_count"`
}

type createTeamReq struct {
	Name string `json:"name"`
}

type patchTeamReq struct {
	Name *string `json:"name"`
}

type replaceTeamMembersReq struct {
	UserIDs []string `json:"user_ids"`
}

// currentUser is the *models.User stashed by RequireAuth/RequireAdmin.
func currentUser(c *fiber.Ctx) *models.User {
	return c.Locals(middleware.UserKey).(*models.User)
}

// ListTeams: GET /api/teams — admin: all teams w/ member count; member:
// own teams only.
func ListTeams(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)
		page := queryInt(c, "page", 1)
		perPage := queryInt(c, "per_page", defaultPerPage)
		// fresh query per statement — reusing one after Count() drags the
		// count(*) select into the Find (same pattern as ListUsers).
		scope := func() *gorm.DB {
			q := gdb.Model(&models.Team{})
			if u.GlobalRole != roleAdmin {
				memberOf := gdb.Model(&models.TeamMember{}).Select("team_id").Where("user_id = ?", u.ID)
				q = q.Where("id IN (?)", memberOf)
			}
			return q
		}
		var total int64
		if err := scope().Count(&total).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not count teams")
		}
		var teams []models.Team
		if err := scope().Order("name").Limit(perPage).Offset((page - 1) * perPage).Find(&teams).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not list teams")
		}
		counts := teamMemberCounts(gdb, teams)
		data := make([]teamJSON, len(teams))
		for i := range teams {
			data[i] = teamJSON{ID: teams[i].ID, Name: teams[i].Name, Slug: teams[i].Slug, MemberCount: counts[teams[i].ID]}
		}
		return c.JSON(fiber.Map{"data": data, "page": page, "per_page": perPage, "total": total})
	}
}

// teamMemberCounts batches member counts for the given teams. On error the
// counts degrade to 0 — the list itself already succeeded.
func teamMemberCounts(gdb *gorm.DB, teams []models.Team) map[string]int64 {
	counts := make(map[string]int64, len(teams))
	if len(teams) == 0 {
		return counts
	}
	ids := make([]string, len(teams))
	for i := range teams {
		ids[i] = teams[i].ID
	}
	var rows []struct {
		TeamID string
		Cnt    int64
	}
	if err := gdb.Model(&models.TeamMember{}).Select("team_id, COUNT(*) AS cnt").
		Where("team_id IN ?", ids).Group("team_id").Scan(&rows).Error; err != nil {
		return counts
	}
	for _, r := range rows {
		counts[r.TeamID] = r.Cnt
	}
	return counts
}

// CreateTeam: POST /api/teams → 201; slug auto-generated unique from name.
func CreateTeam(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req createTeamReq
		if err := c.BodyParser(&req); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		}
		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "name is required")
		}
		if slugify(req.Name) == "" {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "name must contain letters or digits")
		}
		slug, err := uniqueSlug(gdb, req.Name)
		if err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not generate slug")
		}
		team := &models.Team{Name: req.Name, Slug: slug}
		if err := gdb.Create(team).Error; err != nil {
			if isUniqueViolation(err) {
				return httpErr(c, fiber.StatusConflict, "name_exists", "a team with this name already exists")
			}
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not create team")
		}
		return c.Status(fiber.StatusCreated).JSON(teamJSON{ID: team.ID, Name: team.Name, Slug: team.Slug})
	}
}

// slugify lowercases and collapses non-alphanumerics to single dashes,
// trimming leading/trailing dashes.
func slugify(name string) string {
	var b strings.Builder
	dash := true // suppress leading dash
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case !dash:
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

// uniqueSlug derives a free slug from name: "acme", "acme-2", "acme-3", …
// Concurrent creates are backstopped by the unique index (caller maps the
// violation to 409).
func uniqueSlug(gdb *gorm.DB, name string) (string, error) {
	base := slugify(name)
	slug := base
	for i := 2; ; i++ {
		var exists int64
		if err := gdb.Model(&models.Team{}).Where("slug = ?", slug).Count(&exists).Error; err != nil {
			return "", err
		}
		if exists == 0 {
			return slug, nil
		}
		slug = base + "-" + strconv.Itoa(i)
	}
}

// PatchTeam: PATCH /api/teams/:id → 200 team. Slug stays stable across
// renames so external references don't break.
func PatchTeam(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if _, err := uuid.Parse(id); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_uuid", "team id must be a uuid")
		}
		var req patchTeamReq
		if err := c.BodyParser(&req); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		}
		var team models.Team
		if err := gdb.First(&team, "id = ?", id).Error; err != nil {
			return httpErr(c, fiber.StatusNotFound, "team_not_found", "no such team")
		}
		if req.Name != nil {
			name := strings.TrimSpace(*req.Name)
			if name == "" {
				return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", "name must not be empty")
			}
			if err := gdb.Model(&team).Update("name", name).Error; err != nil {
				if isUniqueViolation(err) {
					return httpErr(c, fiber.StatusConflict, "name_exists", "a team with this name already exists")
				}
				return httpErr(c, fiber.StatusInternalServerError, "internal", "could not update team")
			}
			team.Name = name
		}
		counts := teamMemberCounts(gdb, []models.Team{team})
		return c.JSON(teamJSON{ID: team.ID, Name: team.Name, Slug: team.Slug, MemberCount: counts[team.ID]})
	}
}

// DeleteTeam: DELETE /api/teams/:id → 204; 409 while any project (including
// soft-deleted ones — the rows still reference the team) exists.
func DeleteTeam(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if _, err := uuid.Parse(id); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_uuid", "team id must be a uuid")
		}
		var team models.Team
		if err := gdb.First(&team, "id = ?", id).Error; err != nil {
			return httpErr(c, fiber.StatusNotFound, "team_not_found", "no such team")
		}
		var projects int64
		if err := gdb.Unscoped().Model(&models.Project{}).Where("team_id = ?", id).Count(&projects).Error; err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not count projects")
		}
		if projects > 0 {
			return httpErr(c, fiber.StatusConflict, "team_has_projects", "team still has projects")
		}
		// members deleted explicitly: sqlite (tests) has no FK cascade.
		err := gdb.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("team_id = ?", id).Delete(&models.TeamMember{}).Error; err != nil {
				return err
			}
			return tx.Delete(&team).Error
		})
		if err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not delete team")
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}

// teamMemberJSON mirrors the wire shape returned by ListTeamMembers and
// PUT /teams/:id/members for consistency with the project members contract.
type teamMemberJSON struct {
	UserID string `json:"user_id" gorm:"column:user_id"`
	Name   string `json:"name"    gorm:"column:name"`
	Email  string `json:"email"   gorm:"column:email"`
}

// ListTeamMembers: GET /api/teams/:id/members — visible to any team member
// and to global admins. 404 for non-members (no leak).
func ListTeamMembers(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if _, err := uuid.Parse(id); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_uuid", "team id must be a uuid")
		}
		var team models.Team
		if err := gdb.First(&team, "id = ?", id).Error; err != nil {
			return httpErr(c, fiber.StatusNotFound, "team_not_found", "no such team")
		}
		// visibility: global admin or team member
		u := currentUser(c)
		if u.GlobalRole != roleAdmin {
			var cnt int64
			gdb.Model(&models.TeamMember{}).Where("team_id = ? AND user_id = ?", team.ID, u.ID).Count(&cnt)
			if cnt == 0 {
				return httpErr(c, fiber.StatusNotFound, "team_not_found", "no such team")
			}
		}
		var members []teamMemberJSON
		err := gdb.Table("team_members").
			Select("team_members.user_id, users.name, users.email").
			Joins("JOIN users ON users.id = team_members.user_id").
			Where("team_members.team_id = ?", team.ID).
			Order("users.name").
			Scan(&members).Error
		if err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not list members")
		}
		return c.JSON(fiber.Map{"members": members})
	}
}

// ReplaceTeamMembers: PUT /api/teams/:id/members {user_ids} — full replace.
// Every id must be a uuid of an existing user; empty list clears membership.
func ReplaceTeamMembers(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if _, err := uuid.Parse(id); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_uuid", "team id must be a uuid")
		}
		var team models.Team
		if err := gdb.First(&team, "id = ?", id).Error; err != nil {
			return httpErr(c, fiber.StatusNotFound, "team_not_found", "no such team")
		}
		var req replaceTeamMembersReq
		if err := c.BodyParser(&req); err != nil {
			return httpErr(c, fiber.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		}
		if msg := validateUserIDs(gdb, req.UserIDs); msg != "" {
			return httpErr(c, fiber.StatusUnprocessableEntity, "validation_failed", msg)
		}
		err := gdb.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("team_id = ?", team.ID).Delete(&models.TeamMember{}).Error; err != nil {
				return err
			}
			if len(req.UserIDs) == 0 {
				return nil
			}
			rows := make([]models.TeamMember, len(req.UserIDs))
			for i, uid := range req.UserIDs {
				rows[i] = models.TeamMember{TeamID: team.ID, UserID: uid}
			}
			return tx.Create(&rows).Error
		})
		if err != nil {
			return httpErr(c, fiber.StatusInternalServerError, "internal", "could not replace members")
		}
		return c.JSON(teamJSON{ID: team.ID, Name: team.Name, Slug: team.Slug, MemberCount: int64(len(req.UserIDs))})
	}
}

// validateUserIDs: every id a uuid, no duplicates, all referencing existing
// users. Returns the rejection message, "" when valid.
func validateUserIDs(gdb *gorm.DB, ids []string) string {
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if _, err := uuid.Parse(id); err != nil {
			return "user ids must be uuids"
		}
		if seen[id] {
			return "user ids must not contain duplicates"
		}
		seen[id] = true
	}
	if len(ids) == 0 {
		return ""
	}
	var count int64
	if err := gdb.Model(&models.User{}).Where("id IN ?", ids).Count(&count).Error; err != nil {
		return "could not validate user ids"
	}
	if count != int64(len(ids)) {
		return "user ids must reference existing users"
	}
	return ""
}
