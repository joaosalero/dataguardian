# Changelog

## [Unreleased]

### Added
- File analysis (PDF, JPG)
- URL analysis with SSRF protection
- Metadata extraction and classification
- Risk scoring system
- Clean file generation (sanitized output)
- Authenticated clean file download from analysis details
- Dashboard with analysis history
- Deterministic explanation layer
- E2E automated tests (Playwright)
- Docker Compose validation service

### Changed
- UI updated to support file and URL analysis
- Dashboard now creates/selects a default project for first-time users
- Improved security handling and validation
- README aligned with the release-candidate workflow

### Known Limitations
- No pagination or filtering
- Clean files remove selected metadata only; they are not a full malware
  removal or content-disarm guarantee

## [1.0.0] (planned)

Initial public release candidate for the MVP-stable Docker-first product.
