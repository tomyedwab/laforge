package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/tomyedwab/laforge/lib/errors"
	"github.com/tomyedwab/laforge/lib/projects"
)

// ProjectResponse represents the API response format for projects
type ProjectResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ProjectHandler handles project-related API requests
type ProjectHandler struct{}

// NewProjectHandler creates a new project handler
func NewProjectHandler() *ProjectHandler {
	return &ProjectHandler{}
}

// ListProjects handles GET /projects
func (h *ProjectHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	// Get projects directory
	projectsDir, err := projects.GetProjectsDir()
	if err != nil {
		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to get projects directory"}}`, http.StatusInternalServerError)
		return
	}

	// List all projects
	projectList, err := projects.ListProjects(projectsDir)
	if err != nil {
		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to list projects"}}`, http.StatusInternalServerError)
		return
	}

	// Convert to response format
	responseProjects := make([]*ProjectResponse, len(projectList))
	for i, project := range projectList {
		responseProjects[i] = &ProjectResponse{
			ID:          project.ID,
			Name:        project.Name,
			Description: project.Description,
			CreatedAt:   project.CreatedAt,
			UpdatedAt:   project.UpdatedAt,
		}
	}

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"projects": responseProjects,
		},
		"meta": map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetProject handles GET /projects/{project_id}
func (h *ProjectHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectID := vars["project_id"]

	// Load project
	project, err := projects.LoadProject(projectID)
	if err != nil {
		if errors.IsErrorType(err, errors.ErrNotFound) {
			http.Error(w, `{"error":{"code":"NOT_FOUND","message":"Project not found"}}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to load project"}}`, http.StatusInternalServerError)
		}
		return
	}

	responseProject := &ProjectResponse{
		ID:          project.ID,
		Name:        project.Name,
		Description: project.Description,
		CreatedAt:   project.CreatedAt,
		UpdatedAt:   project.UpdatedAt,
	}

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"project": responseProject,
		},
		"meta": map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
