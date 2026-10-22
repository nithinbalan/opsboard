# OpsBoard

OpsBoard is a small but real task/work-item board intended as the application workload for a production-style DevOps platform. It has a Go REST API, Angular frontend, PostgreSQL persistence, database migrations, Dockerfiles, local Compose configuration, tests (Go + Vitest), and CI.

## Architecture

- **Frontend:** Angular 22, served by Nginx in its container.
- **API:** Go 1.25+ (CI uses the latest stable release), JSON REST API.
- **Database:** PostgreSQL 16.
- **Health:** `/health/live` (process liveness) and `/health/ready` (database readiness).
- **Persistence:** SQL migration files in `database/migrations/`.
- **Local development:** Docker Compose.

This repository intentionally contains application code only. AWS infrastructure, Terraform state, EKS, ECR, Argo CD, and environment-specific deployment configuration belong in the separate `opsboard-platform` repository.

## Run locally

Requirements: Docker Engine and Docker Compose plugin.

```bash
cp .env.example .env
docker compose up --build
```

Then open:
- UI: http://localhost:8080
- API liveness: http://localhost:8081/health/live
- API readiness: http://localhost:8081/health/ready
- API: http://localhost:8081/api/v1/tasks

Stop:

```bash
docker compose down
```

Remove the local database volume only when you intentionally want to delete local data:

```bash
docker compose down -v
```

## API

| Method | Path | Purpose |
|---|---|---|
| GET | `/health/live` | Liveness check; does not depend on PostgreSQL |
| GET | `/health/ready` | Readiness check including database connectivity |
| GET | `/api/v1/tasks` | List tasks |
| POST | `/api/v1/tasks` | Create a task |
| GET | `/api/v1/tasks/{id}` | Read a task |
| PATCH | `/api/v1/tasks/{id}` | Update title, description, or status |
| DELETE | `/api/v1/tasks/{id}` | Delete a task |

Valid statuses: `todo`, `in_progress`, `done`.

Example:

```bash
curl http://localhost:8081/api/v1/tasks
curl -X POST http://localhost:8081/api/v1/tasks \
  -H 'Content-Type: application/json' \
  -d '{"title":"Prepare deployment pipeline","description":"Build CI and image scanning","status":"todo"}'
```

## Configuration

Copy `.env.example` to `.env`. The defaults are for local Compose only. Do not use them in shared environments.

| Variable | Default | Description |
|---|---|---|
| `HTTP_ADDR` | `:8080` | API listen address inside its container |
| `DATABASE_URL` | Compose-local URL | PostgreSQL connection string |
| `DB_MAX_CONNS` | `10` | Maximum PostgreSQL pool connections |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:8080` | Comma-separated allowed browser origins |
| `LOG_LEVEL` | `info` | Logging level (`debug`, `info`, `warn`, `error`) |

## Development commands

API:

```bash
cd backend
go test ./...
go vet ./...
```

Frontend:

```bash
cd frontend
npm ci
npm test -- --no-watch
npm run build
```

## Security and production notes

- Run containers as non-root where practical.
- Do not commit `.env`, passwords, tokens, cloud credentials, or production data.
- CI should run tests, static analysis, dependency checks, secret scanning, image scanning, and SBOM generation before release.
- Use a managed secret store for non-local credentials.
- Use a dedicated migration job and a least-privilege database migration identity in production.
- Configure TLS at the ingress/load-balancer layer.
- Add authentication and authorization before exposing this application to untrusted users. The initial version is an internal demo workload and intentionally has no user login.
- PostgreSQL backups, retention, restore drills, resource requests/limits, autoscaling, and SLO alerts are platform/environment responsibilities.
- The included local credentials are development-only.
