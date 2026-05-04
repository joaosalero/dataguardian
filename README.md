# DataGuardian

DataGuardian is a local-first security auditing and authentication demo built as a real engineering portfolio project. It shows a complete product slice: a Go API, a Next.js UI, PostgreSQL, Docker Compose orchestration, browser E2E tests, security checks, and CI.

The current application focuses on stable authentication and project readiness after a Python-to-Go backend migration. The historical Python backend is isolated under `backend-legacy-python/` for reference only. It is not executed by Docker, `start.sh`, CI, or any active runtime script.

## What It Solves

DataGuardian is intended to become a database security review tool. In its current version it provides the stable foundation needed before expanding the product:

- Register and sign in users securely.
- Store users in PostgreSQL.
- Use HttpOnly RS256 JWT session cookies.
- Run the same local stack through deterministic scripts.
- Validate behavior with Go tests, Playwright E2E tests, security scripts, and CI.

This repository is intentionally scoped. Multi-tenant SaaS behavior, RBAC, audit trails, billing, and cloud deployment are future phases, not partially implemented features.

## Architecture

Active runtime:

- Backend: Go 1.25+, `net/http`, `pgx`, Argon2id password hashing, RS256 JWTs.
- Frontend: Next.js, React, TypeScript, Tailwind CSS.
- Database: PostgreSQL 15 through Docker Compose.
- Orchestration: `start.sh`, `scripts/up.sh`, Docker Compose.
- Tests: Go tests and Playwright E2E tests through pytest tooling.
- Security automation: npm audit, repository secret scan, runtime exposure audit.
- CI/CD: GitHub Actions.

Docker Compose starts exactly these services:

```text
db
backend-go
frontend
```

The Go backend is the only active backend runtime and source of truth.

## Requirements

For normal local use:

- Docker and Docker Compose.

For development and automated testing:

- Go 1.25 or newer.
- Node.js 20 and npm.
- Python 3.12 only if you want optional pytest/Playwright E2E testing.

Python is not required for core functionality and is not used as a backend runtime.

## Setup

Install local frontend dependencies:

```bash
./scripts/install.sh
```

This installs optional pytest tooling into `.venv/` and installs frontend dependencies. The application itself still runs without Python or `.venv/`; pytest is only used for optional E2E checks. `.venv/`, `node_modules/`, `.next/`, logs, caches, local databases, and `.env` files are ignored by Git. Docker Compose keeps the container's `node_modules` and `.next` output in Docker volumes so container dev mode does not overwrite host build artifacts.

Optional local `.env`:

```env
APP_NAME=DataGuardian
ENVIRONMENT=dev
DEBUG=true
DATABASE_URL=postgresql://dataguardian:dataguardian@localhost:5434/dataguardian
ACCESS_TOKEN_EXPIRE_MINUTES=30
```

Do not commit `.env` files, JWT keys, tokens, credentials, or production database URLs.

## Usage Modes

### 1. Normal Local Use

Start the application:

```bash
./start.sh manual
```

Open:

```text
Frontend: http://localhost:3000
Backend:  http://localhost:8000
Health:   http://localhost:8000/health
```

`start.sh manual` starts `db`, `backend-go`, and `frontend` through Docker Compose and follows backend/frontend logs. It does not kill unrelated processes. If ports `3000` or `8000` are owned by another process, it stops with a clear error.

Equivalent service-only command:

```bash
./scripts/up.sh
```

### 2. Manual Testing And Visual Verification

Start the stack:

```bash
./start.sh manual
```

Use the development test user:

```text
Username: admin
Password: admin123
```

Then verify:

1. Open `http://localhost:3000/login`.
2. Sign in with `admin / admin123`.
3. Confirm navigation to `/dashboard`.
4. Confirm the dashboard shows the signed-in user.

To run browser E2E tests visibly:

```bash
./start.sh auto --visual
```

The `admin` and `test` local users are created only in `dev` and `test` environments. They are never bootstrapped in production.

### 3. Automated Testing

Run the full local automation flow:

```bash
./start.sh auto
```

This runs security checks, starts the Docker Compose services, runs the runtime audit, and runs Go tests. If pytest is available, it also runs Playwright E2E tests. If pytest is not available, E2E tests are skipped with:

```text
[WARN] pytest not found. Skipping E2E tests.
```

Run the local test bundle:

```bash
./run-tests.sh
```

`./run-tests.sh` always runs Go tests and the frontend build. It uses host Go when Go 1.25+ is available; otherwise it runs Go tests through the Docker Compose `backend-go` service. If pytest is available, it also runs the architecture contract test and Playwright E2E tests. If pytest is missing, those pytest-based checks are skipped with the same warning.

Run individual checks:

```bash
cd backend-go && go test ./...
cd frontend && npm run build
```

### Optional E2E Testing (pytest)

