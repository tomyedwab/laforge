import { useEffect, useState, useRef } from 'preact/hooks';
import { websocketService } from '../services/websocket';
import type { Task, TaskReview, Step } from '../types';

interface UseWebSocketOptions {
  onTaskUpdate?: (task: Task) => void;
  onReviewUpdate?: (review: TaskReview) => void;
  onStepUpdate?: (step: Step) => void;
  enabled?: boolean;
}

export function useWebSocket({
  onTaskUpdate,
  onReviewUpdate,
  onStepUpdate,
  enabled = true,
}: UseWebSocketOptions = {}) {
  const [isConnected, setIsConnected] = useState(false);
  const [connectionError, setConnectionError] = useState<string | null>(null);
  const unsubscribeRef = useRef<(() => void)[]>([]);

  useEffect(() => {
    if (!enabled) {
      return;
    }

    const token = localStorage.getItem(
      import.meta.env.VITE_AUTH_TOKEN_KEY || 'laforge_auth_token'
    );

    // Set up connection status monitoring
    const handleConnect = () => {
      setIsConnected(true);
      setConnectionError(null);
    };

    const handleDisconnect = () => {
      setIsConnected(false);
    };

    const handleError = (error: any) => {
      setConnectionError('WebSocket connection failed');
      console.error('WebSocket error:', error);
    };

    // Set up message handlers
    if (onTaskUpdate) {
      const unsubscribe = websocketService.on(
        'task_updated',
        (data: { task: Task }) => {
          onTaskUpdate(data.task);
        }
      );
      unsubscribeRef.current.push(unsubscribe);
    }

    if (onReviewUpdate) {
      const unsubscribe = websocketService.on(
        'review_updated',
        (data: { review: TaskReview }) => {
          onReviewUpdate(data.review);
        }
      );
      unsubscribeRef.current.push(unsubscribe);
    }

    if (onStepUpdate) {
      const unsubscribe = websocketService.on(
        'step_completed',
        (data: { step: Step }) => {
          onStepUpdate(data.step);
        }
      );
      unsubscribeRef.current.push(unsubscribe);
    }

    // Connect to WebSocket
    websocketService.connect(token);

    // Monitor connection status
    const checkConnection = () => {
      const ws = (websocketService as any).ws;
      if (ws) {
        if (ws.readyState === WebSocket.OPEN) {
          handleConnect();
        } else if (ws.readyState === WebSocket.CLOSED) {
          handleDisconnect();
        }
      }
    };

    checkConnection();
    const interval = setInterval(checkConnection, 1000);

    return () => {
      clearInterval(interval);
      // Unsubscribe from all message handlers
      unsubscribeRef.current.forEach(unsubscribe => unsubscribe());
      unsubscribeRef.current = [];
      // Note: We don't disconnect the WebSocket service here as it might be used by other components
    };
  }, [enabled, onTaskUpdate, onReviewUpdate, onStepUpdate]);

  return {
    isConnected,
    connectionError,
  };
}
