import { useAuth as useAuthContext } from '../contexts/AuthContext';

export function useAuth() {
  const context = useAuthContext();
  return context;
}

export function useIsAuthenticated() {
  const { user, isLoading } = useAuthContext();
  return { isAuthenticated: !!user, isLoading };
}

export function useUser() {
  const { user } = useAuthContext();
  return user;
}