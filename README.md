# kicca

Internal task manager. Kanban board, teams/projects, GitHub issue push.
Docs: [`docs/PRD.md`](docs/PRD.md) · [`docs/architecture.md`](docs/architecture.md) · [`docs/DESIGN.md`](docs/DESIGN.md)

## Stack

- **api/** — Go Fiber, env config, `GET /api/health`
- **web/** — Vite + React + TS, Tailwind v4, shadcn/ui (Base UI primitives), neumorphic dark-default theme
- **postgres:17**, **caddy:2** via compose

## Run (compose)

```bash
cp .env.example .env   # edit secrets
docker compose up --build
```

- app: http://localhost
- api health: http://localhost/api/health

## Run (bare metal, dev)

```bash
# api — terminal 1 (needs postgres running, see DATABASE_URL in .env)
cd api && go run ./cmd/server

# web — terminal 2; proxy target defaults to the compose service name,
# so point it at localhost
cd web && VITE_API_PROXY_TARGET=http://localhost:8080 npm run dev
```

## CI

`.github/workflows/ci.yml` — `go build`/`go vet` for api, `npm run build` for web.
