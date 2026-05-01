# 🛡️ DataGuardian

**DataGuardian** is a database security auditing platform designed to identify structural risks, sensitive data exposure, and insecure patterns before they become real incidents.

This project focuses on **security, pragmatic software engineering, and automated analysis**, providing developers and teams with a fast and reliable way to assess the health of their data systems.

---

## 🎯 Purpose

Modern applications frequently store sensitive data without proper structural validation or security auditing.

This leads to issues such as:

- Plain-text password storage
- Exposure of personally identifiable information (PII)
- Poor database design (e.g., missing primary keys)
- Lack of continuous auditing
- Undetected security vulnerabilities

**DataGuardian addresses these problems through automated analysis and risk scoring.**

---

## 👥 Target Audience

- Backend developers
- Data engineers
- Security engineers (AppSec / SecOps)
- Small and mid-sized teams without dedicated security staff
- Teams needing fast, automated database validation

---

## ⚙️ Core Features

### 🔍 Automated Schema Audit

Detects:

- Sensitive fields (`password`, `token`, `api_key`)
- Potential PII (`email`, `phone`, etc.)
- Structural anti-patterns
- Missing primary keys

---

### 📊 Risk Scoring

Each audit produces a severity-based score:

- LOW
- MEDIUM
- HIGH
- CRITICAL

---

## Environment Variables

Create a `.env` file in the project root:


APP_NAME=DataGuardian
ENVIRONMENT=development
DEBUG=true
SECRET_KEY=change-this-in-production
DATABASE_URL=postgresql://dataguardian:dataguardian@localhost:5434/dataguardian

### 🧾 Audit History

- Stored in PostgreSQL
- Enables traceability
- Supports future monitoring features

---

### 🔐 Authentication (JWT)

The system includes **JWT-based authentication**:

- Secure login
- Token-based access control
- Protected endpoints
- User isolation (data scoped per user)

---

## 🏗️ Architecture

The project follows a **pragmatic and minimal architecture approach**, avoiding overengineering.

### Principles:

- Clear responsibility separation
- Low coupling
- High readability
- Minimal abstraction

### Design decisions:

- ❌ No unnecessary layers
- ❌ No premature design patterns
- ✔️ Simple, maintainable structure
- ✔️ Optimized for AI-assisted development

---

## 🧰 Tech Stack

- **Backend:** FastAPI
- **Database:** PostgreSQL
- **ORM:** SQLAlchemy
- **Auth:** JWT (python-jose)
- **Hashing:** passlib (bcrypt)
- **Testing:** Pytest
- **Containerization:** Docker

---

## 🚀 Getting Started

### 1. Start database


docker compose up -d
2. Run application
cd backend
uvicorn app.main:app --reload
🔐 Authentication Flow
1. Register (if implemented)
curl -X POST http://127.0.0.1:8000/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email": "user@test.com", "password": "123456"}'
2. Login
curl -X POST http://127.0.0.1:8000/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "user@test.com", "password": "123456"}'

Response:

{
  "access_token": "YOUR_TOKEN",
  "token_type": "bearer"
}
3. Use authenticated endpoints
curl http://127.0.0.1:8000/projects \
  -H "Authorization: Bearer YOUR_TOKEN"
📡 API Usage
Create Project
curl -X POST http://127.0.0.1:8000/projects \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "My Project"}'
Run Audit
curl -X POST http://127.0.0.1:8000/projects/1/audit/run \
  -H "Authorization: Bearer YOUR_TOKEN"
Get Audit History
curl http://127.0.0.1:8000/projects/1/audit/history \
  -H "Authorization: Bearer YOUR_TOKEN"
🧪 Tests
pytest
🔐 Security Considerations
Password hashing with bcrypt
Token-based authentication (JWT)
Input validation
Separation of concerns
No sensitive data stored in plain text

Planned improvements:

Rate limiting
Role-based access control
Advanced audit rules
📈 Roadmap
 Real database inspection
 Dashboard (frontend)
 Integration with dbPilot
 Exportable reports
 Continuous monitoring
💡 Why This Project Matters

This is not a generic CRUD application.

It demonstrates:

Security-first thinking
Real-world backend design
Data analysis applied to systems
Pragmatic architecture (anti-overengineering)
Awareness of AI-driven development trade-offs
📌 Technical Notes
Designed for small teams
Built for maintainability
Avoids unnecessary abstraction
Optimized for clarity and evolution
👤 Author

Developed as a professional backend and security-focused portfolio project.