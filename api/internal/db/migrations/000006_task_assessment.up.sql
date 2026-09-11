-- 000006: task assessment — triage note written when assessing an Inbox
-- ticket. Kept separate from description (the user's original words); GitHub
-- issues are created from the assessment when present.
ALTER TABLE tasks ADD COLUMN assessment TEXT NOT NULL DEFAULT '';
