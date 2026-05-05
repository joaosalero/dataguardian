# DataGuardian

DataGuardian is a local-first security auditing and authentication demo built as
a production-style portfolio project. It combines a Go API, a Next.js frontend,
PostgreSQL, Docker Compose orchestration, security checks, CI, and optional
browser E2E testing into a small but realistic product slice.

The project exists to show more than a working screen. It demonstrates how a
system is made runnable, testable, secure by default, and understandable to the
next engineer who has to operate it.

## What It Solves

- Provides a secure authentication foundation for a future database security
  review tool.
- Runs locally through deterministic scripts and Docker Compose.
- Keeps the active backend simple and explicit: Go is the only runtime backend.
- Separates core functionality from optional testing tools so the app works
  without Python.
- Shows security and dependency hygiene as part of normal development, not as an
  afterthought.

## Architecture

```text
Browser
  |
  v
Next.js frontend  --->  Go HTTP API  --->  PostgreSQL 15
                              |
                              v
                       Security middleware
                       Auth, cookies, rate limits
```

### Go Backend

The active backend lives in `backend-go/`. Go was chosen for the consolidated
runtime because it gives the project a small deployment footprint, clear
standard-library HTTP behavior, strong static typing, and straightforward
concurrency and timeout controls.

The API currently handles:

- User registration and login.
- Argon2id password hashing.
- RS256 JWT session cookies.
- Authenticated profile lookup.
- Local dev/test bootstrap users.
- PostgreSQL persistence through `pgx`.
- Security headers, HTTPS enforcement in production, CORS, tenant context, and
  auth rate limiting.

The previous Python backend is intentionally not part of the active runtime.
`backend-legacy-python/` is ignored and treated as historical reference only.

### Next.js Frontend

The frontend lives in `frontend/` and uses Next.js, React, TypeScript, and
Tailwind CSS. It provides login, registration, session verification, logout, and
a minimal dashboard that confirms the Go backend is active.

Frontend dependencies are pinned exactly and installed from
`frontend/package-lock.json` with `npm ci` in CI and local validation.

### Docker Orchestration

`docker-compose.yml` starts exactly these services:

- `db`: PostgreSQL 15
- `backend-go`: Go API
- `frontend`: Next.js dev server

Docker volumes isolate container `node_modules`, Next.js cache, PostgreSQL data,
and Go module cache from the host checkout.

### Optional Python E2E

Python is not a backend runtime. It is used only for optional pytest and
Playwright E2E tests under `tests/e2e/`. All core commands are designed to keep
working when pytest is not installed; E2E checks are skipped with a warning.

## Technical Highlights

- Security-first authentication: Argon2id passwords and RS256 JWT validation.
- HttpOnly session cookies with production `Secure` mode.
- Production config checks for database URL, JWT keys, and encryption key.
- HTTPS enforcement in production through TLS or `X-Forwarded-Proto`.
- Auth rate limiting for login and registration.
- Security headers on both API and frontend responses.
- Dependency scanning with `npm audit --audit-level=high`.
- Repository secret detection through `security/run_security_checks.sh`.
- Runtime exposure audit through `security/audit.sh`.
- CI separation between backend tests, security checks, and frontend build.
- CodeQL-ready structure: source is separated by runtime, generated artifacts
  are ignored, and CI has clear places to add GitHub code scanning.
- Conservative Dependabot policy for frontend packages: patch updates are
  allowed automatically; minor and major upgrades require manual review.

## Requirements

For normal local usage:

- Docker and Docker Compose

For host-side development and validation:

- Go 1.25 or newer, or Docker Compose for Go tests through the container
- Node.js 20 and npm
- Python 3.12 only if you want optional pytest/Playwright E2E tests

## Quick Start

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

Local dev/test users:

```text
admin / admin123
test  / test123
```

These users are bootstrapped only in `dev` and `test` environments.

## Execution Modes

### Manual Usage

```bash
./start.sh manual
```

Manual mode starts PostgreSQL, the Go backend, and the frontend, waits for
readiness, then follows backend and frontend logs.

Equivalent service-only startup:

```bash
./scripts/up.sh
```

### Automated Mode

```bash
./start.sh auto
```

Automated mode runs security checks, starts the Docker stack, runs the runtime
security audit, executes Go backend tests, and runs E2E tests only when pytest is
available.

Visual E2E mode:

```bash
./start.sh auto --visual
```

### Local Test Bundle

```bash
./run-tests.sh
```

This runs Go tests and the frontend production build. If pytest is installed, it
also runs the architecture contract test and browser E2E tests. If pytest is not
installed, E2E is skipped and the command can still pass.

## Setup Commands

Install frontend dependencies only:

