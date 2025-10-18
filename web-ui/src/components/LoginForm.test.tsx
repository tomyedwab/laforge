import { h } from 'preact';
import { render, screen, fireEvent, waitFor } from '@testing-library/preact';
import { LoginForm } from './LoginForm';
import { AuthProvider } from '../contexts/AuthContext';

// Mock fetch
global.fetch = vi.fn();

describe('LoginForm', () => {
  const mockOnSuccess = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
  });

  const renderLoginForm = () => {
    return render(
      <AuthProvider>
        <LoginForm onSuccess={mockOnSuccess} />
      </AuthProvider>
    );
  };

  it('renders login form', () => {
    renderLoginForm();
    
    expect(screen.getByText('Login to LaForge')).toBeInTheDocument();
    expect(screen.getByLabelText('Username')).toBeInTheDocument();
    expect(screen.getByLabelText('Password')).toBeInTheDocument();
    expect(screen.getByText('Login')).toBeInTheDocument();
  });

  it('disables submit button when fields are empty', () => {
    renderLoginForm();
    
    const submitButton = screen.getByText('Login');
    expect(submitButton).toBeDisabled();
  });

  it('enables submit button when fields are filled', async () => {
    renderLoginForm();
    
    const usernameInput = screen.getByLabelText('Username');
    const passwordInput = screen.getByLabelText('Password');
    const submitButton = screen.getByText('Login');

    fireEvent.input(usernameInput, { target: { value: 'testuser' } });
    fireEvent.input(passwordInput, { target: { value: 'testpass' } });

    await waitFor(() => {
      expect(submitButton).not.toBeDisabled();
    });
  });

  it('handles form submission', async () => {
    // Mock successful login response
    (global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        data: {
          token: 'mock-token',
          user_id: 'testuser'
        },
        meta: {
          timestamp: new Date().toISOString(),
          version: '1.0.0'
        }
      })
    });

    renderLoginForm();
    
    const usernameInput = screen.getByLabelText('Username');
    const passwordInput = screen.getByLabelText('Password');
    const submitButton = screen.getByText('Login');

    fireEvent.input(usernameInput, { target: { value: 'testuser' } });
    fireEvent.input(passwordInput, { target: { value: 'testpass' } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        expect.stringContaining('/public/login'),
        expect.objectContaining({
          method: 'POST',
          headers: {
            'Content-Type': 'application/json'
          },
          body: JSON.stringify({
            username: 'testuser',
            password: 'testpass'
          })
        })
      );
    });
  });

  it('calls onSuccess callback after successful login', async () => {
    // Mock successful login response
    (global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        data: {
          token: 'mock-token',
          user_id: 'testuser'
        },
        meta: {
          timestamp: new Date().toISOString(),
          version: '1.0.0'
        }
      })
    });

    renderLoginForm();
    
    const usernameInput = screen.getByLabelText('Username');
    const passwordInput = screen.getByLabelText('Password');
    const submitButton = screen.getByText('Login');

    fireEvent.input(usernameInput, { target: { value: 'testuser' } });
    fireEvent.input(passwordInput, { target: { value: 'testpass' } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockOnSuccess).toHaveBeenCalled();
    });
  });
});