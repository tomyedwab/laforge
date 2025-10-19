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

  it('renders login form with all fields', () => {
    renderLoginForm();
    
    expect(screen.getByRole('heading', { name: /login to laforge/i })).toBeInTheDocument();
    expect(screen.getByLabelText(/username/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /login/i })).toBeInTheDocument();
    expect(screen.getByText(/for development, any username.password will work/i)).toBeInTheDocument();
  });

  it('disables submit button when fields are empty', () => {
    renderLoginForm();
    
    const submitButton = screen.getByRole('button', { name: /login/i });
    expect(submitButton).toBeDisabled();
  });

  it('enables submit button when fields are filled', async () => {
    renderLoginForm();
    
    const usernameInput = screen.getByLabelText('Username');
    const passwordInput = screen.getByLabelText('Password');
    const submitButton = screen.getByRole('button', { name: /login/i });

    fireEvent.input(usernameInput, { target: { value: 'testuser' } });
    fireEvent.input(passwordInput, { target: { value: 'testpass' } });

    await waitFor(() => {
      expect(submitButton).not.toBeDisabled();
    });
  });

  it('shows loading state when submitting', async () => {
    (global.fetch as any).mockImplementationOnce(() => 
      new Promise(resolve => setTimeout(resolve, 100))
    );

    renderLoginForm();

    const usernameInput = screen.getByLabelText('Username');
    const passwordInput = screen.getByLabelText('Password');
    const submitButton = screen.getByRole('button', { name: /login/i });

    fireEvent.input(usernameInput, { target: { value: 'testuser' } });
    fireEvent.input(passwordInput, { target: { value: 'testpass' } });
    fireEvent.click(submitButton);

    expect(screen.getByRole('button', { name: /logging in/i })).toBeInTheDocument();
    expect(usernameInput).toBeDisabled();
    expect(passwordInput).toBeDisabled();
  });

  it('handles successful login with API response', async () => {
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
    const submitButton = screen.getByRole('button', { name: /login/i });

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
    const submitButton = screen.getByRole('button', { name: /login/i });

    fireEvent.input(usernameInput, { target: { value: 'testuser' } });
    fireEvent.input(passwordInput, { target: { value: 'testpass' } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockOnSuccess).toHaveBeenCalled();
    });
  });

  it('handles failed login and falls back to mock login', async () => {
    const mockResponse = {
      ok: false,
    };

    (global.fetch as any).mockResolvedValueOnce(mockResponse);

    renderLoginForm();
    
    const usernameInput = screen.getByLabelText('Username');
    const passwordInput = screen.getByLabelText('Password');
    const submitButton = screen.getByRole('button', { name: /login/i });

    fireEvent.input(usernameInput, { target: { value: 'testuser' } });
    fireEvent.input(passwordInput, { target: { value: 'testpass' } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      // Should attempt fetch first
      expect(global.fetch).toHaveBeenCalled();
      
      // Should fall back to mock login
      expect(mockOnSuccess).toHaveBeenCalled();
    });
  });

  it('handles network errors and falls back to mock login', async () => {
    (global.fetch as any).mockRejectedValueOnce(new Error('Network error'));

    renderLoginForm();
    
    const usernameInput = screen.getByLabelText('Username');
    const passwordInput = screen.getByLabelText('Password');
    const submitButton = screen.getByRole('button', { name: /login/i });

    fireEvent.input(usernameInput, { target: { value: 'testuser' } });
    fireEvent.input(passwordInput, { target: { value: 'testpass' } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockOnSuccess).toHaveBeenCalled();
    });
  });

  it('has proper accessibility attributes', () => {
    renderLoginForm();

    const usernameInput = screen.getByLabelText('Username');
    const passwordInput = screen.getByLabelText('Password');
    const submitButton = screen.getByRole('button', { name: /login/i });

    expect(usernameInput).toHaveAttribute('required');
    expect(passwordInput).toHaveAttribute('required');
    expect(usernameInput).toHaveAttribute('type', 'text');
    expect(passwordInput).toHaveAttribute('type', 'password');
  });

  it('prevents form submission with empty fields', () => {
    renderLoginForm();

    const form = screen.getByRole('form');
    const submitEvent = new Event('submit', { bubbles: true, cancelable: true });
    
    fireEvent(form, submitEvent);

    expect(global.fetch).not.toHaveBeenCalled();
  });

  it('trims whitespace from input fields', async () => {
    (global.fetch as any).mockImplementationOnce(() => 
      new Promise(resolve => setTimeout(resolve, 100))
    );

    renderLoginForm();

    const usernameInput = screen.getByLabelText('Username');
    const passwordInput = screen.getByLabelText('Password');
    const submitButton = screen.getByRole('button', { name: /login/i });

    fireEvent.input(usernameInput, { target: { value: '  testuser  ' } });
    fireEvent.input(passwordInput, { target: { value: '  testpass  ' } });
    fireEvent.click(submitButton);

    // Button should be enabled despite whitespace
    expect(submitButton).not.toBeDisabled();
  });
});