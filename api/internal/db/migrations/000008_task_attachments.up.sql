CREATE TABLE task_attachments (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id      uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    filename     text NOT NULL,
    content_type text NOT NULL,
    size_bytes   bigint NOT NULL,
    storage      text NOT NULL, -- 'local' | 's3' snapshot at upload time
    object_key   text NOT NULL,
    created_by   uuid NOT NULL REFERENCES users(id),
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_task_attachments_task_id ON task_attachments (task_id);
