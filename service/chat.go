package service

import (
	"context"
	"encoding/json"
	"golf-card-game/database"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 90 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = 20 * time.Second

	// Maximum message size allowed from peer.
	maxMessageSize = 512 * 1024
)

// upgrader converts an incoming HTTP request to a WebSocket connection.
// CheckOrigin returns true to allow all origins during local development.
// In production, tighten this to validate r.Origin against allowed hosts.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for local development
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// ChatMessage is the payload exchanged over WebSockets.
type ChatMessage struct {
	Message  string `json:"message"`
	Username string `json:"username"`
	Time     string `json:"time"`
}

// LobbyMessage wraps different message types for the lobby
type LobbyMessage struct {
	Type    string      `json:"type"` // "chat", "player_list"
	Payload interface{} `json:"payload"`
}

// PlayerListPayload contains the list of online players
type PlayerListPayload struct {
	Players []string `json:"players"`
}

var chatRepo database.ChatRepository

func SetChatRepository(repo database.ChatRepository) {
	chatRepo = repo
}

// ChatHub coordinates all chat activity.
type ChatHub struct {
	clients    map[*websocket.Conn]string // maps connection to userID
	broadcast  chan ChatMessage
	register   chan *clientRegistration
	unregister chan *websocket.Conn
	mu         sync.RWMutex
}

type clientRegistration struct {
	conn   *websocket.Conn
	userID string
}

// Hub is the single global instance used by the server.
var Hub = &ChatHub{
	clients:    make(map[*websocket.Conn]string),
	broadcast:  make(chan ChatMessage),
	register:   make(chan *clientRegistration),
	unregister: make(chan *websocket.Conn),
}

func (h *ChatHub) Run() {
	ctx := context.Background()

	for {
		select {
		case reg := <-h.register:
			h.mu.Lock()
			h.clients[reg.conn] = reg.userID
			h.mu.Unlock()

			// Send chat history to the new client from database
			if chatRepo != nil {
				messages, err := chatRepo.GetMessagesByScope(ctx, "global", 50)
				if err != nil {
					log.Printf("Error fetching chat history: %v", err)
				} else {
					for _, msg := range messages {
						lobbyMsg := LobbyMessage{
							Type: "chat",
							Payload: ChatMessage{
								Message:  msg.MessageText,
								Username: msg.SenderUsername,
								Time:     msg.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
							},
						}
						if err := reg.conn.WriteJSON(lobbyMsg); err != nil {
							log.Printf("Error sending history: %v", err)
						}
					}
				}
			}

			// Broadcast updated player list to all clients
			h.broadcastPlayerList()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			h.mu.Unlock()

			// Broadcast updated player list to all clients
			h.broadcastPlayerList()

		case message := <-h.broadcast:
			// Broadcast chat message to all connected clients
			h.mu.RLock()
			lobbyMsg := LobbyMessage{
				Type:    "chat",
				Payload: message,
			}
			for client := range h.clients {
				if err := client.WriteJSON(lobbyMsg); err != nil {
					log.Printf("Error broadcasting: %v", err)
					client.Close()
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// broadcastPlayerList sends the current list of online players to all connected clients
func (h *ChatHub) broadcastPlayerList() {
	ctx := context.Background()

	h.mu.RLock()
	userIDs := make([]string, 0, len(h.clients))
	for _, userID := range h.clients {
		userIDs = append(userIDs, userID)
	}
	h.mu.RUnlock()

	// Get usernames for all connected users
	usernames := make([]string, 0, len(userIDs))
	for _, userID := range userIDs {
		user, err := userService.GetUserByID(ctx, userID)
		if err != nil {
			log.Printf("Error getting user: %v", err)
			continue
		}
		usernames = append(usernames, user.Username)
	}

	// Create player list message
	lobbyMsg := LobbyMessage{
		Type: "player_list",
		Payload: PlayerListPayload{
			Players: usernames,
		},
	}

	// Broadcast to all clients
	h.mu.RLock()
	for client := range h.clients {
		if err := client.WriteJSON(lobbyMsg); err != nil {
			log.Printf("Error broadcasting player list: %v", err)
		}
	}
	h.mu.RUnlock()
}

func ChatHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get user from session
	userID, ok := ctx.Value(userIDKey).(string)
	if !ok || userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get username
	user, err := userService.GetUserByID(ctx, userID)
	if err != nil {
		log.Printf("Error getting user: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	Hub.register <- &clientRegistration{
		conn:   conn,
		userID: userID,
	}

	defer func() {
		Hub.unregister <- conn
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
		log.Printf("Starting ping goroutine for user %s", userID)
		for {
			select {
			case <-done:
				log.Printf("Ping goroutine stopping for user %s", userID)
				return
			case <-ticker.C:
				log.Printf("Sending ping to user %s", userID)
				conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					log.Printf("Ping error for user %s: %v", userID, err)
					return
				}
				log.Printf("Ping sent successfully to user %s", userID)
			}
		}
	}()

	for {
		var msg ChatMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			// Only log unexpected close errors (exclude normal closures, going away, and no status)
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Save message to database
		if chatRepo != nil {
			savedMsg, err := chatRepo.SaveMessage(ctx, userID, "global", msg.Message)
			if err != nil {
				log.Printf("Error saving message: %v", err)
				continue
			}

			// Broadcast with username and saved timestamp
			broadcastMsg := ChatMessage{
				Message:  savedMsg.MessageText,
				Username: user.Username,
				Time:     savedMsg.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			}
			Hub.broadcast <- broadcastMsg
		}
	}
}

// GetChatHistoryHandler returns chat history from database as JSON.
func GetChatHistoryHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if chatRepo == nil {
		http.Error(w, "Chat repository not initialized", http.StatusInternalServerError)
		return
	}

	messages, err := chatRepo.GetMessagesByScope(ctx, "global", 50)
	if err != nil {
		log.Printf("Error fetching chat history: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}
