package bashproxy

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/tom/laforge/orchestrator/internal/types"
)

// TokenManager manages bash proxy authentication tokens
type TokenManager struct {
	mu      sync.RWMutex
	tokens  map[string]*types.BashJobContext
	timeout time.Duration
}

// NewTokenManager creates a new token manager
func NewTokenManager(timeout time.Duration) *TokenManager {
	return &TokenManager{
		tokens:  make(map[string]*types.BashJobContext),
		timeout: timeout,
	}
}

// GenerateToken creates a new authentication token and associates it with a job context
func (tm *TokenManager) GenerateToken(ctx *types.BashJobContext) (string, error) {
	// Generate a random token (32 bytes = 64 hex characters)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	// Store the token with its context
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.tokens[token] = ctx

	// Schedule cleanup after timeout
	if tm.timeout > 0 {
		go tm.scheduleCleanup(token)
	}

	return token, nil
}

// ValidateToken checks if a token is valid and returns its job context
func (tm *TokenManager) ValidateToken(token string) (*types.BashJobContext, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	ctx, ok := tm.tokens[token]
	return ctx, ok
}

// RevokeToken removes a token from the manager
func (tm *TokenManager) RevokeToken(token string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.tokens, token)
}

// scheduleCleanup removes a token after the timeout expires
func (tm *TokenManager) scheduleCleanup(token string) {
	time.Sleep(tm.timeout)
	tm.RevokeToken(token)
}

// GetTokenCount returns the number of active tokens (useful for testing/debugging)
func (tm *TokenManager) GetTokenCount() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.tokens)
}
