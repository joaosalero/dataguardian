package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// ErrNotImplemented marks domain persistence methods reserved for future non-file work.
var ErrNotImplemented = errors.New("not implemented")

// CreateAnalysis stores the root record for one analysis run.
func (s *Store) CreateAnalysis(ctx context.Context, analysis Analysis) (Analysis, error) {
	var created Analysis
	err := s.db.QueryRowContext(
		ctx,
		`INSERT INTO analyses (project_id, input_type, status, summary, completed_at, failure_reason)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, project_id, input_type, status, summary, started_at, completed_at, failure_reason`,
		analysis.ProjectID,
		analysis.InputType,
		analysis.Status,
		analysis.Summary,
		analysis.CompletedAt,
		analysis.FailureReason,
	).Scan(
		&created.ID,
		&created.ProjectID,
		&created.InputType,
		&created.Status,
		&created.Summary,
		&created.StartedAt,
		&created.CompletedAt,
		&created.FailureReason,
	)
	return created, err
}

// CompleteAnalysis marks a synchronous file analysis as completed.
func (s *Store) CompleteAnalysis(ctx context.Context, analysisID int64, summary string) (Analysis, error) {
	var analysis Analysis
	err := s.db.QueryRowContext(
		ctx,
		`UPDATE analyses
		 SET status = $2, summary = $3, completed_at = now(), failure_reason = NULL
		 WHERE id = $1
		 RETURNING id, project_id, input_type, status, summary, started_at, completed_at, failure_reason`,
		analysisID,
		AnalysisStatusCompleted,
		summary,
	).Scan(
		&analysis.ID,
		&analysis.ProjectID,
		&analysis.InputType,
		&analysis.Status,
		&analysis.Summary,
		&analysis.StartedAt,
		&analysis.CompletedAt,
		&analysis.FailureReason,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Analysis{}, ErrNotFound
	}
	return analysis, err
}

// GetAnalysisByID loads one analysis after checking project ownership through the user id.
func (s *Store) GetAnalysisByID(ctx context.Context, userID int64, analysisID int64) (Analysis, error) {
	var analysis Analysis
	err := s.db.QueryRowContext(
		ctx,
		`SELECT a.id, a.project_id, a.input_type, a.status, a.summary, a.started_at, a.completed_at, a.failure_reason
		 FROM analyses a
		 JOIN projects p ON p.id = a.project_id
		 WHERE a.id = $1 AND p.user_id = $2`,
		analysisID,
		userID,
	).Scan(
		&analysis.ID,
		&analysis.ProjectID,
		&analysis.InputType,
		&analysis.Status,
		&analysis.Summary,
		&analysis.StartedAt,
		&analysis.CompletedAt,
		&analysis.FailureReason,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Analysis{}, ErrNotFound
	}
	return analysis, err
}

// ListAnalysesByProject loads analyses for a user-owned project.
func (s *Store) ListAnalysesByProject(ctx context.Context, userID int64, projectID int64) ([]Analysis, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT a.id, a.project_id, a.input_type, a.status, a.summary, a.started_at, a.completed_at, a.failure_reason
		 FROM analyses a
		 JOIN projects p ON p.id = a.project_id
		 WHERE a.project_id = $1 AND p.user_id = $2
		 ORDER BY a.started_at DESC, a.id DESC`,
		projectID,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	analyses := []Analysis{}
	for rows.Next() {
		var analysis Analysis
		if err := rows.Scan(
			&analysis.ID,
			&analysis.ProjectID,
			&analysis.InputType,
			&analysis.Status,
			&analysis.Summary,
			&analysis.StartedAt,
			&analysis.CompletedAt,
			&analysis.FailureReason,
		); err != nil {
			return nil, err
		}
		analyses = append(analyses, analysis)
	}
	return analyses, rows.Err()
}

// CreateFile stores metadata for the original uploaded file.
func (s *Store) CreateFile(ctx context.Context, file File) (File, error) {
	var created File
	err := s.db.QueryRowContext(
		ctx,
		`INSERT INTO analysis_files
		 (analysis_id, original_filename, stored_reference, mime_type, size_bytes, checksum_sha256, extension)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, analysis_id, original_filename, stored_reference, mime_type, size_bytes, checksum_sha256, extension, created_at`,
		file.AnalysisID,
		file.OriginalFilename,
		file.StoredReference,
		file.MimeType,
		file.SizeBytes,
		file.ChecksumSHA256,
		file.Extension,
	).Scan(
		&created.ID,
		&created.AnalysisID,
		&created.OriginalFilename,
		&created.StoredReference,
		&created.MimeType,
		&created.SizeBytes,
		&created.ChecksumSHA256,
		&created.Extension,
		&created.CreatedAt,
	)
	return created, err
}

// FileByAnalysisID loads the file attached to one analysis.
func (s *Store) FileByAnalysisID(ctx context.Context, analysisID int64) (File, error) {
	var file File
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, analysis_id, original_filename, stored_reference, mime_type, size_bytes, checksum_sha256, extension, created_at
		 FROM analysis_files
		 WHERE analysis_id = $1`,
		analysisID,
	).Scan(
		&file.ID,
		&file.AnalysisID,
		&file.OriginalFilename,
		&file.StoredReference,
		&file.MimeType,
		&file.SizeBytes,
		&file.ChecksumSHA256,
		&file.Extension,
		&file.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return File{}, ErrNotFound
	}
	return file, err
}

