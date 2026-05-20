package db

import "time"

// AnalysisStatus describes the lifecycle state of an analysis run.
type AnalysisStatus string

const (
	AnalysisStatusPending    AnalysisStatus = "PENDING"
	AnalysisStatusProcessing AnalysisStatus = "PROCESSING"
	AnalysisStatusCompleted  AnalysisStatus = "COMPLETED"
	AnalysisStatusFailed     AnalysisStatus = "FAILED"
)

// InputType identifies the single target type accepted by an analysis.
type InputType string

const (
	InputTypeFile InputType = "FILE"
	InputTypeURL  InputType = "URL"
)

// FindingType groups findings by the source or format-specific rule family.
type FindingType string

const (
	FindingTypePDF      FindingType = "PDF"
	FindingTypeGeneric  FindingType = "GENERIC"
	FindingTypeMetadata FindingType = "METADATA"
	FindingTypeURL      FindingType = "URL"
)

// Severity represents the explainable impact level of a finding.
type Severity string

const (
	SeverityLow    Severity = "LOW"
	SeverityMedium Severity = "MEDIUM"
	SeverityHigh   Severity = "HIGH"
)

// RiskLevel is the final qualitative risk classification for an analysis.
type RiskLevel string

const (
	RiskLevelLow    RiskLevel = "LOW"
	RiskLevelMedium RiskLevel = "MEDIUM"
	RiskLevelHigh   RiskLevel = "HIGH"
)

// MetadataSensitivity classifies how sensitive a metadata entry is.
type MetadataSensitivity string

const (
	MetadataSensitivityNonSensitive         MetadataSensitivity = "NON_SENSITIVE"
	MetadataSensitivityPotentiallySensitive MetadataSensitivity = "POTENTIALLY_SENSITIVE"
	MetadataSensitivitySensitive            MetadataSensitivity = "SENSITIVE"
)

// MetadataCategory identifies the metadata family or extraction source.
type MetadataCategory string

const (
	MetadataCategoryGeneric MetadataCategory = "GENERIC"
	MetadataCategoryEXIF    MetadataCategory = "EXIF"
	MetadataCategoryPDF     MetadataCategory = "PDF"
	MetadataCategoryOffice  MetadataCategory = "OFFICE"
	MetadataCategoryImage   MetadataCategory = "IMAGE"
	MetadataCategoryURL     MetadataCategory = "URL"
	MetadataCategorySystem  MetadataCategory = "SYSTEM"
)

// MetadataSourceType identifies whether metadata came from a file or fetched URL content.
type MetadataSourceType string

const (
	MetadataSourceTypeFile       MetadataSourceType = "FILE"
	MetadataSourceTypeURLContent MetadataSourceType = "URL_CONTENT"
)

// MetadataConfidence records extraction confidence for a metadata entry.
type MetadataConfidence string

const (
	MetadataConfidenceLow    MetadataConfidence = "LOW"
	MetadataConfidenceMedium MetadataConfidence = "MEDIUM"
	MetadataConfidenceHigh   MetadataConfidence = "HIGH"
)

// FindingEvidenceSource identifies which analysis artifact supports a finding.
type FindingEvidenceSource string

const (
	FindingEvidenceSourceFile     FindingEvidenceSource = "FILE"
	FindingEvidenceSourceURL      FindingEvidenceSource = "URL"
	FindingEvidenceSourceMetadata FindingEvidenceSource = "METADATA"
	FindingEvidenceSourceContent  FindingEvidenceSource = "CONTENT"
)

// FetchStatus records whether remote URL content was fetched.
type FetchStatus string

const (
	FetchStatusNotFetched FetchStatus = "NOT_FETCHED"
	FetchStatusSuccess    FetchStatus = "SUCCESS"
	FetchStatusFailed     FetchStatus = "FAILED"
)

// CleaningStatus records whether a sanitized output file is available.
type CleaningStatus string

const (
	CleaningStatusNotApplicable CleaningStatus = "NOT_APPLICABLE"
	CleaningStatusCompleted     CleaningStatus = "COMPLETED"
	CleaningStatusFailed        CleaningStatus = "FAILED"
)

// Analysis is the root domain record for one file or URL analysis run.
type Analysis struct {
	ID            int64          `json:"id"`
	ProjectID     int64          `json:"project_id"`
	InputType     InputType      `json:"input_type"`
	Status        AnalysisStatus `json:"status"`
	Summary       string         `json:"summary"`
	StartedAt     time.Time      `json:"started_at"`
	CompletedAt   *time.Time     `json:"completed_at"`
	FailureReason *string        `json:"failure_reason"`
}

// AnalysisListFilter describes the lightweight filters used by analysis history.
type AnalysisListFilter struct {
	Page      int
	PageSize  int
	InputType *InputType
	RiskLevel *RiskLevel
	Status    *AnalysisStatus
}

