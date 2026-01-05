package service

import (
	"encoding/json"
	"golf-card-game/business"
	"log"
	"net/http"
)

// CreateGameHandler creates a new game
func CreateGameHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	ctx := r.Context()
	userID, ok := ctx.Value(userIDKey).(string)
	if !ok || userID == "" {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	if gameService == nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Service not initialized"})
		return
	}

	game, err := gameService.CreateGame(ctx, userID)
	if err != nil {
		log.Printf("Error creating game: %v", err)
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create game"})
		return
	}

	jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"publicId": game.PublicID,
		"status":   game.Status,
	})
}

// InvitePlayerHandler invites a player to a game
func InvitePlayerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	ctx := r.Context()
	userID, ok := ctx.Value(userIDKey).(string)
	if !ok || userID == "" {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	var req struct {
		PublicID        string `json:"publicId"`
		InvitedUsername string `json:"invitedUsername"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if req.InvitedUsername == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "InvitedUsername is required"})
		return
	}

	if gameService == nil || userService == nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Service not initialized"})
		return
	}

	// Get the invited user by username
	invitedUser, err := userService.GetUser(ctx, req.InvitedUsername)
	if err != nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "User not found"})
		return
	}

	err = gameService.InvitePlayer(ctx, req.PublicID, invitedUser.UserID, userID)
	if err != nil {
		switch err {
		case business.ErrCannotInviteSelf:
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Cannot invite yourself"})
		case business.ErrGameNotFound:
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Game not found"})
		case business.ErrGameFull:
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Game is full"})
		case business.ErrAlreadyInvited:
			jsonResponse(w, http.StatusConflict, map[string]string{"error": "User already invited"})
		case business.ErrAlreadyInGame:
			jsonResponse(w, http.StatusConflict, map[string]string{"error": "User already in game"})
		case business.ErrInvalidGameStatus:
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Game is not accepting invitations"})
		default:
			log.Printf("Error inviting player: %v", err)
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to invite player"})
		}
		return
	}

	// Get game details for the notification
	game, _, err := gameService.GetGameWithPlayers(ctx, req.PublicID)
	if err == nil {
		// Get inviter username
		inviter, err := userService.GetUserByID(ctx, userID)
		if err == nil {
			// Send WebSocket notification to the invited user
			Hub.SendNotificationToUser(invitedUser.UserID, LobbyMessage{
				Type: "invitation_received",
				Payload: InvitationPayload{
					PublicID:        game.PublicID,
					InviterUsername: inviter.Username,
				},
			})
		}
	}

	jsonResponse(w, http.StatusOK, map[string]string{"message": "Invitation sent"})
}

// AcceptInvitationHandler accepts a game invitation
func AcceptInvitationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	ctx := r.Context()
	userID, ok := ctx.Value(userIDKey).(string)
	if !ok || userID == "" {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	var req struct {
		PublicID string `json:"publicId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if gameService == nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Service not initialized"})
		return
	}

	err := gameService.AcceptInvitation(ctx, req.PublicID, userID)
	if err != nil {
		switch err {
		case business.ErrGameNotFound:
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Game not found"})
		case business.ErrNotInvited:
			jsonResponse(w, http.StatusForbidden, map[string]string{"error": "Not invited to this game"})
		case business.ErrAlreadyInGame:
			jsonResponse(w, http.StatusConflict, map[string]string{"error": "Already in game"})
		case business.ErrInvalidGameStatus:
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Game is not accepting players"})
		default:
			log.Printf("Error accepting invitation: %v", err)
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to accept invitation"})
		}
		return
	}

	// Get game details and notify all active players
	game, players, err := gameService.GetGameWithPlayers(ctx, req.PublicID)
	if err == nil {
		// Get acceptor username
		acceptor, err := userService.GetUserByID(ctx, userID)
		if err == nil {
			// Notify all active players (except the acceptor)
			for _, player := range players {
				if player.UserID != userID && player.IsActive {
					Hub.SendNotificationToUser(player.UserID, LobbyMessage{
						Type: "invitation_accepted",
						Payload: InvitationPayload{
							PublicID:        game.PublicID,
							InviteeUsername: acceptor.Username,
						},
					})
				}
			}
		}
	}

	jsonResponse(w, http.StatusOK, map[string]string{"message": "Invitation accepted"})
}

// DeclineInvitationHandler declines a game invitation
func DeclineInvitationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	ctx := r.Context()
	userID, ok := ctx.Value(userIDKey).(string)
	if !ok || userID == "" {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	var req struct {
		PublicID string `json:"publicId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if gameService == nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Service not initialized"})
		return
	}

	err := gameService.DeclineInvitation(ctx, req.PublicID, userID)
	if err != nil {
		switch err {
		case business.ErrGameNotFound:
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Game not found"})
		case business.ErrNotInvited:
			jsonResponse(w, http.StatusForbidden, map[string]string{"error": "Not invited to this game"})
		default:
			log.Printf("Error declining invitation: %v", err)
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to decline invitation"})
		}
		return
	}

	// Get game details and notify all active players
	game, players, err := gameService.GetGameWithPlayers(ctx, req.PublicID)
	if err == nil {
		// Get decliner username
		decliner, err := userService.GetUserByID(ctx, userID)
		if err == nil {
			// Notify all active players
			for _, player := range players {
				if player.IsActive {
					Hub.SendNotificationToUser(player.UserID, LobbyMessage{
						Type: "invitation_declined",
						Payload: InvitationPayload{
							PublicID:        game.PublicID,
							InviteeUsername: decliner.Username,
						},
					})
				}
			}
		}
	}

	jsonResponse(w, http.StatusOK, map[string]string{"message": "Invitation declined"})
}

// ListGamesHandler returns pending invitations and active games for a user
func ListGamesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	ctx := r.Context()
	userID, ok := ctx.Value(userIDKey).(string)
	if !ok || userID == "" {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	if gameService == nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Service not initialized"})
		return
	}

	// Get pending invitations
	invitations, err := gameService.GetPendingInvitations(ctx, userID)
	if err != nil {
		log.Printf("Error getting invitations: %v", err)
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get invitations"})
		return
	}

	// Get active games
	activeGames, err := gameService.GetActiveGames(ctx, userID)
	if err != nil {
		log.Printf("Error getting active games: %v", err)
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get active games"})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"invitations": invitations,
		"activeGames": activeGames,
	})
}

// GetGameHandler returns game details with players
func GetGameHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	ctx := r.Context()
	userID, ok := ctx.Value(userIDKey).(string)
	if !ok || userID == "" {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	// Parse public ID from query parameter
	publicID := r.URL.Query().Get("publicId")
	if publicID == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "publicId query parameter is required"})
		return
	}

	if gameService == nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Service not initialized"})
		return
	}

	// Validate user has access to this game
	inGame, err := gameService.ValidateUserInGame(ctx, publicID, userID)
	if err != nil {
		log.Printf("Error validating user in game: %v", err)
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to validate access"})
		return
	}
	if !inGame {
		jsonResponse(w, http.StatusForbidden, map[string]string{"error": "You are not a player in this game"})
		return
	}

	game, players, err := gameService.GetGameWithPlayers(ctx, publicID)
	if err != nil {
		if err == business.ErrGameNotFound {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Game not found"})
		} else {
			log.Printf("Error getting game: %v", err)
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get game"})
		}
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"game":    game,
		"players": players,
	})
}
