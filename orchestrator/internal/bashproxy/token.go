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
	mu             sync.RWMutex
	tokens         map[string]*types.BashJobContext
	statusTrackers map[string]*LiveStatusTracker // Maps token to status tracker
	timers         map[string]*time.Timer         // Maps token to cleanup timer
	timeout        time.Duration
}

// NewTokenManager creates a new token manager
func NewTokenManager(timeout time.Duration) *TokenManager {
	return &TokenManager{
		tokens:         make(map[string]*types.BashJobContext),
		statusTrackers: make(map[string]*LiveStatusTracker),
		timers:         make(map[string]*time.Timer),
		timeout:        timeout,
	}
}

// GenerateToken creates a new authentication token and associates it with a job context
func (tm *TokenManager) GenerateToken(ctx *types.BashJobContext, statusTracker *LiveStatusTracker) (string, error) {
	// Generate a random token (32 bytes = 64 hex characters)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	// Store the token with its context and status tracker
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.tokens[token] = ctx
	tm.statusTrackers[token] = statusTracker

	// Schedule cleanup after timeout using a timer that can be stopped
	if tm.timeout > 0 {
		timer := time.AfterFunc(tm.timeout, func() {
			tm.RevokeToken(token)
		})
		tm.timers[token] = timer
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

// RevokeToken removes a token from the manager and stops its cleanup timer
func (tm *TokenManager) RevokeToken(token string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Stop the cleanup timer if it exists
	if timer, exists := tm.timers[token]; exists {
		timer.Stop()
		delete(tm.timers, token)
	}

	delete(tm.tokens, token)
	delete(tm.statusTrackers, token)
}

// GetStatusTracker returns the status tracker for a token
func (tm *TokenManager) GetStatusTracker(token string) *LiveStatusTracker {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.statusTrackers[token]
}


// GetTokenCount returns the number of active tokens (useful for testing/debugging)
func (tm *TokenManager) GetTokenCount() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.tokens)
}
