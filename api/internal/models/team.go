// Team + TeamMember per docs/domain.md.
package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Team: name unique, slug unique URL segment (auto-generated from name).
type Team struct {
	ID   string `gorm:"primaryKey;type:uuid"`
	Name string `gorm:"uniqueIndex;not null"`
	Slug string `gorm:"uniqueIndex;not null"`
}

// BeforeCreate fills a uuid v4 primary key (app-side, both dialects).
func (t *Team) BeforeCreate(_ *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	return nil
}

// TeamMember is the (team_id, user_id) membership join. No team roles v1.
// `ponytail:` add team_role if org grows.
type TeamMember struct {
	TeamID string `gorm:"primaryKey;type:uuid"`
	UserID string `gorm:"primaryKey;type:uuid"`
}
