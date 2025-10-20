package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	"github.com/tomyedwab/laforge/errors"
	"github.com/tomyedwab/laforge/projects"
)

// ArtifactHandler handles artifact-related API requests
type ArtifactHandler struct{}

// NewArtifactHandler creates a new artifact handler
func NewArtifactHandler() *ArtifactHandler {
	return &ArtifactHandler{}
}

// ServeArtifact handles GET /projects/{project_id}/artifacts/{artifact_path}
func (h *ArtifactHandler) ServeArtifact(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectID := vars["project_id"]
	artifactPath := vars["artifact_path"]

	// Load project to get repository path
	project, err := projects.LoadProject(projectID)
	if err != nil {
		if errors.IsErrorType(err, errors.ErrNotFound) {
			http.Error(w, `{"error":{"code":"NOT_FOUND","message":"Project not found"}}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to load project"}}`, http.StatusInternalServerError)
		}
		return
	}

	// Validate repository path exists
	if project.RepositoryPath == "" {
		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Project repository path not configured"}}`, http.StatusInternalServerError)
		return
	}

	// Clean and validate the artifact path
	cleanPath := filepath.Clean(artifactPath)
	if strings.Contains(cleanPath, "..") {
		http.Error(w, `{"error":{"code":"FORBIDDEN","message":"Invalid artifact path"}}`, http.StatusForbidden)
		return
	}

	// Construct full file path
	fullPath := filepath.Join(project.RepositoryPath, cleanPath)

	// Check if file exists and is within repository bounds
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to resolve artifact path"}}`, http.StatusInternalServerError)
		return
	}

	absRepoPath, err := filepath.Abs(project.RepositoryPath)
	if err != nil {
		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to resolve repository path"}}`, http.StatusInternalServerError)
		return
	}

	// Security check: ensure the requested file is within the repository
	if !strings.HasPrefix(absPath, absRepoPath) {
		http.Error(w, `{"error":{"code":"FORBIDDEN","message":"Artifact path outside repository"}}`, http.StatusForbidden)
		return
	}

	// Check if file exists
	fileInfo, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, `{"error":{"code":"NOT_FOUND","message":"Artifact not found"}}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to access artifact"}}`, http.StatusInternalServerError)
		}
		return
	}

	// Ensure it's a regular file (not a directory)
	if !fileInfo.Mode().IsRegular() {
		http.Error(w, `{"error":{"code":"BAD_REQUEST","message":"Artifact is not a regular file"}}`, http.StatusBadRequest)
		return
	}

	// Determine content type based on file extension
	contentType := getContentType(cleanPath)
	w.Header().Set("Content-Type", contentType)

	// Read and serve the file
	data, err := os.ReadFile(absPath)
	if err != nil {
		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to read artifact"}}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// getContentType determines the appropriate content type based on file extension
func getContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".md", ".markdown":
		return "text/markdown"
	case ".txt":
		return "text/plain"
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "text/yaml"
	case ".xml":
		return "application/xml"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".ts":
		return "application/typescript"
	case ".go":
		return "text/x-go"
	case ".py":
		return "text/x-python"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}
