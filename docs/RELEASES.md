# Release Policy

Release candidates require passing CI, dependency and secret scans, a clean Compose validation, updated documentation, and a manual smoke check.

Published archives should include SHA-256 checksums and an SBOM. Releases must not contain `.env`, keys, database volumes, uploaded samples, caches, or generated local state. Native shortcuts wrap Docker Compose and are not separate application runtimes.
