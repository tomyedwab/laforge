# Step Database Schema Reference

This document provides comprehensive technical documentation for the LaForge step database schema and API.

## Overview

The step database is a SQLite database that records every step execution in the LaForge system. It provides a complete audit trail of project execution, enabling step management, rollback functionality, and performance analysis.

## Database Schema

### Primary Table: steps

The main table storing step execution records.

```sql
CREATE TABLE steps (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    active BOOLEAN DEFAULT TRUE,
    parent_step_id INTEGER,
    commit_sha_before TEXT NOT NULL,
    commit_sha_after TEXT,
    agent_config_json TEXT NOT NULL,
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP,
    duration_ms INTEGER,
    token_usage_json TEXT NOT NULL DEFAULT '{}',
    exit_code INTEGER,
    project_id TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (parent_step_id) REFERENCES steps(id)
);
```

#### Column Definitions

| Column | Type | Description | Constraints |
|--------|------|-------------|-------------|
| `id` | INTEGER | Unique step identifier (S1, S2, etc.) | PRIMARY KEY, AUTOINCREMENT |
| `active` | BOOLEAN | Whether step is active (false if rolled back) | DEFAULT TRUE |
| `parent_step_id` | INTEGER | Previous active step ID | FOREIGN KEY to steps(id) |
| `commit_sha_before` | TEXT | Git commit SHA before step execution | NOT NULL |
| `commit_sha_after` | TEXT | Git commit SHA after step completion | NULLABLE |
| `agent_config_json` | TEXT | Serialized agent configuration | NOT NULL |
| `start_time` | TIMESTAMP | Step execution start time | NOT NULL |
| `end_time` | TIMESTAMP | Step completion time | NULLABLE |
| `duration_ms` | INTEGER | Execution duration in milliseconds | NULLABLE |
| `token_usage_json` | TEXT | Token usage statistics | DEFAULT '{}' |
| `exit_code` | INTEGER | Container exit status | NULLABLE |
| `project_id` | TEXT | Project identifier | NOT NULL |
| `created_at` | TIMESTAMP | Record creation time | DEFAULT CURRENT_TIMESTAMP |

### Indexes

```sql
-- Optimize queries by project
CREATE INDEX idx_steps_project_id ON steps(project_id);

-- Optimize active step queries
CREATE INDEX idx_steps_active ON steps(active);

-- Optimize parent step lookups
CREATE INDEX idx_steps_parent_step_id ON steps(parent_step_id);

-- Optimize chronological queries
CREATE INDEX idx_steps_created_at ON steps(created_at);
```

## Data Models

### Step Structure

```go
type Step struct {
    ID              int         `json:"id"`
    Active          bool        `json:"active"`
    ParentStepID    *int        `json:"parent_step_id"`
    CommitSHABefore string      `json:"commit_sha_before"`
    CommitSHAAfter  string      `json:"commit_sha_after"`
    AgentConfig     AgentConfig `json:"agent_config"`
    StartTime       time.Time   `json:"start_time"`
    EndTime         *time.Time  `json:"end_time"`
    DurationMs      *int        `json:"duration_ms"`
    TokenUsage      TokenUsage  `json:"token_usage"`
    ExitCode        *int        `json:"exit_code"`
    ProjectID       string      `json:"project_id"`
    CreatedAt       time.Time   `json:"created_at"`
}
```

### Agent Configuration

```go
type AgentConfig struct {
    Model        string            `json:"model"`
    MaxTokens    int               `json:"max_tokens"`
    Temperature  float64           `json:"temperature"`
    SystemPrompt string            `json:"system_prompt"`
    Tools        []string          `json:"tools"`
    Metadata     map[string]string `json:"metadata"`
}
```

### Token Usage

```go
type TokenUsage struct {
    PromptTokens     int     `json:"prompt_tokens"`
    CompletionTokens int     `json:"completion_tokens"`
    TotalTokens      int     `json:"total_tokens"`
    Cost             float64 `json:"cost"`
}
```

## Database Operations

### Core Operations

#### Create Step
```go
func (sdb *StepDatabase) CreateStep(step *Step) error
```
Creates a new step record with validation for required fields.

#### Get Step
```go
func (sdb *StepDatabase) GetStep(id int) (*Step, error)
```
Retrieves a step by ID with full deserialization of JSON fields.

#### Update Step
```go
func (sdb *StepDatabase) UpdateStep(step *Step) error
```
Updates an existing step record, typically used for completion data.

