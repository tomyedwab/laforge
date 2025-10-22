package projects

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/tomyedwab/laforge/lib/errors"
	"github.com/tomyedwab/laforge/lib/steps"
	"github.com/tomyedwab/laforge/lib/tasks"
)

// Project represents a LaForge project
type Project struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	RepositoryPath string    `json:"repository_path"`
	MainBranch     string    `json:"main_branch"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ProjectConfig represents the project configuration file
type ProjectConfig struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	RepositoryPath string `json:"repository_path"`
	MainBranch     string `json:"main_branch"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// GetLaForgeDir returns the LaForge directory path (~/.laforge)
func GetLaForgeDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".laforge"), nil
}

// GetProjectsDir returns the projects directory path (~/.laforge/projects)
func GetProjectsDir() (string, error) {
	laforgeDir, err := GetLaForgeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(laforgeDir, "projects"), nil
}

// GetProjectDir returns the project directory path for a given project ID
func GetProjectDir(projectID string) (string, error) {
	projectsDir, err := GetProjectsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(projectsDir, projectID), nil
}

// ProjectExists checks if a project with the given ID already exists
func ProjectExists(projectID string) (bool, error) {
	projectDir, err := GetProjectDir(projectID)
	if err != nil {
		return false, err
	}

	_, err = os.Stat(projectDir)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("failed to check project directory: %w", err)
}

// CreateProject creates a new LaForge project with the specified ID and configuration
func CreateProject(projectID string, name string, description string, repositoryPath string, mainBranch string) (*Project, error) {
	// Validate project ID
	if projectID == "" {
		return nil, errors.NewInvalidInputError("project ID cannot be empty")
	}

	// Default to "main" if mainBranch is not specified
	if mainBranch == "" {
		mainBranch = "main"
	}

	// Check if project already exists
	exists, err := ProjectExists(projectID)
	if err != nil {
		return nil, errors.Wrap(errors.ErrUnknown, err, "failed to check if project exists")
	}
	if exists {
		return nil, errors.NewProjectAlreadyExistsError(projectID)
	}

	// Get project directory
	projectDir, err := GetProjectDir(projectID)
	if err != nil {
		return nil, errors.Wrap(errors.ErrUnknown, err, "failed to get project directory")
	}

	// Create projects directory if it doesn't exist
	projectsDir, err := GetProjectsDir()
	if err != nil {
		return nil, errors.Wrap(errors.ErrUnknown, err, "failed to get projects directory")
	}
	if err := os.MkdirAll(projectsDir, 0755); err != nil {
		return nil, errors.Wrapf(errors.ErrPermissionDenied, err, "failed to create projects directory '%s'", projectsDir)
	}

	// Create project directory
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return nil, errors.Wrapf(errors.ErrPermissionDenied, err, "failed to create project directory '%s'", projectDir)
	}

	// Create project metadata
	now := time.Now()
	project := &Project{
		ID:             projectID,
		Name:           name,
		Description:    description,
		RepositoryPath: repositoryPath,
		MainBranch:     mainBranch,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// Create project configuration file
	if err := createProjectConfig(projectDir, project); err != nil {
		// Clean up on error
		os.RemoveAll(projectDir)
		return nil, errors.Wrap(errors.ErrUnknown, err, "failed to create project configuration")
	}

	// Create task database
	if err := createTaskDatabase(projectDir); err != nil {
		// Clean up on error
		os.RemoveAll(projectDir)
		return nil, errors.Wrap(errors.ErrDatabaseOperationFailed, err, "failed to create task database")
	}

	// Create step database
	if err := createStepDatabase(projectDir); err != nil {
		// Clean up on error
		os.RemoveAll(projectDir)
		return nil, errors.Wrap(errors.ErrDatabaseOperationFailed, err, "failed to create step database")
	}

	// Create default agents.yml configuration file
	if err := CreateDefaultAgentsConfig(projectID); err != nil {
		// Clean up on error
		os.RemoveAll(projectDir)
		return nil, errors.Wrap(errors.ErrUnknown, err, "failed to create default agents configuration")
	}

	return project, nil
}

