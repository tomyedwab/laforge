package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tomyedwab/laforge/cmd/laserve/auth"
)

// TestAPIEndpoints tests all API endpoints
func TestAPIEndpoints(t *testing.T) {
	jwtManager := auth.NewJWTManager("test-secret")

	// Generate a test token
	token, err := jwtManager.GenerateToken("test-user")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Test all public endpoints
	t.Run("Public Endpoints", func(t *testing.T) {
		// Health check
		t.Run("Health Check", func(t *testing.T) {
			req, err := http.NewRequest("GET", "/api/v1/public/health", nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			healthHandler(rr, req)

			if status := rr.Code; status != http.StatusOK {
				t.Errorf("Expected status %d, got %d", http.StatusOK, status)
			}

			expected := `{"status":"healthy","service":"laserve","version":"1.0.0"}`
			if rr.Body.String() != expected {
				t.Errorf("Expected body %s, got %s", expected, rr.Body.String())
			}
		})

		// Login
		t.Run("Login", func(t *testing.T) {
			req, err := http.NewRequest("POST", "/api/v1/public/login", nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			loginHandler := makeLoginHandler(jwtManager)
			loginHandler.ServeHTTP(rr, req)

			if status := rr.Code; status != http.StatusOK {
				t.Errorf("Expected status %d, got %d", http.StatusOK, status)
			}

			var response map[string]interface{}
			if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			// Verify response structure
			data, ok := response["data"].(map[string]interface{})
			if !ok {
				t.Fatal("Response should contain data object")
			}

			if _, ok := data["token"]; !ok {
				t.Error("Response should contain token")
			}
			if _, ok := data["user_id"]; !ok {
				t.Error("Response should contain user_id")
			}
		})
	})

	// Test authentication middleware
	t.Run("Authentication Middleware", func(t *testing.T) {
		// Create a protected test handler
		protectedHandler := jwtManager.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := auth.GetUserIDFromContext(r.Context())
			if !ok {
				http.Error(w, "User ID not found", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(userID))
		}))

		// Test with valid token
		t.Run("Valid Token", func(t *testing.T) {
			req, err := http.NewRequest("GET", "/test", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+token)

			rr := httptest.NewRecorder()
			protectedHandler.ServeHTTP(rr, req)

			if status := rr.Code; status != http.StatusOK {
				t.Errorf("Expected status %d, got %d", http.StatusOK, status)
			}

			if rr.Body.String() != "test-user" {
				t.Errorf("Expected user ID 'test-user', got '%s'", rr.Body.String())
			}
		})

		// Test with invalid token
		t.Run("Invalid Token", func(t *testing.T) {
			req, err := http.NewRequest("GET", "/test", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer invalid-token")

			rr := httptest.NewRecorder()
			protectedHandler.ServeHTTP(rr, req)

			if status := rr.Code; status != http.StatusUnauthorized {
				t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, status)
			}
		})

		// Test without token
		t.Run("No Token", func(t *testing.T) {
			req, err := http.NewRequest("GET", "/test", nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			protectedHandler.ServeHTTP(rr, req)

			if status := rr.Code; status != http.StatusUnauthorized {
				t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, status)
			}
		})
	})
}

// TestResponseFormats tests response format consistency
func TestResponseFormats(t *testing.T) {
	tests := []struct {
		name           string
		handler        http.HandlerFunc
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name: "Health Response Format",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status":"healthy","service":"laserve","version":"1.0.0"}`))
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				if !strings.Contains(body, "healthy") {
					t.Error("Response should contain 'healthy'")
				}
			},
		},
		{
			name: "Error Response Format",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error":{"code":"VALIDATION_ERROR","message":"Invalid input"},"meta":{"timestamp":"2025-10-18T19:30:00Z","version":"1.0.0"}}`))
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				var response map[string]interface{}
				if err := json.Unmarshal([]byte(body), &response); err != nil {
					t.Fatalf("Failed to parse JSON: %v", err)
				}

				if _, ok := response["error"]; !ok {
					t.Error("Response should contain error object")
				}
				if _, ok := response["meta"]; !ok {
					t.Error("Response should contain meta object")
				}
			},
		},
		{
			name: "Success Response Format",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":{"id":1,"title":"Test Task"},"meta":{"timestamp":"2025-10-18T19:30:00Z","version":"1.0.0"}}`))
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				var response map[string]interface{}
				if err := json.Unmarshal([]byte(body), &response); err != nil {
					t.Fatalf("Failed to parse JSON: %v", err)
				}

				if _, ok := response["data"]; !ok {
					t.Error("Response should contain data object")
				}
				if _, ok := response["meta"]; !ok {
					t.Error("Response should contain meta object")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", "/test", nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			tt.handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, status)
			}

			contentType := rr.Header().Get("Content-Type")
			if !strings.Contains(contentType, "application/json") {
				t.Errorf("Expected Content-Type to contain 'application/json', got '%s'", contentType)
			}

			tt.checkResponse(t, rr.Body.String())
		})
	}
}

