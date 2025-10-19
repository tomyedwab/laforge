import type { WebSocketMessage, WebSocketSubscribeMessage } from '../types';

export class WebSocketService {
  private ws: WebSocket | null = null;
  private url: string;
  private reconnectInterval = 5000;
  private shouldReconnect = true;
  private messageHandlers: Map<string, Set<(message: any) => void>> = new Map();

  constructor(url: string) {
    this.url = url;
  }

  connect(token?: string): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      return;
    }

    const wsUrl = token ? `${this.url}?token=${token}` : this.url;

    try {
      this.ws = new WebSocket(wsUrl);

      this.ws.onopen = () => {
        console.log('WebSocket connected');
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

      this.ws.onclose = () => {
        console.log('WebSocket disconnected');
        this.ws = null;
        if (this.shouldReconnect) {
          setTimeout(() => this.connect(token), this.reconnectInterval);
        }
      };

      this.ws.onerror = error => {
        console.error('WebSocket error:', error);
      };
    } catch (error) {
      console.error('Failed to create WebSocket connection:', error);
      if (this.shouldReconnect) {
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

// Create singleton instance
const WS_URL = import.meta.env.VITE_WS_URL || 'ws://localhost:8080/api/v1';
const PROJECT_ID = import.meta.env.VITE_PROJECT_ID || 'laforge-main';
export const websocketService = new WebSocketService(
  `${WS_URL}/projects/${PROJECT_ID}/ws`
);
