package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Store struct {
	db *sql.DB
}

type User struct {
	ID             int64
	Email          string
	HashedPassword string
	CreatedAt      time.Time
}

type Project struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Name      string    `json:"name"`
	Target    string    `json:"target"`
	CreatedAt time.Time `json:"created_at"`
}

type Audit struct {
	ID        int64     `json:"id"`
	ProjectID int64     `json:"project_id"`
	Status    string    `json:"status"`
	Summary   string    `json:"summary"`
	Findings  []string  `json:"findings"`
	CreatedAt time.Time `json:"created_at"`
}

func Open(databaseURL string) (*Store, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Init() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS users (
	id SERIAL PRIMARY KEY,
	email VARCHAR UNIQUE NOT NULL,
	encrypted_email TEXT NULL,
	hashed_password VARCHAR NOT NULL,
	is_admin BOOLEAN NOT NULL DEFAULT FALSE,
	must_change_password BOOLEAN NOT NULL DEFAULT FALSE,
	tenant_id VARCHAR NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`)
	if err != nil {
		return err
	}

	for _, statement := range []string{
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS encrypted_email TEXT NULL`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS must_change_password BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS tenant_id VARCHAR NULL`,
	} {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}

	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS projects (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name VARCHAR NOT NULL,
			target VARCHAR NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS audits (
			id SERIAL PRIMARY KEY,
			project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			status VARCHAR NOT NULL,
			summary TEXT NOT NULL,
			findings TEXT[] NOT NULL DEFAULT '{}',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS analyses (
			id SERIAL PRIMARY KEY,
			project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			input_type VARCHAR NOT NULL,
			status VARCHAR NOT NULL,
			summary TEXT NOT NULL DEFAULT '',
			started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			completed_at TIMESTAMPTZ NULL,
			failure_reason TEXT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS analysis_files (
			id SERIAL PRIMARY KEY,
			analysis_id INTEGER NOT NULL UNIQUE REFERENCES analyses(id) ON DELETE CASCADE,
			original_filename VARCHAR NOT NULL,
			stored_reference TEXT NOT NULL,
			mime_type VARCHAR NOT NULL,
			size_bytes BIGINT NOT NULL,
			checksum_sha256 VARCHAR NOT NULL,
			extension VARCHAR NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS analysis_metadata (
			id SERIAL PRIMARY KEY,
			analysis_id INTEGER NOT NULL UNIQUE REFERENCES analyses(id) ON DELETE CASCADE,
			source_type VARCHAR NOT NULL,
			entries JSONB NOT NULL DEFAULT '[]',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS analysis_findings (
			id SERIAL PRIMARY KEY,
			analysis_id INTEGER NOT NULL REFERENCES analyses(id) ON DELETE CASCADE,
			type VARCHAR NOT NULL,
			code VARCHAR NOT NULL,
			title TEXT NOT NULL,
			description TEXT NOT NULL,
			severity VARCHAR NOT NULL,
			evidence JSONB NOT NULL,
			recommendation TEXT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS analysis_risk_scores (
			id SERIAL PRIMARY KEY,
			analysis_id INTEGER NOT NULL UNIQUE REFERENCES analyses(id) ON DELETE CASCADE,
			score INTEGER NOT NULL,
			level VARCHAR NOT NULL,
			drivers JSONB NOT NULL DEFAULT '[]',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS clean_files (
			id SERIAL PRIMARY KEY,
			analysis_id INTEGER NOT NULL UNIQUE REFERENCES analyses(id) ON DELETE CASCADE,
			original_file_id INTEGER NOT NULL REFERENCES analysis_files(id) ON DELETE CASCADE,
			stored_reference TEXT NOT NULL,
			filename VARCHAR NOT NULL,
			mime_type VARCHAR NOT NULL,
			size_bytes BIGINT NOT NULL,
			checksum_sha256 VARCHAR NOT NULL,
			cleaning_status VARCHAR NOT NULL,
			removed_metadata_keys JSONB NOT NULL DEFAULT '[]',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
	} {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	var user User
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, email, hashed_password, created_at FROM users WHERE email = $1`,
		email,
	).Scan(&user.ID, &user.Email, &user.HashedPassword, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return user, err
}

func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	var user User
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, email, hashed_password, created_at FROM users WHERE id = $1`,
		id,
	).Scan(&user.ID, &user.Email, &user.HashedPassword, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return user, err
}

func (s *Store) CreateUser(ctx context.Context, email string, hashedPassword string, tenantID *string) (User, error) {
	var user User
	var tenantValue any
	if tenantID != nil {
		tenantValue = *tenantID
	}
	err := s.db.QueryRowContext(
		ctx,
		`INSERT INTO users (email, hashed_password, tenant_id)
		 VALUES ($1, $2, $3)
		 RETURNING id, email, hashed_password, created_at`,
		email,
		hashedPassword,
		tenantValue,
	).Scan(&user.ID, &user.Email, &user.HashedPassword, &user.CreatedAt)
	return user, err
}

func (s *Store) EnsureLocalUser(
	ctx context.Context,
	email string,
	hashedPassword string,
	isAdmin bool,
) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO users (email, hashed_password, is_admin, must_change_password)
		 VALUES ($1, $2, $3, FALSE)
		 ON CONFLICT (email) DO NOTHING`,
		email,
		hashedPassword,
		isAdmin,
	)
	return err
}

