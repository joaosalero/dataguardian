# Contributing

DataGuardian accepts small, security-conscious changes. Read `AGENTS.md` and `SECURITY.md` first.

1. Create a focused feature or fix branch.
2. Preserve passive inspection, SSRF controls, static previews, and original files.
3. Never commit secrets, runtime artifacts, uploaded samples, or local databases.
4. Update tests and documentation when behavior changes.
5. Run `go test ./...`, `npm run test`, `npm run build`, and the security checks.

Report vulnerabilities privately through GitHub Security Advisories, not public issues.
