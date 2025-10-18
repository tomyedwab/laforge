package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJWTManager(t *testing.T) {
	secret := "test-secret-key"
	userID := "test-user-123"

	jwtManager := NewJWTManager(secret)

	t.Run("Generate and Validate Token", func(t *testing.T) {
		token, err := jwtManager.GenerateToken(userID)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		if token == "" {
			t.Fatal("Generated token should not be empty")
		}

		claims, err := jwtManager.ValidateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}

		if claims.UserID != userID {
			t.Errorf("Expected user ID %s, got %s", userID, claims.UserID)
		}
	})

	t.Run("Invalid Token", func(t *testing.T) {
		_, err := jwtManager.ValidateToken("invalid-token")
		if err == nil {
			t.Fatal("Expected error for invalid token")
		}
	})

	t.Run("Wrong Secret", func(t *testing.T) {
		token, err := jwtManager.GenerateToken(userID)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		wrongManager := NewJWTManager("wrong-secret")
		_, err = wrongManager.ValidateToken(token)
		if err == nil {
			t.Fatal("Expected error for token with wrong secret")
		}
	})
}

func TestAuthMiddleware(t *testing.T) {
	secret := "test-secret-key"
	userID := "test-user-123"
	jwtManager := NewJWTManager(secret)

	// Create a test handler that requires authentication
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userIDFromCtx, ok := GetUserIDFromContext(r.Context())
		if !ok {
			t.Error("User ID not found in context")
			http.Error(w, "User ID not found", http.StatusInternalServerError)
			return
		}

		if userIDFromCtx != userID {
			t.Errorf("Expected user ID %s, got %s", userID, userIDFromCtx)
			http.Error(w, "Wrong user ID", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Authenticated"))
	})

	protectedHandler := jwtManager.AuthMiddleware(testHandler)

	t.Run("Valid Token", func(t *testing.T) {
		token, _ := jwtManager.GenerateToken(userID)

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

		if rr.Body.String() != "Authenticated" {
			t.Errorf("Expected body 'Authenticated', got %s", rr.Body.String())
		}
	})

	t.Run("Missing Authorization Header", func(t *testing.T) {
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

	t.Run("Invalid Authorization Format", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/test", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "InvalidFormat")

		rr := httptest.NewRecorder()
		protectedHandler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, status)
		}
	})

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
}

func TestGetUserIDFromContext(t *testing.T) {
	userID := "test-user-123"
	ctx := context.WithValue(context.Background(), userContextKey, userID)

	retrievedUserID, ok := GetUserIDFromContext(ctx)
	if !ok {
		t.Fatal("Expected to find user ID in context")
	}

	if retrievedUserID != userID {
		t.Errorf("Expected user ID %s, got %s", userID, retrievedUserID)
	}

	// Test with empty context
	emptyCtx := context.Background()
	_, ok = GetUserIDFromContext(emptyCtx)
	if ok {
		t.Fatal("Expected not to find user ID in empty context")
	}
}
