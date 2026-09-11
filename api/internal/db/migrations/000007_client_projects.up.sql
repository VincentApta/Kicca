-- 000007: client role + client→project links
-- Clients are registered users who submit tickets into a project's Inbox.
-- They are NOT team members: they have no ProjectMember row, so every
-- team-scoped query (board, members, labels, GitHub) naturally excludes them.
-- global_role gains 'client' (existing column, no schema change).
CREATE TABLE IF NOT EXISTS client_projects (
  client_id   uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  project_id  uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  created_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (client_id, project_id)
);