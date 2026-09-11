# API Contract — kica

REST, JSON bodies, all under `/api`. Auth = httpOnly cookie `kica_session` (JWT). See [[domain]], [[architecture]].

Errors: `{ "error": { "code": "string", "message": "string" } }`, proper status (400/401/403/404/409/422/500).

## Conventions
- ids: uuid v4 strings.
- enums lowercase snake_case: status `inbox|backlog|in_progress|review|done|blocked|trash`; priority `urgent|high|medium|low`; roles `admin|member|client` (global), `project_admin|member` (project).
- datetimes ISO-8601 UTC. Dates `YYYY-MM-DD`.
- pagination (list endpoints): `?page=1&per_page=50` → `{ data: [...], page, per_page, total }`.

## Endpoints

### Auth
| Method | Path | Body | Response | Notes |
|---|---|---|---|---|
| POST | /auth/login | `{email, password}` | 200 `{user}` + cookie | 401 invalid/disabled |
| POST | /auth/logout | — | 204 | clears cookie |
| GET | /auth/me | — | 200 `{user}` | |

`user` = `{id, email, name, global_role}`.

### Users (global admin)
| Method | Path | Body | Notes |
|---|---|---|---|
| GET | /users | ?page | list |
| POST | /users | `{email, name, password, global_role, project_ids?}` | 201 user; 409 email exists; client role requires ≥1 project_id |
| PATCH | /users/:id | `{name?, global_role?, password?, disabled?, project_ids?}` | project_ids (client only) replaces links; client→member drops them |

`user` (user-management responses) may add `project_ids: []` for client-role rows.

### Teams (global admin; members read)
| Method | Path | Body | Notes |
|---|---|---|---|
| GET | /teams | — | admin: all w/ member count; member: own teams |
| POST | /teams | `{name}` | 201; slug auto from name |
| PATCH | /teams/:id | `{name?}` | |
| DELETE | /teams/:id | — | 409 if projects exist |
| PUT | /teams/:id/members | `{user_ids: []}` | replace membership |

### Projects
| Method | Path | Body | Notes |
|---|---|---|---|
| GET | /projects | — | visible projects only: `{id, team_id, name, key, description}` |
| POST | /projects | `{team_id, name, key, description?}` | admin only; key unique, `^[A-Z][A-Z0-9]{1,9}$` |
| PATCH | /projects/:id | `{name?, description?, key?}` | admin |
| DELETE | /projects/:id | — | admin; soft cascade (sets deleted_at) |
| GET | /projects/:id | — | detail + my_role |
| PUT | /projects/:id/members | `[{user_id, role}]` | admin/project_admin; must keep ≥1 project_admin |
| PUT | /projects/:id/github | `{repo: "owner/name", token}` | admin/project_admin; token never returned |
| GET | /projects/:id/labels · POST | `{name, color}` · list | project_admin manages |
| DELETE | /labels/:id | — | |

