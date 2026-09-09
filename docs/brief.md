# kicca — Project Brief

One-pager. Source of truth = this vault folder. [[PRD]] · [[DESIGN]] · [[architecture]] · [[domain]] · [[api-contract]] · [[tickets]]

## What
Internal task manager web app — Jira-like, self-hosted. Kanban board, multi-project, multi-team. Team members manage work; tasks can create linked GitHub issues (one-way out).

## Why
Team needs shared work tracking tailored to our agile flow (raw-request intake → delivery), company-wide potential. Off-the-shelf tools hard to extend with planned helpdesk intake.

## Future (explicitly not v1)
Helpdesk/ticketing intake: embedded public form on client apps → dumps into Inbox. Data model prepared (Inbox stage exists); no public surface in v1.

## Stack
- **Backend:** Go (Fiber + GORM), PostgreSQL, JWT httpOnly-cookie auth, bcrypt
- **Frontend:** React + TypeScript + Vite, Tailwind, shadcn/ui (Base UI), dnd-kit
- **Infra:** single repo (`kicca/` → `api/` + `web/`), Docker Compose (postgres, api, caddy)
- **Integrations:** GitHub REST API v3, PAT per project, AES-256-GCM encrypted at rest

## Hosting
Eventually `kicca.apta.works`. Local dev first. Name "kicca" provisional — may change.

## Constraints
- Auth: local email+password, admin-managed users (no OAuth v1)
- Fixed workflow (no custom stages): Inbox, Backlog, In Progress, Review, Done, Blocked, Trash(soft-delete)
- Roles: global Admin/Member + per-project admins (multiple allowed)
- All coding delegated to Claude Code; Hermes orchestrates, verifies, merges (Issue → Branch → PR → Merge, no direct commits to main)
