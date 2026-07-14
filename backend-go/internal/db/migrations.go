package db

import (
	"context"
	"fmt"
)

type schemaMigration struct {
	version int
	name    string
	sql     string
}

var schemaMigrations = []schemaMigration{
	{version: 1, name: "analysis_history_indexes", sql: `
CREATE INDEX IF NOT EXISTS idx_projects_user_created ON projects (user_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_analyses_project_started ON analyses (project_id, started_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_analyses_status_type ON analyses (status, input_type);
CREATE INDEX IF NOT EXISTS idx_risk_scores_level_analysis ON analysis_risk_scores (level, analysis_id);`},
}

func (s *Store) applyMigrations(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		return err
	}
	for _, migration := range schemaMigrations {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		var applied bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, migration.version).Scan(&applied); err == nil && !applied {
			if _, err = tx.ExecContext(ctx, migration.sql); err == nil {
				_, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`, migration.version, migration.name)
			}
		}
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("schema migration %d (%s): %w", migration.version, migration.name, err)
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
