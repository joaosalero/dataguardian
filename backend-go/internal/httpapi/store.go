package httpapi

import (
	"context"

	"dataguardian/backend-go/internal/db"
)

// dataStore captures the database operations used by HTTP handlers.
type dataStore interface {
	UserByID(ctx context.Context, id int64) (db.User, error)
	UserByEmail(ctx context.Context, email string) (db.User, error)
	CreateUser(ctx context.Context, email string, hashedPassword string, tenantID *string) (db.User, error)
	EnsureLocalUser(ctx context.Context, email string, hashedPassword string, isAdmin bool) error
	CreateProject(ctx context.Context, userID int64, name string, target string) (db.Project, error)
	ProjectsByUser(ctx context.Context, userID int64) ([]db.Project, error)
	ProjectByID(ctx context.Context, userID int64, projectID int64) (db.Project, error)
	CreateAudit(ctx context.Context, projectID int64, status string, summary string, findings []string) (db.Audit, error)
	AuditsByProject(ctx context.Context, projectID int64) ([]db.Audit, error)

	CreateAnalysis(ctx context.Context, analysis db.Analysis) (db.Analysis, error)
	CompleteAnalysis(ctx context.Context, analysisID int64, summary string) (db.Analysis, error)
	GetAnalysisByID(ctx context.Context, userID int64, analysisID int64) (db.Analysis, error)
	ListAnalysesByProject(ctx context.Context, userID int64, projectID int64) ([]db.Analysis, error)
	ListAnalysesForUser(ctx context.Context, userID int64, filter db.AnalysisListFilter) ([]db.AnalysisListRow, int, error)
	DeleteAnalysis(ctx context.Context, userID int64, analysisID int64) error
	CreateFile(ctx context.Context, file db.File) (db.File, error)
	FileByAnalysisID(ctx context.Context, analysisID int64) (db.File, error)
	CreateURLTarget(ctx context.Context, target db.URLTarget) (db.URLTarget, error)
	URLTargetByAnalysisID(ctx context.Context, analysisID int64) (db.URLTarget, error)
	SaveMetadata(ctx context.Context, metadata db.Metadata) (db.Metadata, error)
	MetadataByAnalysisID(ctx context.Context, analysisID int64) (db.Metadata, error)
	SaveFindings(ctx context.Context, analysisID int64, findings []db.Finding) error
	FindingsByAnalysisID(ctx context.Context, analysisID int64) ([]db.Finding, error)
	SaveRiskScore(ctx context.Context, score db.RiskScore) (db.RiskScore, error)
	RiskScoreByAnalysisID(ctx context.Context, analysisID int64) (db.RiskScore, error)
	SaveCleanFile(ctx context.Context, cleanFile db.CleanFile) (db.CleanFile, error)
	CleanFileByAnalysisID(ctx context.Context, analysisID int64) (db.CleanFile, error)
}
