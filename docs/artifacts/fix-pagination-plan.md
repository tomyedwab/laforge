# Fix Pagination Implementation Plan

## Problem Analysis

The current TaskDashboard has several pagination issues:

1. **Client-side pagination on filtered data**: The UI loads all tasks from the API, then filters them client-side for the "upcoming" and "completed" tabs, and then applies pagination to the filtered results. This means:
   - If there are no upcoming tasks in the first API page, the first page appears empty
   - The pagination controls show incorrect total pages because they're based on filtered data
   - Each tab doesn't have its own proper server-side pagination

2. **Incorrect pagination logic**: The pagination component receives `Math.ceil(processedTasks.length / itemsPerPage)` as totalPages, which is based on the filtered client-side data, not the actual server-side total.

## Solution

### 1. Separate API Calls for Each Tab
Instead of loading all tasks and filtering client-side, make separate API calls for each tab:
- **Upcoming tab**: Call API with `status=todo,in-progress,in-review` 
- **Completed tab**: Call API with `status=completed`

### 2. Proper Server-Side Pagination
Use the pagination metadata returned from the API to display correct page numbers and totals.

### 3. Maintain Separate Pagination State
Each tab should maintain its own pagination state (current page, items per page).

## Implementation Steps

### Step 1: Update TaskDashboard Component
- Remove client-side filtering by tab
- Create separate state for upcoming and completed tasks
- Make separate API calls based on active tab
- Use API pagination metadata instead of client-side calculations

### Step 2: Update API Service
- Ensure the getTasks method properly handles multiple status values
- Verify pagination metadata is correctly returned

### Step 3: Update Pagination Component
- Ensure it works correctly with server-side pagination data
- No changes needed to the component itself, just how it's used

### Step 4: Testing
- Test with various scenarios:
  - More tasks than fit on one page
  - Empty upcoming tasks on first page
  - Switching between tabs with different page sizes
  - Items per page changes

## Code Changes Required

### TaskDashboard.tsx
- Replace `processedTasks` with direct API data
- Add separate state for pagination metadata
- Update `loadTasks` to make tab-specific API calls
- Update pagination props to use server-side data

### API Calls
- Upcoming: `/tasks?status=todo,in-progress,in-review&page=X&limit=Y`
- Completed: `/tasks?status=completed&page=X&limit=Y`

## Expected Behavior After Fix
- Each tab shows correct pagination based on server data
- No empty pages when tasks exist in other pages
- Tab counts are accurate
- Page navigation works correctly for each tab independently