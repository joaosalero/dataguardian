# DataGuardian: passive file and URL inspection

## Problem and implemented scope

DataGuardian gives a user an authenticated place to inspect supported files and
HTTP/HTTPS responses before deciding whether to download or open them. The active
runtime is a Go API, a Next.js frontend, and PostgreSQL, started together with
Docker Compose. The Python files provide optional validation, not a second backend.

This is a deterministic inspection tool. It does not detonate malware, execute
document actions, render websites, or infer that an input is safe because no rule
matched. The project remains under development; the repository's tests and
workflows describe validation coverage, not an independent security certification.

## Architecture and trust boundaries

| Boundary | Implemented behavior | Practical limit |
| --- | --- | --- |
| Browser to API | Authenticated analysis creation, owner-scoped history and artifact downloads | A compromised browser session can exercise that user's access |
| API to untrusted bytes | Size-bounded upload/response processing; passive pattern and metadata inspection | Parsing runs inside the Go API process; there is no per-analysis sandbox |
| API to remote URL | HTTP/HTTPS only, standard ports, public-address validation, checked dial target, manual redirect validation, no ambient HTTP proxy | Application-level SSRF controls do not replace an independent egress boundary |
| API to storage | Original files and optional metadata-cleaned copies persist for later authorized downloads | Docker volumes contain retained user content; operational access to those volumes is privileged |
| Host to local stack | Compose publishes database, API and frontend on loopback by default | Local demo configuration is unsuitable for deliberate remote exposure |

The backend image uses an unprivileged runtime user. Docker isolates the service
from the host under its configured container boundary; it does not isolate one
analysis from another, and file parsing shares the API's process and access.
PDF inspection cards are generated from inert raw text snippets. They are an
inspection aid, not a faithful rendered view of every PDF page.

Source: [Compose](../docker-compose.yml), [backend image](../backend-go/Dockerfile),
[analysis handlers](../backend-go/internal/httpapi/analysis_handlers.go), and
[URL analyzer](../backend-go/internal/analysis/url_analyzer.go).

## Explainable risk model

`ScoreFindings` selects a fixed score from the highest implemented finding severity.
It is not a calibrated probability, a vulnerability severity standard, or an
additive score. Finding count does not increase the score.

| Findings | Score | Level |
| --- | --- | --- |
| None, or low severity only | 10 | LOW |
| At least one medium severity; no high severity | 55 | MEDIUM |
| At least one high severity | 85 | HIGH |

Every finding contributes a driver containing its code, severity, and title. For
example, a plain-text `eval(` match produces a medium finding; a raw PDF
JavaScript marker produces a high finding. These string rules can produce false
positives and miss encoded, compressed, or unfamiliar content. LOW means the
implemented rules did not find a higher-severity signal; it is not a safety verdict.

Source: [file analyzer and scoring](../backend-go/internal/analysis/file_analyzer.go)
and [URL content checks](../backend-go/internal/analysis/url_analyzer.go).

## Metadata removal and preservation

JPEG cleaning removes EXIF APP1 segments. PDF cleaning targets the supported
literal author, producer, and creation-date fields. Matched PDF fields are replaced
with equal-length whitespace so downstream objects and cross-reference tables
retain their original byte offsets. The source bytes remain unchanged.

The PDF operation is intentionally lightweight. It does not parse all PDF syntax,
guarantee removal of compressed or complex metadata, preserve digital signatures,
or remove scripts and embedded objects. A metadata-cleaned artifact remains
untrusted. Keeping original and cleaned artifacts distinct makes that limitation
visible instead of presenting metadata removal as malware disarmament.

Source: [metadata extraction and cleaning](../backend-go/internal/analysis/file_metadata.go)
and [metadata regression tests](../backend-go/internal/analysis/file_metadata_test.go).

## Validation and review

From a complete checkout with the documented development dependencies:

```bash
cd backend-go
go test ./...
```

```bash
pytest tests/test_backend_mode_contract.py
```

The Go tests cover analyzer behavior and the PDF byte-offset preservation
regression. The architecture contract checks the single Go runtime and local-only
default published ports. The optional browser flow requires the running stack;
see the [README](../README.md#e2e-testing). No benchmark or detection-accuracy
result is claimed here.

## Engineering decisions

- Reuse one analysis flow for supported uploads and remote-file responses.
- Keep detection and explanations deterministic and inspectable.
- Bound network content and validate redirect/dial destinations without introducing
  an active browser or a second fetching pipeline.
- Retain artifact ownership checks and make sanitization limits explicit.
- Preserve the compact Go runtime while documenting the separate sandbox and
  network isolation work that would be needed for stronger adversarial containment.
