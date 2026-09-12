# Kica Fixes + Changes Batch Implementation Plan

> **For Hermes:** Implement task-by-task, commit per task. Small fixes direct to main; feature groups via branch → PR → squash.

**Goal:** Ship all FIX items (except GH PAT) and all CHANGE items from the 2026-09-12 gap analysis.

**Architecture:** Backend Go Fiber+GORM handlers per feature; frontend React components. Fixes are token/CSS-level; changes are one endpoint + one component each. Admin-pages split is a pure move-refactor behind a re-export barrel.

**Tech Stack:** Go 1.24, GORM, Postgres (user/db `kicca`), React + Vite + Tailwind v4 + shadcn Base UI, Docker Compose.

**Baseline:** main `13b5626`, 72/72 vitest, go suite green, stack live.

**Commands (every task):**
- Go: `cd api && export PATH=$PATH:/home/vincent/go-sdk/bin && go build ./... && go vet ./... && go test ./...`
- Web: `cd web && npx tsc -b && CI=true npx vitest run`
- Deploy: `cd ~/Projects/Kica && docker compose up -d --build` (background), then health `curl -s http://localhost/api/health`

---

## Phase 1 — FIXES (direct to main, one commit each)

### Task 1: Client-portal status chips font size

**Files:** Modify `web/src/components/client-portal.tsx` (~line 49, StatusChip)

`text-[10px] px-1.5` → `text-xs px-2` (match My Tasks pills from commit `13b5626`).

```tsx
className={`rounded-md border px-2 py-0.5 text-xs font-semibold ${STATUS_COLORS[status] ?? STATUS_COLORS.inbox}`}
```

Verify: tsc + vitest. Commit: `fix: client-portal status chips 10px → text-xs`.

### Task 2: Light-mode readable status/priority color tokens

