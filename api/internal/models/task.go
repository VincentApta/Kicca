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
	StartedAt   *time.Time // analytics: first move into in_progress/review
	DoneAt      *time.Time // analytics: latest move into done (cleared on reopen)
	Estimate    *int       // story points
	Type        string     `gorm:"not null;default:'task'"` // task | bug | feature | chore
	// Assessment: triage note on an Inbox ticket, written by the assessor.
	// GitHub issues are created from this (not description) when present.
	Assessment string `gorm:"not null;default:''"`
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

// TaskEvent: one row per status transition, NULL from_status = creation.
type TaskEvent struct {
	ID         string     `gorm:"primaryKey;type:uuid"`
	TaskID     string     `gorm:"not null;index;type:uuid"`
	ActorID    string     `gorm:"not null;type:uuid"`
	FromStatus *string
	ToStatus   string     `gorm:"not null"`
	At         time.Time
}

// TableName pins the SQL-migration name (same reason as GitHubIssueLink:
// GORM's default would not match migrations/000005_task_analytics.up.sql).
func (TaskEvent) TableName() string { return "task_events" }

// BeforeCreate fills a uuid v4 primary key (app-side, both dialects) and
// stamps At — GORM writes the column explicitly, so the SQL DEFAULT now()
// would never fire (and sqlite in tests has none).
func (e *TaskEvent) BeforeCreate(_ *gorm.DB) error {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	return nil
}

// TaskAttachment: one uploaded image/video per row (#34). Storage/object_key
// point at the storage.Store blob; storage snapshots the backend at upload
// time ('local' | 's3').
type TaskAttachment struct {
	ID          string `gorm:"primaryKey;type:uuid"`
	TaskID      string `gorm:"not null;index;type:uuid"`
	Filename    string `gorm:"not null"`
	ContentType string `gorm:"not null"`
	SizeBytes   int64  `gorm:"not null"`
	Storage     string `gorm:"not null"`
	ObjectKey   string `gorm:"not null"`
	CreatedBy   string `gorm:"not null;type:uuid"`
	CreatedAt   time.Time
}

// TableName pins the SQL-migration name (same reason as TaskEvent).
func (TaskAttachment) TableName() string { return "task_attachments" }

// BeforeCreate fills a uuid v4 primary key (app-side, both dialects).
func (a *TaskAttachment) BeforeCreate(_ *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	return nil
}
