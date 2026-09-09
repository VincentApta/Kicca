CREATE TABLE teams (
    id   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL UNIQUE,
    slug text NOT NULL UNIQUE
);

CREATE TABLE team_members (
    team_id uuid NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (team_id, user_id)
);

CREATE INDEX idx_team_members_user_id ON team_members (user_id);

CREATE TYPE project_role AS ENUM ('project_admin', 'member');

CREATE TABLE projects (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id      uuid NOT NULL REFERENCES teams(id),
    name         text NOT NULL,
    key          text NOT NULL UNIQUE,
    description  text NOT NULL DEFAULT '',
    gh_repo      text,
    gh_token_enc bytea,
    task_seq     bigint NOT NULL DEFAULT 0,
    deleted_at   timestamptz
);

CREATE INDEX idx_projects_team_id ON projects (team_id);
CREATE INDEX idx_projects_deleted_at ON projects (deleted_at);

CREATE TABLE project_members (
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       project_role NOT NULL DEFAULT 'member',
    PRIMARY KEY (project_id, user_id)
);

CREATE INDEX idx_project_members_user_id ON project_members (user_id);
