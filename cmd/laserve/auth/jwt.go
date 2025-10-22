package auth

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID *string `json:"user_id"`
	StepID *int    `json:"step_id"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secretKey string
}

func NewJWTManager(secretKey string) *JWTManager {
	return &JWTManager{
		secretKey: secretKey,
	}
}

func (j *JWTManager) GenerateToken(userID *string, stepID *int) (string, error) {
	claims := &Claims{
		UserID: userID,
		StepID: stepID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.secretKey))
}

func (j *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(j.secretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}

	return claims, nil
}

type ContextKey string

const UserContextKey ContextKey = "user_id"
const StepContextKey ContextKey = "step_id"

func (j *JWTManager) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow CORS preflight requests to pass through without authentication
		if r.Method == "OPTIONS" {
			next.ServeHTTP(w, r)
			return
		}

		var tokenString string

		// Try to get token from Authorization header first (preferred for regular requests)
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			bearerToken := strings.Split(authHeader, " ")
			if len(bearerToken) != 2 || bearerToken[0] != "Bearer" {
				log.Printf("AUTH: Invalid header format: %v", bearerToken)
				http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"Invalid authorization header format"}}`, http.StatusUnauthorized)
				return
			}
			tokenString = bearerToken[1]
		} else {
			// Fallback to query parameter (for WebSocket connections)
			tokenString = r.URL.Query().Get("token")
			if tokenString == "" {
				log.Printf("AUTH: Missing Authorization header and token query param for %s %s", r.Method, r.RequestURI)
				http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"Authorization header or token query parameter required"}}`, http.StatusUnauthorized)
				return
			}
		}

		log.Printf("AUTH: Validating token starting with %s...", tokenString[:min(10, len(tokenString))])
		claims, err := j.ValidateToken(tokenString)
		if err != nil {
			log.Printf("AUTH: Token validation failed: %v", err)
			http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"Invalid or expired token"}}`, http.StatusUnauthorized)
			return
		}

		log.Printf("AUTH: Successfully validated token for user: %s", claims.UserID)
		// Add user ID to context
		ctx := context.WithValue(r.Context(), UserContextKey, claims.UserID)
		ctx = context.WithValue(ctx, StepContextKey, claims.StepID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserContextKey).(*string)
	if ok && userID != nil {
		return *userID, true
	}
	return "", false
}

func GetStepIDFromContext(ctx context.Context) (int, bool) {
	stepID, ok := ctx.Value(StepContextKey).(*int)
	if ok && stepID != nil {
		return *stepID, true
	}
	return 0, false
}