### Tasks
| Method | Path | Body/Query | Notes |
|---|---|---|---|
| GET | /projects/:id/tasks | `?status=&assignee_id=&priority=&label=&q=&page=` (default excludes trash) | `{data, page, per_page, total}` |
| GET | /tasks | `?assignee_id=&page=&per_page=` (assignee optional) | global feed across visible projects; rows add `project_key`, `project_name` |
| GET | /tasks/events | `?days=30&project_id=` (days capped 90) | `{days, data: [{date, counts: {status: n}}]}` end-of-day status counts (#26) |
| POST | /projects/:id/tasks | `{title, description?, status?, priority?, type?, estimate?, assignee_id?, due_date?, label_ids?[]}` | 201 task; defaults status=backlog priority=medium type=task |
| GET | /tasks/:id | — | task + labels + gh_link + comments separate |
| PATCH | /tasks/:id | any task field incl. `status`, `position`, `type`, `estimate` (null clears) | member+ |
| DELETE | /tasks/:id | — | trash (soft); `?purge=1` admin hard delete |
| POST | /tasks/:id/restore | — | back to backlog |
| POST | /tasks/:id/move | `{status, before_task_id?, after_task_id?}` | server computes position; stamps analytics (rule 8) |
| GET/POST | /tasks/:id/comments | `{body}` | response rows add `author` (resolved display name) |
| GET/POST | /tasks/:id/attachments | multipart `file` (images png/jpeg/webp/gif, videos mp4/mov/webm) | type sniffed server-side; size capped ATTACHMENTS_MAX_MB (default 25MB); 422 otherwise |
| GET | /attachments/:id | — | streams blob (Content-Type sniffed, inline); team via project visibility, client via created_by |
| DELETE | /attachments/:id | — | team only (project visibility); 204 |

`task` = `{id, project_id, number, title, description, status, priority, type, estimate, assignee?: user|null, labels: [{id,name,color}], due_date, position, started_at, done_at, created_by, created_at, updated_at, gh_link?: {repo, issue_number, issue_url}|null}`.

`attachment` (team) = `{id, task_id, filename, content_type, size_bytes, created_by, created_at}`. Storage backend (S3 when S3_BUCKET set, else ATTACHMENTS_DIR local) never surfaces; object_key stays server-side. Issue creation from a task embeds S3 attachments as presigned URLs (images `![](url)`, videos bare URL; 24h). Local-storage attachments are skipped with a log line — GitHub has no public upload API.

Enums: `type` = `task|bug|feature|chore`. `estimate` = int >= 0 or null. `started_at`/`done_at` = server-stamped analytics timestamps (rule 8, not client-settable).

### GitHub
| Method | Path | Notes |
|---|---|---|
| POST | /tasks/:id/github/issue | creates GH issue from task (attachments embedded, see Tasks); 201 `{repo, issue_number, issue_url}`; 409 already linked; 422 project not configured; 502 upstream error |

### Client portal (client role only — team users get 403)
| Method | Path | Body | Notes |
|---|---|---|---|
| GET | /client/projects | — | linked projects: `{data: [{id, key, name}]}` |
| POST | /client/tickets | `{project_id, title, description}` | 201 ticket → task status=inbox, created_by=client, per-project numbering; unlinked project 404 (no leak) |
| GET | /client/tickets | ?q=&project_id=&status=&page | own tickets (created_by=me, linked projects), newest-updated first; q matches title/description (case-insensitive), project_id must be a linked project (unlinked → empty, no leak), status = `open` (not done) \| `closed` (done) (#47) |
| GET/POST | /client/tickets/:id/attachments | multipart `file` | own tickets only; listing/streaming limited to attachments the client created |
| GET/POST | /client/tickets/:id/comments | `{body}` (#43) | own tickets only; whole thread (team + client comments), oldest first |

`client ticket` = `{id, project_id, project_key, number, title, description, status, created_at, updated_at}` — assessment and other team-only fields are never serialized.

`client attachment` = `{id, filename, content_type, size_bytes, created_at}` — minimal fields only; created_by and task_id never serialize on the client surface.

`client comment` (#43) = `{id, body, created_at, user: {name}}` — author name only; user_id, email and every other user field never serialize on the client surface.

### Misc
| Method | Path | Notes |
|---|---|---|
| GET | /health | 200 `{status:"ok"}` |

## Status codes locked
401 unauth · 403 forbidden (not member/not admin) · 404 not found OR visible-deny (no leak) · 409 conflict (dup email/key, team w/ projects, task already linked) · 422 validation (also GH unconfigured).

## Auth/permission summary
- Every `/projects/:id/*` and `/tasks/*` route: membership or global admin check first, then role check.
- Client role: 403 on every team route (/users, /teams, /projects, /tasks, /labels); team users: 403 on /client/*. /auth/* stays open to all roles.
- GH token field never serialized in any response.
