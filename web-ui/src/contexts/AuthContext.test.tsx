import { h } from 'preact';
import { renderHook, act, waitFor } from '@testing-library/preact';
import { AuthProvider, useAuth } from './AuthContext';

describe('AuthContext', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('provides authentication context', () => {
    const { result } = renderHook(() => useAuth(), {
      wrapper: AuthProvider
    });

    expect(result.current.user).toBeNull();
    expect(result.current.isLoading).toBe(true);
    expect(result.current.error).toBeNull();
    expect(typeof result.current.login).toBe('function');
    expect(typeof result.current.logout).toBe('function');
    expect(typeof result.current.clearError).toBe('function');
  });

  it('handles login', async () => {
    const { result } = renderHook(() => useAuth(), {
      wrapper: AuthProvider
    });

    // Wait for initial loading to complete
    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    act(() => {
      result.current.login('test-token', 'testuser');
    });

    expect(result.current.user).toEqual({
      id: 'testuser',
      token: 'test-token'
    });
    expect(localStorage.getItem('laforge_auth_token')).toBe('test-token');
    expect(localStorage.getItem('laforge_auth_user')).toBe(JSON.stringify({ id: 'testuser' }));
  });

  it('handles logout', async () => {
    const { result } = renderHook(() => useAuth(), {
      wrapper: AuthProvider
    });

    // Login first
    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    act(() => {
      result.current.login('test-token', 'testuser');
    });

    expect(result.current.user).not.toBeNull();

    // Then logout
    act(() => {
      result.current.logout();
    });

    expect(result.current.user).toBeNull();
    expect(localStorage.getItem('laforge_auth_token')).toBeNull();
    expect(localStorage.getItem('laforge_auth_user')).toBeNull();
  });

  it('loads auth state from localStorage on mount', () => {
    localStorage.setItem('laforge_auth_token', 'existing-token');
    localStorage.setItem('laforge_auth_user', JSON.stringify({ id: 'existinguser' }));

    const { result } = renderHook(() => useAuth(), {
      wrapper: AuthProvider
    });

    expect(result.current.user).toEqual({
      id: 'existinguser',
      token: 'existing-token'
    });
  });

  it('handles corrupted localStorage data gracefully', () => {
    localStorage.setItem('laforge_auth_token', 'existing-token');
    localStorage.setItem('laforge_auth_user', 'invalid-json');

    const { result } = renderHook(() => useAuth(), {
      wrapper: AuthProvider
    });

    expect(result.current.user).toBeNull();
    expect(localStorage.getItem('laforge_auth_token')).toBeNull();
    expect(localStorage.getItem('laforge_auth_user')).toBeNull();
  });

  it('clears errors', async () => {
    const { result } = renderHook(() => useAuth(), {
      wrapper: AuthProvider
    });

    // Simulate an error by directly setting it (this would normally happen in login/logout)
    act(() => {
      result.current.login('', ''); // This will cause an error
    });

    await waitFor(() => {
      expect(result.current.error).not.toBeNull();
    });

    act(() => {
      result.current.clearError();
    });

    expect(result.current.error).toBeNull();
  });
});