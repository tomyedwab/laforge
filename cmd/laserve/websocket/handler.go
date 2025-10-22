package websocket

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/tomyedwab/laforge/cmd/laserve/auth"
)

func createUpgrader() websocket.Upgrader {
	return websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			// Allow connections from localhost on any port
			origin := r.Header.Get("Origin")
			if origin == "" {
				// WebSocket connections from same origin don't have Origin header
				return true
			}
			// Allow localhost connections for development
			if strings.Contains(origin, "localhost") || strings.Contains(origin, "127.0.0.1") {
				return true
			}
			return false
		},
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
}

var upgrader = createUpgrader()

// Client represents a WebSocket client connection
type Client struct {
	conn      *websocket.Conn
	send      chan []byte
	server    *Server
	userID    string
	projectID string
	channels  map[string]bool
}

// Message represents a WebSocket message
type Message struct {
	Type      string      `json:"type"`
	Channel   string      `json:"channel"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// Server manages WebSocket connections
type Server struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan Message
	mu         sync.RWMutex
}

// NewServer creates a new WebSocket server
func NewServer() *Server {
	return &Server{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan Message),
	}
}

// Run starts the WebSocket server
func (s *Server) Run() {
	for {
		select {
		case client := <-s.register:
			s.mu.Lock()
			s.clients[client] = true
			s.mu.Unlock()

			// Send welcome message
			welcomeMsg := Message{
				Type:      "connected",
				Channel:   "system",
				Data:      map[string]string{"message": "Connected to LaForge WebSocket server"},
				Timestamp: time.Now(),
			}
			client.send <- s.encodeMessage(welcomeMsg)

		case client := <-s.unregister:
			s.mu.Lock()
			if _, ok := s.clients[client]; ok {
				delete(s.clients, client)
				close(client.send)
			}
			s.mu.Unlock()

		case message := <-s.broadcast:
			s.mu.RLock()
			for client := range s.clients {
				// Only send to clients subscribed to the channel
				if client.channels[message.Channel] {
					select {
					case client.send <- s.encodeMessage(message):
					default:
						// Client's send channel is full, close it
						close(client.send)
						delete(s.clients, client)
					}
				}
			}
			s.mu.RUnlock()
		}
	}
}

// HandleWebSocket handles WebSocket connections
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Extract user ID from context (set by auth middleware)
	userID, ok := ctx.Value(auth.UserContextKey).(string)
	if !ok {
		http.Error(w, "User ID not found in context", http.StatusInternalServerError)
		return
	}

	// Extract project ID from URL
	vars := mux.Vars(r)
	projectID := vars["project_id"]

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Log the error but don't write to response - upgrade already attempted
		// and the response writer may have been used
		return
	}

	client := &Client{
		conn:      conn,
		send:      make(chan []byte, 256),
		server:    s,
		userID:    userID,
		projectID: projectID,
		channels:  make(map[string]bool),
	}

	// Register the client
	s.register <- client

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}

// readPump pumps messages from the websocket connection to the server
func (c *Client) readPump() {
	defer func() {
		c.server.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// Log the error
			}
			break
		}

		// Handle incoming message
		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			continue // Skip invalid messages
		}

		// Handle different message types
		switch msg.Type {
		case "subscribe":
			c.handleSubscribe(msg)
		case "unsubscribe":
			c.handleUnsubscribe(msg)
		}
	}
}

// writePump pumps messages from the server to the websocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// The server closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			c.conn.WriteMessage(websocket.TextMessage, message)

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleSubscribe handles subscription requests
func (c *Client) handleSubscribe(msg Message) {
	if channels, ok := msg.Data.([]interface{}); ok {
		for _, ch := range channels {
			if channelStr, ok := ch.(string); ok {
				c.channels[channelStr] = true
			}
		}
	}
}

// handleUnsubscribe handles unsubscription requests
func (c *Client) handleUnsubscribe(msg Message) {
	if channels, ok := msg.Data.([]interface{}); ok {
		for _, ch := range channels {
			if channelStr, ok := ch.(string); ok {
				delete(c.channels, channelStr)
			}
		}
	}
}

// encodeMessage encodes a message to JSON
func (s *Server) encodeMessage(msg Message) []byte {
	data, _ := json.Marshal(msg)
	return data
}

// BroadcastTaskUpdate broadcasts a task update to all subscribed clients
func (s *Server) BroadcastTaskUpdate(projectID string, taskID int, status string) {
	message := Message{
		Type:    "task_updated",
		Channel: "tasks",
		Data: map[string]interface{}{
			"task": map[string]interface{}{
				"id":         taskID,
				"status":     status,
				"updated_at": time.Now().Format(time.RFC3339),
			},
		},
		Timestamp: time.Now(),
	}
	s.broadcast <- message
}

// BroadcastReviewUpdate broadcasts a review update to all subscribed clients
func (s *Server) BroadcastReviewUpdate(projectID string, reviewID int, status string) {
	message := Message{
		Type:    "review_updated",
		Channel: "reviews",
		Data: map[string]interface{}{
			"review": map[string]interface{}{
				"id":         reviewID,
				"status":     status,
				"updated_at": time.Now().Format(time.RFC3339),
			},
		},
		Timestamp: time.Now(),
	}
	s.broadcast <- message
}

// BroadcastStepUpdate broadcasts a step update to all subscribed clients
func (s *Server) BroadcastStepUpdate(projectID string, stepID int, status string) {
	message := Message{
		Type:    "step_updated",
		Channel: "steps",
		Data: map[string]interface{}{
			"step": map[string]interface{}{
				"id":         stepID,
				"status":     status,
				"updated_at": time.Now().Format(time.RFC3339),
			},
		},
		Timestamp: time.Now(),
	}
	s.broadcast <- message
}
