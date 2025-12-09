package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"golf-card-game/business"
	"golf-card-game/database"
	"net/http"
	"os"
)

var userService *business.UserService
var nonceManager *business.NonceManager

// SetUserService sets the user service dependency
func SetUserService(us *business.UserService) {
	userService = us
}

// SetNonceManager sets the nonce manager dependency
func SetNonceManager(nm *business.NonceManager) {
	nonceManager = nm
}

type registerRequest struct {
	Username       string `json:"username"`
	Password       string `json:"password"`
	Email          string `json:"email"`
	Nonce          string `json:"nonce"`
	TurnstileToken string `json:"turnstileToken"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// GetRegistrationNonceHandler generates and returns a nonce token for registration
func GetRegistrationNonceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	// Extract client information
	ipAddress := getClientIP(r)
	userAgent := r.Header.Get("User-Agent")

	// Generate nonce
	nonce, err := nonceManager.GenerateNonce(ipAddress, userAgent)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to generate nonce"})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"nonce": nonce})
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

	// Verify Turnstile token
	ipAddress := getClientIP(r)
	if err := verifyTurnstileToken(req.TurnstileToken, ipAddress); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Captcha verification failed"})
		return
	}

	// Validate nonce before proceeding with registration
	userAgent := r.Header.Get("User-Agent")

	if err := nonceManager.ValidateNonce(req.Nonce, ipAddress, userAgent); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid or expired registration token"})
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

// getClientIP extracts the client's IP address from the request
// It checks X-Forwarded-For and X-Real-IP headers for proxied requests
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (common for proxied requests)
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// X-Forwarded-For can contain multiple IPs separated by commas
		// Take the first one
		for i := 0; i < len(forwarded); i++ {
			if forwarded[i] == ',' {
				return forwarded[:i]
			}
		}
		return forwarded
	}

	// Check X-Real-IP header (another common proxy header)
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr, but strip the port
	ip := r.RemoteAddr
	// Remove port if present (IPv4 format: "ip:port" or IPv6 format: "[ip]:port")
	for i := len(ip) - 1; i >= 0; i-- {
		if ip[i] == ':' {
			// Check if this is IPv6 by looking for brackets
			if i > 0 && ip[0] == '[' {
				// IPv6 format: [ip]:port
				return ip[1 : i-1]
			}
			// IPv4 format: ip:port
			return ip[:i]
		}
	}
	return ip
}

// verifyTurnstileToken verifies a Cloudflare Turnstile token
func verifyTurnstileToken(token, remoteIP string) error {
	secretKey := os.Getenv("TURNSTILE_SECRET_KEY")
	if secretKey == "" {
		return fmt.Errorf("turnstile secret key not configured")
	}

	// Prepare the request to Cloudflare's siteverify API
	reqBody := map[string]string{
		"secret":   secretKey,
		"response": token,
		"remoteip": remoteIP,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post(
		"https://challenges.cloudflare.com/turnstile/v0/siteverify",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("failed to verify token: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Success     bool     `json:"success"`
		ErrorCodes  []string `json:"error-codes"`
		ChallengeTS string   `json:"challenge_ts"`
		Hostname    string   `json:"hostname"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("verification failed: %v", result.ErrorCodes)
	}

	return nil
}
