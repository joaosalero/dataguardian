# DataGuardian

DataGuardian is a full-stack database security auditing application. It combines a FastAPI backend, a Next.js frontend, PostgreSQL, secure cookie-based authentication, and automated security checks for dependencies, secrets, and runtime exposure.

The app is designed to be runnable locally while following production-oriented security defaults: Argon2 password hashing, short-lived JWT sessions in HttpOnly cookies, HTTPS enforcement in production, encrypted sensitive user metadata, and CI security gates.

## Contents

1. [Purpose](#purpose)
2. [Stack](#stack)
3. [Installation](#installation)
4. [Usage Modes](#usage-modes)
5. [Default Test Credentials](#default-test-credentials)
6. [Creating Users](#creating-users)
7. [Authentication And Security](#authentication-and-security)
8. [Security Checks And Audit](#security-checks-and-audit)
9. [Testing](#testing)
10. [CI/CD](#cicd)
11. [Troubleshooting](#troubleshooting)

## Purpose

DataGuardian lets users sign in, create projects, run a database security audit, and review audit history. The current audit engine checks a known schema for common security risks:

- Sensitive column names such as password, token, secret, and API key fields.
- Possible PII fields such as email, phone, CPF, and document identifiers.
- Tables without primary keys.
- Severity-based scoring.

The project is intentionally practical: it demonstrates secure authentication, user-scoped data access, audit persistence, browser automation, and DevSecOps checks without unnecessary infrastructure.

## Stack

Backend:

- Python 3.12
- FastAPI
- SQLAlchemy
- PostgreSQL
- Argon2 via `argon2-cffi`
- JWT signing via `python-jose`
- Fernet encryption via `cryptography`
- Pytest

Frontend:

- Next.js
- React
- TypeScript
- Tailwind CSS

Automation:

- Docker Compose
- GitHub Actions
- `pip-audit`
- `npm audit`
- Playwright E2E tests

## Installation

Prerequisites:

- Python 3.12
- Node.js 20
- npm
- Docker and Docker Compose

Install dependencies:

```bash
python3.12 -m venv .venv
source .venv/bin/activate
pip install --upgrade pip
pip install -r backend/requirements.txt

cd frontend
npm ci
cd ..
```

Create a local `.env` file in the project root:

```env
APP_NAME=DataGuardian
ENVIRONMENT=dev
DEBUG=true
SECRET_KEY=development-only-secret-key-change-before-production
DATABASE_URL=postgresql://dataguardian:dataguardian@localhost:5434/dataguardian
ALGORITHM=HS256
ACCESS_TOKEN_EXPIRE_MINUTES=30
```

`.env` is ignored by Git and must never be committed.

## Usage Modes

Manual mode starts the database, backend, and frontend for normal browser use:

```bash
./start.sh manual
```

Open:

```text
http://localhost:3000
```

Automatic mode runs the local security checks, starts services, runs the runtime audit, runs backend tests, and runs E2E tests:

```bash
./start.sh auto
```

`start.sh` detects port conflicts on `3000` and `8000` and stops the process bound to those app ports before starting DataGuardian services.

## Default Test Credentials

In `dev` and `test` environments, the backend ensures this local test user exists:

```text
Username: test
Password: test123
```

This user is never created in production.

When the user table is empty in local development, the backend also creates an admin user and prints a one-time temporary credential to the local console. Production does not print generated credentials; production admin bootstrap must use explicit environment variables.

## Creating Users

Public registration endpoint:

```bash
curl -X POST http://localhost:8000/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"new-user@example.com","password":"StrongPass123"}'
```

CLI user creation:

```bash
python scripts/create_user.py --username admin@example.com --admin --must-change-password
```

The CLI prompts for the password interactively so secrets are not written to shell history.

## Authentication And Security

Password security:

- Passwords are hashed with Argon2.
- Plaintext passwords are never stored.
- Registration enforces basic password strength.

JWT session security:

- JWTs include `sub`, `iat`, and `exp`.
- Tokens are short-lived; default expiration is 30 minutes.
- Invalid tokens return a generic authentication error.
- Tokens are not logged.

Cookie security:

- Login sets the JWT in an HttpOnly cookie.
- The frontend uses `credentials: "include"` and does not store JWTs in `localStorage`.
- Cookies use `SameSite=Lax`.
- Cookies are marked `Secure` in production.

HTTPS strategy:

- Development and test mode allow HTTP on localhost.
- Production mode rejects non-HTTPS requests.
- Reverse proxies should pass `X-Forwarded-Proto: https`.

Database security:

- Sensitive user metadata is encrypted with Fernet.
- Production requires `FERNET_KEY`.
- Encryption keys must come from environment variables and must not be committed.

## Security Checks And Audit

Run baseline security checks:

```bash
./security/run_security_checks.sh
```

Run the runtime audit after services are running:

```bash
./security/audit.sh
```

The security layer validates:

- Python dependencies with `pip-audit`.
- Node dependencies with `npm audit --audit-level=high`.
- Tracked files for sensitive lowercase assignment patterns.
- `.env` ignore and tracking status.
- Backend and frontend runtime responses for stack traces, database URLs, tokens, password markers, and verbose disclosure.

Example report:

```text
[SECURITY AUDIT REPORT]
- Dependency scan: PASS
- Secrets exposure: PASS
- Backend exposure: PASS
- Frontend exposure: PASS
- Risk level: LOW
```

Limitations:

- This is a baseline security layer, not a full penetration test.
- The secret scanner is intentionally lightweight.
- Runtime checks do not replace authenticated authorization testing, fuzzing, or managed DAST tooling.

## Testing

Backend tests:

```bash
.venv/bin/pytest backend/tests
```

Frontend build:

```bash
cd frontend
npm run build
```

E2E test:

```bash
.venv/bin/pytest tests/e2e
```

The E2E test expects the backend on `http://localhost:8000` and the frontend on `http://localhost:3000`.

## CI/CD

GitHub Actions runs on push and pull request:

1. Start PostgreSQL 15 as a service.
2. Install Python dependencies and `pip-audit`.
3. Run Python dependency audit.
4. Run backend tests.
5. Install Node dependencies with `npm ci`.
6. Run `npm audit --audit-level=high`.
7. Run the repository secret scan.
8. Build the frontend.

CI fails on backend test failures, frontend build failures, high severity Node vulnerabilities, Python audit findings, or secret-scan failures.

## Troubleshooting

Port already in use:

```bash
./start.sh manual
```

The start script attempts to stop processes bound to ports `3000` and `8000`. If another service keeps restarting on those ports, stop it manually and rerun the command.

Docker is not running:

```bash
docker compose up -d
```

If Docker reports a daemon error, start Docker Desktop or the Docker service, then rerun `./start.sh manual`.

Backend cannot connect to PostgreSQL:

- Confirm Docker is running.
- Confirm `docker compose ps` shows the database container.
- Confirm `DATABASE_URL` uses local port `5434` for Docker Compose.

Missing dependencies:

```bash
pip install -r backend/requirements.txt
cd frontend
npm ci
```

Security audit cannot reach services:

- Run `./start.sh manual` first, or use `./start.sh auto`.
- Confirm `curl http://localhost:8000/health` returns `{"status":"ok"}`.
- Confirm `curl http://localhost:3000/login` returns the login page.
