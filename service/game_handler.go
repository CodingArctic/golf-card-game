package service

import (
	"context"
	"encoding/json"
	"fmt"
	"golf-card-game/business"
	"golf-card-game/database"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/websocket"
)

// GameHub manages WebSocket connections for all game rooms
type GameHub struct {
	// Map of gameID to room
	rooms map[int]*GameRoom
	mu    sync.RWMutex
}

// GameRoom represents a single game instance with its connected players
type GameRoom struct {
	gameID     int
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
	GameID        int          `json:"gameId"`
	Status        string       `json:"status"`
	CurrentTurn   int          `json:"currentTurn"`
	Players       []PlayerInfo `json:"players"`
	YourCards     []Card       `json:"yourCards"`
	OpponentCards []Card       `json:"opponentCards"`
}

type PlayerInfo struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Score    *int   `json:"score"`
	IsActive bool   `json:"isActive"`
}

type Card struct {
	Suit  string `json:"suit"`  // "back" for face-down, or actual suit
	Value string `json:"value"` // "hidden" for face-down, or actual value
	Index int    `json:"index"` // Position in grid (0-5)
}

// Global game hub instance
var GameHubInstance = &GameHub{
	rooms: make(map[int]*GameRoom),
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
func (h *GameHub) GetOrCreateRoom(gameID int) *GameRoom {
	h.mu.Lock()
	defer h.mu.Unlock()

	if room, exists := h.rooms[gameID]; exists {
		return room
	}

	ctx, cancel := context.WithCancel(context.Background())
	room := &GameRoom{
		gameID:     gameID,
		clients:    make(map[*websocket.Conn]string),
		broadcast:  make(chan GameMessage, 256),
		register:   make(chan *gameClientRegistration),
		unregister: make(chan *websocket.Conn),
		ctx:        ctx,
		cancel:     cancel,
	}

	h.rooms[gameID] = room
	go room.Run()

	return room
}

// CloseRoom shuts down a game room
func (h *GameHub) CloseRoom(gameID int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if room, exists := h.rooms[gameID]; exists {
		room.cancel()
		delete(h.rooms, gameID)
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

			// Send game state to the new client
			r.sendGameState(reg.conn, reg.userID)

			// Send chat history for this game
			r.sendChatHistory(reg.conn)

			// Notify other players someone joined
			r.broadcastPlayerJoined(reg.userID)

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
					log.Printf("Error broadcasting to client in game %d: %v", r.gameID, err)
					client.Close()
					delete(r.clients, client)
				}
			}
			r.mu.RUnlock()
		}
	}
}

func (r *GameRoom) sendGameState(conn *websocket.Conn, userID string) {
	if gameRepo == nil {
		return
	}

	ctx := context.Background()
	game, err := gameRepo.GetGameByID(ctx, r.gameID)
	if err != nil {
		log.Printf("Error getting game: %v", err)
		return
	}

	players, err := gameRepo.GetGamePlayers(ctx, r.gameID)
	if err != nil {
		log.Printf("Error getting players: %v", err)
		return
	}

	// Build player info
	var playerInfos []PlayerInfo
	for _, p := range players {
		if p.IsActive {
			playerInfos = append(playerInfos, PlayerInfo{
				UserID:   p.UserID,
				Username: p.Username,
				Score:    p.Score,
				IsActive: p.IsActive,
			})
		}
	}

	// Initialize face-down cards (6 cards each player in golf)
	yourCards := make([]Card, 6)
	opponentCards := make([]Card, 6)
	for i := 0; i < 6; i++ {
		yourCards[i] = Card{Suit: "back", Value: "hidden", Index: i}
		opponentCards[i] = Card{Suit: "back", Value: "hidden", Index: i}
	}

	state := GameStatePayload{
		GameID:        game.GameID,
		Status:        game.Status,
		CurrentTurn:   0, // TODO: Get from game state
		Players:       playerInfos,
		YourCards:     yourCards,
		OpponentCards: opponentCards,
	}

	payload, _ := json.Marshal(state)
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
	scope := fmt.Sprintf("game:%d", r.gameID)
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

	// Extract gameID from URL path - expecting /api/ws/game/{gameId}
	// Parse game ID from path
	path := r.URL.Path
	var gameIDStr string
	fmt.Sscanf(path, "/api/ws/game/%s", &gameIDStr)

	gameID, err := strconv.Atoi(gameIDStr)
	if err != nil {
		http.Error(w, "Invalid game ID", http.StatusBadRequest)
		return
	}

	// Validate user is in the game
	if gameService == nil || gameRepo == nil {
		http.Error(w, "Service not initialized", http.StatusInternalServerError)
		return
	}

	inGame, err := gameService.ValidateUserInGame(ctx, gameID, userID)
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
	room := GameHubInstance.GetOrCreateRoom(gameID)

	// Register client
	room.register <- &gameClientRegistration{
		conn:   conn,
		userID: userID,
	}

	defer func() {
		room.unregister <- conn
	}()

	// Listen for messages from client
	for {
		var msg GameMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
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

			// Save message to database with game scope
			if chatRepo != nil {
				scope := fmt.Sprintf("game:%d", gameID)
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
			// TODO: Handle game actions (draw card, flip card, end turn, etc.)
			log.Printf("Game action received from user %s in game %d", userID, gameID)
			// For now, just echo back
			room.broadcast <- msg

		default:
			log.Printf("Unknown message type: %s", msg.Type)
		}
	}
}