// CreateURLTarget is reserved for the later URL analysis slice.
func (s *Store) CreateURLTarget(ctx context.Context, target URLTarget) (URLTarget, error) {
	return URLTarget{}, ErrNotImplemented
}

// SaveMetadata stores extracted metadata entries for one analysis.
func (s *Store) SaveMetadata(ctx context.Context, metadata Metadata) (Metadata, error) {
	entries, err := json.Marshal(metadata.Entries)
	if err != nil {
		return Metadata{}, err
	}

	var created Metadata
	var rawEntries []byte
	err = s.db.QueryRowContext(
		ctx,
		`INSERT INTO analysis_metadata (analysis_id, source_type, entries)
		 VALUES ($1, $2, $3::jsonb)
		 RETURNING id, analysis_id, source_type, entries, created_at`,
		metadata.AnalysisID,
		metadata.SourceType,
		string(entries),
	).Scan(&created.ID, &created.AnalysisID, &created.SourceType, &rawEntries, &created.CreatedAt)
	if err != nil {
		return Metadata{}, err
	}
	if err := json.Unmarshal(rawEntries, &created.Entries); err != nil {
		return Metadata{}, err
	}
	return created, nil
}

// MetadataByAnalysisID loads metadata for one analysis.
func (s *Store) MetadataByAnalysisID(ctx context.Context, analysisID int64) (Metadata, error) {
	var metadata Metadata
	var rawEntries []byte
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, analysis_id, source_type, entries, created_at
		 FROM analysis_metadata
		 WHERE analysis_id = $1`,
		analysisID,
	).Scan(&metadata.ID, &metadata.AnalysisID, &metadata.SourceType, &rawEntries, &metadata.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Metadata{}, ErrNotFound
	}
	if err != nil {
		return Metadata{}, err
	}
	if err := json.Unmarshal(rawEntries, &metadata.Entries); err != nil {
		return Metadata{}, err
	}
	return metadata, nil
}

// SaveFindings stores deterministic structured findings for one analysis.
func (s *Store) SaveFindings(ctx context.Context, analysisID int64, findings []Finding) error {
	for _, finding := range findings {
		evidence, err := json.Marshal(finding.Evidence)
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(
			ctx,
			`INSERT INTO analysis_findings
			 (analysis_id, type, code, title, description, severity, evidence, recommendation)
			 VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8)`,
			analysisID,
			finding.Type,
			finding.Code,
			finding.Title,
			finding.Description,
			finding.Severity,
			string(evidence),
			finding.Recommendation,
		); err != nil {
			return err
		}
	}
	return nil
}

// FindingsByAnalysisID loads findings for one analysis.
func (s *Store) FindingsByAnalysisID(ctx context.Context, analysisID int64) ([]Finding, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, analysis_id, type, code, title, description, severity, evidence, recommendation, created_at
		 FROM analysis_findings
		 WHERE analysis_id = $1
		 ORDER BY id ASC`,
		analysisID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	findings := []Finding{}
	for rows.Next() {
		var finding Finding
		var evidence []byte
		if err := rows.Scan(
			&finding.ID,
			&finding.AnalysisID,
			&finding.Type,
			&finding.Code,
			&finding.Title,
			&finding.Description,
			&finding.Severity,
			&evidence,
			&finding.Recommendation,
			&finding.CreatedAt,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(evidence, &finding.Evidence); err != nil {
			return nil, err
		}
		findings = append(findings, finding)
	}
	return findings, rows.Err()
}

// SaveRiskScore stores the final risk score for one analysis.
func (s *Store) SaveRiskScore(ctx context.Context, score RiskScore) (RiskScore, error) {
	drivers, err := json.Marshal(score.Drivers)
	if err != nil {
		return RiskScore{}, err
	}

	var created RiskScore
	var rawDrivers []byte
	err = s.db.QueryRowContext(
		ctx,
		`INSERT INTO analysis_risk_scores (analysis_id, score, level, drivers)
		 VALUES ($1, $2, $3, $4::jsonb)
		 RETURNING id, analysis_id, score, level, drivers, created_at`,
		score.AnalysisID,
		score.Score,
		score.Level,
		string(drivers),
	).Scan(&created.ID, &created.AnalysisID, &created.Score, &created.Level, &rawDrivers, &created.CreatedAt)
	if err != nil {
		return RiskScore{}, err
	}
	if err := json.Unmarshal(rawDrivers, &created.Drivers); err != nil {
		return RiskScore{}, err
	}
	return created, nil
}

// RiskScoreByAnalysisID loads the risk score for one analysis.
func (s *Store) RiskScoreByAnalysisID(ctx context.Context, analysisID int64) (RiskScore, error) {
	var score RiskScore
	var rawDrivers []byte
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, analysis_id, score, level, drivers, created_at
		 FROM analysis_risk_scores
		 WHERE analysis_id = $1`,
		analysisID,
	).Scan(&score.ID, &score.AnalysisID, &score.Score, &score.Level, &rawDrivers, &score.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return RiskScore{}, ErrNotFound
	}
	if err != nil {
		return RiskScore{}, err
	}
	if err := json.Unmarshal(rawDrivers, &score.Drivers); err != nil {
		return RiskScore{}, err
	}
	return score, nil
}

// SaveCleanFile is reserved for a later sanitization slice.
func (s *Store) SaveCleanFile(ctx context.Context, cleanFile CleanFile) (CleanFile, error) {
	return CleanFile{}, ErrNotImplemented
}
