package httpapi

import (
	"time"

	"dataguardian/backend-go/internal/db"
)

// CreateAnalysisRequest is the accepted shape for creating a file or URL analysis.
type CreateAnalysisRequest struct {
	ProjectID int64              `json:"projectId"`
	InputType db.InputType       `json:"inputType"`
	File      *AnalysisFileInput `json:"file,omitempty"`
	URL       *AnalysisURLInput  `json:"url,omitempty"`
}

// AnalysisFileInput identifies an uploaded file reference for skeleton routing.
type AnalysisFileInput struct {
	OriginalFilename string  `json:"originalFilename"`
	StoredReference  string  `json:"storedReference"`
	MimeType         string  `json:"mimeType"`
	SizeBytes        int64   `json:"sizeBytes"`
	ChecksumSHA256   string  `json:"checksumSha256"`
	Extension        *string `json:"extension,omitempty"`
}

// AnalysisURLInput identifies a submitted remote URL for skeleton routing.
type AnalysisURLInput struct {
	OriginalURL string `json:"originalUrl"`
}

// AnalysisResponse is the stable output contract for completed analysis results.
type AnalysisResponse struct {
	AnalysisID    int64                       `json:"analysisId"`
	ProjectID     int64                       `json:"projectId"`
	InputType     db.InputType                `json:"inputType"`
	Status        db.AnalysisStatus           `json:"status"`
	Summary       string                      `json:"summary"`
	StartedAt     time.Time                   `json:"startedAt"`
	CompletedAt   *time.Time                  `json:"completedAt"`
	FailureReason *string                     `json:"failureReason"`
	File          *AnalysisFileReference      `json:"file,omitempty"`
	URLTarget     *AnalysisURLTarget          `json:"urlTarget,omitempty"`
	Findings      []AnalysisFinding           `json:"findings"`
	Metadata      AnalysisMetadata            `json:"metadata"`
	RiskScore     AnalysisRiskScore           `json:"riskScore"`
	CleanFile     *AnalysisCleanFileReference `json:"cleanFile"`
}

// AnalysisFileReference describes the original file in an analysis response.
type AnalysisFileReference struct {
	ID               int64   `json:"id"`
	OriginalFilename string  `json:"originalFilename"`
	MimeType         string  `json:"mimeType"`
	SizeBytes        int64   `json:"sizeBytes"`
	ChecksumSHA256   string  `json:"checksumSha256"`
	Extension        *string `json:"extension"`
}

// AnalysisURLTarget describes the URL fetch attributes in an analysis response.
type AnalysisURLTarget struct {
	ID                 int64          `json:"id"`
	OriginalURL        string         `json:"originalUrl"`
	FinalURL           *string        `json:"finalUrl"`
	RedirectCount      int            `json:"redirectCount"`
	RedirectChain      []string       `json:"redirectChain"`
	UsesHTTPS          bool           `json:"usesHttps"`
	Host               string         `json:"host"`
	ContentType        *string        `json:"contentType"`
	ContentLengthBytes *int64         `json:"contentLengthBytes"`
	HTTPStatusCode     *int           `json:"httpStatusCode"`
	FetchStatus        db.FetchStatus `json:"fetchStatus"`
	FailureReason      *string        `json:"failureReason"`
}

// AnalysisFinding is the response DTO for a structured finding.
type AnalysisFinding struct {
	ID             int64              `json:"id"`
	Type           db.FindingType     `json:"type"`
	Code           string             `json:"code"`
	Title          string             `json:"title"`
	Description    string             `json:"description"`
	Severity       db.Severity        `json:"severity"`
	Evidence       db.FindingEvidence `json:"evidence"`
	Recommendation *string            `json:"recommendation"`
}

// AnalysisMetadata is the response DTO for extracted metadata.
type AnalysisMetadata struct {
	ID         int64                 `json:"id"`
	SourceType db.MetadataSourceType `json:"sourceType"`
	Entries    []db.MetadataEntry    `json:"entries"`
}

// AnalysisRiskScore is the response DTO for risk classification.
type AnalysisRiskScore struct {
	Score   int             `json:"score"`
	Level   db.RiskLevel    `json:"level"`
	Drivers []db.RiskDriver `json:"drivers"`
}

// AnalysisCleanFileReference describes optional sanitized output.
type AnalysisCleanFileReference struct {
	ID                  int64             `json:"id"`
	Filename            string            `json:"filename"`
	MimeType            string            `json:"mimeType"`
	SizeBytes           int64             `json:"sizeBytes"`
	ChecksumSHA256      string            `json:"checksumSha256"`
	CleaningStatus      db.CleaningStatus `json:"cleaningStatus"`
	RemovedMetadataKeys []string          `json:"removedMetadataKeys"`
}
