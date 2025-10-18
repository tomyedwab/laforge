# laserve - LaForge API Server

A Go-based REST API server for the LaForge web UI, providing endpoints for task management, review workflows, step history, and real-time updates.

## Features

- **RESTful API** with consistent JSON responses
- **JWT-based authentication** with middleware protection
- **WebSocket support** for real-time updates
- **Task management** with full CRUD operations
- **Step history tracking** with detailed execution metrics
- **Review workflow management** with feedback system
- **Comprehensive error handling** with standardized responses
- **Pagination support** for large datasets
- **Filtering and search** capabilities

## Installation

```bash
go build -o laserve ./cmd/laserve
```

## Usage

```bash
./laserve -db /path/to/tasks.db -jwt-secret your-secret-key
```

### Command Line Options

- `-host` - Server host (default: "0.0.0.0")
- `-port` - Server port (default: "8080")
- `-db` - Path to tasks database (required)
- `-jwt-secret` - JWT secret for authentication (required)
- `-env` - Environment (development, staging, production)

## API Documentation

### Authentication

All protected endpoints require a Bearer token in the Authorization header:
```
Authorization: Bearer <token>
```

### Response Format

All responses follow a consistent format:

**Success Response:**
```json
{
  "data": <response_data>,
  "meta": {
    "timestamp": "2025-10-18T19:30:00Z",
    "version": "1.0.0"
  }
}
```

**Error Response:**
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

### Public Endpoints

#### Health Check
- `GET /api/v1/public/health` - Check service health
- **Response:** `{"status":"healthy","service":"laserve","version":"1.0.0"}`

#### Login
- `POST /api/v1/public/login` - Generate authentication token
- **Response:** `{"data":{"token":"<jwt_token>","user_id":"test-user"},"meta":{...}}`

### Protected Endpoints (Require Authentication)

#### Task Management

**List Tasks:**
- `GET /api/v1/projects/{project_id}/tasks`
- **Query Parameters:**
  - `status` - Filter by task status (todo, in-progress, in-review, completed)
  - `type` - Filter by task type (EPIC, FEAT, BUG, PLAN, DOC, ARCH, DESIGN, TEST)
  - `parent_id` - Filter by parent task ID
  - `include_children` - Include child tasks (default: false)
  - `include_logs` - Include task logs (default: false)
  - `include_reviews` - Include task reviews (default: false)
  - `page` - Page number for pagination (default: 1)
  - `limit` - Items per page, max 100 (default: 50)

**Get Task:**
- `GET /api/v1/projects/{project_id}/tasks/{task_id}`
- **Query Parameters:** Same as list tasks

**Create Task:**
- `POST /api/v1/projects/{project_id}/tasks`
- **Request Body:**
```json
{
  "title": "[FEAT] New feature implementation",
  "description": "Detailed description",
  "type": "FEAT",
  "parent_id": null,
  "upstream_dependency_id": null,
  "review_required": false
}
```

**Update Task:**
- `PUT /api/v1/projects/{project_id}/tasks/{task_id}`
- **Request Body:** Same as create task

**Update Task Status:**
- `PUT /api/v1/projects/{project_id}/tasks/{task_id}/status`
- **Request Body:** `{"status": "in-progress"}`

**Delete Task:**
- `DELETE /api/v1/projects/{project_id}/tasks/{task_id}`
- **Response:** `{"data":{"message":"Task and all children deleted successfully"},"meta":{...}}`

#### Task Queue

**Get Next Task:**
- `GET /api/v1/projects/{project_id}/tasks/next`
- **Response:** Returns the next task ready for work, or `{"task": null, "message": "No tasks ready for work"}`

#### Task Logs

**Get Task Logs:**
- `GET /api/v1/projects/{project_id}/tasks/{task_id}/logs`
- **Query Parameters:**
  - `page` - Page number (default: 1)
  - `limit` - Items per page, max 100 (default: 50)

**Add Task Log:**
- `POST /api/v1/projects/{project_id}/tasks/{task_id}/logs`
- **Request Body:** `{"message": "Started implementation"}`

#### Task Reviews

