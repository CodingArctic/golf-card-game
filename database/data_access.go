package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
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
	GetUserByID(ctx context.Context, userID string) (*User, error)
	UserExists(ctx context.Context, username string) (bool, error)
	CreateUser(ctx context.Context, username, hashedPassword, email string) (*User, error)
	CreateSession(ctx context.Context, userID, token string, expiresAt time.Time) error
	ValidateSession(ctx context.Context, token string) (string, error) // Returns userID if valid
	DeleteSession(ctx context.Context, token string) error
}

type ChatRepository interface {
	SaveMessage(ctx context.Context, senderUserID, scope, messageText string) (*ChatMessage, error)
	GetMessagesByScope(ctx context.Context, scope string, limit int) ([]*ChatMessage, error)
}

type GameRepository interface {
	CreateGame(ctx context.Context, createdByUserID string, maxPlayers int) (*Game, error)
	GetGameByID(ctx context.Context, gameID int) (*Game, error)
	GetGameByPublicID(ctx context.Context, publicID string) (*Game, error)
	AddPlayer(ctx context.Context, gameID int, userID string, orderIndex int) error
	UpdatePlayerStatus(ctx context.Context, gameID int, userID string, isActive bool, joinedAt *time.Time) error
	GetGamePlayers(ctx context.Context, gameID int) ([]*GamePlayer, error)
	GetPendingInvitations(ctx context.Context, userID string) ([]*GameInvitation, error)
	GetActiveGames(ctx context.Context, userID string) ([]*Game, error)
	UpdateGameStatus(ctx context.Context, gameID int, status string) error
}

type ChatMessage struct {
	ChatMessageID  int
	SenderUserID   string
	SenderUsername string
	Scope          string
	MessageText    string
	CreatedAt      time.Time
}

type Game struct {
	GameID       int        `json:"gameId"`
	PublicID     string     `json:"publicId"`
	CreatedBy    string     `json:"createdBy"`
	CreatedAt    time.Time  `json:"createdAt"`
	Status       string     `json:"status"`
	MaxPlayers   int        `json:"maxPlayers"`
	PlayerCount  int        `json:"playerCount"`
	FinishedAt   *time.Time `json:"finishedAt,omitempty"`
	WinnerUserID *string    `json:"winnerUserId,omitempty"`
}

type GamePlayer struct {
	GamePlayerID int
	GameID       int
	UserID       string
	Username     string
	OrderIndex   int
	JoinedAt     *time.Time
	LeftAt       *time.Time
	Score        *int
	IsActive     bool
}

