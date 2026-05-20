# DataGuardian Backlog

## Release readiness — Completed for first stable candidate

- [x] Expose CleanFile download in UI
- [x] Improve project UX with auto/default project selection
- [x] Ensure core analysis and download errors are visible in UI
- [x] Align README with Docker-first usage and current product behavior
- [x] Cover sanitized-file UI and download action in E2E
- [x] Add user-controlled analysis deletion with owned artifact cleanup
- [x] Add basic storage usage visibility
- [x] Limit operator storage visibility to admin users
- [x] Harden missing artifact and stored preview failure behavior
- [x] Move analysis history pagination/filtering into database queries
- [x] Improve expired-session dashboard redirects
- [x] Harden remote file type mismatch handling
- [x] Add release-candidate checklist
- [x] Harden malformed pagination and invalid config handling
- [x] Improve dashboard action feedback and empty states
- [x] Add lightweight release smoke-validation flow
- [x] Add startup storage directory self-check
- [x] Document rebuild, restart, reset, and smoke validation commands
- [x] Clarify final smoke validation recovery guidance
- [x] Add actionable smoke failure guidance for unreachable services
- [x] Align final README product and sanitized-file wording for RC freeze
- [x] Fail fast when helper startup readiness checks are missing local curl
- [x] Add lightweight release preflight helper for final RC checks
- [x] Document release preflight and smoke helpers in project structure
- [x] Align release title wording with release-candidate readiness
- [x] Add beginner-friendly Windows and Linux/macOS launchers
- [x] Add START-HERE portable ZIP onboarding
- [x] Clarify final RC trust, safety, and passive-inspection wording

---

## P1 — High Priority

- [x] Add pagination to analysis history
- [x] Add filtering (by type, risk, status)
- [ ] Polish loading states for larger analysis histories
- [ ] Improve validation for URL input
- [ ] Improve validation for file upload

---

## P2 — Productization

- [x] Simplify startup around Docker Compose defaults
- [x] Add Docker Compose validation service
- [x] Polish portable ZIP onboarding flow
- [ ] Add screenshots to README
- [ ] Evaluate packaging options (.deb, Windows, macOS)

---

## P3 — Nice to have

- [ ] Dark mode UI
- [ ] Export analysis as JSON file
- [ ] Export report (PDF)
- [ ] Add user profile/settings page

---

## Future considerations

- [ ] Multi-tenant support (already partially present)
- [ ] Background job processing
- [ ] Async analysis queue
- [ ] External integrations (optional)
