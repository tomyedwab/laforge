# WebSocket Connection Errors Fix Plan

## Problem Analysis

### Issue 1: Superfluous Response.WriteHeader Error
**Location**: `/src/cmd/laserve/websocket/handler.go:136`
**Root Cause**: Calling `http.Error()` after a failed WebSocket upgrade attempt. Once `upgrader.Upgrade()` is called, the HTTP response has already been written, making subsequent `http.Error()` calls invalid.

### Issue 2: Vite Proxy EPIPE Error
**Location**: Vite development server proxy configuration
**Root Cause**: The vite proxy might not be properly handling WebSocket connections, causing pipe errors when the connection is interrupted.

## Solution

### Fix 1: WebSocket Handler Error Handling
**File**: `/src/cmd/laserve/websocket/handler.go`
**Changes**:
1. Remove the `http.Error()` call after failed upgrade
2. Simply return after logging the error, as the upgrade failure is already handled

### Fix 2: Vite Proxy Configuration
**File**: `/src/web-ui/vite.config.ts`
**Changes**:
1. Add WebSocket-specific proxy configuration
2. Configure timeout and error handling for WebSocket connections
3. Add connection reset handling

### Fix 3: WebSocket Service Improvements
**File**: `/src/web-ui/src/services/websocket.ts`
**Changes**:
1. Add better error handling for connection failures
2. Implement exponential backoff for reconnection attempts
3. Add connection state monitoring

## Implementation Steps

1. **Fix WebSocket Handler** (5 minutes)
   - Modify error handling in `HandleWebSocket` function
   - Remove problematic `http.Error()` call

2. **Update Vite Configuration** (5 minutes)
   - Enhance WebSocket proxy settings
   - Add connection timeout and error handling

3. **Improve WebSocket Service** (10 minutes)
   - Add better error handling and logging
   - Implement connection state management

4. **Testing** (10 minutes)
   - Test WebSocket connections in development
   - Verify no more EPIPE or superfluous header errors
   - Test reconnection scenarios

## Acceptance Criteria
- [ ] No more "superfluous response.WriteHeader" errors in server logs
- [ ] No more "write EPIPE" errors in vite logs
- [ ] WebSocket connections stay open and stable
- [ ] Automatic reconnection works properly
- [ ] Real-time updates continue to function