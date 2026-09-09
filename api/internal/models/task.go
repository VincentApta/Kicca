// Task, Label, TaskLabel, Comment per docs/domain.md.
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Task: GORM soft delete = trash state (deleted_at set alongside
// status=trash, domain rule 4).
type Task struct {
	ID          string     `gorm:"primaryKey;type:uuid"`
	ProjectID   string     `gorm:"not null;type:uuid;uniqueIndex:idx_tasks_project_number"`
	Number      int64      `gorm:"not null;uniqueIndex:idx_tasks_project_number"`
	Title       string     `gorm:"not null"`
	Description string     `gorm:"not null;default:''"`
	Status      string     `gorm:"not null;default:backlog;index"`
	Priority    string     `gorm:"not null;default:medium"`
	AssigneeID  *string    `gorm:"type:uuid;index"`
	CreatedBy   string     `gorm:"not null;type:uuid"`
	DueDate     *time.Time `gorm:"type:date"`
	Position    float64    `gorm:"not null;default:0"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

// BeforeCreate fills a uuid v4 primary key (app-side, both dialects).
func (t *Task) BeforeCreate(_ *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	return nil
}

// Label: scoped per project, name unique within it.
type Label struct {
	ID        string `gorm:"primaryKey;type:uuid"`
	ProjectID string `gorm:"not null;type:uuid;uniqueIndex:idx_labels_project_name"`
	Name      string `gorm:"not null;uniqueIndex:idx_labels_project_name"`
	Color     string `gorm:"not null;default:''"`
}

// BeforeCreate fills a uuid v4 primary key (app-side, both dialects).
func (l *Label) BeforeCreate(_ *gorm.DB) error {
	if l.ID == "" {
		l.ID = uuid.NewString()
	}
	return nil
}

// TaskLabel: (task_id, label_id) m:n join.
type TaskLabel struct {
	TaskID  string `gorm:"primaryKey;type:uuid"`
	LabelID string `gorm:"primaryKey;type:uuid"`
}

// Comment: editable window 15 min `ponytail:` (domain).
type Comment struct {
	ID        string `gorm:"primaryKey;type:uuid"`
	TaskID    string `gorm:"not null;index;type:uuid"`
	UserID    string `gorm:"not null;type:uuid"`
	Body      string `gorm:"not null"`
	CreatedAt time.Time
}

// BeforeCreate fills a uuid v4 primary key (app-side, both dialects).
func (c *Comment) BeforeCreate(_ *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.NewString()
	}
	return nil
}
