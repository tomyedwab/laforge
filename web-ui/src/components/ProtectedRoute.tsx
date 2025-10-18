import { h } from 'preact';
import { useAuth, useIsAuthenticated } from '../hooks/useAuth';
import { LoginForm } from './LoginForm';

interface ProtectedRouteProps {
  children: h.JSX.Element;
  fallback?: h.JSX.Element;
  redirectTo?: string;
}

export function ProtectedRoute({ children, fallback, redirectTo }: ProtectedRouteProps) {
  const { isAuthenticated, isLoading } = useIsAuthenticated();

  if (isLoading) {
    return (
      <div class="loading-container">
        <div class="loading-spinner">Loading...</div>
      </div>
    );
  }

  if (!isAuthenticated) {
    if (fallback) {
      return fallback;
    }
    
    return <LoginForm />;
  }

  return children;
}

interface AuthGuardProps {
  children: h.JSX.Element;
  required?: boolean;
}

export function AuthGuard({ children, required = true }: AuthGuardProps) {
  return (
    <ProtectedRoute>
      {children}
    </ProtectedRoute>
  );
}