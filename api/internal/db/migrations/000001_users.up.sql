CREATE EXTENSION IF NOT EXISTS citext;

CREATE TYPE global_role AS ENUM ('admin', 'member');

CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email         citext NOT NULL UNIQUE,
    password_hash text   NOT NULL,
    name          text   NOT NULL,
    global_role   global_role NOT NULL DEFAULT 'member',
    disabled_at   timestamptz
);

CREATE INDEX idx_users_disabled_at ON users (disabled_at);
