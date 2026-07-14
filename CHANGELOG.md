# Changelog

## [Unreleased]

### Added
- Deterministic safe demonstration corpus with clean, inert suspicious, and rejected samples
- Reproducible sample generator, checksums, and analyzer behavior tests
- Local JSON and static PDF analysis exports
- Read-only profile page and persistent dark mode
- Versioned schema migration tracking and history indexes
- CSRF and Fetch Metadata request protection
- Real dashboard screenshot and secure packaging guidance
- User-local desktop shortcuts and management menu for Windows, Linux, and macOS
- Installable web manifest without sensitive offline caching
- Contribution, pull request, issue, backup, and release documentation
- File analysis (PDF, JPEG, PNG, plain text)
- URL analysis with SSRF protection
- Metadata extraction and classification
- Risk scoring system
- Sanitized file generation (metadata-cleaned output)
- Authenticated sanitized file download from analysis details
- Dashboard with analysis history
- Deterministic explanation layer
- E2E automated tests (Playwright)
- Docker Compose validation service
- User-controlled analysis deletion with artifact cleanup
- Dashboard pagination, basic filtering, and storage usage visibility
- Lightweight release smoke-validation script
- Lightweight release preflight helper for required files, shell syntax, and
  Docker Compose configuration
- Beginner-friendly launchers for Windows and Linux/macOS
- Root `START-HERE.txt` for portable ZIP onboarding

### Changed
- UI updated to support file and URL analysis
- Dashboard now creates/selects a default project for first-time users
- Improved security handling and validation
- README aligned with the release-candidate workflow
- Analysis, download, preview, and storage routes now use lightweight rate
  limits where appropriate
- Operator storage visibility is limited to admin users
- Stored preview reads and missing artifact handling fail more gracefully
- Analysis history filtering and pagination now run in database queries
- Expired browser sessions redirect with a clearer sign-in message
- Remote file inspection rejects mismatched PDF/image payloads more defensively
- Malformed pagination and invalid runtime configuration now fail explicitly
- Dashboard empty states and deletion/action success messages are clearer
- Backend startup verifies the configured storage directory before serving
- README troubleshooting now covers smoke validation, rebuild, restart, and reset
- Storage startup errors now identify non-directory storage paths more clearly
- Smoke validation now prints actionable recovery guidance when required
  services are unreachable
- README release wording now consistently describes DataGuardian as a
  security inspection tool with metadata-cleaned sanitized files
- Helper startup scripts now fail fast with a clear message when local `curl`
  is unavailable for readiness checks
- Release helper documentation now lists preflight and smoke scripts in the
  project structure
- Release title wording now reflects release-candidate readiness instead of
  implying full production hardening
- README and START-HERE now clarify Docker-first startup, launcher behavior,
  ZIP extraction, optional release checks, and recovery guidance
- Security wording now clarifies passive inspection boundaries and
  sanitized-file limitations for public RC users

### Known Limitations
- Sanitized files remove selected metadata only; they are not a full malware
  removal or content-disarm guarantee
- DataGuardian is not a malware sandbox, detonation system, or interactive
  browser-based website renderer

## [1.0.0] (planned)

Initial public release candidate for the MVP-stable Docker-first product.
