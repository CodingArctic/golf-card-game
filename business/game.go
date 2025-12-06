package business

import (
	"context"
	"errors"
	"fmt"
	"golf-card-game/database"
	"time"
)

var (
	ErrGameNotFound      = errors.New("game not found")
	ErrGameFull          = errors.New("game is full")
	ErrAlreadyInvited    = errors.New("user already invited to this game")
	ErrAlreadyInGame     = errors.New("user is already in this game")
	ErrInvalidGameStatus = errors.New("game is not in a valid state for this operation")
	ErrNotInvited        = errors.New("user is not invited to this game")
	ErrCannotInviteSelf  = errors.New("cannot invite yourself")
)

type GameService struct {
	gameRepo database.GameRepository
	userRepo database.UserRepository
}

func NewGameService(gameRepo database.GameRepository, userRepo database.UserRepository) *GameService {
	return &GameService{
		gameRepo: gameRepo,
		userRepo: userRepo,
	}
}

// CreateGame creates a new 1v1 game and adds the creator as the first player
func (s *GameService) CreateGame(ctx context.Context, createdByUserID string) (*database.Game, error) {
	// Create game with max 2 players for 1v1
	game, err := s.gameRepo.CreateGame(ctx, createdByUserID, 2)
	if err != nil {
		return nil, fmt.Errorf("failed to create game: %w", err)
	}

	// Add creator as first player (order_index = 0, is_active = true, joined immediately)
	err = s.gameRepo.AddPlayer(ctx, game.GameID, createdByUserID, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to add creator to game: %w", err)
	}

	now := time.Now()
	err = s.gameRepo.UpdatePlayerStatus(ctx, game.GameID, createdByUserID, true, &now)
	if err != nil {
		return nil, fmt.Errorf("failed to activate creator: %w", err)
	}

	// Reload game to get updated player count
	game, err = s.gameRepo.GetGameByID(ctx, game.GameID)
	if err != nil {
		return nil, fmt.Errorf("failed to reload game: %w", err)
	}

	return game, nil
}

// InvitePlayer adds a player to the game as a pending invitation
func (s *GameService) InvitePlayer(ctx context.Context, gameID int, invitedUserID, inviterUserID string) error {
	// Validate inviter is not inviting themselves
	if invitedUserID == inviterUserID {
		return ErrCannotInviteSelf
	}

	// Check if invited user exists
	_, err := s.userRepo.GetUserByID(ctx, invitedUserID)
	if err != nil {
		return fmt.Errorf("invited user not found: %w", err)
	}

	// Get game and validate
	game, err := s.gameRepo.GetGameByID(ctx, gameID)
	if err != nil {
		return ErrGameNotFound
	}

	if game.Status != "waiting_for_players" {
		return ErrInvalidGameStatus
	}

	// Get current players
	players, err := s.gameRepo.GetGamePlayers(ctx, gameID)
	if err != nil {
		return fmt.Errorf("failed to get game players: %w", err)
	}

	// Check if game is full
	if len(players) >= game.MaxPlayers {
		return ErrGameFull
	}

	// Check if user is already in the game (active or invited)
	for _, player := range players {
		if player.UserID == invitedUserID {
			if player.IsActive {
				return ErrAlreadyInGame
			}
			return ErrAlreadyInvited
		}
	}

	// Validate inviter is in the game and active
	inviterInGame := false
	for _, player := range players {
		if player.UserID == inviterUserID && player.IsActive {
			inviterInGame = true
			break
		}
	}

	if !inviterInGame {
		return errors.New("inviter is not an active player in this game")
	}

	// Add player with is_active=false, joined_at=NULL (pending invitation)
	// Order index is based on current player count
	orderIndex := len(players)
	err = s.gameRepo.AddPlayer(ctx, gameID, invitedUserID, orderIndex)
	if err != nil {
		return fmt.Errorf("failed to invite player: %w", err)
	}

	return nil
}

