import type { WebSocketMessage, WebSocketSubscribeMessage } from '../types';

export class WebSocketService {
  private ws: WebSocket | null = null;
  private baseUrl: string;
  private projectId: string;
  private reconnectInterval = 5000;
  private shouldReconnect = true;
  private messageHandlers: Map<string, Set<(message: any) => void>> = new Map();

  constructor(baseUrl: string, projectId: string) {
    this.baseUrl = baseUrl;
    this.projectId = projectId;
  }

  setProjectId(projectId: string): void {
    console.log('[WS] setProjectId() called with projectId:', projectId);
    this.projectId = projectId;
    // Reconnect with new project ID
    if (this.ws?.readyState === WebSocket.OPEN) {
      console.log('[WS] Connected, reconnecting with new project ID');
      this.disconnect();
      this.connect();
    }
  }

  connect(token?: string): void {
    console.log('[WS] connect() called, current state:', this.ws?.readyState);
    if (this.ws?.readyState === WebSocket.OPEN) {
      console.log('[WS] Already connected, returning early');
      return;
    }

    const wsUrl = token
      ? `${this.baseUrl}/${this.projectId}/connect?token=${token}`
      : `${this.baseUrl}/${this.projectId}/connect`;

    try {
      this.ws = new WebSocket(wsUrl);

      this.ws.onopen = () => {
        console.log('[WS] ✓ Connection established:', this.baseUrl + '/' + this.projectId + '/connect');
        this.subscribe(['tasks', 'reviews', 'steps']);
      };

      this.ws.onmessage = event => {
        try {
          const message: WebSocketMessage = JSON.parse(event.data);
          this.handleMessage(message);
        } catch (error) {
          console.error('Failed to parse WebSocket message:', error);
        }
      };

      this.ws.onclose = event => {
        console.log(
          `WebSocket disconnected: code=${event.code}, reason=${event.reason}, wasClean=${event.wasClean}`
        );
        this.ws = null;
        if (this.shouldReconnect) {
          console.log(`Reconnecting in ${this.reconnectInterval}ms...`);
          setTimeout(() => this.connect(token), this.reconnectInterval);
        }
      };

      this.ws.onerror = error => {
        console.error('WebSocket error event:', error);
        // Don't reconnect here - let onclose handle it
      };
    } catch (error) {
      console.error('Failed to create WebSocket connection:', error);
      this.ws = null;
      if (this.shouldReconnect) {
        console.log(`Retrying connection in ${this.reconnectInterval}ms...`);
        setTimeout(() => this.connect(token), this.reconnectInterval);
      }
    }
  }

  disconnect(): void {
    this.shouldReconnect = false;
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  subscribe(channels: ('tasks' | 'reviews' | 'steps')[]): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      const message: WebSocketSubscribeMessage = {
        type: 'subscribe',
        channels,
      };
      this.ws.send(JSON.stringify(message));
    }
  }

  on<T>(messageType: string, handler: (data: T) => void): () => void {
    if (!this.messageHandlers.has(messageType)) {
      this.messageHandlers.set(messageType, new Set());
    }
    this.messageHandlers.get(messageType)!.add(handler);

    // Return unsubscribe function
    return () => {
      const handlers = this.messageHandlers.get(messageType);
      if (handlers) {
        handlers.delete(handler);
        if (handlers.size === 0) {
          this.messageHandlers.delete(messageType);
        }
      }
    };
  }

  private handleMessage(message: WebSocketMessage): void {
    const handlers = this.messageHandlers.get(message.type);
    if (handlers) {
      handlers.forEach(handler => {
        try {
          handler(message.data);
        } catch (error) {
          console.error(`Error in message handler for ${message.type}:`, error);
        }
      });
    }
  }
}

// Create singleton instance with dynamic WebSocket URL
const getWebSocketUrl = () => {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const host = window.location.host;
  return `${protocol}//${host}/ws`;
};

export const websocketService = new WebSocketService(getWebSocketUrl(), 'default');
