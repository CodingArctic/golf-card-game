package business

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// NonceManager handles creation and validation of registration nonces
type NonceManager struct {
	nonces map[string]*NonceData
	mu     sync.RWMutex
}

// NonceData contains the nonce information and validation data
type NonceData struct {
	Token     string
	IPAddress string
	UserAgent string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// NewNonceManager creates a new nonce manager instance
func NewNonceManager() *NonceManager {
	nm := &NonceManager{
		nonces: make(map[string]*NonceData),
	}

	// Start cleanup goroutine to remove expired nonces
	go nm.cleanupExpiredNonces()

	return nm
}

// GenerateNonce creates a new nonce token with user information
func (nm *NonceManager) GenerateNonce(ipAddress, userAgent string) (string, error) {
	// Generate random bytes for the nonce
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random nonce: %w", err)
	}

	// Create a unique token by combining random data with user info and timestamp
	timestamp := time.Now().Unix()
	dataToHash := fmt.Sprintf("%s:%s:%s:%d",
		base64.URLEncoding.EncodeToString(randomBytes),
		ipAddress,
		userAgent,
		timestamp,
	)

	// Hash the combined data to create the token
	hash := sha256.Sum256([]byte(dataToHash))
	token := hex.EncodeToString(hash[:])

	// Store the nonce data with 15 minute expiration
	nm.mu.Lock()
	defer nm.mu.Unlock()

	now := time.Now()
	nm.nonces[token] = &NonceData{
		Token:     token,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		CreatedAt: now,
		ExpiresAt: now.Add(15 * time.Minute),
	}

	return token, nil
}

// ValidateNonce checks if a nonce is valid and matches the request information
func (nm *NonceManager) ValidateNonce(token, ipAddress, userAgent string) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	// Check if nonce exists
	nonce, exists := nm.nonces[token]
	if !exists {
		return fmt.Errorf("invalid or expired nonce token")
	}

	// Check if nonce has expired
	if time.Now().After(nonce.ExpiresAt) {
		delete(nm.nonces, token)
		return fmt.Errorf("nonce token has expired")
	}

	// Validate IP address matches
	if nonce.IPAddress != ipAddress {
		return fmt.Errorf("IP address mismatch")
	}

	// Validate user agent matches
	if nonce.UserAgent != userAgent {
		return fmt.Errorf("user agent mismatch")
	}

	// Token is valid - remove it so it can only be used once
	delete(nm.nonces, token)

	return nil
}

// cleanupExpiredNonces runs periodically to remove expired nonces
func (nm *NonceManager) cleanupExpiredNonces() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		nm.mu.Lock()
		now := time.Now()
		for token, nonce := range nm.nonces {
			if now.After(nonce.ExpiresAt) {
				delete(nm.nonces, token)
			}
		}
		nm.mu.Unlock()
	}
}
