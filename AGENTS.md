# AGENTS.md

# DATAGUARDIAN — GLOBAL OPERATIONAL RULES

You are working inside an already mature security-oriented inspection platform.

Project status:

* MVP stable
* Docker-first
* security-oriented
* additive-only evolution
* production-like
* low-risk incremental development only

Current architecture is intentionally:

* compact
* cohesive
* deterministic
* security-first
* passive-inspection-first

The main project risks are:

* unnecessary complexity
* architectural drift
* duplicated systems
* weakened security protections
* overengineering
* token waste
* unnecessary refactors

Always prioritize:

* simplicity
* security
* determinism
* maintainability
* low operational risk
* token optimization

---

# CORE PRODUCT GOALS

DataGuardian is a SAFE INSPECTION PLATFORM.

Main goals:

* inspect suspicious files safely
* inspect suspicious URLs safely
* reduce user exposure before local download/opening
* extract metadata
* classify risk deterministically
* generate explainable findings
* support sanitized file generation
* support safe previews
* support remote-file inspection before local download

The system is NOT:

* a malware sandbox
* a VM detonation platform
* a browser automation framework
* a vulnerability scanner
* an active exploit analysis tool

---

# CURRENT STACK

Backend:

* Go

Frontend:

* Next.js

Database:

* PostgreSQL

Infra:

* Docker Compose
* GitHub Actions
* CodeQL
* Dependabot
* Playwright E2E

---

# CRITICAL ARCHITECTURAL RULES

NEVER:

* redesign architecture
* create parallel pipelines
* create duplicate analyzers
* create duplicate storage systems
* create duplicate preview systems
* replace stable systems unnecessarily
* introduce unnecessary abstractions
* overengineer
* perform speculative refactors
* rewrite stable flows “for cleanliness”
* create heavy orchestration systems
* create hidden background systems
* create unnecessary async systems

ALWAYS:

* reuse existing infrastructure
* reuse existing analysis flow
* reuse existing storage flow
* reuse existing DTOs
* reuse existing dashboard flow
* reuse existing Docker setup
* preserve deterministic behavior
* preserve auditability
* preserve passive inspection model
* preserve isolated/safe preview model
* preserve SSRF protections

---

# SECURITY RULES (MANDATORY)

This project handles:

* potentially malicious files
* untrusted URLs
* remote downloadable content

Treat ALL content as untrusted.

NEVER:

* execute uploaded files
* execute remote files
* execute PDF JavaScript
* render active PDF content
* execute browser JavaScript
* run browser automation for URL inspection
* render remote websites interactively
* expose filesystem paths
* trust redirects blindly
* trust content-types blindly
* weaken SSRF protections

Keep inspection:

* passive
* deterministic
* isolated
* binary-safe
* container-oriented
* non-executing

Current security model intentionally includes:

* passive URL fetching only
* SSRF protections
* safe previews
* metadata sanitization
* isolated processing flow

Do NOT weaken this model.

---

# SSRF RULES

URL analysis is a HIGH-SENSITIVITY area.

Always preserve:

* localhost blocking
* loopback blocking
* private IP blocking
* redirect validation
* timeout limits
* size limits
* protocol restrictions

Do NOT:

* introduce unrestricted fetch behavior
* introduce browser rendering
* introduce headless browser fetching
* bypass existing URL validation

If changing URL analysis:

* inspect existing SSRF protections first
* inspect url_analyzer.go first
* preserve existing security posture

---

# SAFE PREVIEW RULES

Safe Preview exists to reduce user exposure BEFORE local download.

Previews MUST remain:

* static
* read-only
* isolated
* non-executing

Supported lightweight previews may include:

* JPG
* PNG
* TXT
* static PDF previews

Do NOT:

* add browser PDF rendering
* add active preview engines
* add JS-capable viewers
* add browser DOM execution

---

# SANITIZED FILE RULES

Sanitized files are metadata-cleaned copies.

They are NOT:

* malware-cleaned
* guaranteed-safe
* content-sanitized

Always preserve messaging:

