package business

import (
	"context"
	"crypto/rand"
	"encoding/binary"
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

	// Game action errors
	ErrNotYourTurn        = errors.New("it is not your turn")
	ErrInvalidPhase       = errors.New("action not allowed in current game phase")
	ErrInvalidCardIndex   = errors.New("invalid card index")
	ErrCardAlreadyFaceUp  = errors.New("card is already face-up")
	ErrInvalidInitialFlip = errors.New("initial flip must be one card from top row and one from bottom row")
	ErrNoDrawnCard        = errors.New("no card has been drawn yet")
	ErrCardAlreadyDrawn   = errors.New("a card has already been drawn this turn")
	ErrEmptyDeck          = errors.New("deck is empty")
	ErrEmptyDiscard       = errors.New("discard pile is empty")
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

	// Filter out players who have declined or left (left_at is set)
	activePlayers := make([]*database.GamePlayer, 0)
	for _, player := range players {
		if player.LeftAt == nil {
			activePlayers = append(activePlayers, player)
		}
	}

	// Check if game is full (only count active/pending players)
	if len(activePlayers) >= game.MaxPlayers {
		return ErrGameFull
	}

	// Check if user is already in the game (active or invited, but not declined)
	for _, player := range activePlayers {
		if player.UserID == invitedUserID {
			if player.IsActive {
				return ErrAlreadyInGame
			}
			return ErrAlreadyInvited
		}
	}

	// Validate inviter is in the game and active
	inviterInGame := false
	for _, player := range activePlayers {
		if player.UserID == inviterUserID && player.IsActive {
			inviterInGame = true
			break
		}
	}

	if !inviterInGame {
		return errors.New("inviter is not an active player in this game")
	}

	// Add player with is_active=false, joined_at=NULL (pending invitation)
	// Order index is based on current active/pending player count
	orderIndex := len(activePlayers)
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

// findPlayerIndex returns the index of a player by their userID
func findPlayerIndex(state *FullGameState, userID string) (int, error) {
	for i, player := range state.Players {
		if player.UserID == userID {
			return i, nil
		}
	}
	return -1, errors.New("player not found in game")
}

// InitialFlipCard handles flipping a card during the initial flip phase
func (s *GameService) InitialFlipCard(state *FullGameState, userID string, cardIndex int) error {
	if state.Phase != PhaseInitialFlip {
		return ErrInvalidPhase
	}

	playerIdx, err := findPlayerIndex(state, userID)
	if err != nil {
		return err
	}

	// Validate card index
	if cardIndex < 0 || cardIndex > 5 {
		return ErrInvalidCardIndex
	}

	player := &state.Players[playerIdx]

	// Check if card is already face-up
	if player.FaceUp[cardIndex] {
		return ErrCardAlreadyFaceUp
	}

	// Check if player has completed their 2 flips
	if player.InitialFlips >= 2 {
		return errors.New("you have already flipped 2 cards")
	}

	// Validate one from top row (0-2) and one from bottom row (3-5)
	if player.InitialFlips == 1 {
		// Check if this flip follows the rule
		hasTopRow := false
		hasBottomRow := false

		for i := 0; i < 6; i++ {
			if player.FaceUp[i] {
				if i < 3 {
					hasTopRow = true
				} else {
					hasBottomRow = true
				}
			}
		}

		// The new card must be from the other row
		if cardIndex < 3 && hasTopRow {
			return ErrInvalidInitialFlip
		}
		if cardIndex >= 3 && hasBottomRow {
			return ErrInvalidInitialFlip
		}
	}

	// Flip the card
	player.FaceUp[cardIndex] = true
	player.InitialFlips++

	// Check if both players have completed initial flips
	allPlayersReady := true
	for _, p := range state.Players {
		if p.InitialFlips < 2 {
			allPlayersReady = false
			break
		}
	}

	// If all players ready, transition to main game
	if allPlayersReady {
		state.Phase = PhaseMainGame
		state.CurrentTurnIdx = 0 // First player goes first
	}

	return nil
}

// DrawFromDeck draws the top card from the deck
func (s *GameService) DrawFromDeck(state *FullGameState, userID string) error {
	if state.Phase != PhaseMainGame && state.Phase != PhaseFinalRound {
		return ErrInvalidPhase
	}

	playerIdx, err := findPlayerIndex(state, userID)
	if err != nil {
		return err
	}

	if playerIdx != state.CurrentTurnIdx {
		return ErrNotYourTurn
	}

	if state.DrawnCard != nil {
		return ErrCardAlreadyDrawn
	}

	if len(state.Deck) == 0 {
		return ErrEmptyDeck
	}

	// Draw top card from deck
	state.DrawnCard = &state.Deck[0]
	state.Deck = state.Deck[1:]

	return nil
}

// DrawFromDiscard draws the top card from the discard pile
func (s *GameService) DrawFromDiscard(state *FullGameState, userID string) error {
	if state.Phase != PhaseMainGame && state.Phase != PhaseFinalRound {
		return ErrInvalidPhase
	}

	playerIdx, err := findPlayerIndex(state, userID)
	if err != nil {
		return err
	}

	if playerIdx != state.CurrentTurnIdx {
		return ErrNotYourTurn
	}

	if state.DrawnCard != nil {
		return ErrCardAlreadyDrawn
	}

	if len(state.DiscardPile) == 0 {
		return ErrEmptyDiscard
	}

	// Draw top card from discard pile (last element)
	lastIdx := len(state.DiscardPile) - 1
	state.DrawnCard = &state.DiscardPile[lastIdx]
	state.DiscardPile = state.DiscardPile[:lastIdx]

	return nil
}

// SwapCard swaps the drawn card with a card in the player's hand
func (s *GameService) SwapCard(state *FullGameState, userID string, cardIndex int) error {
	if state.Phase != PhaseMainGame && state.Phase != PhaseFinalRound {
		return ErrInvalidPhase
	}

	playerIdx, err := findPlayerIndex(state, userID)
	if err != nil {
		return err
	}

	if playerIdx != state.CurrentTurnIdx {
		return ErrNotYourTurn
	}

	if state.DrawnCard == nil {
		return ErrNoDrawnCard
	}

	// Validate card index
	if cardIndex < 0 || cardIndex > 5 {
		return ErrInvalidCardIndex
	}

	player := &state.Players[playerIdx]

	// Swap the cards
	oldCard := player.Hand[cardIndex]
	player.Hand[cardIndex] = *state.DrawnCard
	player.FaceUp[cardIndex] = true // Card becomes face-up

	// Put old card on discard pile
	state.DiscardPile = append(state.DiscardPile, oldCard)
	state.DrawnCard = nil

	// Check if all cards are face-up
	player.AllCardsFlipped = checkAllCardsFlipped(player)

	// End turn and check for game end
	return s.endTurn(state, playerIdx)
}

// DiscardAndFlip discards the drawn card and flips one of the player's cards
func (s *GameService) DiscardAndFlip(state *FullGameState, userID string, cardIndex int) error {
	if state.Phase != PhaseMainGame && state.Phase != PhaseFinalRound {
		return ErrInvalidPhase
	}

	playerIdx, err := findPlayerIndex(state, userID)
	if err != nil {
		return err
	}

	if playerIdx != state.CurrentTurnIdx {
		return ErrNotYourTurn
	}

	if state.DrawnCard == nil {
		return ErrNoDrawnCard
	}

	// Validate card index
	if cardIndex < 0 || cardIndex > 5 {
		return ErrInvalidCardIndex
	}

	player := &state.Players[playerIdx]

	// Check if card is already face-up
	if player.FaceUp[cardIndex] {
		return ErrCardAlreadyFaceUp
	}

	// Discard the drawn card
	state.DiscardPile = append(state.DiscardPile, *state.DrawnCard)
	state.DrawnCard = nil

	// Flip the chosen card
	player.FaceUp[cardIndex] = true

	// Check if all cards are face-up
	player.AllCardsFlipped = checkAllCardsFlipped(player)

	// End turn and check for game end
	return s.endTurn(state, playerIdx)
}

// checkAllCardsFlipped checks if all 6 cards in a player's hand are face-up
func checkAllCardsFlipped(player *PlayerState) bool {
	for _, faceUp := range player.FaceUp {
		if !faceUp {
			return false
		}
	}
	return true
}

// endTurn handles end of turn logic and checks for game end conditions
func (s *GameService) endTurn(state *FullGameState, currentPlayerIdx int) error {
	player := &state.Players[currentPlayerIdx]

	// Check if this player just flipped all their cards
	if player.AllCardsFlipped && state.Phase == PhaseMainGame {
		// Trigger final round
		state.Phase = PhaseFinalRound
		state.TriggerPlayerIdx = &currentPlayerIdx
		// Each other player gets one more turn
		state.FinalRoundTurns = len(state.Players) - 1
	}

	// If in final round, decrement turns left BEFORE moving to next player
	if state.Phase == PhaseFinalRound {
		state.FinalRoundTurns--
		if state.FinalRoundTurns < 0 {
			// All players have had their final turn
			state.Phase = PhaseFinished
		}
	}

	// Move to next player
	state.CurrentTurnIdx = (state.CurrentTurnIdx + 1) % len(state.Players)

	return nil
}

// getCardValue returns the point value of a card
func getCardValue(card CardDef) int {
	switch card.Rank {
	case "A":
		return 1
	case "2":
		return 2
	case "3":
		return 3
	case "4":
		return 4
	case "5":
		return 5
	case "6":
		return 6
	case "7":
		return 7
	case "8":
		return 8
	case "9":
		return 9
	case "10":
		return 10
	case "J", "Q", "K":
		return 10
	case "Joker":
		return -2
	default:
		return 0
	}
}

// CalculateScore computes a player's score with column matching rules
func CalculateScore(player *PlayerState) int {
	totalScore := 0

	// Check each column (3 columns: 0,3 | 1,4 | 2,5)
	for col := 0; col < 3; col++ {
		topIdx := col        // Top row: 0, 1, 2
		bottomIdx := col + 3 // Bottom row: 3, 4, 5

		topCard := player.Hand[topIdx]
		bottomCard := player.Hand[bottomIdx]

		// Check if both cards are face-up and have matching ranks
		if player.FaceUp[topIdx] && player.FaceUp[bottomIdx] &&
			topCard.Rank == bottomCard.Rank {
			// Matching column - both cards cancel to 0 points
			continue
		}

		// Add points for face-up cards
		if player.FaceUp[topIdx] {
			totalScore += getCardValue(topCard)
		}
		if player.FaceUp[bottomIdx] {
			totalScore += getCardValue(bottomCard)
		}
	}

	return totalScore
}

// flipRemainingCards flips all face-down cards for all players
func flipRemainingCards(state *FullGameState) {
	for i := range state.Players {
		player := &state.Players[i]
		for j := 0; j < 6; j++ {
			if !player.FaceUp[j] {
				player.FaceUp[j] = true
			}
		}
		player.AllCardsFlipped = true
	}
}

// FinishGame calculates final scores, determines winner, and updates database
func (s *GameService) FinishGame(ctx context.Context, state *FullGameState) (string, error) {
	if state.Phase != PhaseFinished {
		return "", errors.New("game is not finished yet")
	}

	// Flip all remaining cards before scoring
	flipRemainingCards(state)

	// Calculate scores for all players
	scores := make(map[string]int)
	lowestScore := int(^uint(0) >> 1) // Max int
	var winnerUserID string

	for i := range state.Players {
		player := &state.Players[i]
		score := CalculateScore(player)
		scores[player.UserID] = score

		if score < lowestScore {
			lowestScore = score
			winnerUserID = player.UserID
		}
	}

	// Update player scores in database
	for userID, score := range scores {
		err := s.gameRepo.UpdatePlayerScore(ctx, state.GameID, userID, score)
		if err != nil {
			return "", fmt.Errorf("failed to update player score: %w", err)
		}
	}

	// Update game status to finished with winner and timestamp
	err := s.gameRepo.FinishGame(ctx, state.GameID, winnerUserID)
	if err != nil {
		return "", fmt.Errorf("failed to finish game: %w", err)
	}

	return winnerUserID, nil
}

// GetFinalScores returns the scores for all players
func GetFinalScores(state *FullGameState) map[string]int {
	scores := make(map[string]int)
	for i := range state.Players {
		player := &state.Players[i]
		scores[player.UserID] = CalculateScore(player)
	}
	return scores
}
