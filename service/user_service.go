package service

import (
	"encoding/json"
	"errors"
	"golf-card-game/business"
	"golf-card-game/database"
	"net/http"
)

var userService *business.UserService

// SetUserService sets the user service dependency
func SetUserService(us *business.UserService) {
	userService = us
}

type registerRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// RegisterHandler creates a new user account
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	user, err := userService.RegisterUser(r.Context(), req.Username, req.Password, req.Email)
	if err != nil {
		// Handle specific error types with appropriate status codes
		if errors.Is(err, database.ErrUserAlreadyExists) {
			jsonResponse(w, http.StatusConflict, map[string]string{"error": "Username already exists"})
			return
		}
		if errors.Is(err, database.ErrEmailAlreadyExists) {
			jsonResponse(w, http.StatusConflict, map[string]string{"error": "Email already exists"})
			return
		}
		// Generic bad request for validation errors
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"message": "User created successfully",
		"user": map[string]string{
			"user_id":  user.UserID,
			"username": user.Username,
			"email":    user.Email,
		},
	})
}

// LoginHandler obtains a session token from business logic and sets it as a cookie
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	token, err := userService.LoginUser(r.Context(), req.Username, req.Password)
	if err != nil {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	// Set cookie (HttpOnly for security; Secure would be added if served over HTTPS)
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		// Secure: true, // enable when using HTTPS
		// Lax mode allows cross-site "safe" requests like GET, but not POST
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400, // 24 hours in seconds
	})

	jsonResponse(w, http.StatusOK, map[string]string{"message": "Logged in successfully"})
}

// LogoutHandler deletes the user's session
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	cookie, err := r.Cookie("session")
	if err == nil && cookie.Value != "" {
		_ = userService.LogoutUser(r.Context(), cookie.Value)
	}

	// Clear the session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		// Secure:   true, // enable when using HTTPS
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1, // Delete cookie
	})

	jsonResponse(w, http.StatusOK, map[string]string{"message": "Logged out successfully"})
}
