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

var ErrNotFound = errors.New("not found")
