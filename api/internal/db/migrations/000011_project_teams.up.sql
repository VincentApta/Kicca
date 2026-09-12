-- Multi-team projects: teams "contribute" to a project instead of one
-- owning team. team_id stays as the owning/creating team (back-compat,
-- NOT NULL); project_teams rows add contributing teams. Owner is seeded
-- into project_teams so the list is complete.
CREATE TABLE IF NOT EXISTS project_teams (
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    team_id    uuid NOT NULL REFERENCES teams(id)    ON DELETE CASCADE,
    PRIMARY KEY (project_id, team_id)
);
INSERT INTO project_teams (project_id, team_id)
SELECT id, team_id FROM projects
ON CONFLICT DO NOTHING;
