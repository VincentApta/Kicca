// Project + ProjectMember per docs/domain.md.
package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Project: gh_token_enc never serialized — handlers project through
// projectJSON (api-contract: GH token never returned).
type Project struct {
	ID          string         `gorm:"primaryKey;type:uuid"`
	TeamID      string         `gorm:"not null;index;type:uuid"`
	Name        string         `gorm:"not null"`
	Key         string         `gorm:"uniqueIndex;not null"` // `ponytail:` index spans soft-deleted rows; partial index on upgrade
	Description string         `gorm:"not null;default:''"`
	GhRepo      *string        // `owner/name`, set in T7
	GhTokenEnc  []byte         // AES-256-GCM ciphertext of PAT, set in T7
	TaskSeq     int64          `gorm:"not null;default:0"` // per-project task counter, used in T4
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

// BeforeCreate fills a uuid v4 primary key (app-side, both dialects).
func (p *Project) BeforeCreate(_ *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	return nil
}

// ProjectMember: (project_id, user_id) join with role. Multiple
// project_admins allowed (domain).
type ProjectMember struct {
	ProjectID string `gorm:"primaryKey;type:uuid"`
	UserID    string `gorm:"primaryKey;type:uuid"`
	Role      string `gorm:"not null;default:member"` // project_admin | member
}