#### List Steps
```go
func (sdb *StepDatabase) ListSteps(projectID string) ([]*Step, error)
```
Retrieves all steps for a project, ordered by creation time.

#### Get Next Step ID
```go
func (sdb *StepDatabase) GetNextStepID() (int, error)
```
Returns the next sequential step ID for new step creation.

### Advanced Operations

#### Deactivate Step
```go
func (sdb *StepDatabase) DeactivateStep(id int) error
```
Marks a single step as inactive (used in rollback operations).

#### Deactivate Steps From ID
```go
func (sdb *StepDatabase) DeactivateStepsFromID(id int) error
```
Marks all steps with ID >= target ID as inactive (rollback operation).

#### Get Active Steps
```go
func (sdb *StepDatabase) GetActiveSteps(projectID string) ([]*Step, error)
```
Retrieves only active steps for a project.

## Step Lifecycle

### 1. Step Creation
- Database connection established
- Next step ID retrieved
- Step record created with start time and initial state
- Parent step ID set to most recent active step

### 2. Step Execution
- Agent runs in container
- Task database copied for isolation
- Git operations tracked
- Real-time logging captured

### 3. Step Completion
- End time recorded
- Duration calculated
- Exit code captured
- Token usage extracted from logs
- Commit SHA after recorded
- Step record updated

### 4. Step Rollback (Optional)
- Target step and all subsequent steps deactivated
- Git repository reset to pre-step state
- Project returns to step execution point

## JSON Serialization

### Agent Configuration Format
```json
{
  "model": "claude-3.5-sonnet",
  "max_tokens": 4000,
  "temperature": 0.7,
  "system_prompt": "Standard LaForge agent configuration",
  "tools": ["bash", "read", "write", "edit"],
  "metadata": {
    "project_type": "go",
    "agent_version": "1.0.0"
  }
}
```

### Token Usage Format
```json
{
  "prompt_tokens": 1234,
  "completion_tokens": 567,
  "total_tokens": 1801,
  "cost": 0.023
}
```

## Performance Considerations

### Database Optimization
- Indexes on frequently queried columns
- Foreign key constraints for data integrity
- Efficient pagination for large step histories
- Connection pooling for concurrent access

### Storage Efficiency
- JSON serialization for complex data structures
- NULLABLE fields for optional data
- Compressed storage for large agent configurations
- Archival strategies for old step data

## Error Handling

### Validation Rules
- Project ID must be non-empty
- Commit SHA before must be valid git hash
- Start time must be before end time
- Parent step ID must reference existing step
- Exit code must be valid integer

### Error Types
- Database connection failures
- Serialization/deserialization errors
- Constraint violations
- Foreign key reference errors

## Security Considerations

### Data Protection
- Step databases stored in project-specific directories
- File system permissions restrict access
- No sensitive data in step records
- Agent configurations sanitized before storage

### Access Control
- Database access limited to laforge CLI
- No network exposure of step data
- Container isolation prevents direct access
- Audit logging for step modifications

## Integration Points

### Git Integration
- Commit SHA tracking for precise state management
- Git reset capabilities for rollback operations
- Branch management for parallel step execution
- Merge conflict detection and resolution

### Task Management Integration
- Independent operation from task system
- Complementary history tracking
- Cross-reference capabilities
- Unified project state management

### Container Integration
- Step isolation through containerization
- Log extraction and analysis
- Token usage monitoring
- Resource usage tracking

## API Reference

### REST API (Future Enhancement)

Potential future API endpoints:

```
GET /api/projects/{project-id}/steps
GET /api/projects/{project-id}/steps/{step-id}
POST /api/projects/{project-id}/steps/{step-id}/rollback
GET /api/projects/{project-id}/steps/active
```

### Command Line Interface

Current CLI commands:

```bash
laforge steps [project-id]
laforge step info [project-id] [step-id]  
laforge step rollback [project-id] [step-id]
```

## Monitoring and Analytics

### Metrics Collection
- Step execution frequency
- Average step duration
- Token usage trends
- Error rate analysis
- Rollback frequency

### Performance Monitoring
- Database query performance
- Storage usage growth
- Index efficiency
- Connection pool utilization

## Maintenance

### Database Maintenance
- Regular integrity checks
- Index optimization
- Storage cleanup
- Backup strategies

### Data Archival
- Old step data archival
- Compression strategies
- Historical data retention
- Performance impact mitigation