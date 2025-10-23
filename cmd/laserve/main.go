package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/tomyedwab/laforge/cmd/laserve/auth"
	"github.com/tomyedwab/laforge/cmd/laserve/handlers"
	"github.com/tomyedwab/laforge/cmd/laserve/websocket"
)

const (
	defaultPort = "8080"
	defaultHost = "0.0.0.0"
)

type Config struct {
	Host        string
	Port        string
	JWTSecret   string
	Environment string
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
	flag.StringVar(&config.JWTSecret, "jwt-secret", "", "JWT secret for authentication")
	flag.StringVar(&config.Environment, "env", "development", "Environment (development, staging, production)")

	flag.Parse()

	return config
}

func validateConfig(config *Config) error {
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

	// Create JWT manager
	jwtManager := auth.NewJWTManager(config.JWTSecret)

	// Create WebSocket server
	wsServer := websocket.NewServer()
	go wsServer.Run() // Start WebSocket server in background

	// Create task handler (without database - will be opened per project)
	taskHandler := handlers.NewTaskHandler(nil, wsServer)

	// Create step handler (without database - will be opened per project)
	stepHandler := handlers.NewStepHandler(wsServer, jwtManager)

	// Create router
	router := setupRouter(jwtManager, taskHandler, stepHandler, wsServer, config)

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

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a custom response writer to capture the status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Log the request
		log.Printf("[%s] %s %s", r.Method, r.RequestURI, r.RemoteAddr)

		// Call the next handler
		next.ServeHTTP(rw, r)

		// Log the response
		duration := time.Since(start)
		log.Printf("[%s] %s %s - Status: %d, Duration: %v",
			r.Method, r.RequestURI, r.RemoteAddr, rw.statusCode, duration)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func corsMiddleware(config *Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// In development, allow any localhost origin
			// In production, restrict to specific origins
			allowOrigin := false
			if config.Environment == "development" {
				// Allow requests from localhost on any port
				if strings.Contains(origin, "localhost") || strings.Contains(origin, "127.0.0.1") {
					allowOrigin = true
				}
			} else {
				// In production, you'd validate against a whitelist
				// For now, just allow localhost
				if strings.Contains(origin, "localhost") || strings.Contains(origin, "127.0.0.1") {
					allowOrigin = true
				}
			}

			if allowOrigin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Max-Age", "3600")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			// Handle preflight OPTIONS requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func setupRouter(jwtManager *auth.JWTManager, taskHandler *handlers.TaskHandler, stepHandler *handlers.StepHandler, wsServer *websocket.Server, config *Config) *mux.Router {
	router := mux.NewRouter()

	// Apply logging middleware first
	router.Use(loggingMiddleware)

	// Apply CORS middleware to all routes
	router.Use(corsMiddleware(config))

	// API versioning
	api := router.PathPrefix("/api/v1").Subrouter()

	// Public routes (no authentication required)
	public := api.PathPrefix("/public").Subrouter()
	public.HandleFunc("/health", healthHandler).Methods("GET")
	public.HandleFunc("/health", corsPreflightHandler).Methods("OPTIONS")
	public.HandleFunc("/login", makeLoginHandler(jwtManager)).Methods("POST")
	public.HandleFunc("/login", corsPreflightHandler).Methods("OPTIONS")

	// Create project handler
	projectHandler := handlers.NewProjectHandler()

	// Create artifact handler
	artifactHandler := handlers.NewArtifactHandler()

	// Protected routes (authentication required)
	protected := api.PathPrefix("/projects").Subrouter()
	protected.Use(jwtManager.AuthMiddleware)

	// Project management routes
	protected.HandleFunc("", projectHandler.ListProjects).Methods("GET")
	protected.HandleFunc("", corsPreflightHandler).Methods("OPTIONS")
	protected.HandleFunc("/{project_id}", projectHandler.GetProject).Methods("GET")
	protected.HandleFunc("/{project_id}", corsPreflightHandler).Methods("OPTIONS")

	// Task management routes
	protected.HandleFunc("/{project_id}/tasks", taskHandler.ListTasks).Methods("GET")
	protected.HandleFunc("/{project_id}/tasks", taskHandler.CreateTask).Methods("POST")
	protected.HandleFunc("/{project_id}/tasks", corsPreflightHandler).Methods("OPTIONS")
	protected.HandleFunc("/{project_id}/tasks/{task_id}", taskHandler.GetTask).Methods("GET")
	protected.HandleFunc("/{project_id}/tasks/{task_id}", taskHandler.UpdateTask).Methods("PUT")
	protected.HandleFunc("/{project_id}/tasks/{task_id}", taskHandler.DeleteTask).Methods("DELETE")
	protected.HandleFunc("/{project_id}/tasks/{task_id}", corsPreflightHandler).Methods("OPTIONS")
	protected.HandleFunc("/{project_id}/tasks/{task_id}/lease", taskHandler.LeaseTask).Methods("POST")
	protected.HandleFunc("/{project_id}/tasks/{task_id}/lease", corsPreflightHandler).Methods("OPTIONS")

	// Task status, logs and reviews routes
	protected.HandleFunc("/{project_id}/tasks/{task_id}/status", taskHandler.UpdateTaskStatus).Methods("PUT")
	protected.HandleFunc("/{project_id}/tasks/{task_id}/status", corsPreflightHandler).Methods("OPTIONS")
	protected.HandleFunc("/{project_id}/tasks/{task_id}/logs", taskHandler.GetTaskLogs).Methods("GET")
	protected.HandleFunc("/{project_id}/tasks/{task_id}/logs", taskHandler.CreateTaskLog).Methods("POST")
	protected.HandleFunc("/{project_id}/tasks/{task_id}/logs", corsPreflightHandler).Methods("OPTIONS")
	protected.HandleFunc("/{project_id}/tasks/{task_id}/reviews", taskHandler.GetTaskReviews).Methods("GET")
	protected.HandleFunc("/{project_id}/tasks/{task_id}/reviews", taskHandler.CreateTaskReview).Methods("POST")
	protected.HandleFunc("/{project_id}/tasks/{task_id}/reviews", corsPreflightHandler).Methods("OPTIONS")

	// Method to queue updates from within a step
	protected.HandleFunc("/{project_id}/tasks/{task_id}/queue", taskHandler.QueueTaskUpdate).Methods("POST")

	// Project reviews routes
	protected.HandleFunc("/{project_id}/reviews", taskHandler.GetProjectReviews).Methods("GET")
	protected.HandleFunc("/{project_id}/reviews", corsPreflightHandler).Methods("OPTIONS")
	protected.HandleFunc("/{project_id}/reviews/{review_id}/feedback", taskHandler.SubmitReviewFeedback).Methods("PUT")
	protected.HandleFunc("/{project_id}/reviews/{review_id}/feedback", corsPreflightHandler).Methods("OPTIONS")

	// Step history routes
	protected.HandleFunc("/{project_id}/steps", stepHandler.ListSteps).Methods("GET")
	protected.HandleFunc("/{project_id}/steps", corsPreflightHandler).Methods("OPTIONS")
	protected.HandleFunc("/{project_id}/steps/{step_id}", stepHandler.GetStep).Methods("GET")
	protected.HandleFunc("/{project_id}/steps/{step_id}", corsPreflightHandler).Methods("OPTIONS")
	protected.HandleFunc("/{project_id}/steps/lease", stepHandler.LeaseStep).Methods("POST")
	protected.HandleFunc("/{project_id}/steps/lease", corsPreflightHandler).Methods("OPTIONS")
	protected.HandleFunc("/{project_id}/steps/finalize", stepHandler.FinalizeStep).Methods("POST")
	protected.HandleFunc("/{project_id}/steps/finalize", corsPreflightHandler).Methods("OPTIONS")

	// Artifact serving routes
	protected.HandleFunc("/{project_id}/artifacts/{artifact_path:.*}", artifactHandler.ServeArtifact).Methods("GET")
	protected.HandleFunc("/{project_id}/artifacts/{artifact_path:.*}", corsPreflightHandler).Methods("OPTIONS")

	// WebSocket route for real-time updates
	wsapi := router.PathPrefix("/ws").Subrouter()
	wsapi.Use(jwtManager.AuthMiddleware)
	wsapi.HandleFunc("/{project_id}/connect", wsServer.HandleWebSocket)

	return router
}

func corsPreflightHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Max-Age", "3600")
	w.WriteHeader(http.StatusOK)
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

		token, err := jwtManager.GenerateToken(&userID, nil)
		if err != nil {
			http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to generate token"}}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"token":"%s","user_id":"%s","meta":{"timestamp":"%s","version":"1.0.0"}}`,
			token, userID, time.Now().Format(time.RFC3339))
	}
}
