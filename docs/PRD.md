# PRD — kicca

Internal task manager for the company. v1 = team task management. See [[brief]] for one-pager, [[DESIGN]] for UI, [[domain]] for entities.

## Goals
1. One internal tool where teams track work across multiple app projects.
2. Kanban board with a fixed agile workflow including Inbox (raw-request dump) for future helpdesk intake.
3. Create GitHub issues from tasks, store the link.
4. Company-wide ready: org has many teams, each team has many projects.

## Non-goals (v1)
- Public/embedded ticket submission form (helpdesk phase — later)
- Email-to-ticket ingestion
- Two-way GitHub sync, webhook imports, issue→task
- Notifications (in-app/email), sprints, Gantt, time tracking, reports
- Custom workflows/stages per project
- Mobile app (responsive web only)

## Personas
| Persona | Scope | Can |
|---|---|---|
| Admin | global | manage users, teams, projects, GitHub config; everything below |
| Project admin | per project (multiple allowed) | manage project members + labels; all task ops |
| Member | per project membership | view/create/edit tasks, comment |

## Workflow (locked)
```
Inbox → Backlog → In Progress → Review → Done
                    ↕ Blocked (side column; unblock = move manually, no state memory)
Trash: soft-delete from any column; restorable
```
- **Inbox** = raw, unassessed requests. Assessing = edit + move to Backlog.
- Any column reachable from any other (drag free-form, no enforcement).
- **Blocked** is a regular column, not a flag. `ponytail:` no previous-state memory; upgrade path: store `blocked_from` if teams complain.

## Task fields (locked)
Title, description (markdown), assignee (0..1), priority (Urgent/High/Medium/Low), labels (0..n, per project), due date (optional), per-project number (`KEY-123`), board position.

## User stories
### Auth & users
- US-1 As admin, I log in with email+password (JWT httpOnly cookie).
- US-2 As admin, I create/disable users, reset passwords, set global role.
- US-3 First run seeds one admin from env vars.
### Teams & projects
- US-4 As admin, I create teams and add members.
- US-5 As admin, I create projects under a team, set project key + GitHub repo mapping.
- US-6 As admin, I add project admins (multiple) and members.
- US-7 As member, I see only projects I'm a member of.
### Board & tasks
- US-8 As member, I view a project kanban board (columns per workflow above) with drag-and-drop cards.
- US-9 As member, I create a task in any column, edit fields inline/detail drawer.
- US-10 As member, I filter/search tasks: text, assignee, priority, label, status; list view alternative to board.
- US-11 As member, I comment on tasks.
- US-12 As member, I soft-delete (trash) and restore tasks; trashed hidden from board.
### GitHub
- US-13 As project admin, I store GitHub PAT + repo (owner/name) per project; PAT encrypted at rest, never returned by API.
- US-14 As member, I push a task to GitHub → issue created (title + body incl. description), link `#number` + URL stored and shown on card/detail. Idempotent: already-linked task errors clearly, no duplicate issue.
### Housekeeping
- US-15 As member, dark theme default; toggle light persists.

## Acceptance criteria (release gate)
- Fresh `docker compose up` → seeded admin → login → create team/project/task → drag across columns → persisted after restart.
- Board ordering stable and correct after arbitrary drags (fractional position).
- Task → GitHub issue works with real PAT against a real repo; link visible.
- Non-member gets 403/404 on project resources; member cannot manage users/teams.
- Keyboard: card menu + move actions reachable without mouse (DnD is pointer-only, acceptable v1).
- WCAG AA text contrast in both themes (neumorphic surfaces must not eat contrast).

## Open questions (decide at build time, not blocking)
- Pagination style for very long Backlog columns (v1: simple "load more").
- Task archive vs delete-purge (v1: trash only, no purge job).
