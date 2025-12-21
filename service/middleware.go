package service

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const userIDKey contextKey = "userID"

// SessionMiddleware ensures that requests have a valid 'session' cookie
// except for public endpoints like /login, /register, and static assets
func SessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSuffix(r.URL.Path, "/")

		// Allow unauthenticated access for public endpoints
		if path == "/" ||
			path == "" ||
			path == "/index.txt" ||
			path == "/favicon.ico" ||
			path == "/api/login" ||
			path == "/api/register" ||
			path == "/api/register/nonce" ||
			path == "/api/logout" ||
			strings.HasPrefix(r.URL.Path, "/login") ||
			strings.HasPrefix(r.URL.Path, "/register") ||
			strings.HasPrefix(r.URL.Path, "/instructions") ||
			strings.HasPrefix(r.URL.Path, "/static/") ||
			strings.HasPrefix(r.URL.Path, "/_next/") {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie("session")
		if err != nil || cookie.Value == "" {
			// Return 401 for API requests, redirect for page requests
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Validate the session token
		userID, err := userService.ValidateSession(r.Context(), cookie.Value)
		if err != nil {
			// Return 401 for API requests, redirect for page requests
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		// Add userID to context
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		// Continue to the underlying handler
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
