package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/tomyedwab/laforge/cmd/laserve/websocket"
	"github.com/tomyedwab/laforge/tasks"
)

type TaskHandler struct {
	db       *sql.DB
	wsServer *websocket.Server
}

func NewTaskHandler(db *sql.DB, wsServer *websocket.Server) *TaskHandler {
	return &TaskHandler{db: db, wsServer: wsServer}
}

// TaskResponse represents the API response format for tasks
type TaskResponse struct {
	ID                   int        `json:"id"`
	Title                string     `json:"title"`
	Description          string     `json:"description"`
	Type                 string     `json:"type"`
	Status               string     `json:"status"`
	ParentID             *int       `json:"parent_id"`
	UpstreamDependencyID *int       `json:"upstream_dependency_id"`
	ReviewRequired       bool       `json:"review_required"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	CompletedAt          *time.Time `json:"completed_at"`
}

// CreateTaskRequest represents the request body for creating a task
type CreateTaskRequest struct {
	Title                string `json:"title"`
	Description          string `json:"description"`
	Type                 string `json:"type"`
	ParentID             *int   `json:"parent_id"`
	UpstreamDependencyID *int   `json:"upstream_dependency_id"`
	ReviewRequired       bool   `json:"review_required"`
}

// UpdateTaskRequest represents the request body for updating a task
type UpdateTaskRequest struct {
	Title                string `json:"title"`
	Description          string `json:"description"`
	Type                 string `json:"type"`
	ParentID             *int   `json:"parent_id"`
	UpstreamDependencyID *int   `json:"upstream_dependency_id"`
	ReviewRequired       bool   `json:"review_required"`
}

// UpdateTaskStatusRequest represents the request body for updating task status
type UpdateTaskStatusRequest struct {
	Status string `json:"status"`
}

// convertTask converts a tasks.Task to TaskResponse
func convertTask(task *tasks.Task) *TaskResponse {
	// Extract task type from title if it follows the format "[TYPE] Title"
	taskType := "FEAT" // default
	if strings.HasPrefix(task.Title, "[") {
		endIdx := strings.Index(task.Title, "]")
		if endIdx > 1 {
			taskType = task.Title[1:endIdx]
		}
	}

	response := &TaskResponse{
		ID:                   task.ID,
		Title:                task.Title,
		Description:          task.Description,
		Type:                 taskType,
		Status:               task.Status,
		ParentID:             task.ParentID,
		UpstreamDependencyID: task.UpstreamDependencyID,
		ReviewRequired:       task.ReviewRequired,
		CreatedAt:            task.CreatedAt,
		UpdatedAt:            task.UpdatedAt,
	}

	// Set completed_at if status is completed
	if task.Status == "completed" {
		response.CompletedAt = &task.UpdatedAt
	}

	return response
}

// ListTasks handles GET /tasks
func (h *TaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	status := r.URL.Query().Get("status")
	taskType := r.URL.Query().Get("type")
	parentIDStr := r.URL.Query().Get("parent_id")
	includeChildren := r.URL.Query().Get("include_children") == "true"
	includeLogs := r.URL.Query().Get("include_logs") == "true"
	includeReviews := r.URL.Query().Get("include_reviews") == "true"

	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	// TODO: Implement include_children, include_logs, include_reviews
	_ = includeChildren
	_ = includeLogs
	_ = includeReviews

	// Get all tasks from database
	dbTasks, err := tasks.ListTasks(h.db)
	if err != nil {
		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to fetch tasks"}}`, http.StatusInternalServerError)
		return
	}

	// Filter tasks based on query parameters
	var filteredTasks []tasks.Task
	for _, task := range dbTasks {
		// Filter by status
		if status != "" && task.Status != status {
			continue
		}

		// Filter by type (extract from title)
		if taskType != "" {
			actualType := "FEAT"
			if strings.HasPrefix(task.Title, "[") {
				if endIdx := strings.Index(task.Title, "]"); endIdx > 1 {
					actualType = task.Title[1:endIdx]
				}
			}
			if actualType != taskType {
				continue
			}
		}

		// Filter by parent_id
		if parentIDStr != "" {
			parentID, err := strconv.Atoi(parentIDStr)
			if err != nil {
				http.Error(w, `{"error":{"code":"VALIDATION_ERROR","message":"Invalid parent_id parameter"}}`, http.StatusBadRequest)
				return
			}
			if task.ParentID == nil || *task.ParentID != parentID {
				continue
			}
		}

		filteredTasks = append(filteredTasks, task)
	}

	// Apply pagination
	total := len(filteredTasks)
	start := (page - 1) * limit
	end := start + limit
	if start >= total {
		filteredTasks = []tasks.Task{}
	} else if end > total {
		filteredTasks = filteredTasks[start:]
	} else {
		filteredTasks = filteredTasks[start:end]
	}

	// Convert to response format
	responseTasks := make([]*TaskResponse, len(filteredTasks))
	for i, task := range filteredTasks {
		responseTasks[i] = convertTask(&task)
	}

	// TODO: Implement include_children, include_logs, include_reviews

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"tasks": responseTasks,
			"pagination": map[string]interface{}{
				"page":  page,
				"limit": limit,
				"total": total,
				"pages": (total + limit - 1) / limit,
			},
		},
		"meta": map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetTask handles GET /tasks/{task_id}
