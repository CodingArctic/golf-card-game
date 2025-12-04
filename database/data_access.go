package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserAlreadyExists  = errors.New("username already exists")
	ErrEmailAlreadyExists = errors.New("email already exists")
)

// Interface - this is what other layers depend on
type UserRepository interface {
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	UserExists(ctx context.Context, username string) (bool, error)
	CreateUser(ctx context.Context, username, hashedPassword, email string) (*User, error)
	CreateSession(ctx context.Context, userID, token string, expiresAt time.Time) error
	ValidateSession(ctx context.Context, token string) (string, error) // Returns userID if valid
	DeleteSession(ctx context.Context, token string) error
}

type postgresUserRepo struct {
	pool *pgxpool.Pool
}

type User struct {
	UserID   string
	Username string
	Password string
	Email    string
}

func NewUserRepository(pool *pgxpool.Pool) UserRepository {
	return &postgresUserRepo{pool: pool}
}

func (r *postgresUserRepo) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	var user User
	err := r.pool.QueryRow(ctx,
		"SELECT user_id, username, password, email FROM users WHERE username = $1", username).
		Scan(&user.UserID, &user.Username, &user.Password, &user.Email)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *postgresUserRepo) UserExists(ctx context.Context, username string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)", username).
		Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *postgresUserRepo) CreateUser(ctx context.Context, username, hashedPassword, email string) (*User, error) {
	var user User
	err := r.pool.QueryRow(ctx,
		"INSERT INTO users (username, password, email) VALUES ($1, $2, $3) RETURNING user_id, username, password, email",
		username, hashedPassword, email).
		Scan(&user.UserID, &user.Username, &user.Password, &user.Email)
	if err != nil {
		// Check for unique constraint violations
		if pgErr, ok := err.(*pgconn.PgError); ok {
			// 23505 is the PostgreSQL error code for unique_violation
			if pgErr.Code == "23505" {
				if pgErr.ConstraintName == "users_username_key" {
					return nil, ErrUserAlreadyExists
				}
				if pgErr.ConstraintName == "users_email_key" {
					return nil, ErrEmailAlreadyExists
				}
			}
		}
		return nil, err
	}
	return &user, nil
}

func (r *postgresUserRepo) CreateSession(ctx context.Context, userID, token string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		"INSERT INTO sessions (user_id, token, expires_at, type) VALUES ($1, $2, $3, 'web')",
		userID, token, expiresAt)
	return err
}

func (r *postgresUserRepo) ValidateSession(ctx context.Context, token string) (string, error) {
	var userID string
	err := r.pool.QueryRow(ctx,
		"SELECT user_id FROM sessions WHERE token = $1 AND expires_at > now()",
		token).Scan(&userID)
	if err != nil {
		return "", err
	}

	// Update last_active
	_, _ = r.pool.Exec(ctx,
		"UPDATE sessions SET last_active = now() WHERE token = $1",
		token)

	return userID, nil
}

func (r *postgresUserRepo) DeleteSession(ctx context.Context, token string) error {
	_, err := r.pool.Exec(ctx,
		"DELETE FROM sessions WHERE token = $1",
		token)
	return err
}
