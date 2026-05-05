# DataGuardian

![CI](https://github.com/joaosalero/dataguardian/actions/workflows/ci.yml/badge.svg)
![Dependabot](https://img.shields.io/badge/dependabot-enabled-brightgreen)
![Go](https://img.shields.io/badge/go-1.25-blue)
![License](https://img.shields.io/badge/license-MIT-green)

DataGuardian is a local-first security auditing and authentication platform
slice. It is built to answer a practical question: how do you give teams a
trustworthy foundation for reviewing sensitive data systems without turning the
demo into a fragile toy?

The current release focuses on the production foundations that matter first:
secure authentication, deterministic local execution, automated validation,
dependency hygiene, and a clear path from developer laptop to CI.

## Product Perspective

DataGuardian is aimed at engineers, security-minded teams, and technical
reviewers who need confidence that a system handling audit data can be started,
tested, inspected, and extended without hidden setup steps.

In a real security product, the first risk is rarely the dashboard. It is weak
authentication, unclear runtime boundaries, unmanaged dependencies, leaked
configuration, and tests that only work on one machine. This project prioritizes
those fundamentals before expanding into larger audit workflows such as RBAC,
tenant isolation, immutable audit events, or deployment automation.

## Engineering Narrative

DataGuardian began as a broader Python-backed prototype and was consolidated
around a Go backend for the active runtime. That migration is intentional: the
project now has one backend source of truth, one Docker runtime path, and one CI
contract.

Go is used for the backend because it fits the shape of the problem:

- Strong static typing for authentication and configuration boundaries.
- Small operational footprint for API services.
- Clear standard-library HTTP behavior.
- Good concurrency and timeout primitives for future audit workloads.
- Straightforward container execution with predictable builds.

Docker Compose is the default runtime because reproducibility is part of the
product. A reviewer should be able to start PostgreSQL, the API, and the
frontend with one command and see the same service topology used by the scripts.

Python remains intentionally optional. It is used only for pytest and Playwright
browser E2E tests. The application, backend, frontend, Docker stack, and core
test bundle do not require Python to run.

## Architecture

```text
Browser
  |
  v
Next.js frontend  --->  Go HTTP API  --->  PostgreSQL 15
                              |
                              v
                    Security middleware and auth
```

Active runtime services:

- `db`: PostgreSQL 15
- `backend-go`: Go API
- `frontend`: Next.js application

The Go API handles registration, login, session validation, logout, project
tracking, baseline audit runs, local dev/test bootstrap users, and PostgreSQL
persistence. The frontend provides login, registration, an authenticated
dashboard, project creation, audit execution, audit results, and session-aware
navigation.

The legacy Python implementation is not part of Docker, CI, startup scripts, or
the active runtime.

## Security Mindset

Security is treated as a workflow property, not a README claim.

- Passwords are hashed with Argon2id.
- Sessions use HttpOnly RS256 JWT cookies.
- JWT validation requires expected claims and rejects unexpected signing
  methods.
- Production configuration requires explicit database URL, JWT keys, and
  encryption key.
- Production requests require HTTPS or trusted proxy forwarding.
- Login and registration are rate limited.
- Backend and frontend responses set basic hardening headers.
- `npm audit --audit-level=high` checks frontend dependency risk.
- `security/run_security_checks.sh` scans for obvious secret exposure and
  verifies `.env` is ignored.
- `security/audit.sh` checks runtime responses for sensitive leakage and risky
  headers.
- CI separates backend tests, security checks, and frontend build so failures
  are easier to reason about.

The repository is also structured for CodeQL adoption: active source is
separated by runtime, generated artifacts are ignored, and CI has clear security
gates where code scanning can be added.

## Execution Modes

### Manual Usage

```bash
./start.sh manual
```

Manual mode starts PostgreSQL, the Go backend, and the frontend through Docker
Compose. It waits for readiness and then follows backend and frontend logs.

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

These users are created only in `dev` and `test` environments.

### Automated Mode

```bash
./start.sh auto
```

Automated mode runs security checks, starts the Docker stack, runs the runtime
security audit, executes Go backend tests, and runs browser E2E tests only when
pytest is available.

Visual browser mode:

```bash
./start.sh auto --visual
```

### Core Test Bundle

```bash
./run-tests.sh
```

This runs Go tests and the frontend production build. If pytest is installed,
it also runs the architecture contract and browser E2E tests. If pytest is not
installed, E2E is skipped gracefully:

```text
[WARN] pytest not found. Skipping E2E tests.
```

## Setup And Commands

Install frontend dependencies:

```bash
cd frontend
npm ci
```

Start services without following logs:

```bash
./scripts/up.sh
```

Install optional E2E tooling:

```bash
./scripts/install.sh
```

`scripts/install.sh` creates `.venv/`, installs `pytest` and
`pytest-playwright`, and runs `npm ci` for the frontend. This is useful for
browser E2E development, but it is not required for normal application usage.

Run backend tests:

```bash
cd backend-go
go test ./...
```

Run frontend type check:

```bash
cd frontend
npm run test
```

Build frontend:

```bash
cd frontend
npm run build
```

Run optional E2E tests after the stack is running:

```bash
pytest tests/e2e
```

Build local backend binaries:

```bash
./scripts/build_go.sh
```

Generated binaries are written under `dist/`, which is intentionally ignored.

## Security Commands

Run dependency, secret, and config checks:

```bash
./security/run_security_checks.sh
```

Run secret-only checks:

```bash
./security/run_security_checks.sh --secrets-only
```

Run runtime exposure audit after services are up:

```bash
./security/audit.sh
```

Inspect ports and Docker state:

```bash
./scripts/doctor.sh
```

## Product API

Authenticated product routes:

- `GET /projects`: list projects for the signed-in user.
- `POST /projects`: create a project with `name` and `target`.
- `GET /projects/{id}`: fetch one project owned by the signed-in user.
- `POST /projects/{id}/audit`: run and store a baseline audit result.
- `GET /projects/{id}/audits`: list audit results for a project.
- `POST /analyses`: create a file analysis from a multipart upload.
- `GET /analyses/{id}`: fetch a completed file analysis result.

### File Analysis

The first analysis slice supports authenticated file uploads only. URL analysis
is not implemented yet.

Example:

```bash
curl -i \
  -X POST http://localhost:8000/analyses \
  -b "dataguardian_session=<session-cookie>" \
  -F "projectId=1" \
  -F "inputType=FILE" \
  -F "file=@sample.pdf"
```

Supported upload types:

- PDF: `application/pdf`
- JPEG: `image/jpeg`

Current limits and behavior:

- Maximum file size is 10 MB.
- Files are stored under the configured `STORAGE_DIR`, defaulting to
  `/tmp/dataguardian/uploads`.
- The backend computes a SHA-256 checksum for every accepted file.
- Minimal analysis scans raw bytes for PDF JavaScript/OpenAction markers,
  base64-like long strings, and `eval(` string patterns.
- Metadata is limited to filename, MIME type, size, and checksum.
- Clean file generation is not implemented yet, so `cleanFile` is `null`.

Security boundary:

- Uploaded files are treated as untrusted binary data.
- Files are not executed, rendered, opened in viewers, or processed by PDF
  engines.
- The current implementation is structured for future sandboxing, but does not
  provide a sandbox yet.

## CI/CD And Dependency Policy

GitHub Actions runs on push and pull request:

- `backend-tests`: PostgreSQL service, Go setup, `go test ./...`, and the
  architecture contract.
- `security-checks`: `npm ci`, `npm audit --audit-level=high`, and repository
  secret checks.
- `frontend-build`: `npm ci` and `npm run build`.

Backend tests and security checks are mandatory. Frontend builds still run on
Dependabot pull requests, but Dependabot frontend build failures are
non-blocking to avoid noisy incompatible dependency PRs. Human pull requests
and pushes still require the frontend build to pass.

Frontend dependency management is deliberately conservative:

- Direct frontend dependencies are pinned to exact versions.
- CI installs from `frontend/package-lock.json` with `npm ci`.
- Dependabot npm version updates are limited to patch-level changes.
- Minor and major framework/tooling upgrades require manual review.

## For Recruiters

DataGuardian is intentionally scoped, but the engineering choices are
production-oriented.

What this project demonstrates:

- Migration judgment: the active backend moved from a Python prototype shape to
  a Go service with one runtime path and clearer operational boundaries.
- Runtime discipline: Docker Compose starts only PostgreSQL, the Go API, and
  the Next.js frontend.
- Security-first design: authentication, cookie handling, config validation,
  dependency scanning, secret detection, and runtime exposure checks are built
  into normal workflows.
- Testing philosophy: fast Go tests cover core auth behavior; frontend builds
  validate the UI contract; optional Playwright E2E covers browser behavior
  without making Python mandatory.
- Trade-off awareness: the project favors a stable foundation over prematurely
  implementing enterprise features such as RBAC, billing, or cloud deployment.
- Operational clarity: scripts, README commands, Docker services, and CI jobs
  describe the same system.

The result is a repository that can be reviewed as both a working product slice
and an engineering sample: small enough to understand, complete enough to run,
and structured enough to extend.

## Release v1.0.0

Tag:

```text
v1.0.0
```

Title:

```text
v1.0.0 - Production-ready DataGuardian
```

Release focus:

- Go backend migration completed for the active runtime.
- Security posture improved with Argon2id passwords, RS256 session tokens,
  hardened cookies, rate limiting, dependency scanning, and secret checks.
- Docker-based execution standardized for local usage and automated flows.
- Optional pytest and Playwright E2E testing isolated from core application
  requirements.
- Repository hygiene cleaned with generated artifacts, secrets, and local caches
  ignored.
- Recruiter-focused architecture documentation added to explain decisions,
  trade-offs, and production-readiness thinking.

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

Missing pytest is expected unless you are actively running browser E2E tests.