**Problem:** `web/src/index.css` defines `--color-status-*` and `--color-priority-*` once in `:root` with dark-mode-bright hexes (#34d399, #f87171…). Used as **text** colors (`text-status-blocked`, `HEALTH_BADGE` in dashboard-page.tsx:181). Bright pastels on light bg = unreadable — same class of bug as the pills.

**Files:**
- Modify `web/src/index.css` `:root` block (~lines 48–58): darker light-mode values
- Modify `web/src/index.css` `.dark` block: add current bright values (they must MOVE here)

`:root` (light) new values:
```css
--color-priority-urgent: #b91c1c;   /* red-700 */
--color-priority-high:   #b45309;   /* amber-700 */
--color-priority-medium: #1d4ed8;   /* blue-700 */
--color-priority-low:    #475569;   /* slate-600 */
--color-status-done:        #047857; /* emerald-700 */
--color-status-blocked:     #b91c1c; /* red-700 */
--color-status-in-progress: #1d4ed8; /* blue-700 */
--color-status-review:      #6d28d9; /* violet-700 */
--color-status-inbox:       #b45309; /* amber-700 */
--color-status-backlog:     #475569; /* slate-600 */
```

`.dark` block additions (current bright values move here):
```css
--color-priority-urgent: #f87171;
--color-priority-high:   #fbbf24;
--color-priority-medium: #60a5fa;
--color-priority-low:    #94a3b8;
--color-status-done:        #34d399;
--color-status-blocked:     #f87171;
--color-status-in-progress: #60a5fa;
--color-status-review:      #c084fc;
--color-status-inbox:       #fbbf24;
--color-status-backlog:     #97a0af;
```

**Audit pass (same commit):** `grep -rn "text-status-\|text-priority-" web/src/components/` — every hit now themes correctly automatically (tokens). Also check `project-analytics.tsx` and `task-drawer.tsx` for hardcoded `text-*-400` on light surfaces; convert hits to tokens or add `dark:` variants like commit `ab8c89e` did.

Verify: tsc + vitest; manual — toggle theme, check dashboard "needs attention" + analytics page.
Commit: `fix: status/priority color tokens readable in light mode`.

### Task 3: client1 password drift — diagnose + harden

**Problem:** client1@example.com login 401 after some deploys; password reset by admin PATCH twice this week. `SeedAdmin` (db.go:55) is create-only-if-empty, so seeding is NOT the cause.

**Diagnose (read-only):**
```bash
docker exec kica-postgres-1 psql -U kicca -d kicca -c \
  "SELECT email, updated_at, created_at FROM users WHERE email='client1@example.com';"
```
Compare `updated_at` vs last deploy time. If `updated_at` bumps on deploy → something writes it; grep `Save(&user)` / `Update.*user` in boot path. If not → drift came from manual resets only; nothing to fix in code.

**Harden regardless (small):** extend optional seeding so dev/staging always has a known client. `api/internal/db/db.go`:
```go
// SeedClient creates an optional dev client from CLIENT_EMAIL/CLIENT_PASSWORD
// iff that email does not exist yet. Never overwrites — password resets go
// through admin PATCH. ponytail: drop when real client onboarding exists.
func SeedClient(gdb *gorm.DB, email, password string) error {
    if email == "" || password == "" { return nil }
    var n int64
    gdb.Model(&models.User{}).Where("email = ?", strings.ToLower(email)).Count(&n)
    if n > 0 { return nil }
    hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil { return err }
    c := &models.User{Email: strings.ToLower(email), PasswordHash: string(hash),
        Name: strings.SplitN(email, "@", 2)[0], GlobalRole: "client"}
    if err := gdb.Create(c).Error; err != nil { return fmt.Errorf("seed client: %w", err) }
    log.Printf("seed: created client %s", c.Email)
    return nil
}
```
Wire in `api/cmd/server/main.go` next to SeedAdmin (read `CLIENT_EMAIL`/`CLIENT_PASSWORD`, add to `.env.example` if present). Test in `db` package or skip (thin wrapper over tested pattern) — one test: second call is a no-op.

Commit: `fix: optional CLIENT seed (create-only), diagnose client1 drift`.

**STOP — report diagnosis to Vincent before Phase 2.**

---

## Phase 2 — CHANGE: split admin-pages.tsx (refactor, branch `refactor-admin-pages` → PR)

### Task 4: Extract shared shell + split into 4 files

**Files:**
- Create `web/src/components/admin/shared.tsx` — move `PageShell`, `Field`, `RowSkeletons` (admin-pages.tsx:74–159) + any shared imports/types. Export all three.
- Create `web/src/components/admin/users.tsx` — move `UsersPage` (160), `ProjectLinksPicker` (350), `CreateUserDialog` (393), `EditUserDialog` (540). Import shell parts from `./shared`.
- Create `web/src/components/admin/teams.tsx` — move `TeamsPage` (702), `TeamDialog` (814), `TeamMembersDialog` (901).
- Create `web/src/components/admin/project-settings.tsx` — move `ProjectSettingsPage` (1201), `GeneralTab` (1264), `MembersTab` (1419), `LabelsTab` (1574), `FirstProjectDialog` (1022).
- Rewrite `web/src/components/admin-pages.tsx` as re-export barrel:
```tsx
export { UsersPage } from './admin/users'
export { TeamsPage } from './admin/teams'
export { ProjectSettingsPage, FirstProjectDialog } from './admin/project-settings'
```
Check what workspace.tsx/admin-pages consumers actually import first (`grep -rn "from './admin-pages'" web/src`) — barrel keeps them untouched; if only 3–4 imports, update them directly and delete barrel instead.

**Rules:** pure move — zero logic edits. Shared state/props crossing files = props or move down. Watch circulars: dialogs stay co-located with their page.

Verify: tsc, 72/72 vitest, `grep -c "useState" web/src/components/admin-pages.tsx` → 0 or file gone.
Commit + PR: `refactor: split admin-pages (1853 lines) into admin/ modules`. CI green → squash merge → delete branch.

---

## Phase 3 — CHANGE: MyTasks pagination (branch `feat-mytasks-pagination` → PR, can pair with Task 6)

### Task 5: True count + Show more

Backend already returns `Paginated<Task>` with `meta.total`; frontend hides it.

**Files:** Modify `web/src/components/my-tasks-page.tsx`:
- State: `const [page, setPage] = useState(1)`, `const [total, setTotal] = useState(0)`
- Fetch: `api.myFetchMyTasks(state.user.id, page)` → `setTasks(prev => page === 1 ? res.data : [...prev, ...res.data])`; `setTotal(res.meta.total)`
- Header: `{total} task{total !== 1 && 's'}` (was tasks.length)
- After list: `{tasks.length < total && <Button variant="outline" onClick={() => setPage(p => p + 1)}>Show more ({total - tasks.length} left)</Button>}`
- Reset page to 1 when `state.user.id` changes (effect dep already there — add setPage(1)).

Check `Paginated<T>` shape in `web/src/lib/types.ts` for exact `meta` fields before coding.

Verify: tsc + vitest; if no test exists for MyTasksPage rendering, none required (display-only change).
Commit: `feat: my-tasks true total count + show more pagination`.

### Task 6: Export date range

**Files:**
- Modify `api/internal/handlers/export.go` — both `ExportTasks` + `ClientExportTickets`: parse `from`,`to` query (`2006-01-02`), invalid → 400. Filter: `from` → `created_at >= from 00:00`, `to` → `created_at < to+1d`. Empty = current behavior.
- Test `api/internal/handlers/export_test.go` (or routes_test): create 2 tasks, different `created_at` (raw SQL update), export `?from=<d>` → 1 row; `?to=` → other row; invalid → 400.
- Modify `web/src/lib/api.ts` `downloadCsv` (+client variant): optional `{from?, to?}` → query string.
- Modify `web/src/components/export-buttons.tsx`: small Popover (existing Base UI pattern) with two `<input type="date">` + Export button passing values. Team + client variants.

Commit: `feat: CSV export date range (from/to)`. Same PR as Task 5 if both ready; else own PR.

---

## Phase 4 — CHANGE: Global search (branch `feat-global-search` → PR)

### Task 7: Backend search endpoint

**Files:**
- Create `api/internal/handlers/search.go`:
```go
// GET /api/search?q= — team-wide. Tasks: title/description ILIKE across
// projects visible to the caller (admin: all; member: project_member rows),
// not soft-deleted, newest first, limit 20. Projects: name/key ILIKE, limit 10.
```
Shape:
```go
type searchTask struct { ID, ProjectID, ProjectKey, Title, Status string; Number int64 }
type searchOut struct { Tasks []searchTask `json:"tasks"`; Projects []searchOutProject `json:"projects"` }
```
SQL: one query tasks join projects (+ `project_members` existence subquery when not admin), one projects. `q` trimmed; len<2 → empty result, 200. Min 2 chars guard avoids index chatter.
- Route `api/internal/handlers/router.go`: `api.Get("/search", Search(gdb))` inside RequireTeam group (team-only v1; client portal already has ticket search).
- Test `routes_test.go` `TestSearch`: admin finds task by title fragment in another project; member without membership gets 0; q<2 → empty.

Commit: `feat: GET /api/search — tasks + projects`.

### Task 8: Frontend search dropdown

**Files:**
- `web/src/lib/api.ts` + `types.ts`: `search(q)` → `SearchResult` type.
- Modify `web/src/components/topbar.tsx`: when `search.trim().length >= 2` and view is board/list — debounce 250ms (`useRef` timer), call `api.search`, render dropdown under input (absolute, z-50, same styling as notification popover in `notifications.tsx`): Projects section (click → needs project switch — call `onOpenTask(taskId, projectId)` which already routes) + Tasks rows (project_key #number — title — status dot). Click outside closes. Escape closes. **Keep existing filter behavior** — dropdown is additive; Enter still applies board filter.
- Workspace already passes `onOpenNotification` (task nav) into Topbar — reuse that prop for search clicks (rename optional; reuse fine).

Verify: tsc + vitest (add render test if cheap: type ≥2 chars → mocked api.search returns row → row visible; else manual).
Commit: `feat: global search dropdown in topbar`.

---

## Phase 5 — CHANGE: Notifications SSE (branch `feat-notif-sse` → PR, riskiest — last)

### Task 9: SSE stream endpoint + EventSource client

**Design:** server-tick SSE — single goroutine per connection polls the SAME `ListNotifications` relevance query every 10s, pushes `{count, items}` only on change (hash by latest event id + count). No write-path hooks, no pubsub map. Clients drop polling.

**Files:**
- Create `api/internal/handlers/notifications_stream.go`:
```go
// GET /api/notifications/stream — SSE. Sends unread {count, items} on connect
// and whenever they change (10s server tick). Replaces client polling.
// ponytail: pub/sub push (write-path hooks + per-user channels) when >20 users.
```
Fiber pattern: set `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`, disable compression; loop `select { case <-c.Context().Done(): return; case <-tick.C: ... }`; write `data: %s\n\n` via `c.Context().Write`/flush. Copy relevance SQL from `notifications.go` — extract shared helper `fetchUnread(gdb, user) []notifJSON` used by both GET and stream (DRY).
- Route: inside existing notifications group (cookie-auth works for EventSource — same-origin).
- Modify `web/src/components/notifications.tsx`: replace `setInterval` with `new EventSource('/api/notifications')`… note base URL — reuse `api.ts` base (check how `req` builds URLs; EventSource needs absolute `/api/notifications/stream`). `onmessage` → setItems. `onerror` → close + **fallback to 30s poll** (keep old code path as fallback function).
- Docker/Caddy: verify streaming not buffered. If chunks arrive batched, add Caddy `flush_interval -1` to the api reverse_proxy block in `Caddyfile`. Test: `curl -N http://localhost/api/notifications/stream` (with cookie jar) — expect immediate `data:` line.
- Keep `GET /api/notifications` endpoint (used on mount + fallback).

Test: Go — reuse TestNotifications fixtures; stream test optional (hard with httptest; assert headers only if cheap). Manual verify primary.
Commit: `feat: SSE notifications stream, poll as fallback`.

---

## Validation (every phase end)

1. `go build ./... && go vet ./... && go test ./...` — green
2. `npx tsc -b` clean, `CI=true npx vitest run` — 72+ passing (new tests add)
3. Deploy once per merged PR: `docker compose up -d --build`
4. Live smoke: login admin + client1, bell loads, board loads, export with range returns filtered CSV, topbar search finds cross-project task
5. `git log --oneline` — squash merges only, no stray branches (`git branch --merged | grep -v main`)

## Risks / Trade-offs

- **Admin split (Task 4):** zero-diff enforcement is manual — review PR with `git diff --stat` only (moves show as rewrite; accept, verify by `git show -M` rename detection).
- **SSE (Task 9):** proxy buffering is the classic failure — flush test before wiring UI. Fallback poll keeps worst case = today's behavior.
- **Global search member scoping:** project_members subquery must match `loadVisibleProject` semantics exactly — copy its predicate, don't reinvent.
- **client1 (Task 3):** may be a non-bug (manual resets only). Diagnosis decides; seed hardening lands either way.
- **No DB migrations in this batch** — worst case SSE needs none; all queries use existing columns.

## Open Questions (answer during Phase 1 stop)

1. client1 diagnosis result — fix-forward or document?
2. Export range UI: popover OK, or prefer dialog?
3. Search: include comments in v1? (Plan: no — tasks+projects only.)
