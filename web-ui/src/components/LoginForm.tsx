import { h } from 'preact';
import { useState } from 'preact/hooks';
import { useAuth } from '../hooks/useAuth';

interface LoginFormProps {
  onSuccess?: () => void;
}

export function LoginForm({ onSuccess }: LoginFormProps) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const { login, error, clearError } = useAuth();

  const handleSubmit = async (e: Event) => {
    e.preventDefault();

    if (!username.trim() || !password.trim()) {
      return;
    }

    setIsLoading(true);
    clearError();

    try {
      // For now, we'll use a simple hardcoded login
      // In a real app, this would call the login API endpoint
      const response = await fetch(
        `${import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api/v1'}/public/login`,
        {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({ username, password }),
        }
      );

      if (!response.ok) {
        throw new Error('Invalid credentials');
      }

      const data = await response.json();

      // Store the token and user info
      login(data.token, data.user_id);

      if (onSuccess) {
        onSuccess();
      }
    } catch (error) {
      console.error('Login failed:', error);
      // For development, we'll accept any non-empty credentials
      // and generate a mock token
      if (username.trim() && password.trim()) {
        const mockToken = btoa(`${username}:${Date.now()}`);
        login(mockToken, username);
        if (onSuccess) {
          onSuccess();
        }
      }
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div class="login-form">
      <form onSubmit={handleSubmit}>
        <h2>Login to LaForge</h2>

        {error && <div class="error-message">{error}</div>}

        <div class="form-group">
          <label htmlFor="username">Username</label>
          <input
            id="username"
            type="text"
            value={username}
            onInput={e => setUsername((e.target as HTMLInputElement).value)}
            disabled={isLoading}
            required
          />
        </div>

        <div class="form-group">
          <label htmlFor="password">Password</label>
          <input
            id="password"
            type="password"
            value={password}
            onInput={e => setPassword((e.target as HTMLInputElement).value)}
            disabled={isLoading}
            required
          />
        </div>

        <button
          type="submit"
          disabled={isLoading || !username.trim() || !password.trim()}
        >
          {isLoading ? 'Logging in...' : 'Login'}
        </button>

        <div class="login-help">
          <p>For development, any username/password will work.</p>
        </div>
      </form>
    </div>
  );
}
