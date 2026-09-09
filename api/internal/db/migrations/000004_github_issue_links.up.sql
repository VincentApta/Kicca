CREATE TABLE github_issue_links (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id      uuid NOT NULL UNIQUE REFERENCES tasks(id) ON DELETE CASCADE,
    repo         text NOT NULL,
    issue_number bigint NOT NULL,
    issue_url    text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now()
);
