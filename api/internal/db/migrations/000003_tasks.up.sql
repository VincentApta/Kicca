CREATE TYPE task_status AS ENUM ('inbox', 'backlog', 'in_progress', 'review', 'done', 'blocked', 'trash');
CREATE TYPE task_priority AS ENUM ('urgent', 'high', 'medium', 'low');

CREATE TABLE tasks (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  uuid NOT NULL REFERENCES projects(id),
    number      bigint NOT NULL,
    title       text NOT NULL,
    description text NOT NULL DEFAULT '',
    status      task_status NOT NULL DEFAULT 'backlog',
    priority    task_priority NOT NULL DEFAULT 'medium',
    assignee_id uuid REFERENCES users(id),
    created_by  uuid NOT NULL REFERENCES users(id),
    due_date    date,
    position    double precision NOT NULL DEFAULT 0,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz,
    UNIQUE (project_id, number)
);

CREATE INDEX idx_tasks_deleted_at ON tasks (deleted_at);
CREATE INDEX idx_tasks_status ON tasks (status);

CREATE TABLE labels (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name       text NOT NULL,
    color      text NOT NULL DEFAULT '',
    UNIQUE (project_id, name)
);

CREATE TABLE task_labels (
    task_id  uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    label_id uuid NOT NULL REFERENCES labels(id) ON DELETE CASCADE,
    PRIMARY KEY (task_id, label_id)
);

CREATE INDEX idx_task_labels_label_id ON task_labels (label_id);

CREATE TABLE comments (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id    uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users(id),
    body       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_comments_task_id ON comments (task_id);
