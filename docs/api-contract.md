# API Contract — kicca

REST, JSON bodies, all under `/api`. Auth = httpOnly cookie `kicca_session` (JWT). See [[domain]], [[architecture]].

Errors: `{ "error": { "code": "string", "message": "string" } }`, proper status (400/401/403/404/409/422/500).

## Conventions
- ids: uuid v4 strings.
- enums lowercase snake_case: status `inbox|backlog|in_progress|review|done|blocked|trash`; priority `urgent|high|medium|low`; roles `admin|member` (global), `project_admin|member` (project).
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
| POST | /users | `{email, name, password, global_role}` | 201 user; 409 email exists |
| PATCH | /users/:id | `{name?, global_role?, password?, disabled?}` | |

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
| GET/POST | /tasks/:id/comments | `{body}` | |

`task` = `{id, project_id, number, title, description, status, priority, type, estimate, assignee?: user|null, labels: [{id,name,color}], due_date, position, started_at, done_at, created_by, created_at, updated_at, gh_link?: {repo, issue_number, issue_url}|null}`.

Enums: `type` = `task|bug|feature|chore`. `estimate` = int >= 0 or null. `started_at`/`done_at` = server-stamped analytics timestamps (rule 8, not client-settable).

### GitHub
| Method | Path | Notes |
|---|---|---|
| POST | /tasks/:id/github/issue | creates GH issue from task; 201 `{repo, issue_number, issue_url}`; 409 already linked; 422 project not configured; 502 upstream error |

### Misc
| Method | Path | Notes |
|---|---|---|
| GET | /health | 200 `{status:"ok"}` |

## Status codes locked
401 unauth · 403 forbidden (not member/not admin) · 404 not found OR visible-deny (no leak) · 409 conflict (dup email/key, team w/ projects, task already linked) · 422 validation (also GH unconfigured).

## Auth/permission summary
- Every `/projects/:id/*` and `/tasks/*` route: membership or global admin check first, then role check.
- GH token field never serialized in any response.