```bash
cd frontend
npm ci
```

Optional E2E tooling setup:

```bash
./scripts/install.sh
```

`scripts/install.sh` creates `.venv/`, installs `pytest` and
`pytest-playwright`, and runs `npm ci` for the frontend. This is useful for
browser E2E development, but it is not required to run the application.

## Test Commands

Backend tests:

```bash
cd backend-go
go test ./...
```

Frontend type check:

```bash
cd frontend
npm run test
```

Frontend build:

```bash
cd frontend
npm run build
```

Full local bundle:

```bash
./run-tests.sh
```

Optional E2E tests, after the stack is running and pytest tooling is installed:

```bash
pytest tests/e2e
```

Build local backend distributables:

```bash
./scripts/build_go.sh
```

This writes generated binaries under `dist/`, which is intentionally ignored.

## Security Commands

Static dependency and secret checks:

```bash
./security/run_security_checks.sh
```

Secret-only checks:

```bash
./security/run_security_checks.sh --secrets-only
```

Runtime exposure audit, after the stack is running:

```bash
./security/audit.sh
```

Diagnostics for ports and Docker state:

```bash
./scripts/doctor.sh
```

## CI/CD

GitHub Actions runs on push and pull request:

- `backend-tests`: starts PostgreSQL, sets up Go, runs `go test ./...`, and runs
  the architecture contract test.
- `security-checks`: installs frontend dependencies, runs
  `npm audit --audit-level=high`, and runs repository secret checks.
- `frontend-build`: installs frontend dependencies and runs `npm run build`.

Backend tests and security checks are mandatory. The frontend build still runs
on every push and pull request, but frontend build failures from Dependabot pull
requests are non-blocking so automated frontend dependency PRs cannot make the
main pipeline noisy. Human pull requests and pushes still require a passing
frontend build.

CI does not run the legacy Python backend.

## Dependency Management

Frontend dependency updates are intentionally conservative:

- Direct frontend dependencies are exact-version pinned in
  `frontend/package.json`.
- CI uses `npm ci` and `frontend/package-lock.json` for deterministic installs.
- Dependabot npm updates are limited to patch-level changes.
- Semver minor and major frontend updates require manual review.

Major framework or tooling upgrades, including Next.js, React, TypeScript,
Tailwind CSS, and React type packages, should be handled in dedicated PRs with
local build, type-check, and browser validation.

Security findings remain mandatory. Fix them through safe patch updates when
available, or through a manually reviewed upgrade when no patch path exists.

## For Recruiters

DataGuardian is intentionally small in product scope and serious in engineering
scope. It is designed to show production-level thinking without pretending to be
a finished enterprise platform.

What to look for:

- Architecture decisions: the backend was consolidated to Go for a clearer
  runtime model, simpler deployment path, and stronger typed core.
- Migration discipline: the legacy Python backend is isolated from Docker,
  scripts, and CI so there is one active source of truth.
- Security posture: password hashing, JWT validation, HttpOnly cookies,
  production config guards, rate limiting, security headers, dependency audit,
  secret scanning, and runtime exposure checks are part of the default workflow.
- Testing strategy: Go unit tests cover core auth behavior, CI verifies the
  architecture contract, frontend builds are deterministic, and browser E2E is
  available without making Python mandatory for normal operation.
- Operational clarity: scripts are explicit, Docker is the default runtime path,
  generated artifacts are ignored, and README commands match actual behavior.

The result is a project that can be cloned, started, inspected, and extended by
another engineer without relying on hidden setup steps.

## Project Structure

```text
backend-go/              Active Go backend
frontend/                Next.js application
tests/                   Architecture and optional Playwright E2E tests
scripts/                 Install, startup, diagnostics, and build helpers
security/                Dependency, secret, and runtime security checks
.github/workflows/       CI pipeline
.github/dependabot.yml   Dependency automation policy
docker-compose.yml       Local runtime stack
start.sh                 Main manual/automated orchestrator
run-tests.sh             Local validation bundle
```

## Troubleshooting

Port conflict:

```bash
./scripts/doctor.sh
```

If a non-DataGuardian process owns port `3000`, `8000`, or `5434`, stop it
manually and rerun startup. The scripts do not kill unrelated processes.

Docker not running:

```bash
docker compose config --services
docker compose up -d db backend-go frontend
```

Frontend cannot reach backend:

- Confirm `curl http://localhost:8000/health` returns `{"status":"ok"}`.
- Confirm `NEXT_PUBLIC_API_URL` is `http://localhost:8000` for local Docker use.
- Restart with `./scripts/up.sh`.

Missing pytest:

```text
[WARN] pytest not found. Skipping E2E tests.
```

This is expected unless you are actively running browser E2E tests.
