// Package models holds GORM entities for kicca.
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User per docs/domain.md. Email is stored lowercased; the Postgres column is
// citext, sqlite tests rely on the lowercase normalization.
type User struct {
	ID           string     `gorm:"primaryKey;type:uuid"`
	Email        string     `gorm:"uniqueIndex;not null"`
	PasswordHash string     `gorm:"not null"`
	Name         string     `gorm:"not null"`
	// GlobalRole: admin | member | client. Clients submit tickets into
	// linked projects only — they have no ProjectMember row, so every
	// team-scoped query naturally excludes them.
	GlobalRole   string     `gorm:"not null;default:member"`
	DisabledAt   *time.Time `gorm:"index"`
}

// ClientProject links a client to the projects they may submit tickets into.
type ClientProject struct {
	ClientID   string    `gorm:"primaryKey;type:uuid"`
	ProjectID  string    `gorm:"primaryKey;type:uuid"`
	CreatedAt  time.Time `gorm:"autoMigration:false"`
}

// BeforeCreate fills a uuid v4 primary key so both dialects (PG via migration
// default, sqlite in tests) get ids from the app, not the DB.
func (u *User) BeforeCreate(_ *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	return nil
}

// Disabled reports whether the user is soft-disabled (blocks login/session).
func (u *User) Disabled() bool { return u.DisabledAt != nil }
