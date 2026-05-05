package httpapi

import (
	"errors"

	"dataguardian/backend-go/internal/db"
)

// FindingExplanation is explanatory response text derived only from stored finding data.
type FindingExplanation struct {
	Explanation    string
	Recommendation string
}

// ExplanationProvider is the future integration point for richer AI explainers.
type ExplanationProvider interface {
	ExplainFinding(finding db.Finding) (FindingExplanation, error)
}

var explanationProvider ExplanationProvider = templateExplanationProvider{}

type templateExplanationProvider struct{}

// ExplainFinding is deterministic and never calls external services or fetches content.
func (templateExplanationProvider) ExplainFinding(finding db.Finding) (FindingExplanation, error) {
	if finding.Code == "" {
		return FindingExplanation{}, errors.New("missing finding code")
	}
	if explanation, ok := findingExplanations[finding.Code]; ok {
		return explanation, nil
	}
	return FindingExplanation{
		Explanation:    "This finding was produced by a deterministic DataGuardian rule. Review the evidence, severity, and surrounding metadata before sharing or opening the analyzed target.",
		Recommendation: "Review this finding with the owning team and reduce exposure before using the target in trusted environments.",
	}, nil
}

var findingExplanations = map[string]FindingExplanation{
	"PDF_JS_DETECTED": {
		Explanation:    "This PDF contains embedded JavaScript markers. Scripted PDF behavior can run when opened by some viewers and is commonly abused in malicious documents.",
		Recommendation: "Avoid opening this file in untrusted environments and remove embedded scripts before sharing.",
	},
	"PDF_OPENACTION_DETECTED": {
		Explanation:    "This PDF contains an OpenAction marker, which can trigger behavior automatically when the document is opened.",
		Recommendation: "Inspect and remove automatic open actions before distributing the document.",
	},
	"GENERIC_BASE64_PATTERN": {
		Explanation:    "The content contains a long base64-like string. Encoded blobs can be benign, but they are also used to hide scripts, payloads, or copied secrets.",
		Recommendation: "Decode and review the value in a controlled environment before trusting or sharing the content.",
	},
	"GENERIC_EVAL_PATTERN": {
		Explanation:    "The content contains an eval-like pattern. Dynamic evaluation can execute strings as code and often increases the risk of script injection or obfuscation.",
		Recommendation: "Remove dynamic evaluation patterns unless there is a documented and reviewed business need.",
	},
	"METADATA_GPS_EXPOSED": {
		Explanation:    "The file contains GPS metadata. Location metadata can reveal where a file was created or captured.",
		Recommendation: "Remove GPS metadata before sharing externally or storing in broadly accessible systems.",
	},
	"METADATA_AUTHOR_PRESENT": {
		Explanation:    "The file includes author, device, or tool metadata. These fields can reveal people, software, or workflows tied to the file.",
		Recommendation: "Remove authoring metadata before sharing outside the intended audience.",
	},
	"METADATA_SUSPICIOUS_PRESENT": {
		Explanation:    "The metadata indicates potentially risky document structure, such as embedded objects.",
		Recommendation: "Review embedded objects and remove anything not required for the document's purpose.",
	},
	"URL_NO_HTTPS": {
		Explanation:    "The submitted URL uses HTTP instead of HTTPS, so traffic may be observable or modified in transit.",
		Recommendation: "Prefer HTTPS URLs and avoid submitting credentials or sensitive data over plain HTTP.",
	},
	"URL_REDIRECT_DETECTED": {
		Explanation:    "The URL redirected before returning final content. Redirects can be legitimate, but they can also hide the final destination.",
		Recommendation: "Review the redirect chain and confirm the final destination is expected.",
	},
	"URL_FETCH_FAILED": {
		Explanation:    "DataGuardian could not fetch the URL within its safety limits. The target may be unavailable, blocked, too large, or outside allowed network boundaries.",
		Recommendation: "Verify the URL and retry only if the destination is trusted and reachable.",
	},
	"URL_SUSPICIOUS_CONTENT": {
		Explanation:    "The fetched URL content contains suspicious string patterns such as encoded data or dynamic evaluation markers.",
		Recommendation: "Review the fetched content in a controlled environment before trusting the page or sharing the URL.",
	},
}
