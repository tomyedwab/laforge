package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/tomyedwab/laforge/cmd/laserve/auth"
)

func TestHealthHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/api/v1/public/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(healthHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	expected := `{"status":"healthy","service":"laserve","version":"1.0.0"}`
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}

func TestLoginHandler(t *testing.T) {
	jwtManager := auth.NewJWTManager("test-secret")
	loginHandler := makeLoginHandler(jwtManager)

	req, err := http.NewRequest("POST", "/api/v1/public/login", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	loginHandler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check that response contains token and user_id
	body := rr.Body.String()
	if !strings.Contains(body, "token") {
		t.Error("Response should contain token")
	}
	if !strings.Contains(body, "test-user") {
		t.Error("Response should contain test-user")
	}
	if !strings.Contains(body, "data") {
		t.Error("Response should be wrapped in data object")
	}
	if !strings.Contains(body, "meta") {
		t.Error("Response should contain meta object")
	}
}

func TestParseFlags(t *testing.T) {
	// Save original args
	oldArgs := make([]string, len(os.Args))
	copy(oldArgs, os.Args)
	defer func() {
		os.Args = oldArgs
	}()

	// Test with required flags
	os.Args = []string{"cmd", "-jwt-secret", "secret"}
	config := parseFlags()

	if config.JWTSecret != "secret" {
		t.Errorf("expected JWT secret 'secret', got %s", config.JWTSecret)
	}
	if config.Host != defaultHost {
		t.Errorf("expected default host %s, got %s", defaultHost, config.Host)
	}
	if config.Port != defaultPort {
		t.Errorf("expected default port %s, got %s", defaultPort, config.Port)
	}
}

func TestRunValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config without database path",
			config: &Config{
				JWTSecret: "secret",
			},
			wantErr: false,
		},
		{
			name: "missing JWT secret",
			config: &Config{
				Host: defaultHost,
				Port: defaultPort,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
