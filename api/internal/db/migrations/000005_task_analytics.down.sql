DROP TABLE task_events;

ALTER TABLE tasks
    DROP COLUMN started_at,
    DROP COLUMN done_at,
    DROP COLUMN estimate,
    DROP COLUMN type;
