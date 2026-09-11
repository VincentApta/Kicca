ALTER TABLE tasks
    ADD COLUMN started_at timestamptz,
    ADD COLUMN done_at timestamptz,
    ADD COLUMN estimate int,
    ADD COLUMN type text NOT NULL DEFAULT 'task';

CREATE TABLE task_events (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id     uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    actor_id    uuid NOT NULL REFERENCES users(id),
    from_status text,
    to_status   text NOT NULL,
    at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_task_events_task_id ON task_events (task_id);
CREATE INDEX idx_task_events_at ON task_events (at);
