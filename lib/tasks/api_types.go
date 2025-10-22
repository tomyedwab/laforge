package tasks

import (
	"strings"
	"time"
)

// TaskResponse represents the API response format for tasks
type TaskResponse struct {
	ID                   int        `json:"id"`
	Title                string     `json:"title"`
	Description          string     `json:"description"`
	AcceptanceCriteria   string     `json:"acceptance_criteria"`
	Type                 string     `json:"type"`
	Status               string     `json:"status"`
	ParentID             *int       `json:"parent_id"`
	UpstreamDependencyID *int       `json:"upstream_dependency_id"`
	ReviewRequired       bool       `json:"review_required"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	CompletedAt          *time.Time `json:"completed_at"`
}

type PaginationResponse struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
	Total int `json:"total"`
	Pages int `json:"pages"`
}

type MetaResponse struct {
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
}

type TaskLogResponse struct {
	ID        int       `json:"id"`
	TaskID    int       `json:"task_id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

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

type TaskListResponse struct {
	Data struct {
		Tasks      []*TaskResponse    `json:"tasks"`
		Pagination PaginationResponse `json:"pagination"`
	} `json:"data"`
	Meta MetaResponse `json:"meta"`
}

type SingleTaskResponse struct {
	Task         *TaskResponse         `json:"tasks"`
	TaskChildren []*TaskResponse       `json:"task_children"`
	TaskLogs     []*TaskLogResponse    `json:"task_logs"`
	TaskReviews  []*TaskReviewResponse `json:"task_reviews"`
	Meta         MetaResponse          `json:"meta"`
}

// UpdateTaskStatusRequest represents the request body for updating task status
type UpdateTaskStatusRequest struct {
	Status string `json:"status"`
}

// CreateTaskLogRequest represents the request body for creating a task log
type CreateTaskLogRequest struct {
	Message string `json:"message"`
}

// CreateTaskReviewRequest represents the request body for creating a task review
type CreateTaskReviewRequest struct {
	Message    string  `json:"message"`
	Attachment *string `json:"attachment"`
}

// ConvertTask converts a tasks.Task to TaskResponse
func ConvertTask(task *Task) *TaskResponse {
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
		AcceptanceCriteria:   task.AcceptanceCriteria,
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
