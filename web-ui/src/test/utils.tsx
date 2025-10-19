import { render } from '@testing-library/preact';
import { h } from 'preact';
import { AuthProvider } from '../contexts/AuthContext';

// Mock environment variables
const mockEnv = {
  VITE_API_BASE_URL: 'http://localhost:8080/api/v1',
  VITE_AUTH_TOKEN_KEY: 'laforge_auth_token',
  VITE_WS_URL: 'ws://localhost:8080/ws',
};

// @ts-ignore
import.meta.env = { ...import.meta.env, ...mockEnv };

// Test wrapper with providers
export function renderWithProviders(ui: h.JSX.Element, options = {}) {
  return render(
    <AuthProvider>
      {ui}
    </AuthProvider>,
    options
  );
}

// Mock API responses
export const mockApiResponses = {
  login: {
    success: {
      data: {
        token: 'mock-token',
        user_id: 'test-user',
      },
    },
    error: {
      error: 'Invalid credentials',
    },
  },
  tasks: {
    list: {
      tasks: [
        {
          id: 1,
          title: 'Test Task',
          description: 'Test description',
          type: 'FEAT',
          status: 'todo',
          created_at: '2024-01-01T00:00:00Z',
          updated_at: '2024-01-01T00:00:00Z',
          children: [],
        },
      ],
      total: 1,
      page: 1,
      pages: 1,
    },
  },
};

// Mock WebSocket
export class MockWebSocket {
  static instances: MockWebSocket[] = [];
  url: string;
  readyState: number;
  onopen: ((event: Event) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;

  constructor(url: string) {
    this.url = url;
    this.readyState = WebSocket.CONNECTING;
    MockWebSocket.instances.push(this);
    
    // Simulate connection after a short delay
    setTimeout(() => {
      this.readyState = WebSocket.OPEN;
      this.onopen?.(new Event('open'));
    }, 10);
  }

  send(data: string) {
    // Mock send implementation
  }

  close() {
    this.readyState = WebSocket.CLOSED;
    this.onclose?.(new CloseEvent('close'));
  }

  static reset() {
    MockWebSocket.instances = [];
  }
}

// Replace global WebSocket with mock
export function setupWebSocketMock() {
  (global as any).WebSocket = MockWebSocket;
}

// Clean up after tests
export function cleanupWebSocketMock() {
  MockWebSocket.reset();
}