func (h *TaskHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskIDStr := vars["task_id"]

	taskID, err := strconv.Atoi(taskIDStr)
	if err != nil {
		http.Error(w, `{"error":{"code":"VALIDATION_ERROR","message":"Invalid task ID"}}`, http.StatusBadRequest)
		return
	}

	includeChildren := r.URL.Query().Get("include_children") == "true"
	includeLogs := r.URL.Query().Get("include_logs") == "true"
	includeReviews := r.URL.Query().Get("include_reviews") == "true"

	task, err := tasks.GetTask(h.db, taskID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, `{"error":{"code":"NOT_FOUND","message":"Task not found"}}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to fetch task"}}`, http.StatusInternalServerError)
		}
		return
	}

	responseTask := convertTask(task)

	// TODO: Implement include_children, include_logs, include_reviews
	_ = includeChildren
	_ = includeLogs
	_ = includeReviews

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"task": responseTask,
		},
		"meta": map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateTask handles POST /tasks
func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":{"code":"VALIDATION_ERROR","message":"Invalid request body"}}`, http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Title == "" {
		http.Error(w, `{"error":{"code":"VALIDATION_ERROR","message":"Title is required"}}`, http.StatusBadRequest)
		return
	}

	// Create task in database
	taskID, err := tasks.AddTaskWithDetails(h.db, req.Title, req.Description, "", req.UpstreamDependencyID, req.ReviewRequired, req.ParentID)
	if err != nil {
		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to create task"}}`, http.StatusInternalServerError)
		return
	}

	// Fetch the created task
	createdTask, err := tasks.GetTask(h.db, taskID)
	if err != nil {
		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to fetch created task"}}`, http.StatusInternalServerError)
		return
	}

	responseTask := convertTask(createdTask)

	// Broadcast task creation via WebSocket
	if h.wsServer != nil {
		vars := mux.Vars(r)
		projectID := vars["project_id"]
		h.wsServer.BroadcastTaskUpdate(projectID, taskID, responseTask.Status)
	}

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"task": responseTask,
		},
		"meta": map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// UpdateTask handles PUT /tasks/{task_id}
func (h *TaskHandler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskIDStr := vars["task_id"]

	taskID, err := strconv.Atoi(taskIDStr)
	if err != nil {
		http.Error(w, `{"error":{"code":"VALIDATION_ERROR","message":"Invalid task ID"}}`, http.StatusBadRequest)
		return
	}

	var req UpdateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":{"code":"VALIDATION_ERROR","message":"Invalid request body"}}`, http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Title == "" {
		http.Error(w, `{"error":{"code":"VALIDATION_ERROR","message":"Title is required"}}`, http.StatusBadRequest)
		return
	}

	// Get existing task
	existingTask, err := tasks.GetTask(h.db, taskID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, `{"error":{"code":"NOT_FOUND","message":"Task not found"}}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to fetch task"}}`, http.StatusInternalServerError)
		}
		return
	}

	// Update task fields
	existingTask.Title = req.Title
	existingTask.Description = req.Description
	existingTask.ParentID = req.ParentID
	existingTask.UpstreamDependencyID = req.UpstreamDependencyID
	existingTask.ReviewRequired = req.ReviewRequired

	// TODO: Implement actual update in database
	// For now, we'll just return the updated task
	// This requires adding an UpdateTask function to the tasks package

	responseTask := convertTask(existingTask)

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"task": responseTask,
		},
		"meta": map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UpdateTaskStatus handles PUT /tasks/{task_id}/status
