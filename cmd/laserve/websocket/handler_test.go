package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
)

func TestMessageEncoding(t *testing.T) {
	server := NewServer()

	msg := Message{
		Type:      "test",
		Channel:   "tasks",
		Data:      map[string]string{"key": "value"},
		Timestamp: time.Now(),
	}

	encoded := server.encodeMessage(msg)
	if encoded == nil {
		t.Fatal("encodeMessage should not return nil")
	}

	// Verify it's valid JSON
	if !strings.Contains(string(encoded), "test") {
		t.Error("Encoded message should contain the message type")
	}
}

func TestServerRun(t *testing.T) {
	server := NewServer()

	// Start the server in a goroutine
	go server.Run()

	// Give it time to start
	time.Sleep(10 * time.Millisecond)

	// Test that we can register a client
	client := &Client{
		conn:      nil, // Will be set by WebSocket upgrade
		send:      make(chan []byte, 256),
		server:    server,
		userID:    "test-user",
		projectID: "test-project",
		channels:  make(map[string]bool),
	}

	// Register the client
	server.register <- client

	// Give it time to process
	time.Sleep(10 * time.Millisecond)

	// Verify client is registered
	server.mu.RLock()
	if !server.clients[client] {
		t.Error("Client should be registered")
	}
	server.mu.RUnlock()

	// Test broadcasting
	server.BroadcastTaskUpdate("test-project", 1, "completed")

	// Give it time to process
	time.Sleep(10 * time.Millisecond)

	// Unregister the client
	server.unregister <- client

	// Give it time to process
	time.Sleep(10 * time.Millisecond)

	// Verify client is unregistered
	server.mu.RLock()
	if server.clients[client] {
		t.Error("Client should be unregistered")
	}
	server.mu.RUnlock()
}

func TestClientSubscribe(t *testing.T) {
	server := NewServer()
	go server.Run()

	client := &Client{
		conn:      nil,
		send:      make(chan []byte, 256),
		server:    server,
		userID:    "test-user",
		projectID: "test-project",
		channels:  make(map[string]bool),
	}

	// Test subscribe
	subscribeMsg := Message{
		Type: "subscribe",
		Data: []interface{}{"tasks", "reviews"},
	}
	client.handleSubscribe(subscribeMsg)

	if !client.channels["tasks"] {
		t.Error("Client should be subscribed to tasks channel")
	}
	if !client.channels["reviews"] {
		t.Error("Client should be subscribed to reviews channel")
	}

	// Test unsubscribe
	unsubscribeMsg := Message{
		Type: "unsubscribe",
		Data: []interface{}{"tasks"},
	}
	client.handleUnsubscribe(unsubscribeMsg)

	if client.channels["tasks"] {
		t.Error("Client should not be subscribed to tasks channel")
	}
	if !client.channels["reviews"] {
		t.Error("Client should still be subscribed to reviews channel")
	}
}

func TestBroadcastFunctions(t *testing.T) {
	server := NewServer()
	go server.Run()

	// Test task update broadcast
	server.BroadcastTaskUpdate("test-project", 1, "completed")

	// Give it time to process
	time.Sleep(10 * time.Millisecond)

	// Test review update broadcast
	server.BroadcastReviewUpdate("test-project", 1, "approved")

	// Give it time to process
	time.Sleep(10 * time.Millisecond)

	// Test step update broadcast
	server.BroadcastStepUpdate("test-project", 1, "completed")

	// Give it time to process
	time.Sleep(10 * time.Millisecond)
}

func TestWebSocketHandler(t *testing.T) {
	server := NewServer()
	go server.Run()

	// Create a test router with the WebSocket handler
	router := mux.NewRouter()
	router.HandleFunc("/api/v1/projects/{project_id}/ws", server.HandleWebSocket)

	// Create test server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/v1/projects/test-project/ws"

	// Note: This is a simplified test. In a real scenario, you'd need to:
	// 1. Set up proper authentication context
	// 2. Handle the WebSocket upgrade properly
	// 3. Test actual WebSocket communication

	// For now, just verify the handler exists and can be called
	req, err := http.NewRequest("GET", wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Add user ID to context (simulating auth middleware)
	ctx := context.WithValue(req.Context(), "user_id", "test-user")
	req = req.WithContext(ctx)

	// This will fail because we can't actually upgrade in test environment
	// but it verifies the handler is properly set up
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Should get an error because we can't upgrade HTTP to WebSocket in test
	if rr.Code == http.StatusOK {
		t.Error("Expected error for WebSocket upgrade in test environment")
	}
}

func TestMessageStructure(t *testing.T) {
	// Test task update message structure
	msg := Message{
		Type:    "task_updated",
		Channel: "tasks",
		Data: map[string]interface{}{
			"task": map[string]interface{}{
				"id":         1,
				"status":     "completed",
				"updated_at": time.Now().Format(time.RFC3339),
			},
		},
		Timestamp: time.Now(),
	}

	encoded, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	var decoded Message
	err = json.Unmarshal(encoded, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if decoded.Type != msg.Type {
		t.Errorf("Expected type %s, got %s", msg.Type, decoded.Type)
	}
	if decoded.Channel != msg.Channel {
		t.Errorf("Expected channel %s, got %s", msg.Channel, decoded.Channel)
	}
}

func TestUpgraderConfiguration(t *testing.T) {
	// Test that the upgrader is properly configured
	r := &http.Request{
		Header: make(http.Header),
	}

	// Should allow any origin for now
	if !upgrader.CheckOrigin(r) {
		t.Error("Upgrader should allow any origin")
	}

	// Test buffer sizes
	if upgrader.ReadBufferSize != 1024 {
		t.Error("Expected ReadBufferSize to be 1024")
	}
	if upgrader.WriteBufferSize != 1024 {
		t.Error("Expected WriteBufferSize to be 1024")
	}
}
