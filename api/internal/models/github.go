// GitHubIssueLink per docs/domain.md — one per task (unique task_id).
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type GitHubIssueLink struct {
	ID          string    `gorm:"primaryKey;type:uuid"`
	TaskID      string    `gorm:"not null;uniqueIndex;type:uuid"`
	Repo        string    `gorm:"not null"` // owner/name at link time
	IssueNumber int64     `gorm:"not null"`
	IssueURL    string    `gorm:"not null"`
	CreatedAt   time.Time
}

// BeforeCreate fills a uuid v4 primary key (app-side, both dialects).
func (l *GitHubIssueLink) BeforeCreate(_ *gorm.DB) error {
	if l.ID == "" {
		l.ID = uuid.NewString()
	}
	return nil
}
