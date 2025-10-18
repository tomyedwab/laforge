# LaForge Web UI API Definition

## Overview

This document defines the complete REST API specification for the LaForge web UI server (`laserve`). The API provides endpoints for task management, review workflows, step history, and real-time updates through WebSocket connections.

## Architecture

The API follows RESTful principles with consistent JSON responses, proper HTTP status codes, and comprehensive error handling. The server will be implemented in Go using the existing API design as a foundation.

## Base URL and Versioning

```
https://<host>/api/v1/projects/<project_id>/
```

All endpoints are versioned with `/api/v1/` prefix to allow future API evolution.

## Authentication

API requests require authentication via Bearer token in the Authorization header:

```
Authorization: Bearer <token>
```

Token generation and management will be handled through a separate authentication service.

## Common Response Formats

### Success Response Structure
All successful responses follow a consistent wrapper format:

```json
{
  "data": <response_data>,
  "meta": {
    "timestamp": "2025-10-18T19:30:00Z",
    "version": "1.0.0"
  }
}
```

### Error Response Structure
All errors follow a standardized format:

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human readable error message",
    "details": {
      "field": "Additional error context"
    }
  },
  "meta": {
    "timestamp": "2025-10-18T19:30:00Z",
    "version": "1.0.0"
  }
}
```

### Common Error Codes
- `UNAUTHORIZED` - Invalid or missing authentication
- `FORBIDDEN` - Insufficient permissions
- `NOT_FOUND` - Resource not found
- `VALIDATION_ERROR` - Request validation failed
- `CONFLICT` - Resource conflict (e.g., duplicate task)
- `INTERNAL_ERROR` - Server internal error

## Data Models

### Task
```json
{
  "id": 1,
  "title": "[FEAT] Implement user authentication",
  "description": "Create authentication system for the web UI",
  "type": "FEAT",
  "status": "in-progress",
  "parent_id": null,
  "upstream_dependency_id": null,
  "review_required": true,
  "created_at": "2025-10-18T19:00:00Z",
  "updated_at": "2025-10-18T19:30:00Z",
  "completed_at": null,
  "children": [],
  "logs": [],
  "reviews": []
}
```

**Task Status Values:**
- `todo` - Task not yet started
- `in-progress` - Task currently being worked on  
- `in-review` - Task awaiting review/feedback
- `completed` - Task completed

**Task Type Values:**
- `EPIC` - Large project consisting of multiple sub-tasks
- `FEAT` - New feature or enhancement
- `BUG` - Bug or issue that needs fixing
- `PLAN` - Task to break down work into smaller pieces
- `DOC` - Documentation task
- `ARCH` - Architectural decision task
- `DESIGN` - Design task
- `TEST` - Testing task

### Task Log
```json
{
  "id": 1,
  "task_id": 1,
  "message": "Started implementation of authentication endpoints",
  "created_at": "2025-10-18T19:30:00Z"
}
```

### Task Review
```json
{
  "id": 1,
  "task_id": 1,
  "message": "Please review the authentication API design",
  "attachment": "docs/artifacts/auth-api-design.md",
  "status": "pending",
  "feedback": null,
  "created_at": "2025-10-18T19:30:00Z",
  "updated_at": "2025-10-18T19:30:00Z"
}
```

**Review Status Values:**
- `pending` - Review awaiting feedback
- `approved` - Review approved
- `rejected` - Review rejected with feedback

### Step
```json
{
  "id": 1,
  "project_id": "laforge-main",
  "active": true,
  "parent_step_id": null,
  "commit_before": "abc123",
  "commit_after": "def456", 
  "agent_config": {},
  "start_time": "2025-10-18T19:00:00Z",
  "end_time": "2025-10-18T19:30:00Z",
  "duration_ms": 1800000,
  "prompt_tokens": 1500,
  "completion_tokens": 800,
  "total_tokens": 2300,
  "cost_usd": 0.023,
  "exit_code": 0
}
```

## API Endpoints

### Task Management

#### GET /tasks
Retrieve all tasks with optional filtering and pagination.

**Query Parameters:**
- `status` (optional) - Filter by task status
- `type` (optional) - Filter by task type
- `parent_id` (optional) - Filter by parent task ID
- `include_children` (optional, default: false) - Include child tasks
- `include_logs` (optional, default: false) - Include task logs
- `include_reviews` (optional, default: false) - Include task reviews
- `page` (optional, default: 1) - Page number for pagination
- `limit` (optional, default: 50) - Items per page (max: 100)

**Response:**
```json
{
  "data": {
    "tasks": [
      {
        "id": 1,
        "title": "[FEAT] Implement user authentication",
        "description": "Create authentication system for the web UI",
        "type": "FEAT",
        "status": "in-progress",
        "parent_id": null,
        "created_at": "2025-10-18T19:00:00Z",
        "updated_at": "2025-10-18T19:30:00Z"
      }
    ],
    "pagination": {
      "page": 1,
      "limit": 50,
      "total": 1,
      "pages": 1
    }
  }
}
```

#### GET /tasks/{task_id}
Retrieve a specific task by ID.

**Path Parameters:**
- `task_id` (required) - Task ID

**Query Parameters:**
- `include_children` (optional, default: false) - Include child tasks
- `include_logs` (optional, default: false) - Include task logs  
- `include_reviews` (optional, default: false) - Include task reviews

**Response:**
```json
{
  "data": {
    "task": {
      "id": 1,
      "title": "[FEAT] Implement user authentication",
      "description": "Create authentication system for the web UI",
      "type": "FEAT",
      "status": "in-progress",
      "parent_id": null,
      "upstream_dependency_id": null,
      "review_required": true,
      "created_at": "2025-10-18T19:00:00Z",
      "updated_at": "2025-10-18T19:30:00Z",
      "completed_at": null,
      "children": [],
      "logs": [],
      "reviews": []
    }
  }
}
```

#### POST /tasks
Create a new task.

**Request Body:**
```json
{
  "title": "[FEAT] New feature implementation",
  "description": "Detailed description of the feature",
  "type": "FEAT",
  "parent_id": null,
  "upstream_dependency_id": null,
  "review_required": false
}
```

**Response:**
```json
{
  "data": {
    "task": {
      "id": 2,
      "title": "[FEAT] New feature implementation",
      "description": "Detailed description of the feature",
      "type": "FEAT",
      "status": "todo",
      "parent_id": null,
      "upstream_dependency_id": null,
      "review_required": false,
      "created_at": "2025-10-18T20:00:00Z",
      "updated_at": "2025-10-18T20:00:00Z",
      "completed_at": null
    }
  }
}
```

#### PUT /tasks/{task_id}
Update an existing task.

**Path Parameters:**
- `task_id` (required) - Task ID

**Request Body:**
```json
{
  "title": "Updated task title",
  "description": "Updated description",
  "type": "FEAT",
  "parent_id": null,
  "upstream_dependency_id": null,
  "review_required": true
}
```

**Response:**
Returns the updated task object with the same structure as GET /tasks/{task_id}

#### PUT /tasks/{task_id}/status
Update only the task status.

**Path Parameters:**
- `task_id` (required) - Task ID

**Request Body:**
```json
{
  "status": "in-progress"
}
```

**Response:**
Returns the updated task object with the same structure as GET /tasks/{task_id}

#### DELETE /tasks/{task_id}
Delete a task and all its children.

**Path Parameters:**
- `task_id` (required) - Task ID

**Response:**
```json
{
  "data": {
    "message": "Task and all children deleted successfully"
  }
}
```

### Task Queue Management

#### GET /tasks/next
Retrieve the next task that is ready for work. Returns tasks in 'todo', 'in-progress', or 'in-review' status where all upstream dependencies are completed.

**Response (task available):**
```json
{
  "data": {
    "task": {
      "id": 1,
      "title": "[FEAT] Implement user authentication",
      "description": "Create authentication system for the web UI",
      "type": "FEAT",
      "status": "todo",
      "parent_id": null,
      "created_at": "2025-10-18T19:00:00Z",
      "updated_at": "2025-10-18T19:00:00Z"
    }
  }
}
```

**Response (no tasks ready):**
```json
{
  "data": {
    "task": null,
    "message": "No tasks ready for work"
  }
}
```

### Task Logs

#### GET /tasks/{task_id}/logs
Retrieve task logs with pagination.

**Path Parameters:**
- `task_id` (required) - Task ID

**Query Parameters:**
- `page` (optional, default: 1) - Page number
- `limit` (optional, default: 50) - Items per page (max: 100)

**Response:**
```json
{
  "data": {
    "logs": [
      {
        "id": 1,
        "task_id": 1,
        "message": "Started implementation of authentication endpoints",
        "created_at": "2025-10-18T19:30:00Z"
      }
    ],
    "pagination": {
      "page": 1,
      "limit": 50,
      "total": 1,
      "pages": 1
    }
  }
}
```

#### POST /tasks/{task_id}/logs
Add a log entry to a task.

**Path Parameters:**
- `task_id` (required) - Task ID

**Request Body:**
```json
{
  "message": "Started working on API implementation"
}
```

**Response:**
```json
{
  "data": {
    "log": {
      "id": 2,
      "task_id": 1,
      "message": "Started working on API implementation",
      "created_at": "2025-10-18T20:00:00Z"
    }
  }
}
```

### Task Reviews

#### GET /tasks/{task_id}/reviews
Retrieve task reviews.

**Path Parameters:**
- `task_id` (required) - Task ID

**Response:**
```json
{
  "data": {
    "reviews": [
      {
        "id": 1,
        "task_id": 1,
        "message": "Please review the authentication API design",
        "attachment": "docs/artifacts/auth-api-design.md",
        "status": "pending",
        "feedback": null,
        "created_at": "2025-10-18T19:30:00Z",
        "updated_at": "2025-10-18T19:30:00Z"
      }
    ]
  }
}
```

#### POST /tasks/{task_id}/reviews
Create a review request for a task.

**Path Parameters:**
- `task_id` (required) - Task ID

**Request Body:**
```json
{
  "message": "Please review the authentication API design",
  "attachment": "docs/artifacts/auth-api-design.md"
}
```

**Response:**
```json
{
  "data": {
    "review": {
      "id": 1,
      "task_id": 1,
      "message": "Please review the authentication API design",
      "attachment": "docs/artifacts/auth-api-design.md",
      "status": "pending",
      "feedback": null,
      "created_at": "2025-10-18T19:30:00Z",
      "updated_at": "2025-10-18T19:30:00Z"
    }
  }
}
```

### Review Management

#### GET /reviews/{review_id}
Retrieve a specific review by ID.

**Path Parameters:**
- `review_id` (required) - Review ID

**Response:**
Returns the review object with the same structure as individual reviews above.

#### PUT /reviews/{review_id}/feedback
Submit feedback on a review.

**Path Parameters:**
- `review_id` (required) - Review ID

**Request Body:**
```json
{
  "status": "approved",
  "feedback": "API design looks good, proceed with implementation"
}
```

**Response:**
Returns the updated review object with feedback and new status.

### Step History

#### GET /steps
Retrieve step history with optional filtering and pagination.

**Query Parameters:**
- `active` (optional) - Filter by active status (true/false)
- `page` (optional, default: 1) - Page number
- `limit` (optional, default: 50) - Items per page (max: 100)

**Response:**
```json
{
  "data": {
    "steps": [
      {
        "id": 1,
        "project_id": "laforge-main",
        "active": true,
        "parent_step_id": null,
        "commit_before": "abc123",
        "commit_after": "def456",
        "start_time": "2025-10-18T19:00:00Z",
        "end_time": "2025-10-18T19:30:00Z",
        "duration_ms": 1800000,
        "prompt_tokens": 1500,
        "completion_tokens": 800,
        "total_tokens": 2300,
        "cost_usd": 0.023,
        "exit_code": 0
      }
    ],
    "pagination": {
      "page": 1,
      "limit": 50,
      "total": 1,
      "pages": 1
    }
  }
}
```

#### GET /steps/{step_id}
Retrieve a specific step by ID.

**Path Parameters:**
- `step_id` (required) - Step ID

**Response:**
Returns the step object with the same structure as individual steps above.

### Real-time Updates (WebSocket)

#### WebSocket Connection
Connect to WebSocket endpoint for real-time updates:

```
wss://<host>/api/v1/projects/<project_id>/ws
```

**Authentication:**
WebSocket connections require authentication via query parameter:
```
wss://<host>/api/v1/projects/<project_id>/ws?token=<auth_token>
```

**Message Types:**

**Client → Server:**
```json
{
  "type": "subscribe",
  "channels": ["tasks", "reviews", "steps"]
}
```

**Server → Client:**
```json
{
  "type": "task_updated",
  "data": {
    "task": {
      "id": 1,
      "status": "completed",
      "updated_at": "2025-10-18T20:00:00Z"
    }
  }
}
```

Available channels:
- `tasks` - Task status and content updates
- `reviews` - Review status and feedback updates  
- `steps` - Step completion and history updates

## Implementation Requirements

### Technical Stack
- **Language:** Go
- **Framework:** Standard library with gorilla/mux for routing
- **Database:** SQLite (existing task database)
- **WebSocket:** gorilla/websocket
- **Authentication:** JWT tokens
- **Validation:** go-playground/validator

### Performance Requirements
- API response time: < 200ms for standard requests
- WebSocket message delivery: < 100ms
- Support for 100+ concurrent WebSocket connections
- Database query optimization for task tree operations

### Security Requirements
- JWT-based authentication for all endpoints
- Rate limiting: 100 requests per minute per user
- Input validation and sanitization
- SQL injection prevention through parameterized queries
- CORS configuration for web UI domain

### Error Handling
- Consistent error response format
- Proper HTTP status codes
- Detailed error logging
- Graceful degradation for database connectivity issues

### Testing Requirements
- Unit tests for all API endpoints (>80% coverage)
- Integration tests for database operations
- WebSocket connection and message tests
- Load testing for concurrent user scenarios

## Deployment Considerations

### Configuration
- Environment-based configuration (dev/staging/prod)
- Database connection pooling
- WebSocket connection limits
- Logging levels and output destinations

### Monitoring
- API request/response logging
- Error rate monitoring
- Performance metrics collection
- WebSocket connection health checks

### Scaling
- Horizontal scaling support through load balancers
- WebSocket sticky sessions for connection persistence
- Database read replicas for query optimization