package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Interface - this is what other layers depend on
type UserRepository interface {
	GetUserByUsername(ctx context.Context, username string) (*User, error)
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
