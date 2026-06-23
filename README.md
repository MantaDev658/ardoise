# Ardoise

An open-source expense splitting app. Track shared costs, manage groups, simplify debts, and settle up. Use the hosted version, or — since it's open source — run your own copy.

> _"Ardoise" is French slang for a running tab — the slate a café keeps of what you owe._

[![CI](https://github.com/MantaDev658/ardoise/actions/workflows/ci.yml/badge.svg)](https://github.com/MantaDev658/ardoise/actions/workflows/ci.yml)
[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8.svg?logo=go&logoColor=white)](apps/backend/go.mod)
[![SvelteKit](https://img.shields.io/badge/SvelteKit-2-FF3E00.svg?logo=svelte&logoColor=white)](apps/frontend/package.json)

---

## Contents

- [Features](#features)
- [Architecture](#architecture)
- [Prerequisites](#prerequisites)
- [Getting Started](#getting-started)
- [API Reference](#api-reference)
- [Split Strategies](#split-strategies)
- [Frontend](#frontend)
- [CSV CLI](#csv-cli)
- [Development](#development)
- [Deployment](#deployment)
- [Contributing](#contributing)
- [License](#license)

---

## Features

- **JWT authentication** — stateless auth via signed tokens; no session storage
- **Group management** — create groups, add/remove members with outstanding-balance guards
- **Four split strategies** — even, exact, percentage, and proportional shares
- **Penny-perfect math** — a custom `money` engine distributes remainders deterministically; no floating-point drift
- **Debt simplification** — greedy algorithm reduces N bilateral debts to the minimum number of transactions
- **Cursor-based pagination** — all list endpoints paginated with `?limit` and `?cursor`
- **Retro UI** — Windows 95 design system: beveled borders, navy title bars, tiled desktop background
- **CSV CLI** — standalone binary for offline expense parsing from a ledger file

---

## Architecture

Go workspace monorepo. Two modules, two binaries, one SvelteKit SPA.

```
ardoise/
├── apps/backend/
│   ├── cmd/api/            # REST API server
│   ├── cmd/cli/            # CSV expense parser (standalone binary)
│   └── internal/core/
│       ├── domain/         # Entities, allocation logic, error types
│       ├── application/    # Use-case services
│       └── infrastructure/
│           ├── http/       # Handlers split by service, CORS + auth middleware
│           ├── postgres/   # Repository implementations, migrations
│           └── csv/        # CSV ledger parser
├── apps/frontend/
│   ├── src/
│   │   ├── lib/
│   │   │   ├── api/        # Typed fetch wrappers (expenses, groups, users, settlements)
│   │   │   ├── components/ # Win95 component library (Window, Button, Input, ...)
│   │   │   └── stores/     # Auth store (localStorage), toast notifications
│   │   └── routes/         # SvelteKit file-based routing
│   ├── svelte.config.js    # adapter-static — outputs plain static files
│   └── vite.config.ts      # /api proxy for local dev
├── libs/shared/money/      # Zero-dependency money type (int64 cents)
└── go.work                 # Go workspace linking both modules
```

Domain layer has no external dependencies. Infrastructure depends on domain, never the reverse.

---

## Prerequisites

**Backend**
- Go 1.26+
- Docker (for the local PostgreSQL instance)
- `make`
- `golang-migrate`: `make setup-migrate`
- `golangci-lint`: `make setup-lint`

**Frontend**
- [Bun](https://bun.sh) — `curl -fsSL https://bun.sh/install | bash`

---

## Getting Started

```bash
git clone https://github.com/MantaDev658/ardoise.git
cd ardoise

# Install frontend dependencies
make frontend-install

# Start everything (Postgres + migrations + API + frontend dev server)
make dev
```

`make dev` starts Postgres, applies any pending migrations, and runs the API
(`localhost:8080`) and frontend dev server (`localhost:5173`) concurrently.
Ctrl+C stops both.

### Environment variables

Create a `.env` file in the repo root (loaded automatically by the API):

| Variable | Required | Description |
|---|---|---|
| `DATABASE_URL` | Yes | PostgreSQL connection string |
| `JWT_SECRET` | Yes | Secret used to sign and verify JWT tokens |
| `CORS_ORIGIN` | No | Allowed CORS origin (default `*`; lock to your frontend URL in prod) |
| `PORT` | No | API listen port (default `8080`) |

Example `.env`:
```
DATABASE_URL=postgresql://postgres:password@localhost:5432/ardoise?sslmode=disable
JWT_SECRET=change-me-in-production
CORS_ORIGIN=http://localhost:5173
```

---

## API Reference

All endpoints except `/auth/*` require `Authorization: Bearer <token>`.

Currency amounts are always in **cents** (integer). `$10.00` = `1000`.

Paginated endpoints accept `?limit=N` (default 20, max 100) and `?cursor=<RFC3339Nano>`. Responses include `"next_cursor"` when more pages exist.

### Auth

| Method | Path | Description |
|---|---|---|
| POST | `/auth/register` | Create a user account |
| POST | `/auth/login` | Authenticate and receive a JWT |

### Expenses

| Method | Path | Description |
|---|---|---|
| POST | `/expenses` | Create an expense; payer is taken from the JWT |
| GET | `/expenses` | List expenses; filter by `?group_id=` |
| PUT | `/expenses/{id}` | Update an expense |
| DELETE | `/expenses/{id}` | Delete an expense |

### Balances & Settlements

| Method | Path | Description |
|---|---|---|
| GET | `/balances` | Net balances and suggested settlements; filter by `?group_id=` |
| GET | `/friends/{user_id}/balances` | Bilateral friend balances for the authenticated user |
| POST | `/settlements` | Record a debt payment between two users |

### Users

| Method | Path | Description |
|---|---|---|
| GET | `/users` | List all users |
| PUT | `/users/{id}` | Update display name |
| DELETE | `/users/{id}` | Soft-delete a user |

### Groups

| Method | Path | Description |
|---|---|---|
| POST | `/groups` | Create a group; creator is taken from the JWT |
| GET | `/groups` | List groups for the authenticated user |
| PUT | `/groups/{id}` | Rename a group |
| DELETE | `/groups/{id}` | Delete a group |
| POST | `/groups/{id}/members` | Add a member |
| DELETE | `/groups/{id}/members/{user_id}` | Remove a member (blocked if outstanding balance) |

### Request bodies

**POST /auth/register**
```json
{ "id": "alice", "display_name": "Alice", "password": "..." }
```

**POST /expenses**
```json
{
  "group_id": "optional-group-uuid",
  "description": "Dinner",
  "total_cents": 9000,
  "split_type": "EVEN",
  "splits": [{ "user_id": "alice" }, { "user_id": "bob" }]
}
```

**POST /settlements**
```json
{ "receiver_id": "bob", "amount_cents": 5000, "group_id": "optional" }
```

---

## Split Strategies

| Strategy | `split_type` | `splits` format |
|---|---|---|
| Even | `EVEN` | `[{"user_id": "alice"}, ...]` — total divided equally; remainders go to first |
| Exact | `EXACT` | `[{"user_id": "alice", "value": 2000}, ...]` — values must sum to `total_cents` |
| Percentage | `PERCENT` | `[{"user_id": "alice", "value": 25}, ...]` — values must sum to 100 |
| Shares | `SHARES` | `[{"user_id": "alice", "value": 2}, ...]` — proportional to share counts |

---

## Frontend

The frontend is a SvelteKit SPA built with Tailwind CSS and a Windows 95 design system. It compiles to static files (`apps/frontend/build/`) and can be served from any static host.

### Component library

| Component | Description |
|---|---|
| `Window` | Win95 window frame with navy gradient title bar |
| `Button` | Beveled button — variants: default, primary, danger, success |
| `Input` | Inset sunken text input |
| `Select` | Inset dropdown |
| `HRule` | 3D groove horizontal rule |
| `Marquee` | CSS-only scrolling announcement bar |
| `HitCounter` | Black/green monospace stat display |
| `Badge` | Pulsing label — variants: red, green, yellow, navy |
| `Toast` | Fixed system-tray notifications with auto-dismiss |
| `Nav` | Taskbar-style navigation bar with logout |

### Frontend development commands

```bash
make frontend-install   # install Bun dependencies
make frontend-dev       # start Vite dev server (localhost:5173)
make frontend-build     # type-check (svelte-check) + production build
make frontend-test      # run unit tests with Bun
make dev                # start everything (recommended)
```

The dev server proxies `/api/*` → `http://localhost:8080` to avoid CORS in development.

---

## CSV CLI

Parse a CSV ledger file offline and print settle-up instructions:

```bash
make run-cli
# or:
cd apps/backend && go run cmd/cli/main.go -file=../../test_expenses.csv
```

CSV format — one expense per row, minimum 7 columns:

```
Date,Description,Category,TotalCents,Payer,Strategy,Participants...
2026-04-01,Dinner,Food,9000,Alice,EVEN,Alice,Bob,Charlie
2026-04-02,Hotel,Lodging,30000,Charlie,EXACT,Alice:10000,Bob:10000,Charlie:10000
```

---

## Development

```bash
make check             # build + lint (run before every commit)
make test-unit         # unit tests with coverage
make test-race         # race detector
make test-fuzz         # 30s fuzz run on domain package
make test-integration  # postgres integration tests (requires TEST_DB_URL)
make frontend-test     # frontend unit tests
make migrate-up        # apply pending migrations
make migrate-down      # roll back last migration
```

CI runs the backend and frontend jobs in parallel. Both must pass before merging.
The linter (`golangci-lint`) enforces doc comments, error wrapping, and a no-shadow rule.

---

## Deployment

A hosted version is available, so self-hosting is entirely optional — but the code is
open source and deploys anywhere. Ardoise ships as a single container — a multi-stage build (`apps/backend/Dockerfile`)
compiles the SvelteKit frontend to static files and bundles them alongside the Go API
binary, which serves both the API and the SPA on one port. The repo is configured for
[Fly.io](https://fly.io) via `fly.toml` (region `sjc`, `shared-cpu-1x`/256MB, HTTPS
forced, health-checked on `/health`).

```bash
fly launch --no-deploy        # first time only — reuses the committed fly.toml

# Provision a Postgres database and attach it (sets DATABASE_URL automatically)
fly postgres create
fly postgres attach <db-app-name>

# Set the remaining secrets
fly secrets set JWT_SECRET=$(openssl rand -hex 32) \
                CORS_ORIGIN=https://your-app.fly.dev

fly deploy
```

`DATABASE_URL`, `JWT_SECRET`, and `CORS_ORIGIN` must be set (see
[Environment variables](#environment-variables)). The container runs database
migrations on startup. Any host that can run the Docker image and reach a
PostgreSQL instance works — Fly.io is just the preconfigured default.

---

## Contributing

1. Fork the repo and create a branch from `main`.
2. `make check` and `make frontend-build` must pass clean.
3. Include tests for any new behaviour.
4. Open a pull request — describe what changed and why.

---

## License

[AGPL-3.0](LICENSE)
