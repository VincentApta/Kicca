# Tickets — kica

Single repo, single Claude Code agent, sequential batches. GitHub flow: Issue → Branch → PR → Review → Merge. No direct commits to main.

| # | Ticket | Scope | Depends |
|---|--------|-------|---------|
| T1 | Repo scaffold + Compose | monorepo layout, Dockerfile.api/.web, compose (api/web/postgres/caddy), .env.example, health endpoint, Vite+shadcn init w/ neumorphic tokens, CI build check | — |
| T2 | Auth + users | User model+migration, login/logout/me, JWT cookie middleware, seed admin, user CRUD + disable, api tests | T1 |
| T3 | Teams + projects | Team/TeamMember/Project/ProjectMember models+CRUD, visibility scoping, key uniqueness, permission matrix tests | T2 |
| T4 | Tasks core | Task/Label/Comment models, CRUD, task numbering, filters/list endpoint, soft-delete/restore, position math + rebalance, tests | T3 |
| T5 | Board UI | sidebar, project switcher, board columns + dnd-kit drag/move, card, detail drawer, list view + filters, empty/loading/error states, theme toggle | T1, T4 (mock API until ready) |
| T6 | Admin UI | users/teams/projects management screens, project members + labels settings | T5 |
| T7 | GitHub integration | GH config endpoint (AES-GCM encrypt PAT), create-issue service + endpoint, link display, idempotency, mocked upstream tests | T4 |
| T8 | Hardening + docs | permission e2e pass, seed demo script optional, README (run/deploy), Compose prod notes (Caddy TLS kica.apta.works), backup pg_dump note | T5, T6, T7 |

Rules: each ticket = one Issue + one PR; agent runs `go build/test` + `npm run build` before handoff; Hermes verifies independently before merge. Vault docs updated first if scope changes ([[PRD]] · [[api-contract]] · [[DESIGN]]).