func (h *TaskHandler) UpdateTaskStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskIDStr := vars["task_id"]

	taskID, err := strconv.Atoi(taskIDStr)
	if err != nil {
		http.Error(w, `{"error":{"code":"VALIDATION_ERROR","message":"Invalid task ID"}}`, http.StatusBadRequest)
		return
	}

	var req UpdateTaskStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":{"code":"VALIDATION_ERROR","message":"Invalid request body"}}`, http.StatusBadRequest)
		return
	}

	// Validate status
	validStatuses := map[string]bool{
		"todo":        true,
		"in-progress": true,
		"in-review":   true,
		"completed":   true,
	}
	if !validStatuses[req.Status] {
		http.Error(w, `{"error":{"code":"VALIDATION_ERROR","message":"Invalid status"}}`, http.StatusBadRequest)
		return
	}

	// Update task status in database
	err = tasks.UpdateTaskStatus(h.db, taskID, req.Status)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, `{"error":{"code":"NOT_FOUND","message":"Task not found"}}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to update task status"}}`, http.StatusInternalServerError)
		}
		return
	}

	// Broadcast task status update via WebSocket
	if h.wsServer != nil {
		vars := mux.Vars(r)
		projectID := vars["project_id"]
		h.wsServer.BroadcastTaskUpdate(projectID, taskID, req.Status)
	}

	// Fetch updated task
	updatedTask, err := tasks.GetTask(h.db, taskID)
	if err != nil {
		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to fetch updated task"}}`, http.StatusInternalServerError)
		return
	}

	responseTask := convertTask(updatedTask)

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"task": responseTask,
		},
		"meta": map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DeleteTask handles DELETE /tasks/{task_id}
func (h *TaskHandler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskIDStr := vars["task_id"]

	taskID, err := strconv.Atoi(taskIDStr)
	if err != nil {
		http.Error(w, `{"error":{"code":"VALIDATION_ERROR","message":"Invalid task ID"}}`, http.StatusBadRequest)
		return
	}

	// Check if task exists
	_, err = tasks.GetTask(h.db, taskID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, `{"error":{"code":"NOT_FOUND","message":"Task not found"}}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to fetch task"}}`, http.StatusInternalServerError)
		}
		return
	}

	// Delete task and all children
	err = tasks.DeleteTask(h.db, taskID)
	if err != nil {
		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to delete task"}}`, http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"message": "Task and all children deleted successfully",
		},
		"meta": map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// TaskLogResponse represents the API response format for task logs
type TaskLogResponse struct {
	ID        int       `json:"id"`
	TaskID    int       `json:"task_id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateTaskLogRequest represents the request body for creating a task log
type CreateTaskLogRequest struct {
	Message string `json:"message"`
}

// GetTaskLogs handles GET /tasks/{task_id}/logs
func (h *TaskHandler) GetTaskLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskIDStr := vars["task_id"]

	taskID, err := strconv.Atoi(taskIDStr)
	if err != nil {
		http.Error(w, `{"error":{"code":"VALIDATION_ERROR","message":"Invalid task ID"}}`, http.StatusBadRequest)
		return
	}

	// Parse pagination parameters
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	// Get task logs from database
	dbLogs, err := tasks.GetTaskLogs(h.db, taskID)
	if err != nil {
		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to fetch task logs"}}`, http.StatusInternalServerError)
		return
	}

	// Convert to response format
	logs := make([]*TaskLogResponse, len(dbLogs))
	for i, log := range dbLogs {
		logs[i] = &TaskLogResponse{
			ID:        log.ID,
			TaskID:    log.TaskID,
			Message:   log.Message,
			CreatedAt: log.CreatedAt,
		}
	}

	// Apply pagination
	total := len(logs)
	start := (page - 1) * limit
	end := start + limit
	if start >= total {
		logs = []*TaskLogResponse{}
	} else if end > total {
		logs = logs[start:]
	} else {
		logs = logs[start:end]
	}

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"logs": logs,
			"pagination": map[string]interface{}{
				"page":  page,
				"limit": limit,
				"total": total,
				"pages": (total + limit - 1) / limit,
			},
		},
		"meta": map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateTaskLog handles POST /tasks/{task_id}/logs
func (h *TaskHandler) CreateTaskLog(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskIDStr := vars["task_id"]

	taskID, err := strconv.Atoi(taskIDStr)
	if err != nil {
		http.Error(w, `{"error":{"code":"VALIDATION_ERROR","message":"Invalid task ID"}}`, http.StatusBadRequest)
		return
	}

	var req CreateTaskLogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":{"code":"VALIDATION_ERROR","message":"Invalid request body"}}`, http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Message == "" {
		http.Error(w, `{"error":{"code":"VALIDATION_ERROR","message":"Message is required"}}`, http.StatusBadRequest)
		return
	}

	// Check if task exists
	_, err = tasks.GetTask(h.db, taskID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, `{"error":{"code":"NOT_FOUND","message":"Task not found"}}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to fetch task"}}`, http.StatusInternalServerError)
		}
		return
	}

	// Create log entry
	err = tasks.AddTaskLog(h.db, taskID, req.Message)
	if err != nil {
		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to create task log"}}`, http.StatusInternalServerError)
		return
	}

	// Fetch the created log (we need to get the ID and timestamp)
	// For now, we'll create a response with the provided data
	// In a real implementation, we'd fetch the created log from the database
	responseLog := &TaskLogResponse{
		TaskID:    taskID,
		Message:   req.Message,
		CreatedAt: time.Now(),
	}

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"log": responseLog,
		},
		"meta": map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetNextTask handles GET /tasks/next
