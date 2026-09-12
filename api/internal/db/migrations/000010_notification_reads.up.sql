-- Notifications watermark table (omitted from 000009 by mistake — that
-- migration only extended task_events). One row per user: last_read_at.
CREATE TABLE IF NOT EXISTS notification_reads (
    user_id      uuid PRIMARY KEY,
    last_read_at timestamptz NOT NULL
);
