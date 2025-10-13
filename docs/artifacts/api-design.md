# LaForge Web UI API Specification

## Overview
This document specifies the REST API for the LaForge web UI to track task status and reviews. The API provides endpoints for retrieving task information, managing task status, handling reviews, and accessing task logs.

## Base URL
All API endpoints are prefixed with: `/api/v1/projects/<project_id>/`

## Authentication
API requests should include authentication headers (implementation TBD based on deployment requirements).

## Data Models

### Task
```json
{
  "id": 1,
  "title": "Design API for tracking task status",
  "parent_id": null,
  "status": "in-review",
  "created_at": "2025-10-10T02:30:06Z",
  "updated_at": "2025-10-10T02:30:49Z",
  "children": [],
  "logs": [],
  "reviews": []
}
```

### Task Status Values
- `todo` - Task not yet started
- `in-progress` - Task currently being worked on
- `in-review` - Task awaiting review/feedback
- `completed` - Task completed

### Task Log
```json
{
  "id": 1,
  "task_id": 1,
  "message": "Deliverable: A technical API specification including endpoints and parameters has been reviewed and approved.",
  "created_at": "2025-10-10T02:30:49Z"
}
```

### Task Review
```json
{
  "id": 1,
  "task_id": 1,
  "message": "Please review this API design for completeness",
  "attachment": "docs/artifacts/api-design.md",
  "status": "pending",
  "feedback": null,
  "created_at": "2025-10-10T02:30:49Z",
  "updated_at": "2025-10-10T02:30:49Z"
}
```

### Review Status Values
- `pending` - Review awaiting feedback
- `approved` - Review approved
- `rejected` - Review rejected with feedback

## API Endpoints

### Task Management

#### GET /tasks
Retrieve all tasks with optional filtering.

**Query Parameters:**
- `status` (optional) - Filter by task status
- `parent_id` (optional) - Filter by parent task ID
- `include_children` (optional) - Include child tasks in response
- `include_logs` (optional) - Include task logs in response
- `include_reviews` (optional) - Include task reviews in response

**Response:**
```json
{
  "tasks": [
    {
      "id": 1,
      "title": "Design API for tracking task status",
      "parent_id": null,
      "status": "in-review",
      "created_at": "2025-10-10T02:30:06Z",
      "updated_at": "2025-10-10T02:30:49Z"
    }
  ],
  "total": 1
}
```

#### GET /tasks/{task_id}
Retrieve a specific task by ID.

**Path Parameters:**
- `task_id` (required) - Task ID

**Query Parameters:**
- `include_children` (optional) - Include child tasks in response
- `include_logs` (optional) - Include task logs in response
- `include_reviews` (optional) - Include task reviews in response

**Response:**
```json
{
  "task": {
    "id": 1,
    "title": "Design API for tracking task status",
    "parent_id": null,
    "status": "in-review",
    "created_at": "2025-10-10T02:30:06Z",
    "updated_at": "2025-10-10T02:30:49Z",
    "children": [],
    "logs": [],
    "reviews": []
  }
}
```

#### POST /tasks
Create a new task.

**Request Body:**
```json
{
  "title": "New task title",
  "parent_id": null
}
```

**Response:**
```json
{
  "task": {
    "id": 2,
    "title": "New task title",
    "parent_id": null,
    "status": "todo",
    "created_at": "2025-10-10T03:00:00Z",
    "updated_at": "2025-10-10T03:00:00Z"
  }
}
```

#### PUT /tasks/{task_id}/status
Update task status.

**Path Parameters:**
- `task_id` (required) - Task ID

**Request Body:**
```json
{
  "status": "in-progress"
}
```

**Response:**
```json
{
  "task": {
    "id": 1,
    "title": "Design API for tracking task status",
    "parent_id": null,
    "status": "in-progress",
    "created_at": "2025-10-10T02:30:06Z",
    "updated_at": "2025-10-10T03:00:00Z"
  }
}
```

#### DELETE /tasks/{task_id}
Delete a task.

**Path Parameters:**
- `task_id` (required) - Task ID

**Response:**
```json
{
  "message": "Task deleted successfully"
}
```