// createProjectConfig creates the project configuration file
func createProjectConfig(projectDir string, project *Project) error {
	configPath := filepath.Join(projectDir, "project.json")

	config := ProjectConfig{
		ID:             project.ID,
		Name:           project.Name,
		Description:    project.Description,
		RepositoryPath: project.RepositoryPath,
		MainBranch:     project.MainBranch,
		CreatedAt:      project.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      project.UpdatedAt.Format(time.RFC3339),
	}

	file, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	// Use JSON encoder for proper formatting
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(config); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// createTaskDatabase creates the task database for the project
func createTaskDatabase(projectDir string) error {
	dbPath := filepath.Join(projectDir, "tasks.db")

	// Set environment variable for database path
	oldDBPath := os.Getenv("TASKS_DB_PATH")
	os.Setenv("TASKS_DB_PATH", dbPath)
	defer func() {
		if oldDBPath != "" {
			os.Setenv("TASKS_DB_PATH", oldDBPath)
		} else {
			os.Unsetenv("TASKS_DB_PATH")
		}
	}()

	// Initialize database using tasks package
	db, err := tasks.InitDB()
	if err != nil {
		return fmt.Errorf("failed to initialize task database: %w", err)
	}
	defer db.Close()

	return nil
}

// LoadProject loads a project from the given project ID
func LoadProject(projectID string) (*Project, error) {
	// Check if project exists
	exists, err := ProjectExists(projectID)
	if err != nil {
		return nil, errors.Wrap(errors.ErrUnknown, err, "failed to check if project exists")
	}
	if !exists {
		return nil, errors.NewProjectNotFoundError(projectID)
	}

	// Get project directory
	projectDir, err := GetProjectDir(projectID)
	if err != nil {
		return nil, errors.Wrap(errors.ErrUnknown, err, "failed to get project directory")
	}

	// Load project configuration
	configPath := filepath.Join(projectDir, "project.json")
	file, err := os.Open(configPath)
	if err != nil {
		return nil, errors.Wrap(errors.ErrNotFound, err, "failed to open project configuration")
	}
	defer file.Close()

	var config ProjectConfig
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, errors.Wrap(errors.ErrUnknown, err, "failed to parse project configuration")
	}

	// Parse timestamps
	createdAt, err := time.Parse(time.RFC3339, config.CreatedAt)
	if err != nil {
		return nil, errors.Wrap(errors.ErrUnknown, err, "failed to parse created_at timestamp")
	}

	updatedAt, err := time.Parse(time.RFC3339, config.UpdatedAt)
	if err != nil {
		return nil, errors.Wrap(errors.ErrUnknown, err, "failed to parse updated_at timestamp")
	}

	// Default to "main" if MainBranch is not set (for backwards compatibility)
	mainBranch := config.MainBranch
	if mainBranch == "" {
		mainBranch = "main"
	}

	project := &Project{
		ID:             config.ID,
		Name:           config.Name,
		Description:    config.Description,
		RepositoryPath: config.RepositoryPath,
		MainBranch:     mainBranch,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}

	return project, nil
}

// createStepDatabase creates the step database for the project
func createStepDatabase(projectDir string) error {
	dbPath := filepath.Join(projectDir, "steps.db")

	// Use the steps package to initialize the database
	_, err := steps.InitStepDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize step database: %w", err)
	}

	return nil
}

// GetProjectTaskDatabase returns the path to the project's task database
func GetProjectTaskDatabase(projectID string) (string, error) {
	projectDir, err := GetProjectDir(projectID)
	if err != nil {
		return "", errors.Wrap(errors.ErrUnknown, err, "failed to get project directory")
	}
	return filepath.Join(projectDir, "tasks.db"), nil
}

// GetProjectStepDatabase returns the path to the project's step database
func GetProjectStepDatabase(projectID string) (string, error) {
	projectDir, err := GetProjectDir(projectID)
	if err != nil {
		return "", errors.Wrap(errors.ErrUnknown, err, "failed to get project directory")
	}
	return filepath.Join(projectDir, "steps.db"), nil
}

// OpenProjectTaskDatabase opens the task database for the given project
func OpenProjectTaskDatabase(projectID string) (*sql.DB, error) {
	dbPath, err := GetProjectTaskDatabase(projectID)
	if err != nil {
		return nil, err
	}

	// Temporarily set the environment variable
	oldDBPath := os.Getenv("TASKS_DB_PATH")
	os.Setenv("TASKS_DB_PATH", dbPath)
	defer func() {
		if oldDBPath != "" {
			os.Setenv("TASKS_DB_PATH", oldDBPath)
		} else {
			os.Unsetenv("TASKS_DB_PATH")
		}
	}()

	db, err := tasks.InitDB()
	if err != nil {
		return nil, errors.Wrap(errors.ErrDatabaseConnectionFailed, err, "failed to open project task database")
	}

	return db, nil
}

// ListProjects returns a list of all available projects
func ListProjects(projectsDir string) ([]*Project, error) {
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Project{}, nil // Return empty list if projects directory doesn't exist
		}
		return nil, fmt.Errorf("failed to read projects directory: %w", err)
	}

	var projectList []*Project
	for _, entry := range entries {
		if entry.IsDir() {
			projectID := entry.Name()
			project, err := LoadProject(projectID)
			if err != nil {
				// Skip projects that can't be loaded (maybe corrupted or incomplete)
				continue
			}
			projectList = append(projectList, project)
		}
	}

	return projectList, nil
}

// OpenProjectStepDatabase opens the step database for the given project
func OpenProjectStepDatabase(projectID string) (*steps.StepDatabase, error) {
	dbPath, err := GetProjectStepDatabase(projectID)
	if err != nil {
		return nil, err
	}

	sdb, err := steps.InitStepDB(dbPath)
	if err != nil {
		return nil, errors.Wrap(errors.ErrDatabaseConnectionFailed, err, "failed to open project step database")
	}

	return sdb, nil
}
