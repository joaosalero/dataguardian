# DataGuardian

## Short Description

DataGuardian is a full-stack database security auditing platform. It combines a FastAPI backend, a Next.js frontend, PostgreSQL, JWT authentication, and an automated audit flow that helps identify simple database security risks such as sensitive column names, possible PII fields, and missing primary keys.

The project is intentionally pragmatic: it is built to be easy to run, easy to read, and easy to maintain by a small team.

## Table of Contents

1. [Project Overview](#project-overview)
2. [Problem Statement](#problem-statement)
3. [Target Audience](#target-audience)
4. [Main Features](#main-features)
5. [System Architecture](#system-architecture)
6. [Tech Stack](#tech-stack)
7. [Environment Variables](#environment-variables)
8. [Installation Guide](#installation-guide)
9. [Running the Application](#running-the-application)
10. [Authentication Guide](#authentication-guide)
11. [Using the Application Step by Step](#using-the-application-step-by-step)
12. [Running Automated Tests](#running-automated-tests)
13. [Running the Automated Browser Demo / E2E Test](#running-the-automated-browser-demo--e2e-test)
14. [API Usage Examples](#api-usage-examples)
15. [Security](#security)
16. [Security Considerations](#security-considerations)
17. [Testing Strategy](#testing-strategy)
18. [CI/CD](#cicd)
19. [Technical Decisions](#technical-decisions)
20. [Roadmap](#roadmap)
21. [Troubleshooting](#troubleshooting)
22. [Author](#author)

## Project Overview

DataGuardian is a portfolio-grade security auditing application for database-backed systems. Users sign in, create projects, run audits, and review audit history from a browser.

The current audit engine uses a known schema input inside the backend service and applies rules for:

- Sensitive field names such as `password`, `token`, `secret`, and `api_key`
- Possible PII fields such as `email`, `phone`, `cpf`, and document identifiers
- Tables without a primary key
- Severity-based risk scoring

Audit runs and findings are persisted in PostgreSQL, which makes the app useful for demonstrating authentication, protected resources, persistence, security-oriented backend logic, and browser-level automated validation.

## Problem Statement

Small teams often store sensitive data without a repeatable way to review database design risks. Common issues include:

- Sensitive fields being stored without enough protection
- PII columns being created without clear review
- Tables missing primary keys
- Security checks being manual, inconsistent, or forgotten
- Lack of traceability for previous reviews

DataGuardian solves this at a practical level by providing a simple workflow for creating projects, running audits, viewing findings, and keeping audit history.

## Target Audience

DataGuardian is useful for:

- Backend developers who want to understand security-oriented API design
- Full-stack developers reviewing a realistic FastAPI and Next.js project
- Security engineers evaluating early-stage audit workflows
- Recruiters and hiring teams reviewing practical engineering skills
- Small teams that need a lightweight example of authenticated audit tooling

## Main Features

- JWT-based login
- Password hashing with bcrypt through Passlib
- Protected project routes
- User-scoped project access
- Project creation and listing
- Audit execution from the project detail page
- Audit history persisted in PostgreSQL
- Rule-based audit engine
- Severity-based score calculation
- Backend request logging
- Safe generic response for unhandled backend errors
- FastAPI CORS configuration for the local Next.js frontend
- Pytest backend test suite
- Playwright browser E2E test
- Docker Compose PostgreSQL setup
- GitHub Actions CI pipeline
- Dependabot configuration for GitHub dependency updates

## System Architecture

The project uses a simple full-stack architecture:

```text
Browser
  |
  | http://localhost:3000
  v
Next.js frontend
  |
  | JSON API requests with JWT bearer token
  | http://localhost:8000
  v
FastAPI backend
  |
  | SQLAlchemy
  v
PostgreSQL running through Docker Compose
```

Backend structure:

```text
backend/app
  main.py                 FastAPI app, middleware, routes, health check
  auth.py                 Login, JWT creation, current-user dependency
  core/
    config.py             Environment-based settings
    database.py           SQLAlchemy engine/session setup
  api/
    project_routes.py     Project endpoints
    audit_routes.py       Audit endpoints
  services/
    project_service.py    Project business logic
    audit_service.py      Audit execution and persistence
  audit/
    engine.py             Rule orchestration
    rules.py              Audit rules
    scoring.py            Score calculation
  models/                 SQLAlchemy models
  schemas/                Pydantic request/response schemas
```

Frontend structure:

```text
frontend/app
  login/page.tsx          Login form and token storage
  dashboard/page.tsx      Project list and project creation
  projects/[id]/page.tsx  Audit execution and audit history
  page.tsx                Redirects to login
```

The architecture deliberately avoids unnecessary layers. Routes call small services, services use SQLAlchemy and simple repository logic where it already exists, and the audit engine remains explicit and easy to inspect.

## Tech Stack

Backend:

- Python 3.12
- FastAPI
- Uvicorn
- SQLAlchemy
- PostgreSQL
- python-jose for JWT
- Passlib with bcrypt for password hashing
- Pytest
- HTTPX test client

Frontend:

- Next.js 14
- React 18
- TypeScript
- Tailwind CSS

Infrastructure and automation:

- Docker Compose
- PostgreSQL 15 container
- Playwright for E2E browser testing
- GitHub Actions CI
- Dependabot

## Environment Variables

Create a `.env` file in the project root:

```env
APP_NAME=DataGuardian
ENVIRONMENT=development
DEBUG=true
SECRET_KEY=change-this-in-production
DATABASE_URL=postgresql://dataguardian:dataguardian@localhost:5434/dataguardian
ALGORITHM=HS256
ACCESS_TOKEN_EXPIRE_MINUTES=30
```

Important security notes:

- Never use the default `SECRET_KEY` in production.
- Generate a secure random key before deploying.
- `.env` must not be committed to version control.
- Local Docker PostgreSQL uses host port `5434`, not `5432`.
- The frontend defaults to `http://localhost:8000` for API requests. You can override it with `NEXT_PUBLIC_API_BASE_URL` if needed.

## Installation Guide

These steps assume a fresh clone on a machine with Git, Python 3.12, Node.js 20, npm, Docker, and Docker Compose installed.

1. Clone the repository:

```bash
git clone <repository-url>
cd dataguardian
```

2. Create and activate a Python virtual environment:

```bash
python3.12 -m venv .venv
source .venv/bin/activate
```

On Windows PowerShell:

```powershell
py -3.12 -m venv .venv
.\.venv\Scripts\Activate.ps1
```

3. Install backend dependencies:

```bash
pip install --upgrade pip
pip install -r backend/requirements.txt
pip install pytest-playwright
```

4. Install frontend dependencies:

```bash
cd frontend
npm install
cd ..
```

5. Create `.env` in the project root using the example from [Environment Variables](#environment-variables).

6. Start PostgreSQL:

```bash
docker compose up -d
```

7. Run the backend:

```bash
cd backend
uvicorn app.main:app --reload --port 8000
```

8. In a second terminal, run the frontend:

```bash
cd frontend
npm run dev
```

9. Open the application:

```text
http://localhost:3000
```

## Running the Application

PostgreSQL runs through Docker Compose:

```bash
docker compose up -d
```

Backend:

```bash
cd backend
uvicorn app.main:app --reload --port 8000
```

Frontend:

```bash
cd frontend
npm run dev
```

Browser URL:

```text
http://localhost:3000
```

Backend health check:

```bash
curl http://localhost:8000/health
```

Expected response:

```json
{"status":"ok"}
```

## Authentication Guide

Authentication uses JWT bearer tokens.

Current flow:

1. A user record exists in the database.
2. The user submits username and password from the login page.
3. The backend verifies the bcrypt password hash.
4. The backend returns a JWT access token.
5. The frontend stores the token in `localStorage` as `dataguardian_token`.
6. Protected frontend pages send `Authorization: Bearer <token>` to the backend.
7. The backend decodes the token and loads the current user.

The current app does not expose a public registration page or registration endpoint. For local testing, create a demo user directly:

```bash
PYTHONPATH=backend python -c "import app.models; from app.core.database import Base, engine, SessionLocal; from app.auth import hash_password; from app.models.user import User; Base.metadata.create_all(bind=engine); db=SessionLocal(); email='demo@dataguardian.dev'; demo_password='strong-password'; user=db.query(User).filter(User.email==email).one_or_none(); user = user or User(email=email, hashed_password=hash_password(demo_password)); user.hashed_password=hash_password(demo_password); db.add(user); db.commit(); db.close(); print('User ready: demo@dataguardian.dev / strong-password')"
```

Then sign in with:

```text
Username: demo@dataguardian.dev
Password: strong-password
```

## Using the Application Step by Step

1. Start Docker PostgreSQL.
2. Start the FastAPI backend on port `8000`.
3. Start the Next.js frontend on port `3000`.
4. Create the local demo user using the command in [Authentication Guide](#authentication-guide).
5. Open `http://localhost:3000`.
6. Sign in.
7. Create a project from the dashboard.
8. Click the project.
9. Click `Run audit`.
10. Review the latest audit findings and audit history.
11. Use `Sign out` to clear the local token.

## Running Automated Tests

Backend tests:

```bash
pytest backend/tests
```

Frontend TypeScript check:

```bash
cd frontend
npm test
```

Frontend production build:

```bash
cd frontend
npm run build
```

Root helper script:

```bash
./run-tests.sh
```

Backend helper script:

```bash
cd backend
./run_tests.sh
```

## Running the Automated Browser Demo / E2E Test

The E2E test is a browser demo that shows the system working automatically. It opens the app, logs in, creates a project, opens the project, runs an audit, and validates the result.

Prerequisites:

- Docker PostgreSQL is running.
- Backend is running on `http://localhost:8000`.
- Frontend is running on `http://localhost:3000`.
- Playwright browser dependencies are installed.

Install Playwright browsers:

```bash
python -m playwright install --with-deps chromium
```

Run backend:

```bash
cd backend
uvicorn app.main:app --reload --port 8000
```

Run frontend in another terminal:

```bash
cd frontend
npm run dev
```

Run the E2E test from the project root:

```bash
pytest tests/e2e -s
```

By default, the test can run headless. To watch the browser, run:

```bash
PLAYWRIGHT_HEADLESS=0 pytest tests/e2e -s
```

The E2E test creates or updates its own test user:

```text
e2e@dataguardian.dev / strong-password
```

## API Usage Examples

The backend runs at:

```text
http://localhost:8000
```

Login:

```bash
curl -X POST http://localhost:8000/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "demo@dataguardian.dev", "password": "strong-password"}'
```

Example response:

```json
{
  "access_token": "YOUR_TOKEN",
  "token_type": "bearer"
}
```

Get current user:

```bash
curl http://localhost:8000/auth/me \
  -H "Authorization: Bearer YOUR_TOKEN"
```

List projects:

```bash
curl http://localhost:8000/projects \
  -H "Authorization: Bearer YOUR_TOKEN"
```

Create project:

```bash
curl -X POST http://localhost:8000/projects \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "Customer Database", "description": "Demo project"}'
```

Get project:

```bash
curl http://localhost:8000/projects/1 \
  -H "Authorization: Bearer YOUR_TOKEN"
```

Run audit:

```bash
curl -X POST http://localhost:8000/projects/1/audit/run \
  -H "Authorization: Bearer YOUR_TOKEN"
```

Get audit history:

```bash
curl http://localhost:8000/projects/1/audit/history \
  -H "Authorization: Bearer YOUR_TOKEN"
```

## Security

DataGuardian includes a baseline local security layer intended to catch common issues before code reaches CI. It checks dependency vulnerabilities, accidental secret exposure, tracked environment files, and basic runtime response exposure from the backend and frontend.

Run all local security checks from the project root:

```bash
./security/run_security_checks.sh
```

Run the structured audit after starting the backend on `http://localhost:8000` and the frontend on `http://localhost:3000`:

```bash
./security/audit.sh
```

The security checks validate:

- Python dependencies with `pip-audit`.
- Node dependencies with `npm audit --audit-level=high`.
- Tracked repository files for sensitive lowercase assignment patterns for password, secret, token, and API key values.
- Configuration hygiene, including confirmation that `.env` is ignored and not tracked.
- Runtime responses for stack traces, database URLs, tokens, password markers, and verbose framework disclosure.

Example successful local check:

```text
[SECURITY CHECKS]
[PASS] Python dependency scan
[PASS] Node dependency scan
[PASS] Secret scan
[PASS] Config validation
[PASS] Security checks completed
```

Example audit report:

```text
[SECURITY AUDIT REPORT]
- Dependency scan: PASS
- Secrets exposure: PASS
- Backend exposure: PASS
- Frontend exposure: PASS
- Risk level: LOW
```

Limitations:

- This is not a full penetration test.
- The secret scanner is intentionally lightweight and does not replace a managed secret-scanning platform.
- Runtime checks cover baseline exposure only; they do not perform authenticated authorization testing or deep application fuzzing.
- Dependency results depend on the vulnerability databases used by `pip-audit` and `npm audit`.

## Security Considerations

Current security-related behavior:

- Passwords are hashed with bcrypt before storage.
- Login returns JWT access tokens.
- Protected backend routes require a bearer token.
- Projects are scoped to the authenticated user.
- CORS allows the local frontend origin.
- Unexpected backend errors return a generic response body.
- Request logging records method, path, and status code.
- Production startup rejects missing, short, or known placeholder `SECRET_KEY` values.
- Backend and frontend responses include basic hardening headers.
- CI blocks high-severity dependency vulnerabilities and secret-scan failures.

Important limitations:

- The current project is a development/demo system, not a complete production security product.
- There is no public registration flow.
- There is no role-based access control yet.
- There is no rate limiting yet.
- JWTs are stored in `localStorage`, which is simple for this project but has tradeoffs for production systems.
- The audit engine currently uses a simulated schema inside the service, not live database introspection.

## Testing Strategy

The test suite favors direct, practical tests:

- Backend tests use FastAPI/HTTPX integration-style requests.
- Authentication tests verify login, invalid credentials, token-protected routes, and user isolation.
- Project tests verify creation, validation, listing, and ownership boundaries.
- Audit tests verify rule behavior, score calculation, persistence, and API access.
- Health tests verify `/health`, CORS preflight behavior, and safe handling of unexpected errors.
- E2E tests use Playwright to validate the complete browser flow.

The project avoids excessive mocking because most behavior is easier to verify through real request and persistence boundaries.

## CI/CD

GitHub Actions is configured in:

```text
.github/workflows/ci.yml
```

The CI pipeline runs on every push and pull request.

Current CI flow:

1. Start PostgreSQL 15 as a GitHub Actions service.
2. Set up Python 3.12.
3. Install backend dependencies and `pip-audit`.
4. Run `pip-audit -r backend/requirements.txt`.
5. Run `pytest backend/tests`.
6. Set up Node.js 20.
7. Run `npm ci`.
8. Run `npm audit --audit-level=high`.
9. Run the repository secret scan.
10. Run `npm run build`.

Dependabot is also configured under `.github/dependabot.yml` for dependency update support.

## Technical Decisions

The project intentionally uses a simple architecture.

Key decisions:

- FastAPI was chosen for clear request handling, dependency injection, and strong testing support.
- SQLAlchemy is used directly because it is enough for the current persistence needs.
- JWT authentication is implemented in one backend module to keep the flow easy to follow.
- The frontend stores the token in local storage for a simple demo workflow.
- The audit engine is rule-based because the current audit checks are explicit and small.
- The backend has a small service layer where business logic benefits from being separated from route definitions.
- No ports/adapters, CQRS, event sourcing, or complex domain layers are used.

Why this avoids overengineering:

- There is one implementation of each major behavior, so interfaces would add indirection without value.
- The code is grouped by practical responsibility, not by theoretical architecture boundaries.
- Most features can be understood by reading one route file and one nearby service.
- The audit rules are explicit data and functions, which makes them easy to modify.
- The system is optimized for small-team maintainability and AI-assisted navigation.

## Roadmap

Planned improvements:

- Add a safe registration or admin user creation flow.
- Replace the simulated schema with real database introspection.
- Add more audit rules for indexes, constraints, nullable sensitive fields, and encryption hints.
- Add severity filtering in the frontend.
- Add exportable audit reports.
- Add role-based access control if multiple user roles become necessary.
- Add rate limiting for authentication endpoints.
- Add refresh-token or cookie-based auth for a stronger production deployment model.
- Add deployment documentation for a cloud environment.

These are roadmap items, not completed features.

## Troubleshooting

### Port 5432, 5433, or 5434 is already in use

The local Docker Compose file maps PostgreSQL to host port `5434`.

Check running containers:

```bash
docker ps
```

Stop the DataGuardian database:

```bash
docker compose down
```

If another service uses `5434`, change the host port in `docker-compose.yml` and update `DATABASE_URL` in `.env`.

### Docker is not running

Start Docker Desktop or your Docker service, then run:

```bash
docker compose up -d
```

### Backend is not running on port 8000

Check:

```bash
curl http://localhost:8000/health
```

Start it:

```bash
cd backend
uvicorn app.main:app --reload --port 8000
```

### Frontend is not running on port 3000

Start it:

```bash
cd frontend
npm run dev
```

Open:

```text
http://localhost:3000
```

### Login shows "Failed to fetch" or cannot reach API

This usually means the browser cannot reach the backend.

Check:

- Backend is running on `http://localhost:8000`.
- Frontend is running on `http://localhost:3000`.
- `NEXT_PUBLIC_API_BASE_URL` is not pointing to the wrong host.
- CORS allows `http://localhost:3000`.
- PostgreSQL is running and the backend started successfully.

### Login returns "Invalid username or password"

Create the local demo user again:

```bash
PYTHONPATH=backend python -c "import app.models; from app.core.database import Base, engine, SessionLocal; from app.auth import hash_password; from app.models.user import User; Base.metadata.create_all(bind=engine); db=SessionLocal(); email='demo@dataguardian.dev'; demo_password='strong-password'; user=db.query(User).filter(User.email==email).one_or_none(); user = user or User(email=email, hashed_password=hash_password(demo_password)); user.hashed_password=hash_password(demo_password); db.add(user); db.commit(); db.close(); print('User ready: demo@dataguardian.dev / strong-password')"
```

### Playwright is missing browser or OS dependencies

Install them:

```bash
python -m playwright install --with-deps chromium
```

Then run:

```bash
pytest tests/e2e -s
```

### Database connection fails

Confirm Docker PostgreSQL is running:

```bash
docker compose ps
```

Confirm `.env` uses:

```env
DATABASE_URL=postgresql://dataguardian:dataguardian@localhost:5434/dataguardian
```

## Author

Developed as a professional full-stack, backend, and security-focused portfolio project.