### Task Queue Management

#### POST /tasks/{task_id}/queue
Add a task to the work queue.

**Path Parameters:**
- `task_id` (required) - Task ID

**Response:**
```json
{
  "message": "Task added to queue successfully"
}
```

#### GET /queue/next
Retrieve the next task from the work queue.

**Response:**
```json
{
  "task": {
    "id": 1,
    "title": "Design API for tracking task status",
    "parent_id": null,
    "status": "in-review",
    "created_at": "2025-10-10T02:30:06Z",
    "updated_at": "2025-10-10T02:30:49Z"
  }
}
```

### Task Logs

#### GET /tasks/{task_id}/logs
Retrieve task logs.

**Path Parameters:**
- `task_id` (required) - Task ID

**Response:**
```json
{
  "logs": [
    {
      "id": 1,
      "task_id": 1,
      "message": "Deliverable: A technical API specification including endpoints and parameters has been reviewed and approved.",
      "created_at": "2025-10-10T02:30:49Z"
    }
  ]
}
```

#### POST /tasks/{task_id}/logs
Add a log entry to a task.

**Path Parameters:**
- `task_id` (required) - Task ID

**Request Body:**
```json
{
  "message": "Started working on API design"
}
```

**Response:**
```json
{
  "log": {
    "id": 2,
    "task_id": 1,
    "message": "Started working on API design",
    "created_at": "2025-10-10T03:00:00Z"
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
  "reviews": [
    {
      "id": 1,
      "task_id": 1,
      "message": "Please review this API design for completeness",
      "attachment": "docs/artifacts/api-design.md",
      "status": "pending",
      "feedback": null,
      "created_at": "2025-10-10T02:30:49Z",
      "updated_at": "2025-10-10T02:30:49Z"
    }
  ]
}
```

#### POST /tasks/{task_id}/reviews
Create a review request for a task.

**Path Parameters:**
- `task_id` (required) - Task ID

**Request Body:**
```json
{
  "message": "Please review this API design for completeness",
  "attachment": "docs/artifacts/api-design.md"
}
```

**Response:**
```json
{
  "review": {
    "id": 1,
    "task_id": 1,
    "message": "Please review this API design for completeness",
    "attachment": "docs/artifacts/api-design.md",
    "status": "pending",
    "feedback": null,
    "created_at": "2025-10-10T02:30:49Z",
    "updated_at": "2025-10-10T02:30:49Z"
  }
}
```

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
```json
{
  "review": {
    "id": 1,
    "task_id": 1,
    "message": "Please review this API design for completeness",
    "attachment": "docs/artifacts/api-design.md",
    "status": "approved",
    "feedback": "API design looks good, proceed with implementation",
    "created_at": "2025-10-10T02:30:49Z",
    "updated_at": "2025-10-10T03:00:00Z"
  }
}
```

## Error Responses

All endpoints return consistent error responses:

```json
{
  "error": {
    "code": "TASK_NOT_FOUND",
    "message": "Task T999 not found",
    "details": {}
  }
}
```

Common error codes:
- `TASK_NOT_FOUND` - Requested task does not exist
- `INVALID_STATUS` - Invalid task status provided
- `VALIDATION_ERROR` - Request validation failed
- `DATABASE_ERROR` - Database operation failed
- `INTERNAL_ERROR` - Internal server error

## Real-time Updates

The web UI should support real-time updates for task status changes and new reviews. This can be implemented using:

1. **WebSocket connections** for live updates
2. **Server-Sent Events (SSE)** for one-way updates
3. **Polling** with appropriate intervals as fallback

Recommended approach: WebSocket for bidirectional communication, allowing the UI to receive updates and send commands.

## Implementation Notes

1. **Database Transactions**: All status updates should use database transactions to ensure consistency
2. **Validation**: Implement proper validation for all input parameters
3. **Pagination**: Add pagination for list endpoints when dealing with large datasets
4. **Caching**: Consider caching frequently accessed data (task lists, current queue)
5. **Rate Limiting**: Implement rate limiting to prevent abuse
6. **Audit Logging**: Log all API operations for debugging and audit purposes