func (h *TaskHandler) GetNextTask(w http.ResponseWriter, r *http.Request) {
	nextTask, err := tasks.GetNextTask(h.db)
	if err != nil {
		if err == sql.ErrNoRows {
			// No tasks ready for work
			response := map[string]interface{}{
				"data": map[string]interface{}{
					"task":    nil,
					"message": "No tasks ready for work",
				},
				"meta": map[string]interface{}{
					"timestamp": time.Now().Format(time.RFC3339),
					"version":   "1.0.0",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to fetch next task"}}`, http.StatusInternalServerError)
		return
	}

	responseTask := convertTask(nextTask)

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"task": responseTask,
		},
		"meta": map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// TaskReviewResponse represents the API response format for task reviews
type TaskReviewResponse struct {
	ID         int       `json:"id"`
	TaskID     int       `json:"task_id"`
	Message    string    `json:"message"`
	Attachment *string   `json:"attachment"`
	Status     string    `json:"status"`
	Feedback   *string   `json:"feedback"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// CreateTaskReviewRequest represents the request body for creating a task review
type CreateTaskReviewRequest struct {
	Message    string  `json:"message"`
	Attachment *string `json:"attachment"`
}

// GetTaskReviews handles GET /tasks/{task_id}/reviews
func (h *TaskHandler) GetTaskReviews(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskIDStr := vars["task_id"]

	taskID, err := strconv.Atoi(taskIDStr)
	if err != nil {
		http.Error(w, `{"error":{"code":"VALIDATION_ERROR","message":"Invalid task ID"}}`, http.StatusBadRequest)
		return
	}

	// Check if task exists
	_, err = tasks.GetTask(h.db, taskID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, `{"error":{"code":"NOT_FOUND","message":"Task not found"}}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to fetch task"}}`, http.StatusInternalServerError)
		}
		return
	}

	// Get task reviews from database
	dbReviews, err := tasks.GetTaskReviews(h.db, taskID)
	if err != nil {
		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to fetch task reviews"}}`, http.StatusInternalServerError)
		return
	}

	// Convert to response format
	reviews := make([]*TaskReviewResponse, len(dbReviews))
	for i, review := range dbReviews {
		reviews[i] = &TaskReviewResponse{
			ID:         review.ID,
			TaskID:     review.TaskID,
			Message:    review.Message,
			Attachment: review.Attachment,
			Status:     review.Status,
			Feedback:   review.Feedback,
			CreatedAt:  review.CreatedAt,
			UpdatedAt:  review.UpdatedAt,
		}
	}

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"reviews": reviews,
		},
		"meta": map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateTaskReview handles POST /tasks/{task_id}/reviews
func (h *TaskHandler) CreateTaskReview(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskIDStr := vars["task_id"]

	taskID, err := strconv.Atoi(taskIDStr)
	if err != nil {
		http.Error(w, `{"error":{"code":"VALIDATION_ERROR","message":"Invalid task ID"}}`, http.StatusBadRequest)
		return
	}

	var req CreateTaskReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":{"code":"VALIDATION_ERROR","message":"Invalid request body"}}`, http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Message == "" {
		http.Error(w, `{"error":{"code":"VALIDATION_ERROR","message":"Message is required"}}`, http.StatusBadRequest)
		return
	}

	// Check if task exists
	_, err = tasks.GetTask(h.db, taskID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, `{"error":{"code":"NOT_FOUND","message":"Task not found"}}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to fetch task"}}`, http.StatusInternalServerError)
		}
		return
	}

	// Create review
	err = tasks.CreateReview(h.db, taskID, req.Message, req.Attachment)
	if err != nil {
		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to create task review"}}`, http.StatusInternalServerError)
		return
	}

	// Broadcast review creation via WebSocket
	if h.wsServer != nil {
		vars := mux.Vars(r)
		projectID := vars["project_id"]
		// Note: We need to get the review ID from the database, for now we'll use a placeholder
		h.wsServer.BroadcastReviewUpdate(projectID, 0, "pending")
	}

	// For now, we will create a response with the provided data
	// In a real implementation, we would fetch the created review from the database
	responseReview := &TaskReviewResponse{
		TaskID:     taskID,
		Message:    req.Message,
		Attachment: req.Attachment,
		Status:     "pending", // Default status for new reviews
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"review": responseReview,
		},
		"meta": map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}
