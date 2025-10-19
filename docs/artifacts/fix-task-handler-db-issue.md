# Fix TaskHandler Database Access Issue

## Problem Analysis

The TaskForm component is not showing parent tasks in the dropdown because the backend API is failing. The issue is in the TaskHandler struct and functions:

1. **TaskHandler struct** is missing a `db` field
2. **Multiple functions** are trying to use `h.db` which doesn't exist
3. **Constructor** doesn't accept a database parameter
4. **Main.go** passes a database parameter to NewTaskHandler but the function signature doesn't accept it

## Root Cause

In `/src/cmd/laserve/main.go` line 101:
```go
taskHandler := handlers.NewTaskHandler(db, wsServer)
```

But in `/src/cmd/laserve/handlers/tasks.go` line 22:
```go
func NewTaskHandler(wsServer *websocket.Server) *TaskHandler {
    return &TaskHandler{wsServer: wsServer}
}
```

The constructor doesn't accept a database parameter, but main.go is trying to pass one.

## Solution

1. Add `db *sql.DB` field to TaskHandler struct
2. Update NewTaskHandler to accept and store the database parameter
3. Update all functions that use `h.db` to use the project database instead
4. The correct approach is to use `h.getProjectDB(projectID)` for project-specific operations

## Implementation Plan

### Step 1: Fix TaskHandler struct and constructor
- Add `db *sql.DB` field to TaskHandler
- Update NewTaskHandler to accept database parameter

### Step 2: Fix functions that incorrectly use h.db
- GetTask (line 250)
- CreateTask (lines 296, 303)
- UpdateTask (line 357)
- UpdateTaskStatus (lines 425, 443)
- DeleteTask (lines 477, 488)
- GetTaskLogs (line 548)
- CreateTaskLog (lines 621, 632)
- GetNextTask (line 664)
- GetTaskReviews (lines 733, 744)
- CreateTaskReview (lines 803, 814)

### Step 3: Test the fix
- Verify TaskForm now loads parent tasks correctly
- Test all affected API endpoints

## Alternative Solution

The task suggests using autocomplete instead of dropdown for better UX with many tasks. This could be implemented as a separate enhancement after fixing the backend issue.

## Files to Modify

- `/src/cmd/laserve/handlers/tasks.go` - Fix struct, constructor, and all functions
- Test with `/src/web-ui/src/components/TaskForm.tsx`