// AnalysisListRow is the database-shaped row for analysis history.
type AnalysisListRow struct {
	AnalysisID int64
	ProjectID  int64
	InputType  InputType
	Status     AnalysisStatus
	RiskLevel  RiskLevel
	CreatedAt  time.Time
}

// File represents the original uploaded file associated with an analysis.
type File struct {
	ID               int64     `json:"id"`
	AnalysisID       int64     `json:"analysis_id"`
	OriginalFilename string    `json:"original_filename"`
	StoredReference  string    `json:"stored_reference"`
	MimeType         string    `json:"mime_type"`
	SizeBytes        int64     `json:"size_bytes"`
	ChecksumSHA256   string    `json:"checksum_sha256"`
	Extension        *string   `json:"extension"`
	CreatedAt        time.Time `json:"created_at"`
}

// URLTarget represents a remote URL submitted for basic content risk analysis.
type URLTarget struct {
	ID                 int64       `json:"id"`
	AnalysisID         int64       `json:"analysis_id"`
	OriginalURL        string      `json:"original_url"`
	FinalURL           *string     `json:"final_url"`
	RedirectCount      int         `json:"redirect_count"`
	RedirectChain      []string    `json:"redirect_chain"`
	UsesHTTPS          bool        `json:"uses_https"`
	Host               string      `json:"host"`
	ContentType        *string     `json:"content_type"`
	ContentLengthBytes *int64      `json:"content_length_bytes"`
	HTTPStatusCode     *int        `json:"http_status_code"`
	FetchedAt          *time.Time  `json:"fetched_at"`
	FetchStatus        FetchStatus `json:"fetch_status"`
	FailureReason      *string     `json:"failure_reason"`
	MetadataPreview    string      `json:"metadata_preview"`
}

// Metadata stores structured metadata extracted during an analysis.
type Metadata struct {
	ID         int64              `json:"id"`
	AnalysisID int64              `json:"analysis_id"`
	SourceType MetadataSourceType `json:"source_type"`
	Entries    []MetadataEntry    `json:"entries"`
	CreatedAt  time.Time          `json:"created_at"`
}

// MetadataEntry is one typed metadata value with sensitivity classification.
type MetadataEntry struct {
	Key         string              `json:"key"`
	Value       any                 `json:"value"`
	Category    MetadataCategory    `json:"category"`
	Sensitivity MetadataSensitivity `json:"sensitivity"`
	Source      string              `json:"source"`
	Confidence  MetadataConfidence  `json:"confidence"`
}

// Finding is a deterministic and explainable result from analysis rules.
type Finding struct {
	ID             int64           `json:"id"`
	AnalysisID     int64           `json:"analysis_id"`
	Type           FindingType     `json:"type"`
	Code           string          `json:"code"`
	Title          string          `json:"title"`
	Description    string          `json:"description"`
	Severity       Severity        `json:"severity"`
	Evidence       FindingEvidence `json:"evidence"`
	Recommendation *string         `json:"recommendation"`
	CreatedAt      time.Time       `json:"created_at"`
}

// FindingEvidence explains the source and rule that produced a finding.
type FindingEvidence struct {
	Source       FindingEvidenceSource `json:"source"`
	Location     *string               `json:"location"`
	MatchedValue *string               `json:"matched_value"`
	MetadataKey  *string               `json:"metadata_key"`
	RuleID       string                `json:"rule_id"`
}

// RiskScore is the final numeric and qualitative risk result for an analysis.
type RiskScore struct {
	ID         int64        `json:"id"`
	AnalysisID int64        `json:"analysis_id"`
	Score      int          `json:"score"`
	Level      RiskLevel    `json:"level"`
	Drivers    []RiskDriver `json:"drivers"`
	CreatedAt  time.Time    `json:"created_at"`
}

// RiskDriver records which finding contributed to the final risk result.
type RiskDriver struct {
	FindingCode string   `json:"finding_code"`
	Severity    Severity `json:"severity"`
	Reason      string   `json:"reason"`
}

// CleanFile represents a sanitized file derived from the original upload.
type CleanFile struct {
	ID                  int64          `json:"id"`
	AnalysisID          int64          `json:"analysis_id"`
	OriginalFileID      int64          `json:"original_file_id"`
	StoredReference     string         `json:"stored_reference"`
	Filename            string         `json:"filename"`
	MimeType            string         `json:"mime_type"`
	SizeBytes           int64          `json:"size_bytes"`
	ChecksumSHA256      string         `json:"checksum_sha256"`
	CleaningStatus      CleaningStatus `json:"cleaning_status"`
	RemovedMetadataKeys []string       `json:"removed_metadata_keys"`
	CreatedAt           time.Time      `json:"created_at"`
}
