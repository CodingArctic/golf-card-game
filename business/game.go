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

// CardDef represents a single playing card in the game
type CardDef struct {
	Suit string `json:"suit"` // "hearts", "diamonds", "clubs", "spades", "joker"
	Rank string `json:"rank"` // "A", "2"-"10", "J", "Q", "K", "Joker"
}

// GamePhase represents the current phase of the game
type GamePhase string

const (
	PhaseInitialFlip GamePhase = "initial_flip" // Players selecting their 2 initial cards to flip
	PhaseMainGame    GamePhase = "main_game"    // Normal turn-based gameplay
	PhaseFinalRound  GamePhase = "final_round"  // One player flipped all cards, others get last turn
	PhaseFinished    GamePhase = "finished"     // Game completed
)

// PlayerState represents a single player's game state
type PlayerState struct {
	UserID          string     `json:"userId"`
	Hand            [6]CardDef `json:"hand"`            // Player's 6 cards in 3x2 grid
	FaceUp          [6]bool    `json:"faceUp"`          // Which cards are revealed (true = face-up)
	InitialFlips    int        `json:"initialFlips"`    // Count of initial flips (0-2)
	AllCardsFlipped bool       `json:"allCardsFlipped"` // True when all 6 cards are face-up
}

// FullGameState represents the complete state of a game
type FullGameState struct {
	GameID           int           `json:"gameId"`
	Phase            GamePhase     `json:"phase"`
	Deck             []CardDef     `json:"deck"`             // Remaining cards to draw from
	DiscardPile      []CardDef     `json:"discardPile"`      // Face-up discard stack (last card is top)
	Players          []PlayerState `json:"players"`          // Player states (indexed by order_index)
	CurrentTurnIdx   int           `json:"currentTurnIdx"`   // Index into Players array for whose turn it is
	DrawnCard        *CardDef      `json:"drawnCard"`        // Card currently drawn (waiting for swap/discard decision)
	TriggerPlayerIdx *int          `json:"triggerPlayerIdx"` // Index of player who flipped all cards (triggers final round)
	FinalRoundTurns  int           `json:"finalRoundTurns"`  // Remaining turns in final round
	Version          int           `json:"version"`          // For optimistic locking
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

// Game Engine Functions

// createDeck creates a shuffled standard deck with 2 jokers (54 cards total)
func createDeck() []CardDef {
	suits := []string{"hearts", "diamonds", "clubs", "spades"}
	ranks := []string{"A", "2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K"}

	deck := make([]CardDef, 0, 54)

	// Add standard 52 cards
	for _, suit := range suits {
		for _, rank := range ranks {
			deck = append(deck, CardDef{Suit: suit, Rank: rank})
		}
	}

	// Add 2 jokers
	deck = append(deck, CardDef{Suit: "joker", Rank: "Joker"})
	deck = append(deck, CardDef{Suit: "joker", Rank: "Joker"})

	// Shuffle using Fisher-Yates algorithm
	for i := len(deck) - 1; i > 0; i-- {
		j := randInt(i + 1)
		deck[i], deck[j] = deck[j], deck[i]
	}

	return deck
}

// randInt returns a cryptographically random integer in range [0, n)
func randInt(n int) int {
	if n <= 0 {
		return 0
	}

	var b [8]byte
	_, err := rand.Read(b[:])
	if err != nil {
		// Fallback to time-based if crypto/rand fails
		return int(time.Now().UnixNano()) % n
	}

	return int(binary.BigEndian.Uint64(b[:]) % uint64(n))
}

// InitializeGame creates the initial game state when all players have joined
func (s *GameService) InitializeGame(ctx context.Context, gameID int, playerUserIDs []string) (*FullGameState, error) {
	if len(playerUserIDs) != 2 {
		return nil, errors.New("game requires exactly 2 players")
	}

	// Create and shuffle deck
	deck := createDeck()

	// Deal 6 cards to each player
	players := make([]PlayerState, 2)
	for i := 0; i < 2; i++ {
		var hand [6]CardDef
		for j := 0; j < 6; j++ {
			hand[j] = deck[0]
			deck = deck[1:]
		}

		players[i] = PlayerState{
			UserID:          playerUserIDs[i],
			Hand:            hand,
			FaceUp:          [6]bool{false, false, false, false, false, false},
			InitialFlips:    0,
			AllCardsFlipped: false,
		}
	}

	// Create discard pile with first card from deck
	discardPile := []CardDef{deck[0]}
	deck = deck[1:]

	// Create initial game state
	state := &FullGameState{
		GameID:           gameID,
		Phase:            PhaseInitialFlip,
		Deck:             deck,
		DiscardPile:      discardPile,
		Players:          players,
		CurrentTurnIdx:   0,
		DrawnCard:        nil,
		TriggerPlayerIdx: nil,
		FinalRoundTurns:  0,
		Version:          1,
	}

	return state, nil
}
