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
  const callbacksRef = useRef({ onTaskUpdate, onReviewUpdate, onStepUpdate });

  // Update callback refs whenever callbacks change (but don't trigger effect)
  callbacksRef.current = { onTaskUpdate, onReviewUpdate, onStepUpdate };

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

    // Set up message handlers using refs to avoid re-triggering effect
    if (callbacksRef.current.onTaskUpdate) {
      const unsubscribe = websocketService.on(
        'task_updated',
        (data: { task: Task }) => {
          callbacksRef.current.onTaskUpdate?.(data.task);
        }
      );
      unsubscribeRef.current.push(unsubscribe);
    }

    if (callbacksRef.current.onReviewUpdate) {
      const unsubscribe = websocketService.on(
        'review_updated',
        (data: { review: TaskReview }) => {
          callbacksRef.current.onReviewUpdate?.(data.review);
        }
      );
      unsubscribeRef.current.push(unsubscribe);
    }

    if (callbacksRef.current.onStepUpdate) {
      const unsubscribe = websocketService.on(
        'step_completed',
        (data: { step: Step }) => {
          callbacksRef.current.onStepUpdate?.(data.step);
        }
      );
      unsubscribeRef.current.push(unsubscribe);
    }

    // Connect to WebSocket
    websocketService.connect(token || undefined);

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
  }, [enabled]);

  return {
    isConnected,
    connectionError,
  };
}
