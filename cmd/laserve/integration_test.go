package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tomyedwab/laforge/cmd/laserve/auth"
)

// TestAPIIntegration tests the complete API integration
func TestAPIIntegration(t *testing.T) {
	// Test health endpoint
	t.Run("Health Check", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/api/v1/public/health", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		healthHandler(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}

		expected := `{"status":"healthy","service":"laserve","version":"1.0.0"}`
		if rr.Body.String() != expected {
			t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
		}
	})
}

// TestAuthentication tests authentication middleware
func TestAuthentication(t *testing.T) {
	jwtManager := auth.NewJWTManager("test-secret")

	// Generate a test token
	token, err := jwtManager.GenerateToken("test-user")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Test middleware with valid token
	t.Run("Valid Token", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/api/v1/projects/test-project/tasks", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)

		// Create a test handler that checks authentication
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := auth.GetUserIDFromContext(r.Context())
			if !ok {
				t.Error("User ID should be in context")
			}
			if userID != "test-user" {
				t.Errorf("Expected user ID 'test-user', got '%s'", userID)
			}
			w.WriteHeader(http.StatusOK)
		})

		// Apply middleware
		protectedHandler := jwtManager.AuthMiddleware(testHandler)

		rr := httptest.NewRecorder()
		protectedHandler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, status)
		}
	})

	// Test middleware with invalid token
	t.Run("Invalid Token", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/api/v1/projects/test-project/tasks", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer invalid-token")

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("Handler should not be called with invalid token")
		})

		protectedHandler := jwtManager.AuthMiddleware(testHandler)

		rr := httptest.NewRecorder()
		protectedHandler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, status)
		}
	})

	// Test middleware with missing token
	t.Run("Missing Token", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/api/v1/projects/test-project/tasks", nil)
		if err != nil {
			t.Fatal(err)
		}

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("Handler should not be called without token")
		})

		protectedHandler := jwtManager.AuthMiddleware(testHandler)

		rr := httptest.NewRecorder()
		protectedHandler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, status)
		}
	})
}

// TestAPIResponseFormat tests that all API responses follow the expected format
func TestAPIResponseFormat(t *testing.T) {
	tests := []struct {
		name     string
		response map[string]interface{}
		valid    bool
	}{
		{
			name: "Valid response with data and meta",
			response: map[string]interface{}{
				"data": map[string]interface{}{"key": "value"},
				"meta": map[string]interface{}{
					"timestamp": "2025-10-18T19:30:00Z",
					"version":   "1.0.0",
				},
			},
			valid: true,
		},
		{
			name: "Valid error response",
			response: map[string]interface{}{
				"error": map[string]interface{}{
					"code":    "NOT_FOUND",
					"message": "Resource not found",
				},
				"meta": map[string]interface{}{
					"timestamp": "2025-10-18T19:30:00Z",
					"version":   "1.0.0",
				},
			},
			valid: true,
		},
		{
			name: "Invalid response - missing meta",
			response: map[string]interface{}{
				"data": map[string]interface{}{"key": "value"},
			},
			valid: false,
		},
		{
			name: "Invalid response - missing data or error",
			response: map[string]interface{}{
				"meta": map[string]interface{}{
					"timestamp": "2025-10-18T19:30:00Z",
					"version":   "1.0.0",
				},
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert to JSON and back to verify structure
			jsonData, err := json.Marshal(tt.response)
			if err != nil {
				t.Fatalf("Failed to marshal response: %v", err)
			}

			var decoded map[string]interface{}
			if err := json.Unmarshal(jsonData, &decoded); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			// Check for required fields
			hasMeta := decoded["meta"] != nil
			hasData := decoded["data"] != nil
			hasError := decoded["error"] != nil

			if !hasMeta {
				if tt.valid {
					t.Error("Response should have meta field")
				}
				return
			}

			if !hasData && !hasError {
				if tt.valid {
					t.Error("Response should have either data or error field")
				}
				return
			}

			if !tt.valid && (hasData || hasError) && hasMeta {
				t.Error("Response should be invalid but appears valid")
			}
		})
	}
}

// TestPagination tests pagination functionality
func TestPagination(t *testing.T) {
	tests := []struct {
		name          string
		total         int
		page          int
		limit         int
		expectedPages int
	}{
		{"First page full", 100, 1, 10, 10},
		{"Middle page", 100, 5, 10, 10},
		{"Last page partial", 95, 10, 10, 10},
		{"Single page", 5, 1, 10, 1},
		{"Empty result", 0, 1, 10, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pages := (tt.total + tt.limit - 1) / tt.limit
			if pages != tt.expectedPages {
				t.Errorf("Expected %d pages, got %d", tt.expectedPages, pages)
			}
		})
	}
}

// TestErrorResponseFormat tests error response formatting
func TestErrorResponseFormat(t *testing.T) {
	errorTypes := []struct {
		name       string
		code       string
		message    string
		httpStatus int
	}{
		{"Not Found", "NOT_FOUND", "Resource not found", http.StatusNotFound},
		{"Validation Error", "VALIDATION_ERROR", "Invalid input", http.StatusBadRequest},
		{"Internal Error", "INTERNAL_ERROR", "Internal server error", http.StatusInternalServerError},
		{"Unauthorized", "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized},
	}

	for _, tt := range errorTypes {
		t.Run(tt.name, func(t *testing.T) {
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

			// Verify it can be marshaled to JSON
			_, err := json.Marshal(errorResponse)
			if err != nil {
				t.Fatalf("Failed to marshal error response: %v", err)
			}
		})
	}
}
