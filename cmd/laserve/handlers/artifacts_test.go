package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorilla/mux"
)

func TestServeArtifact(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "laforge-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFile := filepath.Join(tempDir, "test.md")
	testContent := "# Test Artifact\nThis is a test artifact."
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// We would need to mock the LoadProject function or create a real project
	// For now, this is a basic structure test
	handler := NewArtifactHandler()

	req, err := http.NewRequest("GET", "/api/v1/projects/test-project/artifacts/test.md", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Add path variables
	vars := map[string]string{
		"project_id":    "test-project",
		"artifact_path": "test.md",
	}
	req = mux.SetURLVars(req, vars)

	rr := httptest.NewRecorder()
	handler.ServeArtifact(rr, req)

	// This test will fail because we don't have a real project loaded
	// But it demonstrates the structure
	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("Expected status 404 for non-existent project, got %v", status)
	}
}

func TestGetContentType(t *testing.T) {
	tests := []struct {
		filename    string
		contentType string
	}{
		{"test.md", "text/markdown"},
		{"test.txt", "text/plain"},
		{"test.json", "application/json"},
		{"test.yaml", "text/yaml"},
		{"test.yml", "text/yaml"},
		{"test.html", "text/html"},
		{"test.css", "text/css"},
		{"test.js", "application/javascript"},
		{"test.ts", "application/typescript"},
		{"test.go", "text/x-go"},
		{"test.py", "text/x-python"},
		{"test.jpg", "image/jpeg"},
		{"test.jpeg", "image/jpeg"},
		{"test.png", "image/png"},
		{"test.gif", "image/gif"},
		{"test.svg", "image/svg+xml"},
		{"test.webp", "image/webp"},
		{"test.unknown", "application/octet-stream"},
	}

	for _, test := range tests {
		result := getContentType(test.filename)
		if result != test.contentType {
			t.Errorf("For filename %s, expected content type %s, got %s", test.filename, test.contentType, result)
		}
	}
}
