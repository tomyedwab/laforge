# Step Database Implementation Plan

## Overview
This plan outlines the implementation of a step database for the LaForge CLI to record each step execution with detailed metadata, as specified in `docs/specs/laforge.md`.

## Requirements Analysis

From the specs, each step should store:
- Step ID (incrementing integer starting at 1)
- Active flag (boolean) to identify steps that have been rolled back
- Parent step ID (most recent active ID preceding this step)
- Commit SHAs before and after the step
- Agent configuration used to run the step
- Metadata for tracking start/end times and token usage

## Current State Analysis

### Existing Infrastructure
- **Task Database**: Already implemented in `tasks/tasks.go` with SQLite
- **Project Structure**: Projects have their own directory with `tasks.db`
- **Step Execution**: Currently generates step IDs via `logging.GenerateStepID()` (timestamp-based)
- **Database Package**: Has utilities for copying and managing SQLite databases

### Gaps Identified
1. No dedicated step database schema
2. No step recording mechanism in the `runStep` function
3. No step ID management (currently uses timestamp-based IDs)
4. No step metadata tracking (commit SHAs, agent config, timing, token usage)

## Implementation Plan

### Phase 1: Database Schema Design

Create a new `steps` table with the following structure:

```sql
CREATE TABLE steps (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    active BOOLEAN DEFAULT TRUE,
    parent_step_id INTEGER,
    commit_sha_before TEXT,
    commit_sha_after TEXT,
    agent_config_json TEXT,
    start_time TIMESTAMP,
    end_time TIMESTAMP,
    duration_ms INTEGER,
    token_usage_json TEXT,
    exit_code INTEGER,
    project_id TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (parent_step_id) REFERENCES steps(id)
);
```

### Phase 2: Core Step Database Package

Create new package `steps` with the following components:

1. **Step Model** (`steps/step.go`):
   - `Step` struct with all required fields
   - `AgentConfig` struct for JSON serialization
   - `TokenUsage` struct for JSON serialization

2. **Database Operations** (`steps/database.go`):
   - `InitStepDB()` - Initialize step database with schema
   - `CreateStep()` - Create a new step record
   - `GetStep()` - Retrieve step by ID
   - `GetLatestActiveStep()` - Get the most recent active step
   - `UpdateStep()` - Update step with completion data
   - `DeactivateStep()` - Mark step as inactive (for rollback)
   - `ListSteps()` - List steps for a project

### Phase 3: Integration with Existing Code

1. **Step ID Management**:
   - Replace timestamp-based step IDs with sequential integers
   - Update `logging.GenerateStepID()` to use step database
   - Modify step logger to use integer step IDs

2. **Step Recording in `runStep`**:
   - Create step record at the beginning of step execution
   - Capture commit SHA before step starts
   - Record agent configuration used
   - Update step record with completion data (commit SHA after, timing, exit code)

3. **Project Integration**:
   - Add step database creation to `projects.CreateProject()`
   - Add `GetProjectStepDatabase()` function to projects package
   - Ensure step database is copied during step isolation

### Phase 4: Enhanced Logging and Monitoring

1. **Step Logger Updates**:
   - Add step database operations to step logger
   - Log step creation, updates, and completion
   - Include step metadata in log entries

2. **Token Usage Tracking**:
   - Extend container metrics to capture token usage
   - Store token usage data in step database
   - Add token usage reporting capabilities

### Phase 5: CLI Enhancements

1. **New Commands**:
   - `laforge steps [project-id]` - List all steps for a project
   - `laforge step info [project-id] [step-id]` - Show detailed step information
   - `laforge step rollback [project-id] [step-id]` - Rollback to a specific step

2. **Step Information Display**:
   - Show step history in project overview
   - Display step details including timing and resource usage
   - Show step relationships (parent/child)

## Implementation Tasks

### Task T3: Create Step Database Schema and Core Package
- Create `steps/step.go` with data models
- Create `steps/database.go` with database operations
- Create `steps/database_test.go` with comprehensive tests
- Add step database initialization to project creation

### Task T4: Integrate Step Recording into Step Execution
- Modify `runStep` function to record step lifecycle
- Update step ID generation to use sequential integers
- Capture commit SHAs before and after step execution
- Record agent configuration and timing data

### Task T5: Add Step Management Commands
- Implement `laforge steps` command for listing steps
- Implement `laforge step info` command for detailed view
- Add step information to existing project commands

### Task T6: Add Token Usage Tracking
- Extend container metrics to capture token usage
- Update step database schema to store token usage
- Add token usage reporting to step commands

### Task T7: Add Step Rollback Functionality
- Implement step deactivation mechanism
- Add rollback command to revert to previous step
- Update git operations to support rollback

## Technical Considerations

### Database Location
- Step database should be separate from task database
- Location: `~/.laforge/projects/<project-id>/steps.db`
- Allows independent backup/restore of step history

### Step ID Format
- Use sequential integers starting at 1
- Format: `1`, `2`, `3` (not prefixed)
- Maintains compatibility with existing logging

### Error Handling
- Step recording should not fail the step execution
- Graceful degradation if step database is unavailable
- Proper cleanup of failed step records

### Performance Impact
- Minimal overhead for step recording
- Database operations should be transactional
- Consider batch operations for step queries

## Testing Strategy

1. **Unit Tests**:
   - Test all database operations
   - Test step ID generation and management
   - Test error handling and edge cases

2. **Integration Tests**:
   - Test step recording during actual step execution
   - Test step database isolation during step execution
   - Test step command functionality

3. **Performance Tests**:
   - Measure overhead of step recording
   - Test with large numbers of steps
   - Test database performance under load

## Success Criteria

- All step executions are recorded in the step database
- Step IDs are sequential integers starting at 1
- Step metadata includes all required fields (commit SHAs, timing, agent config)
- Step commands provide useful information about step history
- No significant performance impact on step execution
- Comprehensive test coverage for new functionality

## Future Enhancements

- Step comparison and diff functionality
- Step analytics and reporting
- Integration with external monitoring systems
- Step template and replay capabilities