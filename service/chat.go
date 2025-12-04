package service

import (
	"context"
	"encoding/json"
	"golf-card-game/database"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// upgrader converts an incoming HTTP request to a WebSocket connection.
// CheckOrigin returns true to allow all origins during local development.
// In production, tighten this to validate r.Origin against allowed hosts.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for local development
	},
}

// ChatMessage is the payload exchanged over WebSockets.
type ChatMessage struct {
	Message  string `json:"message"`
	Username string `json:"username"`
	Time     string `json:"time"`
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
						chatMsg := ChatMessage{
							Message:  msg.MessageText,
							Username: msg.SenderUsername,
							Time:     msg.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
						}
						if err := reg.conn.WriteJSON(chatMsg); err != nil {
							log.Printf("Error sending history: %v", err)
						}
					}
				}
			}

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			// Broadcast to all connected clients
			h.mu.RLock()
			for client := range h.clients {
				if err := client.WriteJSON(message); err != nil {
					log.Printf("Error broadcasting: %v", err)
					client.Close()
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
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

	for {
		var msg ChatMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
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