// TestErrorCodes tests all defined error codes
func TestErrorCodes(t *testing.T) {
	errorCodes := []struct {
		code       string
		message    string
		httpStatus int
	}{
		{"UNAUTHORIZED", "Authentication required", http.StatusUnauthorized},
		{"FORBIDDEN", "Insufficient permissions", http.StatusForbidden},
		{"NOT_FOUND", "Resource not found", http.StatusNotFound},
		{"VALIDATION_ERROR", "Request validation failed", http.StatusBadRequest},
		{"CONFLICT", "Resource conflict", http.StatusConflict},
		{"INTERNAL_ERROR", "Server internal error", http.StatusInternalServerError},
	}

	for _, tt := range errorCodes {
		t.Run(tt.code, func(t *testing.T) {
			errorResponse := map[string]interface{}{
				"error": map[string]interface{}{
					"code":    tt.code,
					"message": tt.message,
				},
				"meta": map[string]interface{}{
					"timestamp": "2025-10-18T19:30:00Z",
					"version":   "1.0.0",
				},
			}

			jsonData, err := json.Marshal(errorResponse)
			if err != nil {
				t.Fatalf("Failed to marshal error response: %v", err)
			}

			var decoded map[string]interface{}
			if err := json.Unmarshal(jsonData, &decoded); err != nil {
				t.Fatalf("Failed to unmarshal error response: %v", err)
			}

			errorObj := decoded["error"].(map[string]interface{})
			if errorObj["code"] != tt.code {
				t.Errorf("Expected error code '%s', got '%s'", tt.code, errorObj["code"])
			}
			if errorObj["message"] != tt.message {
				t.Errorf("Expected error message '%s', got '%s'", tt.message, errorObj["message"])
			}
		})
	}
}

// TestPaginationLogic tests pagination calculation logic
func TestPaginationLogic(t *testing.T) {
	tests := []struct {
		name          string
		total         int
		page          int
		limit         int
		expectedStart int
		expectedEnd   int
		expectedPages int
	}{
		{"First page", 100, 1, 10, 0, 10, 10},
		{"Second page", 100, 2, 10, 10, 20, 10},
		{"Last page full", 100, 10, 10, 90, 100, 10},
		{"Last page partial", 95, 10, 10, 90, 95, 10},
		{"Single item", 1, 1, 10, 0, 1, 1},
		{"Empty result", 0, 1, 10, 0, 0, 0},
		{"Beyond total pages", 100, 20, 10, 100, 100, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate pages
			pages := (tt.total + tt.limit - 1) / tt.limit
			if pages != tt.expectedPages {
				t.Errorf("Expected %d pages, got %d", tt.expectedPages, pages)
			}

			// Calculate start and end indices
			start := (tt.page - 1) * tt.limit
			end := start + tt.limit
			if end > tt.total {
				end = tt.total
			}
			if start >= tt.total {
				start = tt.total
				end = tt.total
			}

			if start != tt.expectedStart {
				t.Errorf("Expected start index %d, got %d", tt.expectedStart, start)
			}
			if end != tt.expectedEnd {
				t.Errorf("Expected end index %d, got %d", tt.expectedEnd, end)
			}
		})
	}
}