type GameInvitation struct {
	GameID            int       `json:"gameId"`
	PublicID          string    `json:"publicId"`
	GamePlayerID      int       `json:"gamePlayerId"`
	InvitedBy         string    `json:"invitedBy"`
	InvitedByUsername string    `json:"invitedByUsername"`
	CreatedAt         time.Time `json:"createdAt"`
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

func (r *postgresUserRepo) GetUserByID(ctx context.Context, userID string) (*User, error) {
	var user User
	err := r.pool.QueryRow(ctx,
		"SELECT user_id, username, password, email FROM users WHERE user_id = $1", userID).
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

// Chat Repository Implementation
type postgresChatRepo struct {
	pool *pgxpool.Pool
}

func NewChatRepository(pool *pgxpool.Pool) ChatRepository {
	return &postgresChatRepo{pool: pool}
}

func (r *postgresChatRepo) SaveMessage(ctx context.Context, senderUserID, scope, messageText string) (*ChatMessage, error) {
	var msg ChatMessage
	var gameID *int
	var dbScope string

	// Parse scope - "global" or "game:123"
	if scope == "global" {
		dbScope = "global"
	} else {
		// Extract game ID from "game:123" format
		var gid int
		if _, err := fmt.Sscanf(scope, "game:%d", &gid); err == nil {
			dbScope = "game"
			gameID = &gid
		} else {
			dbScope = "global"
		}
	}

	err := r.pool.QueryRow(ctx,
		`INSERT INTO chat_messages (sender_user_id, scope, game_id, message_text) 
		 VALUES ($1, $2, $3, $4) 
		 RETURNING chat_message_id, sender_user_id, scope, message_text, created_at`,
		senderUserID, dbScope, gameID, messageText).
		Scan(&msg.ChatMessageID, &msg.SenderUserID, &msg.Scope, &msg.MessageText, &msg.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

func (r *postgresChatRepo) GetMessagesByScope(ctx context.Context, scope string, limit int) ([]*ChatMessage, error) {
	var rows pgx.Rows
	var err error

	// Parse scope - "global" or "game:123"
	if scope == "global" {
		rows, err = r.pool.Query(ctx,
			`SELECT cm.chat_message_id, cm.sender_user_id, u.username, cm.scope, cm.message_text, cm.created_at
			 FROM chat_messages cm
			 JOIN users u ON cm.sender_user_id = u.user_id
			 WHERE cm.scope = 'global'
			 ORDER BY cm.created_at DESC
			 LIMIT $1`,
			limit)
	} else {
		// Extract game ID from "game:123" format
		var gameID int
		if _, err := fmt.Sscanf(scope, "game:%d", &gameID); err == nil {
			rows, err = r.pool.Query(ctx,
				`SELECT cm.chat_message_id, cm.sender_user_id, u.username, cm.scope, cm.message_text, cm.created_at
				 FROM chat_messages cm
				 JOIN users u ON cm.sender_user_id = u.user_id
				 WHERE cm.scope = 'game' AND cm.game_id = $1
				 ORDER BY cm.created_at DESC
				 LIMIT $2`,
				gameID, limit)
		} else {
			// Invalid scope format, return empty
			return []*ChatMessage{}, nil
		}
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*ChatMessage
	for rows.Next() {
		var msg ChatMessage
		err := rows.Scan(&msg.ChatMessageID, &msg.SenderUserID, &msg.SenderUsername, &msg.Scope, &msg.MessageText, &msg.CreatedAt)
		if err != nil {
			return nil, err
		}
		messages = append(messages, &msg)
	}

	// Reverse to get chronological order (oldest first)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, rows.Err()
}

// Game Repository Implementation
type postgresGameRepo struct {
	pool *pgxpool.Pool
}

func NewGameRepository(pool *pgxpool.Pool) GameRepository {
	return &postgresGameRepo{pool: pool}
}

func (r *postgresGameRepo) CreateGame(ctx context.Context, createdByUserID string, maxPlayers int) (*Game, error) {
	var game Game
	err := r.pool.QueryRow(ctx,
		`INSERT INTO games (created_by, max_players, player_count, status) 
		 VALUES ($1, $2, 0, 'waiting_for_players') 
		 RETURNING game_id, public_id, created_by, created_at, status, max_players, player_count, finished_at, winner_user_id`,
		createdByUserID, maxPlayers).
		Scan(&game.GameID, &game.PublicID, &game.CreatedBy, &game.CreatedAt, &game.Status, &game.MaxPlayers, &game.PlayerCount, &game.FinishedAt, &game.WinnerUserID)
	if err != nil {
		return nil, err
	}
	return &game, nil
}

func (r *postgresGameRepo) GetGameByID(ctx context.Context, gameID int) (*Game, error) {
	var game Game
	err := r.pool.QueryRow(ctx,
		`SELECT game_id, public_id, created_by, created_at, status, max_players, player_count, finished_at, winner_user_id
		 FROM games WHERE game_id = $1`,
		gameID).
		Scan(&game.GameID, &game.PublicID, &game.CreatedBy, &game.CreatedAt, &game.Status, &game.MaxPlayers, &game.PlayerCount, &game.FinishedAt, &game.WinnerUserID)
	if err != nil {
		return nil, err
	}
	return &game, nil
}

func (r *postgresGameRepo) GetGameByPublicID(ctx context.Context, publicID string) (*Game, error) {
	var game Game
	err := r.pool.QueryRow(ctx,
		`SELECT game_id, public_id, created_by, created_at, status, max_players, player_count, finished_at, winner_user_id
		 FROM games WHERE public_id = $1`,
		publicID).
		Scan(&game.GameID, &game.PublicID, &game.CreatedBy, &game.CreatedAt, &game.Status, &game.MaxPlayers, &game.PlayerCount, &game.FinishedAt, &game.WinnerUserID)
	if err != nil {
		return nil, err
	}
	return &game, nil
}

func (r *postgresGameRepo) AddPlayer(ctx context.Context, gameID int, userID string, orderIndex int) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO game_players (game_id, user_id, order_index, is_active, joined_at) 
		 VALUES ($1, $2, $3, false, NULL)`,
		gameID, userID, orderIndex)
	return err
}

func (r *postgresGameRepo) UpdatePlayerStatus(ctx context.Context, gameID int, userID string, isActive bool, joinedAt *time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE game_players 
		 SET is_active = $3, joined_at = $4
		 WHERE game_id = $1 AND user_id = $2`,
		gameID, userID, isActive, joinedAt)
	return err
}

func (r *postgresGameRepo) GetGamePlayers(ctx context.Context, gameID int) ([]*GamePlayer, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT gp.game_player_id, gp.game_id, gp.user_id, u.username, gp.order_index, 
		        gp.joined_at, gp.left_at, gp.score, gp.is_active
		 FROM game_players gp
		 JOIN users u ON gp.user_id = u.user_id
		 WHERE gp.game_id = $1
		 ORDER BY gp.order_index`,
		gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var players []*GamePlayer
	for rows.Next() {
		var player GamePlayer
		err := rows.Scan(&player.GamePlayerID, &player.GameID, &player.UserID, &player.Username,
			&player.OrderIndex, &player.JoinedAt, &player.LeftAt, &player.Score, &player.IsActive)
		if err != nil {
			return nil, err
		}
		players = append(players, &player)
	}

	return players, rows.Err()
}

func (r *postgresGameRepo) GetPendingInvitations(ctx context.Context, userID string) ([]*GameInvitation, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT g.game_id, g.public_id, gp.game_player_id, g.created_by, u.username, g.created_at
		 FROM game_players gp
		 JOIN games g ON gp.game_id = g.game_id
		 JOIN users u ON g.created_by = u.user_id
		 WHERE gp.user_id = $1 
		   AND gp.is_active = false 
		   AND gp.joined_at IS NULL
		   AND g.status = 'waiting_for_players'
		 ORDER BY g.created_at DESC`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invitations []*GameInvitation
	for rows.Next() {
		var inv GameInvitation
		err := rows.Scan(&inv.GameID, &inv.PublicID, &inv.GamePlayerID, &inv.InvitedBy, &inv.InvitedByUsername, &inv.CreatedAt)
		if err != nil {
			return nil, err
		}
		invitations = append(invitations, &inv)
	}

	return invitations, rows.Err()
}

func (r *postgresGameRepo) GetActiveGames(ctx context.Context, userID string) ([]*Game, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT g.game_id, g.public_id, g.created_by, g.created_at, g.status, 
		        g.max_players, 
		        (SELECT COUNT(*) FROM game_players WHERE game_id = g.game_id AND is_active = true)::int as player_count,
		        g.finished_at, g.winner_user_id
		 FROM games g
		 JOIN game_players gp ON g.game_id = gp.game_id
		 WHERE gp.user_id = $1 
		   AND gp.is_active = true
		   AND g.status IN ('waiting_for_players', 'in_progress')
		 ORDER BY g.created_at DESC`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []*Game
	for rows.Next() {
		var game Game
		err := rows.Scan(&game.GameID, &game.PublicID, &game.CreatedBy, &game.CreatedAt,
			&game.Status, &game.MaxPlayers, &game.PlayerCount, &game.FinishedAt, &game.WinnerUserID)
		if err != nil {
			return nil, err
		}
		games = append(games, &game)
	}

	return games, rows.Err()
}

func (r *postgresGameRepo) UpdateGameStatus(ctx context.Context, gameID int, status string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE games SET status = $2 WHERE game_id = $1`,
		gameID, status)
	return err
}
