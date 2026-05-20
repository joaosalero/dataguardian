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

// ListAnalysesForUser loads one filtered page of analysis history for a user.
func (s *Store) ListAnalysesForUser(ctx context.Context, userID int64, filter AnalysisListFilter) ([]AnalysisListRow, int, error) {
	inputType := ""
	if filter.InputType != nil {
		inputType = string(*filter.InputType)
	}
	riskLevel := ""
	if filter.RiskLevel != nil {
		riskLevel = string(*filter.RiskLevel)
	}
	status := ""
	if filter.Status != nil {
		status = string(*filter.Status)
	}
	offset := (filter.Page - 1) * filter.PageSize
	if offset < 0 {
		offset = 0
	}

	var totalItems int
	err := s.db.QueryRowContext(
		ctx,
		`SELECT count(*)
		 FROM analyses a
		 JOIN projects p ON p.id = a.project_id
		 JOIN analysis_risk_scores r ON r.analysis_id = a.id
		 WHERE p.user_id = $1
		   AND ($2 = '' OR a.input_type = $2)
		   AND ($3 = '' OR r.level = $3)
		   AND ($4 = '' OR a.status = $4)`,
		userID,
		inputType,
		riskLevel,
		status,
	).Scan(&totalItems)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(
		ctx,
		`SELECT a.id, a.project_id, a.input_type, a.status, r.level, a.started_at
		 FROM analyses a
		 JOIN projects p ON p.id = a.project_id
		 JOIN analysis_risk_scores r ON r.analysis_id = a.id
		 WHERE p.user_id = $1
		   AND ($2 = '' OR a.input_type = $2)
		   AND ($3 = '' OR r.level = $3)
		   AND ($4 = '' OR a.status = $4)
		 ORDER BY a.started_at DESC, a.id DESC
		 LIMIT $5 OFFSET $6`,
		userID,
		inputType,
		riskLevel,
		status,
		filter.PageSize,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []AnalysisListRow{}
	for rows.Next() {
		var item AnalysisListRow
		if err := rows.Scan(&item.AnalysisID, &item.ProjectID, &item.InputType, &item.Status, &item.RiskLevel, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, totalItems, rows.Err()
}

// DeleteAnalysis removes an analysis owned by the user. Child rows are removed by database cascades.
func (s *Store) DeleteAnalysis(ctx context.Context, userID int64, analysisID int64) error {
	result, err := s.db.ExecContext(
		ctx,
		`DELETE FROM analyses a
		 USING projects p
		 WHERE a.project_id = p.id AND a.id = $1 AND p.user_id = $2`,
		analysisID,
		userID,
	)
	if err != nil {
		return err
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if deleted == 0 {
		return ErrNotFound
	}
	return nil
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

// CreateURLTarget stores the passive fetch result for a URL analysis.
func (s *Store) CreateURLTarget(ctx context.Context, target URLTarget) (URLTarget, error) {
	redirectChain, err := json.Marshal(target.RedirectChain)
	if err != nil {
		return URLTarget{}, err
	}

	var created URLTarget
	var rawRedirectChain []byte
	err = s.db.QueryRowContext(
		ctx,
		`INSERT INTO analysis_url_targets
		 (analysis_id, original_url, final_url, redirect_count, redirect_chain, uses_https, host,
		  content_type, content_length_bytes, http_status_code, fetched_at, fetch_status, failure_reason)
		 VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, $8, $9, $10, $11, $12, $13)
		 RETURNING id, analysis_id, original_url, final_url, redirect_count, redirect_chain, uses_https, host,
		  content_type, content_length_bytes, http_status_code, fetched_at, fetch_status, failure_reason`,
		target.AnalysisID,
		target.OriginalURL,
		target.FinalURL,
		target.RedirectCount,
		string(redirectChain),
		target.UsesHTTPS,
		target.Host,
		target.ContentType,
		target.ContentLengthBytes,
		target.HTTPStatusCode,
		target.FetchedAt,
		target.FetchStatus,
		target.FailureReason,
	).Scan(
		&created.ID,
		&created.AnalysisID,
		&created.OriginalURL,
		&created.FinalURL,
		&created.RedirectCount,
		&rawRedirectChain,
		&created.UsesHTTPS,
		&created.Host,
		&created.ContentType,
		&created.ContentLengthBytes,
		&created.HTTPStatusCode,
		&created.FetchedAt,
		&created.FetchStatus,
		&created.FailureReason,
	)
	if err != nil {
		return URLTarget{}, err
	}
	if err := json.Unmarshal(rawRedirectChain, &created.RedirectChain); err != nil {
		return URLTarget{}, err
	}
	return created, nil
}

// URLTargetByAnalysisID loads the passive fetch result attached to one URL analysis.
func (s *Store) URLTargetByAnalysisID(ctx context.Context, analysisID int64) (URLTarget, error) {
	var target URLTarget
	var rawRedirectChain []byte
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, analysis_id, original_url, final_url, redirect_count, redirect_chain, uses_https, host,
		  content_type, content_length_bytes, http_status_code, fetched_at, fetch_status, failure_reason
		 FROM analysis_url_targets
		 WHERE analysis_id = $1`,
		analysisID,
	).Scan(
		&target.ID,
		&target.AnalysisID,
		&target.OriginalURL,
		&target.FinalURL,
		&target.RedirectCount,
		&rawRedirectChain,
		&target.UsesHTTPS,
		&target.Host,
		&target.ContentType,
		&target.ContentLengthBytes,
		&target.HTTPStatusCode,
		&target.FetchedAt,
		&target.FetchStatus,
		&target.FailureReason,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return URLTarget{}, ErrNotFound
	}
	if err != nil {
		return URLTarget{}, err
	}
	if err := json.Unmarshal(rawRedirectChain, &target.RedirectChain); err != nil {
		return URLTarget{}, err
	}
	return target, nil
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

// SaveCleanFile stores the sanitized output metadata for one file analysis.
func (s *Store) SaveCleanFile(ctx context.Context, cleanFile CleanFile) (CleanFile, error) {
	removedKeys, err := json.Marshal(cleanFile.RemovedMetadataKeys)
	if err != nil {
		return CleanFile{}, err
	}

	var created CleanFile
	var rawRemovedKeys []byte
	err = s.db.QueryRowContext(
		ctx,
		`INSERT INTO clean_files
		 (analysis_id, original_file_id, stored_reference, filename, mime_type, size_bytes,
		  checksum_sha256, cleaning_status, removed_metadata_keys)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb)
		 RETURNING id, analysis_id, original_file_id, stored_reference, filename, mime_type,
		  size_bytes, checksum_sha256, cleaning_status, removed_metadata_keys, created_at`,
		cleanFile.AnalysisID,
		cleanFile.OriginalFileID,
		cleanFile.StoredReference,
		cleanFile.Filename,
		cleanFile.MimeType,
		cleanFile.SizeBytes,
		cleanFile.ChecksumSHA256,
		cleanFile.CleaningStatus,
		string(removedKeys),
	).Scan(
		&created.ID,
		&created.AnalysisID,
		&created.OriginalFileID,
		&created.StoredReference,
		&created.Filename,
		&created.MimeType,
		&created.SizeBytes,
		&created.ChecksumSHA256,
		&created.CleaningStatus,
		&rawRemovedKeys,
		&created.CreatedAt,
	)
	if err != nil {
		return CleanFile{}, err
	}
	if err := json.Unmarshal(rawRemovedKeys, &created.RemovedMetadataKeys); err != nil {
		return CleanFile{}, err
	}
	return created, nil
}

// CleanFileByAnalysisID loads the sanitized output metadata for one file analysis.
func (s *Store) CleanFileByAnalysisID(ctx context.Context, analysisID int64) (CleanFile, error) {
	var cleanFile CleanFile
	var rawRemovedKeys []byte
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, analysis_id, original_file_id, stored_reference, filename, mime_type,
		  size_bytes, checksum_sha256, cleaning_status, removed_metadata_keys, created_at
		 FROM clean_files
		 WHERE analysis_id = $1`,
		analysisID,
	).Scan(
		&cleanFile.ID,
		&cleanFile.AnalysisID,
		&cleanFile.OriginalFileID,
		&cleanFile.StoredReference,
		&cleanFile.Filename,
		&cleanFile.MimeType,
		&cleanFile.SizeBytes,
		&cleanFile.ChecksumSHA256,
		&cleanFile.CleaningStatus,
		&rawRemovedKeys,
		&cleanFile.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return CleanFile{}, ErrNotFound
	}
	if err != nil {
		return CleanFile{}, err
	}
	if err := json.Unmarshal(rawRemovedKeys, &cleanFile.RemovedMetadataKeys); err != nil {
		return CleanFile{}, err
	}
	return cleanFile, nil
}

// StoredFileReferences returns persisted storage references that must remain downloadable.
func (s *Store) StoredFileReferences(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT stored_reference FROM analysis_files WHERE stored_reference <> ''
		 UNION
		 SELECT stored_reference FROM clean_files WHERE stored_reference <> ''`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	references := []string{}
	for rows.Next() {
		var reference string
		if err := rows.Scan(&reference); err != nil {
			return nil, err
		}
		references = append(references, reference)
	}
	return references, rows.Err()
}
