package service

import (
	"encoding/json"
	"log"
	"net/http"
)

// jsonResponse writes the given payload as JSON with the provided status code.
// If encoding fails, it logs the error and writes a 500 response.
func jsonResponse(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// scope the err variable to the 'if' block to avoid shadowing issues
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("json encode error: %v", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
	}
}
