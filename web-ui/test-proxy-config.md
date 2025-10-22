# Vite Proxy Configuration Test

This document demonstrates the proxy configuration changes made for Task T48.

## Configuration Changes

### 1. Vite Config (vite.config.ts)
```typescript
server: {
  port: 3000,
  host: true,
  proxy: {
    '/api': {
      target: 'http://localhost:8080',
      changeOrigin: true,
      secure: false,
      ws: true,  // Enable WebSocket proxying
    },
  },
}
```

### 2. Environment Variables
- `.env.development`: `VITE_API_BASE_URL=/api/v1`
- `.env.example`: `VITE_API_BASE_URL=/api/v1`
- `.env.production`: `VITE_API_BASE_URL=/api/v1`

### 3. API Service (api.ts)
```typescript
const getApiBaseUrl = () => {
  const envUrl = import.meta.env.VITE_API_BASE_URL;
  if (envUrl && envUrl.startsWith('/')) {
    // Relative URL - use current domain
    return envUrl;
  }
  return envUrl || 'http://localhost:8080/api/v1';
};
```

### 4. WebSocket Service (websocket.ts)
```typescript
const getWebSocketUrl = () => {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const host = window.location.host;
  return import.meta.env.VITE_WS_URL || `${protocol}//${host}/api/v1`;
};
```

## How It Works

1. **Development Mode**: When running `npm run dev`, Vite dev server starts on port 3000
2. **API Requests**: Requests to `/api/v1/*` are proxied to `http://localhost:8080/api/v1/*`
3. **WebSocket Connections**: WS connections to `/api/v1/*` are proxied to `ws://localhost:8080/api/v1/*`
4. **Cross-Origin Access**: Since all requests go through the same domain:port, CORS issues are avoided
5. **Remote Access**: You can now access from other machines via `http://<ip-address>:3000`

## Testing

To test the configuration:

1. Start the API server: `./bin/laserve --host 0.0.0.0 --port 8080`
2. Start the web UI: `cd web-ui && npm run dev`
3. Access the UI at `http://localhost:3000` or `http://<your-ip>:3000`
4. API requests should work through the proxy
5. WebSocket connections should work through the proxy

## Benefits

- ✅ No more hardcoded localhost URLs
- ✅ Works when accessing from other machines
- ✅ Consistent API endpoint configuration
- ✅ WebSocket support through proxy
- ✅ Development and production parity