// AcceptInvitation activates a player's participation in a game
func (s *GameService) AcceptInvitation(ctx context.Context, gameID int, userID string) error {
	// Get game
	game, err := s.gameRepo.GetGameByID(ctx, gameID)
	if err != nil {
		return ErrGameNotFound
	}

	if game.Status != "waiting_for_players" {
		return ErrInvalidGameStatus
	}

	// Get players
	players, err := s.gameRepo.GetGamePlayers(ctx, gameID)
	if err != nil {
		return fmt.Errorf("failed to get game players: %w", err)
	}

	// Find the user's player record
	var userPlayer *database.GamePlayer
	for _, player := range players {
		if player.UserID == userID {
			userPlayer = player
			break
		}
	}

	if userPlayer == nil {
		return ErrNotInvited
	}

	if userPlayer.IsActive {
		return ErrAlreadyInGame
	}

	// Activate the player
	now := time.Now()
	err = s.gameRepo.UpdatePlayerStatus(ctx, gameID, userID, true, &now)
	if err != nil {
		return fmt.Errorf("failed to accept invitation: %w", err)
	}

	// Count active players
	activeCount := 0
	for _, player := range players {
		if player.IsActive || player.UserID == userID {
			activeCount++
		}
	}

	// If we now have max players, start the game
	if activeCount >= game.MaxPlayers {
		err = s.gameRepo.UpdateGameStatus(ctx, gameID, "in_progress")
		if err != nil {
			return fmt.Errorf("failed to start game: %w", err)
		}
	}

	return nil
}

// DeclineInvitation removes a pending invitation
func (s *GameService) DeclineInvitation(ctx context.Context, gameID int, userID string) error {
	// Get players
	players, err := s.gameRepo.GetGamePlayers(ctx, gameID)
	if err != nil {
		return fmt.Errorf("failed to get game players: %w", err)
	}

	// Find the user's player record
	var userPlayer *database.GamePlayer
	for _, player := range players {
		if player.UserID == userID {
			userPlayer = player
			break
		}
	}

	if userPlayer == nil {
		return ErrNotInvited
	}

	if userPlayer.IsActive {
		return errors.New("cannot decline - already accepted")
	}

	// Mark as left (alternative: could delete the record entirely)
	now := time.Now()
	err = s.gameRepo.UpdatePlayerStatus(ctx, gameID, userID, false, &now)
	if err != nil {
		return fmt.Errorf("failed to decline invitation: %w", err)
	}

	return nil
}

// GetGameWithPlayers retrieves a game and its players
func (s *GameService) GetGameWithPlayers(ctx context.Context, gameID int) (*database.Game, []*database.GamePlayer, error) {
	game, err := s.gameRepo.GetGameByID(ctx, gameID)
	if err != nil {
		return nil, nil, ErrGameNotFound
	}

	players, err := s.gameRepo.GetGamePlayers(ctx, gameID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get players: %w", err)
	}

	return game, players, nil
}

// GetPendingInvitations retrieves all pending invitations for a user
func (s *GameService) GetPendingInvitations(ctx context.Context, userID string) ([]*database.GameInvitation, error) {
	invitations, err := s.gameRepo.GetPendingInvitations(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get invitations: %w", err)
	}
	return invitations, nil
}

// GetActiveGames retrieves all active games for a user
func (s *GameService) GetActiveGames(ctx context.Context, userID string) ([]*database.Game, error) {
	games, err := s.gameRepo.GetActiveGames(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active games: %w", err)
	}
	return games, nil
}

// GetGameByPublicID retrieves a game by its public ID (for URL-based access)
func (s *GameService) GetGameByPublicID(ctx context.Context, publicID string) (*database.Game, error) {
	game, err := s.gameRepo.GetGameByPublicID(ctx, publicID)
	if err != nil {
		return nil, ErrGameNotFound
	}
	return game, nil
}

// ValidateUserInGame checks if a user is an active player in a game
func (s *GameService) ValidateUserInGame(ctx context.Context, gameID int, userID string) (bool, error) {
	players, err := s.gameRepo.GetGamePlayers(ctx, gameID)
	if err != nil {
		return false, fmt.Errorf("failed to get players: %w", err)
	}

	for _, player := range players {
		if player.UserID == userID && player.IsActive {
			return true, nil
		}
	}

	return false, nil
}
