package service

import (
	"context"
	"encoding/json"
	"fmt"
	"golf-card-game/business"
	"golf-card-game/database"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// GameHub manages WebSocket connections for all game rooms
type GameHub struct {
	// Map of publicID to room
	rooms map[string]*GameRoom
	mu    sync.RWMutex
}

// GameRoom represents a single game instance with its connected players
type GameRoom struct {
	publicID   string
	clients    map[*websocket.Conn]string // conn -> userID
	broadcast  chan GameMessage
	register   chan *gameClientRegistration
	unregister chan *websocket.Conn
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

type gameClientRegistration struct {
	conn   *websocket.Conn
	userID string
}

// GameMessage represents any message sent in a game room
type GameMessage struct {
	Type    string          `json:"type"` // "chat", "state", "action", "player_joined", "player_left"
	Payload json.RawMessage `json:"payload"`
}

// ChatPayload for chat messages within a game
type ChatPayload struct {
	Message  string `json:"message"`
	Username string `json:"username"`
	Time     string `json:"time"`
}

// GameStatePayload represents the current state of the game
type GameStatePayload struct {
	PublicID        string       `json:"publicId"`
	Status          string       `json:"status"`
	Phase           string       `json:"phase"`
	CurrentPlayerID string       `json:"currentPlayerId"`
	CurrentUserId   string       `json:"currentUserId"`
	CurrentTurn     int          `json:"currentTurn"`
	Players         []PlayerInfo `json:"players"`
	YourCards       []Card       `json:"yourCards"`
	OpponentCards   []Card       `json:"opponentCards"`
	DrawnCard       *Card        `json:"drawnCard"`
	DiscardTopCard  *Card        `json:"discardTopCard"`
	DeckCount       int          `json:"deckCount"`
}

type PlayerInfo struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Score    *int   `json:"score"`
	IsActive bool   `json:"isActive"`
	IsYou    bool   `json:"isYou"`
}

type Card struct {
	Suit  string `json:"suit"`  // "back" for face-down, or actual suit
	Value string `json:"value"` // "hidden" for face-down, or actual value
	Index int    `json:"index"` // Position in grid (0-5)
}

// ActionPayload for game actions
type ActionPayload struct {
	Action string          `json:"action"` // "initial_flip", "draw_deck", "draw_discard", "swap_card", "discard_flip"
	Data   json.RawMessage `json:"data"`
}

// CardIndexData for actions that require a card index
type CardIndexData struct {
	Index int `json:"index"` // 0-5
}

// ErrorPayload for action errors
type ErrorPayload struct {
	Error string `json:"error"`
}

// Global game hub instance
var GameHubInstance = &GameHub{
	rooms: make(map[string]*GameRoom),
}

var gameRepo database.GameRepository
var gameService *business.GameService

func SetGameRepository(repo database.GameRepository) {
	gameRepo = repo
}

func SetGameService(gs *business.GameService) {
	gameService = gs
}

// GetOrCreateRoom returns an existing room or creates a new one
func (h *GameHub) GetOrCreateRoom(publicID string) *GameRoom {
	h.mu.Lock()
	defer h.mu.Unlock()

	if room, exists := h.rooms[publicID]; exists {
		return room
	}

	ctx, cancel := context.WithCancel(context.Background())
	room := &GameRoom{
		publicID:   publicID,
		clients:    make(map[*websocket.Conn]string),
		broadcast:  make(chan GameMessage, 256),
		register:   make(chan *gameClientRegistration),
		unregister: make(chan *websocket.Conn),
		ctx:        ctx,
		cancel:     cancel,
	}

	h.rooms[publicID] = room
	go room.Run()

	return room
}

// CloseRoom shuts down a game room
func (h *GameHub) CloseRoom(publicID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if room, exists := h.rooms[publicID]; exists {
		room.cancel()
		delete(h.rooms, publicID)
	}
}

// Run manages the game room's lifecycle
func (r *GameRoom) Run() {
	for {
		select {
		case <-r.ctx.Done():
			// Clean up all connections
			r.mu.Lock()
			for conn := range r.clients {
				conn.Close()
			}
			r.mu.Unlock()
			return

		case reg := <-r.register:
			r.mu.Lock()
			r.clients[reg.conn] = reg.userID
			r.mu.Unlock()

			// Send chat history for this game
			r.sendChatHistory(reg.conn)

			// Notify other players someone joined
			r.broadcastPlayerJoined(reg.userID)

			// Broadcast game state to ALL players (including the one who just joined)
			// This ensures everyone gets updated when the second player joins
			ctx := context.Background()

			// Load or initialize game state
			var state *business.FullGameState
			stateJSON, _, err := gameRepo.LoadGameState(ctx, r.publicID)

			if err != nil {
				// No state exists yet - check if we should initialize
				game, err := gameRepo.GetGameByPublicID(ctx, r.publicID)
				if err == nil && game.Status == "in_progress" {
					// Game just started, initialize the state
					players, err := gameRepo.GetGamePlayers(ctx, r.publicID)
					if err == nil {
						activePlayers := []string{}
						for _, p := range players {
							if p.IsActive {
								activePlayers = append(activePlayers, p.UserID)
							}
						}

						if len(activePlayers) == 2 {
							newState, err := gameService.InitializeGame(ctx, r.publicID, activePlayers)
							if err == nil {
								// Save the initial state
								stateJSON, _ := json.Marshal(newState)
								if err := gameRepo.SaveGameState(ctx, r.publicID, stateJSON); err == nil {
									state = newState
								}
							}
						}
					}
				}
			} else {
				// Parse existing state
				var parsedState business.FullGameState
				if err := json.Unmarshal(stateJSON, &parsedState); err == nil {
					parsedState.PublicID = r.publicID // Ensure PublicID is set
					state = &parsedState
				}
			}

			broadcastGameState(r, r.publicID, state)

		case conn := <-r.unregister:
			r.mu.Lock()
			if userID, ok := r.clients[conn]; ok {
				delete(r.clients, conn)
				conn.Close()
				r.mu.Unlock()

				// Notify other players someone left
				r.broadcastPlayerLeft(userID)
			} else {
				r.mu.Unlock()
			}

		case message := <-r.broadcast:
			// Broadcast to all connected clients in this room
			r.mu.RLock()
			for client := range r.clients {
				if err := client.WriteJSON(message); err != nil {
					log.Printf("Error broadcasting to client in game %s: %v", r.publicID, err)
					client.Close()
					delete(r.clients, client)
				}
			}
			r.mu.RUnlock()
		}
	}
}

func (r *GameRoom) sendGameState(conn *websocket.Conn, userID string) {
	if gameRepo == nil || gameService == nil {
		return
	}

	ctx := context.Background()
	game, err := gameRepo.GetGameByPublicID(ctx, r.publicID)
	if err != nil {
		log.Printf("Error getting game: %v", err)
		return
	}

	players, err := gameRepo.GetGamePlayers(ctx, r.publicID)
	if err != nil {
		log.Printf("Error getting players: %v", err)
		return
	}

	// Try to load existing game state
	stateJSON, _, err := gameRepo.LoadGameState(ctx, r.publicID)
	var state *business.FullGameState

	if err != nil {
		// No state exists yet - check if we should initialize
		if game.Status == "in_progress" {
			// Game just started, initialize the state
			activePlayers := []string{}
			for _, p := range players {
				if p.IsActive {
					activePlayers = append(activePlayers, p.UserID)
				}
			}

			if len(activePlayers) == 2 {
				newState, err := gameService.InitializeGame(ctx, r.publicID, activePlayers)
				if err != nil {
					log.Printf("Error initializing game: %v", err)
					return
				}

				// Save the initial state
				stateJSON, _ := json.Marshal(newState)
				if err := gameRepo.SaveGameState(ctx, r.publicID, stateJSON); err != nil {
					log.Printf("Error saving initial state: %v", err)
					return
				}

				state = newState
			} else {
				log.Printf("Cannot initialize game: need 2 active players, have %d", len(activePlayers))
				// Send waiting state without game state
				state = nil
			}
		} else {
			// Game not started yet, send waiting state
			log.Printf("Game in waiting_for_players status, sending lobby state")
			state = nil
		}
	} else {
		// Parse existing state
		var parsedState business.FullGameState
		if err := json.Unmarshal(stateJSON, &parsedState); err != nil {
			log.Printf("Error parsing game state: %v", err)
			return
		}
		parsedState.PublicID = r.publicID // Ensure PublicID is set
		state = &parsedState
	}

	// Build and send personalized state
	statePayload := buildGameStatePayload(game, state, players, userID)
	payload, _ := json.Marshal(statePayload)
	msg := GameMessage{
		Type:    "state",
		Payload: payload,
	}

	if err := conn.WriteJSON(msg); err != nil {
		log.Printf("Error sending game state: %v", err)
	}
}

func (r *GameRoom) sendChatHistory(conn *websocket.Conn) {
	if chatRepo == nil {
		return
	}

	ctx := context.Background()
	scope := fmt.Sprintf("game:%s", r.publicID)
	messages, err := chatRepo.GetMessagesByScope(ctx, scope, 50)
	if err != nil {
		log.Printf("Error fetching game chat history: %v", err)
		return
	}

	for _, msg := range messages {
		chatPayload := ChatPayload{
			Message:  msg.MessageText,
			Username: msg.SenderUsername,
			Time:     msg.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		payload, _ := json.Marshal(chatPayload)
		gameMsg := GameMessage{
			Type:    "chat",
			Payload: payload,
		}
		if err := conn.WriteJSON(gameMsg); err != nil {
			log.Printf("Error sending chat history: %v", err)
		}
	}
}

func (r *GameRoom) broadcastPlayerJoined(userID string) {
	payload, _ := json.Marshal(map[string]string{"userId": userID})
	msg := GameMessage{
		Type:    "player_joined",
		Payload: payload,
	}
	r.broadcast <- msg
}

func (r *GameRoom) broadcastPlayerLeft(userID string) {
	payload, _ := json.Marshal(map[string]string{"userId": userID})
	msg := GameMessage{
		Type:    "player_left",
		Payload: payload,
	}
	r.broadcast <- msg
}

// GameWebSocketHandler handles WebSocket connections for a specific game
func GameWebSocketHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get user from session
	userID, ok := ctx.Value(userIDKey).(string)
	if !ok || userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Extract publicID from URL path - expecting /api/ws/game/{publicId}
	// Parse public ID from path
	path := r.URL.Path
	var publicID string
	fmt.Sscanf(path, "/api/ws/game/%s", &publicID)

	if publicID == "" {
		http.Error(w, "Invalid game ID", http.StatusBadRequest)
		return
	}

	// Validate user is in the game
	if gameService == nil || gameRepo == nil {
		http.Error(w, "Service not initialized", http.StatusInternalServerError)
		return
	}

	inGame, err := gameService.ValidateUserInGame(ctx, publicID, userID)
	if err != nil {
		log.Printf("Error validating user in game: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !inGame {
		http.Error(w, "You are not a player in this game", http.StatusForbidden)
		return
	}

	// Get username
	user, err := userService.GetUserByID(ctx, userID)
	if err != nil {
		log.Printf("Error getting user: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	// Get or create room for this game
	room := GameHubInstance.GetOrCreateRoom(publicID)

	// Register client
	room.register <- &gameClientRegistration{
		conn:   conn,
		userID: userID,
	}

	defer func() {
		room.unregister <- conn
	}()

	// Configure connection for heartbeat
	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Start ping ticker
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	done := make(chan struct{})
	defer close(done)

	// Start goroutine to send pings
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()

	// Listen for messages from client
	for {
		var msg GameMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			// Only log unexpected close errors (exclude normal closures, going away, and no status)
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle different message types
		switch msg.Type {
		case "chat":
			var chatPayload ChatPayload
			if err := json.Unmarshal(msg.Payload, &chatPayload); err != nil {
				log.Printf("Error unmarshaling chat payload: %v", err)
				continue
			}

			// Validate message length (max 500 characters)
			if len(chatPayload.Message) > 500 {
				log.Printf("Message too long from user %s in game %s: %d characters", userID, publicID, len(chatPayload.Message))
				continue
			}

			// Save message to database with game scope
			if chatRepo != nil {
				scope := fmt.Sprintf("game:%s", publicID)
				savedMsg, err := chatRepo.SaveMessage(ctx, userID, scope, chatPayload.Message)
				if err != nil {
					log.Printf("Error saving game chat message: %v", err)
					continue
				}

				// Broadcast to room
				broadcastPayload := ChatPayload{
					Message:  savedMsg.MessageText,
					Username: user.Username,
					Time:     savedMsg.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
				}
				payload, _ := json.Marshal(broadcastPayload)
				room.broadcast <- GameMessage{
					Type:    "chat",
					Payload: payload,
				}
			}

		case "action":
			// Handle game actions
			var actionPayload ActionPayload
			if err := json.Unmarshal(msg.Payload, &actionPayload); err != nil {
				log.Printf("Failed to unmarshal action payload: %v", err)
				sendError(conn, "Invalid action payload")
				continue
			}

			// Load current game state
			stateJSON, version, err := gameRepo.LoadGameState(context.Background(), publicID)
			if err != nil {
				log.Printf("Failed to load game state: %v", err)
				sendError(conn, "Failed to load game state")
				continue
			}

			var state business.FullGameState
			if err := json.Unmarshal(stateJSON, &state); err != nil {
				log.Printf("Failed to unmarshal game state: %v", err)
				sendError(conn, "Failed to parse game state")
				continue
			}
			state.PublicID = publicID // Ensure PublicID is set

			// Execute action based on type
			var actionErr error
			switch actionPayload.Action {
			case "initial_flip":
				var data CardIndexData
				if err := json.Unmarshal(actionPayload.Data, &data); err != nil {
					sendError(conn, "Invalid card index")
					continue
				}
				actionErr = gameService.InitialFlipCard(&state, userID, data.Index)

			case "draw_deck":
				actionErr = gameService.DrawFromDeck(&state, userID)

			case "draw_discard":
				actionErr = gameService.DrawFromDiscard(&state, userID)

			case "swap_card":
				var data CardIndexData
				if err := json.Unmarshal(actionPayload.Data, &data); err != nil {
					sendError(conn, "Invalid card index")
					continue
				}
				actionErr = gameService.SwapCard(&state, userID, data.Index)

			case "discard_flip":
				var data CardIndexData
				if err := json.Unmarshal(actionPayload.Data, &data); err != nil {
					sendError(conn, "Invalid card index")
					continue
				}
				actionErr = gameService.DiscardAndFlip(&state, userID, data.Index)

			default:
				sendError(conn, fmt.Sprintf("Unknown action: %s", actionPayload.Action))
				continue
			}

			// Handle action errors
			if actionErr != nil {
				log.Printf("Action error for user %s: %v", userID, actionErr)
				sendError(conn, actionErr.Error())
				continue
			}

			// Save updated state with optimistic locking
			updatedStateJSON, err := json.Marshal(state)
			if err != nil {
				log.Printf("Failed to marshal updated state: %v", err)
				sendError(conn, "Failed to save game state")
				continue
			}

			err = gameRepo.UpdateGameState(context.Background(), publicID, updatedStateJSON, version)
			if err != nil {
				log.Printf("Failed to update game state: %v", err)
				sendError(conn, "Failed to save game state (version conflict)")
				continue
			}

			// Check if game is finished
			if state.Phase == business.PhaseFinished {
				winnerUserID, err := gameService.FinishGame(context.Background(), &state)
				if err != nil {
					log.Printf("Failed to finish game: %v", err)
				} else {
					log.Printf("Game %s finished, winner: %s", publicID, winnerUserID)

					// Save state again after flipping remaining cards
					finalStateJSON, _ := json.Marshal(state)
					gameRepo.UpdateGameState(context.Background(), publicID, finalStateJSON, version+1)

					// Broadcast game end notification
					broadcastGameEnd(room, publicID, &state, winnerUserID)
				}
			}

			// Broadcast updated state to all players
			broadcastGameState(room, publicID, &state)

		default:
			log.Printf("Unknown message type: %s", msg.Type)
		}
	}
}

// sendError sends an error message to a specific client
func sendError(conn *websocket.Conn, errorMsg string) {
	errPayload, _ := json.Marshal(ErrorPayload{Error: errorMsg})
	msg := GameMessage{
		Type:    "error",
		Payload: errPayload,
	}
	if err := conn.WriteJSON(msg); err != nil {
		log.Printf("Failed to send error message: %v", err)
	}
}

// broadcastGameState broadcasts the current game state to all clients in the room
func broadcastGameState(room *GameRoom, publicID string, state *business.FullGameState) {
	// Get game from database for status
	game, err := gameRepo.GetGameByPublicID(context.Background(), publicID)
	if err != nil {
		log.Printf("Failed to get game: %v", err)
		return
	}

	// Get players with usernames
	players, err := gameRepo.GetGamePlayers(context.Background(), publicID)
	if err != nil {
		log.Printf("Failed to get players: %v", err)
		return
	}

	// Send personalized state to each connected client
	room.mu.RLock()
	defer room.mu.RUnlock()

	for conn, userID := range room.clients {
		statePayload := buildGameStatePayload(game, state, players, userID)
		payload, _ := json.Marshal(statePayload)
		msg := GameMessage{
			Type:    "state",
			Payload: payload,
		}

		if err := conn.WriteJSON(msg); err != nil {
			log.Printf("Failed to send state to user %s: %v", userID, err)
		}
	}
}

// GameEndPayload for game end notification
type GameEndPayload struct {
	WinnerUserID   string         `json:"winnerUserId"`
	WinnerUsername string         `json:"winnerUsername"`
	Scores         map[string]int `json:"scores"`
}

// broadcastGameEnd sends game end notification to all players
func broadcastGameEnd(room *GameRoom, publicID string, state *business.FullGameState, winnerUserID string) {
	// Get players to get usernames
	players, err := gameRepo.GetGamePlayers(context.Background(), publicID)
	if err != nil {
		log.Printf("Failed to get players: %v", err)
		return
	}

	// Build scores map and find winner username
	scores := business.GetFinalScores(state)
	var winnerUsername string
	for _, p := range players {
		if p.UserID == winnerUserID {
			winnerUsername = p.Username
			break
		}
	}

	endPayload := GameEndPayload{
		WinnerUserID:   winnerUserID,
		WinnerUsername: winnerUsername,
		Scores:         scores,
	}

	payload, _ := json.Marshal(endPayload)
	msg := GameMessage{
		Type:    "game_end",
		Payload: payload,
	}

	room.mu.RLock()
	defer room.mu.RUnlock()

	for conn := range room.clients {
		if err := conn.WriteJSON(msg); err != nil {
			log.Printf("Failed to send game end notification: %v", err)
		}
	}
}

// buildGameStatePayload creates a personalized state payload for a specific user
func buildGameStatePayload(game *database.Game, state *business.FullGameState, dbPlayers []*database.GamePlayer, viewerUserID string) GameStatePayload {
	// Build player info list - only include active players for frontend
	// (DB still tracks invited players with is_active=false)
	playerInfos := make([]PlayerInfo, 0, len(dbPlayers))
	for _, p := range dbPlayers {
		// Only include active players in the frontend state
		if p.IsActive {
			playerInfos = append(playerInfos, PlayerInfo{
				UserID:   p.UserID,
				Username: p.Username,
				Score:    p.Score,
				IsActive: p.IsActive,
				IsYou:    p.UserID == viewerUserID,
			})
		}
	}

	// If no game state yet (waiting for players), return minimal payload
	if state == nil {
		return GameStatePayload{
			PublicID:        game.PublicID,
			Status:          game.Status,
			Phase:           "waiting",
			CurrentPlayerID: "",
			CurrentUserId:   viewerUserID,
			CurrentTurn:     0,
			Players:         playerInfos,
			YourCards:       []Card{},
			OpponentCards:   []Card{},
			DrawnCard:       nil,
			DiscardTopCard:  nil,
			DeckCount:       0,
		}
	}

	// Find viewer's player index
	var yourCards []Card
	var opponentCards []Card
	var currentPlayerID string

	if len(state.Players) > 0 {
		currentPlayerID = state.Players[state.CurrentTurnIdx].UserID
	}

	for _, player := range state.Players {
		cards := make([]Card, 6)
		isViewer := player.UserID == viewerUserID

		for i := 0; i < 6; i++ {
			if player.FaceUp[i] {
				// Show actual card if face-up (visible to everyone)
				cards[i] = Card{
					Suit:  player.Hand[i].Suit,
					Value: player.Hand[i].Rank,
					Index: i,
				}
			} else {
				// Hide face-down cards
				cards[i] = Card{
					Suit:  "back",
					Value: "hidden",
					Index: i,
				}
			}
		}

		if isViewer {
			yourCards = cards
		} else {
			opponentCards = cards
		}
	}

	// Convert drawn card (only show to current player if it's their turn)
	var drawnCard *Card
	if state.DrawnCard != nil && currentPlayerID == viewerUserID {
		drawnCard = &Card{
			Suit:  state.DrawnCard.Suit,
			Value: state.DrawnCard.Rank,
			Index: -1, // Not in grid yet
		}
	}

	// Convert discard pile top card
	var discardTopCard *Card
	if len(state.DiscardPile) > 0 {
		topCard := state.DiscardPile[len(state.DiscardPile)-1]
		discardTopCard = &Card{
			Suit:  topCard.Suit,
			Value: topCard.Rank,
			Index: -1,
		}
	}

	return GameStatePayload{
		PublicID:        game.PublicID,
		Status:          game.Status,
		Phase:           string(state.Phase),
		CurrentPlayerID: currentPlayerID,
		CurrentUserId:   viewerUserID,
		CurrentTurn:     state.CurrentTurnIdx,
		Players:         playerInfos,
		YourCards:       yourCards,
		OpponentCards:   opponentCards,
		DrawnCard:       drawnCard,
		DiscardTopCard:  discardTopCard,
		DeckCount:       len(state.Deck),
	}
}
