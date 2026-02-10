package bashproxy

import (
	"fmt"
	"sync"
	"time"
)

// FormatCommentHeader creates a standardized header for live status comments
func FormatCommentHeader(promptType, model string) string {
	return fmt.Sprintf("🤖 **Laforge Agent** — `%s` with `%s`", promptType, model)
}

const (
	// MaxStatusMessages is the maximum number of status messages to keep
	// Prevents unbounded growth of comment body
	MaxStatusMessages = 50
)

// StatusMessage represents a single status update with timestamp
type StatusMessage struct {
	Timestamp time.Time
	Message   string
}

// LiveStatusTracker tracks accumulated status messages for a live comment
type LiveStatusTracker struct {
	mu       sync.RWMutex
	messages []StatusMessage
	header   string // Header text (model, prompt info, etc.)
}

// NewLiveStatusTracker creates a new status tracker
func NewLiveStatusTracker(header string) *LiveStatusTracker {
	return &LiveStatusTracker{
		messages: make([]StatusMessage, 0),
		header:   header,
	}
}

// AddMessage adds a new status message with current timestamp
// If the number of messages exceeds MaxStatusMessages, the oldest messages are dropped
func (lst *LiveStatusTracker) AddMessage(message string) {
	lst.mu.Lock()
	defer lst.mu.Unlock()

	lst.messages = append(lst.messages, StatusMessage{
		Timestamp: time.Now().UTC(),
		Message:   message,
	})

	// Keep only the most recent MaxStatusMessages messages
	if len(lst.messages) > MaxStatusMessages {
		lst.messages = lst.messages[len(lst.messages)-MaxStatusMessages:]
	}
}

// SetHeader updates the header text
func (lst *LiveStatusTracker) SetHeader(header string) {
	lst.mu.Lock()
	defer lst.mu.Unlock()
	lst.header = header
}

// BuildCommentBody builds the full comment body with all status messages
func (lst *LiveStatusTracker) BuildCommentBody() string {
	lst.mu.RLock()
	defer lst.mu.RUnlock()

	body := lst.header

	if len(lst.messages) > 0 {
		body += "\n\n**Status updates:**\n"
		for _, msg := range lst.messages {
			// Format timestamp as HH:MM:SS
			timeStr := msg.Timestamp.Format("15:04:05")
			body += fmt.Sprintf("- ⏳ %s — %s\n", timeStr, msg.Message)
		}
	}

	return body
}

// GetMessageCount returns the number of status messages
func (lst *LiveStatusTracker) GetMessageCount() int {
	lst.mu.RLock()
	defer lst.mu.RUnlock()
	return len(lst.messages)
}
