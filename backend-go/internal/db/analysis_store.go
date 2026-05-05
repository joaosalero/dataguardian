package db

import (
	"context"
	"errors"
)

// ErrNotImplemented marks domain persistence methods reserved for future pipeline work.
var ErrNotImplemented = errors.New("not implemented")

// CreateAnalysis reserves the store boundary for creating analysis runs.
func (s *Store) CreateAnalysis(ctx context.Context, analysis Analysis) (Analysis, error) {
	return Analysis{}, ErrNotImplemented
}

// GetAnalysisByID reserves the store boundary for loading one user-scoped analysis.
func (s *Store) GetAnalysisByID(ctx context.Context, userID int64, analysisID int64) (Analysis, error) {
	return Analysis{}, ErrNotImplemented
}

// ListAnalysesByProject reserves the store boundary for listing analyses under a project.
func (s *Store) ListAnalysesByProject(ctx context.Context, userID int64, projectID int64) ([]Analysis, error) {
	return nil, ErrNotImplemented
}

// CreateFile reserves the store boundary for attaching an uploaded file to an analysis.
func (s *Store) CreateFile(ctx context.Context, file File) (File, error) {
	return File{}, ErrNotImplemented
}

// CreateURLTarget reserves the store boundary for attaching a URL target to an analysis.
func (s *Store) CreateURLTarget(ctx context.Context, target URLTarget) (URLTarget, error) {
	return URLTarget{}, ErrNotImplemented
}

// SaveMetadata reserves the store boundary for storing extracted metadata.
func (s *Store) SaveMetadata(ctx context.Context, metadata Metadata) (Metadata, error) {
	return Metadata{}, ErrNotImplemented
}

// SaveFindings reserves the store boundary for storing structured findings.
func (s *Store) SaveFindings(ctx context.Context, analysisID int64, findings []Finding) error {
	return ErrNotImplemented
}

// SaveRiskScore reserves the store boundary for storing final risk classification.
func (s *Store) SaveRiskScore(ctx context.Context, score RiskScore) (RiskScore, error) {
	return RiskScore{}, ErrNotImplemented
}

// SaveCleanFile reserves the store boundary for storing sanitized output metadata.
func (s *Store) SaveCleanFile(ctx context.Context, cleanFile CleanFile) (CleanFile, error) {
	return CleanFile{}, ErrNotImplemented
}
