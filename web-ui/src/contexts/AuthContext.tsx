import { createContext, h } from 'preact';
import { useContext, useState, useEffect } from 'preact/hooks';

interface AuthUser {
  id: string;
  token: string;
}

interface AuthContextType {
  user: AuthUser | null;
  isLoading: boolean;
  error: string | null;
  login: (token: string, userId: string) => void;
  logout: () => void;
  clearError: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

const AUTH_TOKEN_KEY = import.meta.env.VITE_AUTH_TOKEN_KEY || 'laforge_auth_token';
const AUTH_USER_KEY = import.meta.env.VITE_AUTH_USER_KEY || 'laforge_auth_user';

export function AuthProvider({ children }: { children: h.JSX.Element }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Load auth state from localStorage on mount
  useEffect(() => {
    const loadAuthState = () => {
      try {
        const token = localStorage.getItem(AUTH_TOKEN_KEY);
        const userData = localStorage.getItem(AUTH_USER_KEY);
        
        if (token && userData) {
          const parsedUser = JSON.parse(userData);
          setUser({ id: parsedUser.id, token });
        }
      } catch (error) {
        console.error('Failed to load auth state:', error);
        // Clear potentially corrupted data
        localStorage.removeItem(AUTH_TOKEN_KEY);
        localStorage.removeItem(AUTH_USER_KEY);
      } finally {
        setIsLoading(false);
      }
    };

    loadAuthState();
  }, []);

  const login = (token: string, userId: string) => {
    try {
      setIsLoading(true);
      setError(null);
      
      const userData = { id: userId, token };
      
      // Store in localStorage
      localStorage.setItem(AUTH_TOKEN_KEY, token);
      localStorage.setItem(AUTH_USER_KEY, JSON.stringify({ id: userId }));
      
      // Update state
      setUser(userData);
    } catch (error) {
      setError('Failed to save authentication data');
      console.error('Login error:', error);
    } finally {
      setIsLoading(false);
    }
  };

  const logout = () => {
    try {
      setIsLoading(true);
      
      // Clear from localStorage
      localStorage.removeItem(AUTH_TOKEN_KEY);
      localStorage.removeItem(AUTH_USER_KEY);
      
      // Update state
      setUser(null);
      setError(null);
    } catch (error) {
      setError('Failed to clear authentication data');
      console.error('Logout error:', error);
    } finally {
      setIsLoading(false);
    }
  };

  const clearError = () => {
    setError(null);
  };

  const value: AuthContextType = {
    user,
    isLoading,
    error,
    login,
    logout,
    clearError,
  };

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}