* sanitization removes metadata only
* original files are preserved
* sanitized files may still contain malicious content

Never overwrite originals.

---

# TOKEN OPTIMIZATION RULES

ALWAYS optimize token usage.

Prioritize:

* minimal diffs
* minimal file touches
* cohesive implementation passes
* additive-only changes
* focused repository inspection
* direct fixes

Avoid:

* full repository scans unless explicitly requested
* large rewrites
* broad refactors
* unnecessary cleanup passes
* rewriting stable code “for style”
* splitting implementation into many tiny passes

Whenever possible:

* implement maximum cohesive scope per pass
* minimize future re-refactors
* preserve stable files

---

# TESTING POLICY

IMPORTANT:
The human operator performs most manual validation.

DO NOT spend excessive tokens on automated testing unless:

* explicitly requested
* security-critical
* architecture-critical
* CI-critical

Default behavior:

* lightweight validations only

Prefer:

* go test ./...
* npm run build
* focused E2E updates only when flows changed
* targeted validation only

Avoid:

* unnecessary full E2E rewrites
* large test refactors
* expensive validation flows
* unnecessary Playwright expansion

Always update tests IF behavior changes.

---

# README & DOCUMENTATION POLICY

Whenever user-facing behavior changes:

* update README
* keep docs aligned with reality
* avoid outdated instructions

README should remain:

* beginner-friendly
* Docker-first
* cross-platform
* practical
* concise

Support explicitly:

* Linux
* Windows
* macOS

---

# .GITIGNORE POLICY

Always inspect whether new artifacts should be ignored.

If new local/generated/runtime artifacts appear:

* update .gitignore when necessary

Never commit:

* .env
* local secrets
* caches
* IDE metadata
* local logs
* local databases
* node_modules
* .venv
* generated runtime artifacts

---

# EXECUTION POLICY

Before implementing:

1. inspect existing modules first
2. inspect existing integration points first
3. inspect existing flows first
4. identify reusable infrastructure first
5. identify minimal implementation strategy first

Then:

* implement ONLY requested scope
* preserve architecture
* preserve security posture
* preserve deterministic behavior

---

# IMPLEMENTATION RULES

Prefer:

* isolated additions
* local extensions
* backwards-compatible changes
* deterministic behavior
* existing patterns
* cohesive implementation

Avoid:

* broad refactors
* touching unrelated files
* abstraction explosion
* speculative future-proofing
* unnecessary frameworks
* “clean architecture” rewrites

---

# COMMUNICATION STYLE

Be:

* concise
* technical
* direct
* incremental
* architecture-aware
* security-aware

Avoid:

* unnecessary explanations
* speculative recommendations
* generic cleanup suggestions

---

# HALLUCINATION PREVENTION

DO NOT:

* invent files
* invent APIs
* invent routes
* invent integrations
* invent infrastructure
* assume architecture
* fabricate security guarantees

If uncertainty exists:

* inspect repository first
* inspect existing flows first
* ask for clarification instead of guessing

---

# VALIDATION POLICY

After implementation:

* summarize modified files
* summarize integration points
* summarize validations executed
* summarize architectural safety
* summarize security impact

Keep validations lightweight unless explicitly requested.

---

# BRANCH POLICY

IMPORTANT:
From now on, all non-trivial changes should use feature branches.

Avoid direct changes on master for:

* security changes
* architecture changes
* dependency updates
* product features
* UX changes

Prefer:

* feature/*
* fix/*
* security/*
* docs/*

---

# CURRENT KNOWN PRIORITIES

High-priority future areas:

* SSRF hardening improvements
* UX refinements
* safe download flow polish
* release preparation
* productization/distribution
* pagination/filtering

Current architecture should remain:

* compact
* stable
* Docker-first
* passive-inspection-oriented

---

# FINAL RULE

The project is already advanced.

The biggest risks now are:

* unnecessary complexity
* architectural drift
* weakening security
* overengineering
* token waste
* destabilizing stable flows

Always prioritize:

* security
* simplicity
* cohesion
* stability
* deterministic behavior
* low-risk incremental evolution
