# 🛡️ DataGuardian

**DataGuardian** is a database security auditing platform designed to identify structural risks, sensitive data exposure, and insecure patterns before they become real incidents.

This project focuses on **security, pragmatic software engineering, and automated analysis**, providing developers and teams with a fast and reliable way to assess the health of their data systems.

---

## 🎯 Purpose

Modern applications frequently store sensitive data without proper structural validation or security auditing.

This leads to common issues such as:

- Plain-text password storage
- Exposure of personally identifiable information (PII)
- Poor database design (e.g., missing primary keys)
- Lack of continuous auditing
- Security vulnerabilities going unnoticed

**DataGuardian solves this by providing automated schema analysis and risk assessment.**

---

## 👥 Target Audience

- Backend developers
- Data engineers
- Security engineers (AppSec / SecOps)
- Small and mid-sized teams without dedicated security staff
- Teams needing **fast, automated validation of database structures**

---

## ⚙️ Core Features

### 🔍 Automated Schema Audit

Analyzes database-like structures and detects:

- Sensitive fields (`password`, `token`, `api_key`)
- Potential PII (`email`, `phone`, `cpf`, etc.)
- Tables without primary keys
- Structural anti-patterns

---

### 📊 Risk Scoring System

Each audit produces a score based on severity:

- LOW
- MEDIUM
- HIGH
- CRITICAL

Provides a quick and actionable overview of system risk.

---

### 🧾 Audit History

- Stored in PostgreSQL
- Enables tracking and evolution over time
- Foundation for future monitoring features

---

### 🧠 Audit Engine

Custom rule-based engine:

- Pattern detection
- Risk classification
- Extensible design for future rules

---

### 🔌 REST API

Main endpoints:

- Create projects
- Run audits
- Retrieve audit history

---

## 🏗️ Architecture

This project follows a **pragmatic and minimal architecture approach**, intentionally avoiding overengineering.

### Principles:

- Clear separation of responsibilities
- Business logic isolated where necessary
- Low coupling
- High readability and maintainability

### Key Decisions:

- ❌ Avoided excessive layers and abstractions
- ❌ No premature design patterns
- ✔️ Optimized for clarity and real-world usage
- ✔️ Designed to work well with AI-assisted development

---

## 🧰 Tech Stack

- **Backend:** FastAPI (Python)
- **Database:** PostgreSQL
- **ORM:** SQLAlchemy
- **Testing:** Pytest
- **Containerization:** Docker
- **API:** REST

---

## 🚀 Getting Started

### 1. Start the database

```bash
docker compose up -d
````

---

### 2. Run the application

```bash
cd backend
uvicorn app.main:app --reload
```

---

### 3. Create a project

```bash
curl -X POST http://127.0.0.1:8000/projects \
  -H "Content-Type: application/json" \
  -d '{"name": "My Project"}'
```

---

### 4. Run an audit

```bash
curl -X POST http://127.0.0.1:8000/projects/1/audit/run
```

---

### 5. Check audit history

```bash
curl http://127.0.0.1:8000/projects/1/audit/history
```

---

## 🧪 Running Tests

```bash
pytest
```

---

## 🔐 Security (Current Status)

The project already considers:

* Input validation
* Structured architecture
* Preparation for authentication

### Planned improvements:

* JWT-based authentication
* Role-based access control
* API abuse protection (rate limiting)

---

## 📈 Roadmap

* [ ] Authentication and user management
* [ ] Real database schema inspection (not simulated)
* [ ] Frontend dashboard
* [ ] Integration with dbPilot
* [ ] Exportable audit reports
* [ ] Continuous monitoring

---

## 💡 Why This Project Matters

This is not a generic CRUD project.

It demonstrates:

* Real-world security concerns
* Data analysis applied to backend systems
* Pragmatic architecture (avoiding overengineering)
* Integration between API, database, and audit logic
* Awareness of AI-assisted development trade-offs

---

## 📌 Technical Notes

* Built with a focus on clarity over complexity
* Designed for small teams and real-world use
* Structured for incremental evolution
* Prioritizes maintainability and developer efficiency

---

## 👤 Author

Developed as a professional portfolio project focused on backend engineering, security, and data analysis.

---

```

