# Task Sorting and Filtering Server-Side Implementation Plan

## Problem Statement
Currently, task sorting and search filtering are performed client-side, which means:
- Only the current page of results is sorted/filtered
- Performance issues with large datasets
- Inconsistent user experience

## Current State Analysis
- **Client-side**: Sorting by created_at, updated_at, title, status, type with asc/desc order
- **Client-side**: Text search in title and description fields  
- **Server-side**: Basic filtering by status, type, parent_id with pagination
- **Database**: Simple SELECT * FROM tasks ORDER BY id (no filtering/sorting parameters)

## Proposed Solution
Move all sorting and filtering to server-side by extending the API and database layer.

## Implementation Plan

### Phase 1: Extend Database Layer
**Files to modify:**
- `/src/tasks/tasks.go` - Add filtering and sorting to ListTasks()

**Changes needed:**
- Add `ListTasksOptions` struct with sorting and search parameters
- Implement SQL query building with WHERE clauses for search
- Add ORDER BY clauses for sorting
- Use parameterized queries to prevent SQL injection

### Phase 2: Extend API Layer  
**Files to modify:**
- `/src/cmd/laserve/handlers/tasks.go` - Add new query parameters

**New parameters to support:**
- `sort_by`: created_at, updated_at, title, status, type
- `sort_order`: asc, desc  
- `search`: text search in title and description

### Phase 3: Update Web UI
**Files to modify:**
- `/src/web-ui/src/components/TaskDashboard.tsx` - Remove client-side sorting/search
- `/src/web-ui/src/services/api.ts` - Add new parameters to API calls

**Changes needed:**
- Remove useMemo-based sorting and filtering
- Pass sort/search parameters to API calls
- Handle loading states during API calls

### Phase 4: Testing and Validation
- Unit tests for database layer changes
- API integration tests for new parameters
- UI tests for updated functionality
- Performance testing with large datasets

## Technical Details

### Database Query Changes
```sql
-- Current query
SELECT * FROM tasks ORDER BY id

-- New query with filtering and sorting
SELECT * FROM tasks 
WHERE (title LIKE ? OR description LIKE ?) 
AND status IN (?)
AND type = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?
```

### API Parameter Examples
```
GET /projects/{project_id}/tasks
?status=todo,in-progress
&sort_by=created_at
&sort_order=desc
&search=authentication
&page=1
&limit=25
```

## Acceptance Criteria
- [ ] All sorting moved to server-side
- [ ] All search filtering moved to server-side  
- [ ] Results are sorted/filtered across entire dataset, not just current page
- [ ] API response times remain acceptable
- [ ] Existing functionality preserved
- [ ] Tests cover new functionality

## Risks and Considerations
- Database query performance with large datasets
- SQL injection prevention with search parameters
- Backward compatibility with existing API calls
- WebSocket real-time updates compatibility