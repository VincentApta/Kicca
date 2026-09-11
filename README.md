# kica

Internal task manager: kanban board, teams/projects, GitHub issue push.

Docs: [`docs/PRD.md`](docs/PRD.md) · [`docs/domain.md`](docs/domain.md) · [`docs/api-contract.md`](docs/api-contract.md) · [`docs/architecture.md`](docs/architecture.md) · [`docs/DESIGN.md`](docs/DESIGN.md)

## Stack

- **api/** — Go Fiber + GORM, Postgres 17, golang-migrate SQL migrations, JWT session cookie
- **web/** — Vite + React + TS, Tailwind v4, shadcn/ui (Base UI primitives), neumorphic dark theme
- **caddy:2** edge reverse proxy (compose)

## Quick start (docker compose)

```bash
cp .env.example .env          # then edit the secrets (see table below)
docker compose up --build -d
```

- app: http://localhost
- api health: http://localhost/api/health

### First login

On first boot with an empty database the api seeds one global admin from
`ADMIN_EMAIL` / `ADMIN_PASSWORD` (log line `seed: created admin …`). Log in
with those credentials, then create real users under **Admin → Users** and
change the seed password (or disable once you have a second admin — the api
refuses to demote/disable the last one).

## Environment variables

| Var | Required | Default (compose) | Notes |
|---|---|---|---|
| `DATABASE_URL` | yes | `postgres://kica:${POSTGRES_PASSWORD}@postgres:5432/kica?sslmode=disable` | bare-metal runs point at `localhost` |
| `JWT_SECRET` | yes | `dev-secret-change-me` | **≥ 16 chars**, the api refuses to boot otherwise; use `openssl rand -hex 32` |
| `ADMIN_EMAIL` / `ADMIN_PASSWORD` | first boot | `admin@example.com` / `admin-change-me` | seed admin, only used while the users table is empty |
| `GH_ENC_KEY` | for GitHub | empty (feature off) | 32 bytes base64 (`openssl rand -base64 32`), AES-256-GCM key for stored GitHub PATs; invalid base64/wrong length = boot failure |
| `GITHUB_API_BASE` | no | `https://api.github.com` | override for tests |
| `POSTGRES_PASSWORD` | yes | `kica` | compose postgres credential |
| `HTTP_PORT` | no | `80` | host port Caddy binds |
| `PORT` (api only) | no | `8080` | api listen port |
| `VITE_API_PROXY_TARGET` | web dev only | `http://api:8080` | vite dev proxy; set to `http://localhost:8080` when the api runs outside compose |

The api also logs a startup line with the port and applied-migration count.

## Dev workflow

Trunk-based with short-lived branches — commit history is the audit log
(`T4: …` messages map to tickets in `docs/tickets.md`):

```bash
git checkout -b t9-thing main
# … work, keeping "go build/vet/test" (api) and "npm run build && npx vitest run" (web) green
git push -u origin t9-thing   # then open a PR to main; CI runs both suites
```

CI (`.github/workflows/ci.yml`): `go build`/`go vet`/`go test` for api,
`npm run build` + `vitest` for web.

### Bare metal (dev)

```bash
# api — terminal 1 (needs postgres; DATABASE_URL in .env points at localhost)
cd api && go run ./cmd/server

# web — terminal 2
cd web && VITE_API_PROXY_TARGET=http://localhost:8080 npm run dev
```

Tests: `cd api && go test ./...` · `cd web && npx vitest run`.

## Deploy notes (kica.apta.works)

Single-host compose behind Caddy. TLS is automatic once the site address and
DNS are real — swap the edge listener in `deploy/Caddyfile`:

```caddyfile
kica.apta.works {
	handle /api/* {
		reverse_proxy api:8080
	}
	handle {
		reverse_proxy web:80
	}
}
```

Then: point an A/AAAA record for `kica.apta.works` at the host, publish 443
(`HTTP_PORT` covers only the :80 dev default — expose `443:443` for prod), and
let Caddy manage ACME certs (its data volume must persist). Behind the proxy
the login rate limiter keys on the proxy IP for all clients — fine for a small
internal team; set trusted-proxy handling before scaling.

### Backups (pg_dump cron)

Nightly dump to the host, keep 14 days (`docker compose exec` + host cron —
`crontab -e`):

```cron
15 3 * * * docker exec kica-postgres-1 pg_dump -U kica kica | gzip > /var/backups/kica/kica-$(date +\%F).sql.gz && find /var/backups/kica -name 'kica-*.sql.gz' -mtime +14 -delete
```

(The postgres container name comes from `docker compose ps`; create
`/var/backups/kica` first. `GH_ENC_KEY` must stay constant across restores or
stored PATs become undecryptable — back it up with the same care as the DB.)
