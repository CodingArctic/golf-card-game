package business

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"golf-card-game/database"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	userRepo database.UserRepository // Interface, not concrete type
}

func NewUserService(userRepo database.UserRepository) *UserService {
	return &UserService{userRepo: userRepo}
}

func (s *UserService) GetUser(ctx context.Context, uuid string) (*database.User, error) {
	// Add business logic here if needed
	return s.userRepo.GetUserByUsername(ctx, uuid)
}

// RegisterUser creates a new user with a hashed password
func (s *UserService) RegisterUser(ctx context.Context, username, password, email string) (*database.User, error) {
	// Validate inputs
	if username == "" || password == "" {
		return nil, errors.New("username and password are required")
	}

	if len(password) < 8 {
		return nil, errors.New("password must be at least 8 characters")
	}

	// Check if user already exists
	exists, err := s.userRepo.UserExists(ctx, username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, database.ErrUserAlreadyExists
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// Create the user
	user, err := s.userRepo.CreateUser(ctx, username, string(hashedPassword), email)
	if err != nil {
		// Handle database-level errors (e.g., email constraint violation)
		if errors.Is(err, database.ErrUserAlreadyExists) || errors.Is(err, database.ErrEmailAlreadyExists) {
			return nil, err
		}
		return nil, err
	}

	return user, nil
}

// LoginUser validates credentials and returns a session token
func (s *UserService) LoginUser(ctx context.Context, username, password string) (string, error) {
	// Get user from database
	user, err := s.userRepo.GetUserByUsername(ctx, username)
	if err != nil {
		return "", errors.New("invalid username or password")
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return "", errors.New("invalid username or password")
	}

	// Generate session token
	token, err := generateSecureToken()
	if err != nil {
		return "", err
	}

	// Create session (expires in 24 hours)
	expiresAt := time.Now().Add(24 * time.Hour)
	err = s.userRepo.CreateSession(ctx, user.UserID, token, expiresAt)
	if err != nil {
		return "", err
	}

	return token, nil
}

// ValidateSession checks if a session token is valid and returns the user ID
func (s *UserService) ValidateSession(ctx context.Context, token string) (string, error) {
	return s.userRepo.ValidateSession(ctx, token)
}

// LogoutUser deletes the session
func (s *UserService) LogoutUser(ctx context.Context, token string) error {
	return s.userRepo.DeleteSession(ctx, token)
}

// generateSecureToken creates a cryptographically secure random token
// TODO - replace with specific token generation methods discussed in class
func generateSecureToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