func (s *Store) CreateProject(ctx context.Context, userID int64, name string, target string) (Project, error) {
	var project Project
	err := s.db.QueryRowContext(
		ctx,
		`INSERT INTO projects (user_id, name, target)
		 VALUES ($1, $2, $3)
		 RETURNING id, user_id, name, target, created_at`,
		userID,
		name,
		target,
	).Scan(&project.ID, &project.UserID, &project.Name, &project.Target, &project.CreatedAt)
	return project, err
}

func (s *Store) ProjectsByUser(ctx context.Context, userID int64) ([]Project, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, user_id, name, target, created_at
		 FROM projects
		 WHERE user_id = $1
		 ORDER BY created_at DESC, id DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	projects := []Project{}
	for rows.Next() {
		var project Project
		if err := rows.Scan(&project.ID, &project.UserID, &project.Name, &project.Target, &project.CreatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func (s *Store) ProjectByID(ctx context.Context, userID int64, projectID int64) (Project, error) {
	var project Project
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, user_id, name, target, created_at
		 FROM projects
		 WHERE id = $1 AND user_id = $2`,
		projectID,
		userID,
	).Scan(&project.ID, &project.UserID, &project.Name, &project.Target, &project.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	return project, err
}

func (s *Store) CreateAudit(ctx context.Context, projectID int64, status string, summary string, findings []string) (Audit, error) {
	var audit Audit
	err := s.db.QueryRowContext(
		ctx,
		`INSERT INTO audits (project_id, status, summary, findings)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, project_id, status, summary, findings, created_at`,
		projectID,
		status,
		summary,
		findings,
	).Scan(&audit.ID, &audit.ProjectID, &audit.Status, &audit.Summary, &audit.Findings, &audit.CreatedAt)
	return audit, err
}

func (s *Store) AuditsByProject(ctx context.Context, projectID int64) ([]Audit, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, project_id, status, summary, findings, created_at
		 FROM audits
		 WHERE project_id = $1
		 ORDER BY created_at DESC, id DESC`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	audits := []Audit{}
	for rows.Next() {
		var audit Audit
		if err := rows.Scan(&audit.ID, &audit.ProjectID, &audit.Status, &audit.Summary, &audit.Findings, &audit.CreatedAt); err != nil {
			return nil, err
		}
		audits = append(audits, audit)
	}
	return audits, rows.Err()
}

var ErrNotFound = errors.New("not found")
