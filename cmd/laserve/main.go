package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/tomyedwab/laforge/cmd/laserve/auth"
	"github.com/tomyedwab/laforge/cmd/laserve/handlers"
	"github.com/tomyedwab/laforge/cmd/laserve/websocket"
	"github.com/tomyedwab/laforge/steps"
)

const (
	defaultPort = "8080"
	defaultHost = "0.0.0.0"
)

type Config struct {
	Host         string
	Port         string
	DatabasePath string
	JWTSecret    string
	Environment  string
}

func main() {
	config := parseFlags()

	if err := run(config); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func parseFlags() *Config {
	config := &Config{}

	flag.StringVar(&config.Host, "host", defaultHost, "Server host")
	flag.StringVar(&config.Port, "port", defaultPort, "Server port")
	flag.StringVar(&config.DatabasePath, "db", "", "Path to tasks database")
	flag.StringVar(&config.JWTSecret, "jwt-secret", "", "JWT secret for authentication")
	flag.StringVar(&config.Environment, "env", "development", "Environment (development, staging, production)")

	flag.Parse()

	return config
}

func validateConfig(config *Config) error {
	if config.DatabasePath == "" {
		return fmt.Errorf("database path is required")
	}
	if config.JWTSecret == "" {
		return fmt.Errorf("JWT secret is required")
	}
	return nil
}

func run(config *Config) error {
	// Validate required configuration
	if err := validateConfig(config); err != nil {
		return err
	}

	// Initialize database connection
	db, err := sql.Open("sqlite3", config.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to connect to database: %v", err)
	}

	// Initialize step database
	stepDB, err := steps.InitStepDB(config.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to initialize step database: %v", err)
	}
	defer stepDB.Close()

	// Create JWT manager
	jwtManager := auth.NewJWTManager(config.JWTSecret)

	// Create task handler
	taskHandler := handlers.NewTaskHandler(db)

	// Create step handler
	stepHandler := handlers.NewStepHandler(stepDB)

	// Create WebSocket server
	wsServer := websocket.NewServer()
	go wsServer.Run() // Start WebSocket server in background

	// Create router
	router := setupRouter(jwtManager, taskHandler, stepHandler, wsServer)

	// Create HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", config.Host, config.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting laserve API server on %s:%s", config.Host, config.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return srv.Shutdown(ctx)
}

func setupRouter(jwtManager *auth.JWTManager, taskHandler *handlers.TaskHandler, stepHandler *handlers.StepHandler, wsServer *websocket.Server) *mux.Router {
	router := mux.NewRouter()

	// API versioning
	api := router.PathPrefix("/api/v1").Subrouter()

	// Public routes (no authentication required)
	public := api.PathPrefix("/public").Subrouter()
	public.HandleFunc("/health", healthHandler).Methods("GET")
	public.HandleFunc("/login", makeLoginHandler(jwtManager)).Methods("POST")

	// Protected routes (authentication required)
	protected := api.PathPrefix("/projects").Subrouter()
	protected.Use(jwtManager.AuthMiddleware)

	// Task management routes
	protected.HandleFunc("/{project_id}/tasks", taskHandler.ListTasks).Methods("GET")
	protected.HandleFunc("/{project_id}/tasks", taskHandler.CreateTask).Methods("POST")
	protected.HandleFunc("/{project_id}/tasks/next", taskHandler.GetNextTask).Methods("GET")
	protected.HandleFunc("/{project_id}/tasks/{task_id}", taskHandler.GetTask).Methods("GET")
	protected.HandleFunc("/{project_id}/tasks/{task_id}", taskHandler.UpdateTask).Methods("PUT")
	protected.HandleFunc("/{project_id}/tasks/{task_id}/status", taskHandler.UpdateTaskStatus).Methods("PUT")
	protected.HandleFunc("/{project_id}/tasks/{task_id}", taskHandler.DeleteTask).Methods("DELETE")

	// Task logs and reviews routes
	protected.HandleFunc("/{project_id}/tasks/{task_id}/logs", taskHandler.GetTaskLogs).Methods("GET")
	protected.HandleFunc("/{project_id}/tasks/{task_id}/logs", taskHandler.CreateTaskLog).Methods("POST")
	protected.HandleFunc("/{project_id}/tasks/{task_id}/reviews", taskHandler.GetTaskReviews).Methods("GET")
	protected.HandleFunc("/{project_id}/tasks/{task_id}/reviews", taskHandler.CreateTaskReview).Methods("POST")

	// Step history routes
	protected.HandleFunc("/{project_id}/steps", stepHandler.ListSteps).Methods("GET")
	protected.HandleFunc("/{project_id}/steps/{step_id}", stepHandler.GetStep).Methods("GET")

	// WebSocket route for real-time updates
	protected.HandleFunc("/{project_id}/ws", wsServer.HandleWebSocket)

	return router
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"status":"healthy","service":"laserve","version":"1.0.0"}`)
}

func makeLoginHandler(jwtManager *auth.JWTManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// For now, use a simple hardcoded user for testing
		// In production, this would validate credentials against a database
		userID := "test-user"

		token, err := jwtManager.GenerateToken(userID)
		if err != nil {
			http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to generate token"}}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"data":{"token":"%s","user_id":"%s"},"meta":{"timestamp":"%s","version":"1.0.0"}}`,
			token, userID, time.Now().Format(time.RFC3339))
	}
}