**Get Task Reviews:**
- `GET /api/v1/projects/{project_id}/tasks/{task_id}/reviews`

**Create Review Request:**
- `POST /api/v1/projects/{project_id}/tasks/{task_id}/reviews`
- **Request Body:**
```json
{
  "message": "Please review the API design",
  "attachment": "docs/artifacts/api-design.md"
}
```

#### Step History

**List Steps:**
- `GET /api/v1/projects/{project_id}/steps`
- **Query Parameters:**
  - `active` - Filter by active status (true/false)
  - `page` - Page number (default: 1)
  - `limit` - Items per page, max 100 (default: 50)

**Get Step:**
- `GET /api/v1/projects/{project_id}/steps/{step_id}`

#### WebSocket Real-time Updates

**Connect to WebSocket:**
- `wss://host/api/v1/projects/{project_id}/ws?token=<auth_token>`

**Subscribe to Channels:**
```json
{
  "type": "subscribe",
  "channels": ["tasks", "reviews", "steps"]
}
```

**Available Channels:**
- `tasks` - Task status and content updates
- `reviews` - Review status and feedback updates
- `steps` - Step completion and history updates

**Message Types:**
- `task_updated` - Task status changed
- `review_updated` - Review status changed
- `step_updated` - Step status changed

## Development

### Running Tests
```bash
go test ./cmd/laserve/... -v
```

### Test Coverage
```bash
go test ./cmd/laserve/... -cover
```

### Building
```bash
go build -o laserve ./cmd/laserve
```

### Running with Docker
```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o laserve ./cmd/laserve

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/laserve .
CMD ["./laserve"]
```

## Configuration

### Environment Variables
- `LASERVE_HOST` - Server host (overrides -host flag)
- `LASERVE_PORT` - Server port (overrides -port flag)
- `LASERVE_DB_PATH` - Database path (overrides -db flag)
- `LASERVE_JWT_SECRET` - JWT secret (overrides -jwt-secret flag)
- `LASERVE_ENV` - Environment (overrides -env flag)

### Database Schema
The server uses SQLite databases with the following main tables:
- `tasks` - Task definitions and metadata
- `task_logs` - Task activity logs
- `task_reviews` - Review requests and feedback
- `steps` - Step execution history

## Error Codes

| Code | Description | HTTP Status |
|------|-------------|-------------|
| `UNAUTHORIZED` | Invalid or missing authentication | 401 |
| `FORBIDDEN` | Insufficient permissions | 403 |
| `NOT_FOUND` | Resource not found | 404 |
| `VALIDATION_ERROR` | Request validation failed | 400 |
| `CONFLICT` | Resource conflict | 409 |
| `INTERNAL_ERROR` | Server internal error | 500 |

## Performance

- API response time: < 200ms for standard requests
- WebSocket message delivery: < 100ms
- Support for 100+ concurrent WebSocket connections
- Database query optimization for task tree operations

## Security

- JWT-based authentication for all endpoints
- Rate limiting: 100 requests per minute per user
- Input validation and sanitization
- SQL injection prevention through parameterized queries
- CORS configuration for web UI domain

## Dependencies

- `github.com/gorilla/mux` - HTTP routing
- `github.com/gorilla/websocket` - WebSocket support
- `github.com/golang-jwt/jwt/v5` - JWT authentication
- `github.com/mattn/go-sqlite3` - SQLite database driver
- Standard library for core functionality

## API Examples

### Authenticate and Get Tasks
```bash
# Login to get token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/public/login | jq -r .data.token)

# Use token to get tasks
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/projects/my-project/tasks
```

### Create Task with Review
```bash
curl -X POST http://localhost:8080/api/v1/projects/my-project/tasks \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "[FEAT] Implement user authentication",
    "description": "Create authentication system for the web UI",
    "type": "FEAT",
    "review_required": true
  }'
```

### WebSocket Connection
```javascript
const ws = new WebSocket('ws://localhost:8080/api/v1/projects/my-project/ws?token=' + token);

ws.onopen = () => {
  ws.send(JSON.stringify({
    type: 'subscribe',
    channels: ['tasks', 'reviews']
  }));
};

ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  console.log('Received:', message);
};
```