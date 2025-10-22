import type { WebSocketMessage, WebSocketSubscribeMessage } from '../types';

export class WebSocketService {
  private ws: WebSocket | null = null;
  private baseUrl: string;
  private projectId: string;
  private reconnectInterval = 5000;
  private shouldReconnect = true;
  private messageHandlers: Map<string, Set<(message: any) => void>> = new Map();

  constructor(baseUrl: string, projectId: string = 'laforge-main') {
    this.baseUrl = baseUrl;
    this.projectId = projectId;
  }

  setProjectId(projectId: string): void {
    this.projectId = projectId;
    // Reconnect with new project ID
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.disconnect();
      this.connect();
    }
  }

  connect(token?: string): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      return;
    }

    const wsUrl = token ? 
      `${this.baseUrl}/projects/${this.projectId}/ws?token=${token}` : 
      `${this.baseUrl}/projects/${this.projectId}/ws`;

    try {
      this.ws = new WebSocket(wsUrl);

      this.ws.onopen = () => {
        console.log('WebSocket connected successfully');
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

      this.ws.onclose = (event) => {
        console.log(`WebSocket disconnected: code=${event.code}, reason=${event.reason}, wasClean=${event.wasClean}`);
        this.ws = null;
        if (this.shouldReconnect) {
          console.log(`Reconnecting in ${this.reconnectInterval}ms...`);
          setTimeout(() => this.connect(token), this.reconnectInterval);
        }
      };

      this.ws.onerror = (error) => {
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
  return import.meta.env.VITE_WS_URL || `${protocol}//${host}/api/v1`;
};

export const websocketService = new WebSocketService(getWebSocketUrl());