The project works without Python. Python and pytest are only needed for optional browser E2E testing.

Install pytest tooling globally or in your own environment:

```bash
pip install pytest pytest-playwright
```

Or keep it outside the repository:

```bash
python3 -m venv /tmp/dataguardian-pytest
/tmp/dataguardian-pytest/bin/pip install pytest pytest-playwright
```

If you use a pytest binary outside `PATH`, set `PYTEST_BIN`:

```bash
PYTEST_BIN=/path/to/pytest ./start.sh auto
PYTEST_BIN=/path/to/pytest ./run-tests.sh
```

The script detection order is:

1. Use `PYTEST_BIN` if it is set.
2. Else use `pytest` from `PATH` if available.
3. Else print the warning and skip E2E tests.

## Security Model

Authentication:

- Passwords are hashed with Argon2id.
- Plaintext passwords are never stored.
- JWT access tokens are signed with RS256.
- JWT validation requires `exp`, `iat`, and `sub`.
- The signing algorithm is pinned to RS256.
- `/auth/login` and `/auth/register` are rate limited.

Cookies:

- Sessions are stored in HttpOnly cookies.
- Cookies use `SameSite=Lax`.
- Cookies are marked `Secure` in production.
- The frontend uses `credentials: "include"` and does not store JWTs in `localStorage`.

Production assumptions:

- `ENVIRONMENT=prod` requires `DATABASE_URL`, `JWT_PRIVATE_KEY`, `JWT_PUBLIC_KEY`, and `FERNET_KEY`.
- JWT keys must come from a secret manager or deployment environment.
- Production requests must arrive over HTTPS or through a proxy that sets `X-Forwarded-Proto: https`.
- Fernet is reserved for future reversible sensitive data encryption. It is not used for passwords.

Security commands:

```bash
./security/run_security_checks.sh
./security/run_security_checks.sh --secrets-only
./security/audit.sh
```

The security scripts check Node dependency risk, tracked/untracked repository files for obvious secret assignments, `.env` ignore/tracking status, and runtime responses for stack traces or sensitive leakage.

## CI/CD

GitHub Actions runs on push and pull request:

1. Start PostgreSQL 15.
2. Set up Go from `backend-go/go.mod`.
3. Run `go test ./...`.
4. Set up Python only for the architecture contract test.
5. Install frontend dependencies with `npm ci`.
6. Run `npm audit --audit-level=high`.
7. Run the repository secret scan.
8. Build the frontend.

CI does not run the legacy Python backend.

## Project Structure

```text
backend-go/              Active Go backend
backend-legacy-python/   Reference-only legacy backend
frontend/                Next.js application
scripts/                 Local setup, diagnostics, startup, build helpers
security/                Security check and runtime audit scripts
tests/                   Architecture and Playwright E2E tests
.github/workflows/       CI pipeline
docker-compose.yml       Local runtime stack
start.sh                 Main orchestrator
run-tests.sh             Local non-E2E test bundle
```

## Versioning Readiness

Stabilized for this version:

- Go-only backend runtime.
- Docker Compose starts only `db`, `backend-go`, and `frontend`.
- Deterministic startup scripts.
- No Python backend execution in active automation.
- Dev/test bootstrap user guarded by environment.
- Go tests, architecture contract test, frontend build, E2E flow, and security scripts.
- `.env`, `.venv`, build outputs, caches, logs, local databases, and dependency folders ignored by Git.

Intentionally not implemented yet:

- Multi-tenant isolation.
- RBAC.
- Audit trail.
- Enterprise billing.
- Cloud infrastructure.
- Production deployment automation.

## Roadmap

Next phases:

- Rebuild the database audit engine in Go.
- Add tenant isolation.
- Add RBAC.
- Add immutable audit trail events.
- Add enterprise SaaS packaging and deployment guidance.
- Add deeper security testing and production observability.

## Troubleshooting

Port conflict:

```bash
./scripts/doctor.sh
```

If a non-DataGuardian process owns port `3000` or `8000`, stop it manually and rerun startup. The scripts do not kill arbitrary processes.

Docker not running:

```bash
docker compose config --services
docker compose up -d db backend-go frontend
```

If Docker reports a daemon error, start Docker Desktop or the Docker service first.

Database startup issues:

- Confirm `docker compose ps` shows `dataguardian_db`.
- Confirm local PostgreSQL is mapped to port `5434`.
- Confirm the backend uses `db:5432` inside Docker Compose.

Frontend cannot reach backend:

- Confirm `curl http://localhost:8000/health` returns `{"status":"ok"}`.
- Confirm `NEXT_PUBLIC_API_URL` is `http://localhost:8000` for local Docker use.
- Restart with `./scripts/up.sh`.

Missing test tooling:

```bash
./scripts/install.sh
```

Then rerun:

```bash
./run-tests.sh
./start.sh auto
```
