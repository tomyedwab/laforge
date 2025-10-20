# Project Switching Implementation Test Plan

## Overview
This document outlines the testing approach for the project switching feature implemented in the LaForge web UI.

## Implementation Summary

### Components Created/Modified:

1. **ProjectContext.tsx** - New context provider for managing project state
   - Manages selected project state
   - Handles local storage persistence
   - Provides projects list management

2. **ProjectSelector.tsx** - New dropdown component for project selection
   - Fetches projects from API
   - Provides dropdown interface with search/filter capabilities
   - Handles loading and error states
   - Triggers page reload on project change

3. **api.ts** - Updated to support dynamic project ID
   - Removed hardcoded PROJECT_ID constant
   - Added setProjectId() and getProjectId() methods
   - All API endpoints now use dynamic project ID

4. **websocket.ts** - Updated for dynamic project support
   - Added setProjectId() method
   - Reconnects WebSocket when project changes

5. **Header.tsx** - Integrated project selector
   - Added project selector to header layout
   - Maintains existing navigation structure

6. **App.tsx** - Updated main application structure
   - Integrated ProjectProvider
   - Handles WebSocket project updates
   - Simplified component structure

7. **AppContent.tsx** - New component for content management
   - Handles project loading states
   - Manages task selection and detail views
   - Provides responsive layout

## Test Scenarios

### 1. Project Loading
- **Test**: Load web UI without any projects
- **Expected**: Shows "No projects available" message
- **Test**: Load web UI with multiple projects
- **Expected**: Shows project selector with all projects listed

### 2. Project Selection
- **Test**: Select different project from dropdown
- **Expected**: 
  - Page reloads with new project data
  - API calls use new project ID
  - WebSocket reconnects to new project
  - Selected project saved to localStorage

### 3. Local Storage Persistence
- **Test**: Select project, close browser, reopen
- **Expected**: Previously selected project is automatically loaded
- **Test**: Clear localStorage and refresh
- **Expected**: First available project is selected automatically

### 4. API Integration
- **Test**: Verify all API endpoints use correct project ID
- **Expected**: 
  - Task requests use selected project ID
  - Step history requests use selected project ID
  - Review requests use selected project ID

### 5. WebSocket Integration
- **Test**: Switch projects while WebSocket is connected
- **Expected**: WebSocket disconnects and reconnects to new project
- **Test**: Verify real-time updates work after project switch
- **Expected**: Updates received for newly selected project only

### 6. Error Handling
- **Test**: API fails to load projects
- **Expected**: Shows error message with retry button
- **Test**: Network error during project operations
- **Expected**: Appropriate error messages displayed

### 7. Responsive Design
- **Test**: Project selector on mobile devices
- **Expected**: Properly sized and accessible
- **Test**: Dropdown behavior on touch devices
- **Expected**: Touch-friendly interactions

### 8. Accessibility
- **Test**: Keyboard navigation of project selector
- **Expected**: Full keyboard accessibility
- **Test**: Screen reader compatibility
- **Expected**: Proper ARIA labels and announcements

## Manual Testing Steps

1. **Setup**:
   - Ensure laserve backend is running
   - Have multiple projects available in ~/.laforge/projects/
   - Clear browser localStorage

2. **Initial Load**:
   - Open web UI
   - Verify project selector appears in header
   - Check that projects are loaded and displayed

3. **Project Switching**:
   - Click project selector
   - Select different project
   - Verify page reloads
   - Check that data corresponds to new project

4. **Persistence**:
   - Select a project
   - Refresh browser
   - Verify same project is selected
   - Check localStorage contains project data

5. **Error Cases**:
   - Stop laserve backend
   - Try to load projects
   - Verify error handling works
   - Restart backend and test retry

## Automated Testing Considerations

- Unit tests for ProjectContext logic
- Component tests for ProjectSelector
- Integration tests for API project switching
- WebSocket reconnection tests
- Local storage persistence tests

## Browser Compatibility

Test on:
- Chrome/Chromium
- Firefox
- Safari
- Edge

Mobile browsers:
- iOS Safari
- Chrome Mobile
- Samsung Internet

## Performance Considerations

- Project selector should not block UI rendering
- API calls should be cached appropriately
- WebSocket reconnection should be efficient
- Local storage operations should be minimal

## Security Considerations

- Project IDs should be validated
- No sensitive data in localStorage
- Proper authentication for all project operations
- WebSocket connections should be authenticated