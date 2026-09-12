-- Notifications (#notifications): task_events gains a type column so it can
-- carry assignment + comment events, not just status transitions. Existing
-- rows are status transitions (type = 'status').
ALTER TABLE task_events ADD COLUMN type text NOT NULL DEFAULT 'status';
ALTER TABLE task_events ADD COLUMN comment_id uuid REFERENCES comments(id) ON DELETE SET NULL;
CREATE INDEX idx_task_events_type ON task_events (type);