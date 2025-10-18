import { h } from 'preact';
import { useAuth, useUser } from '../hooks/useAuth';

interface HeaderProps {
  title?: string;
}

export function Header({ title = 'LaForge' }: HeaderProps) {
  const { logout } = useAuth();
  const user = useUser();

  const handleLogout = () => {
    if (confirm('Are you sure you want to logout?')) {
      logout();
    }
  };

  return (
    <header class="app-header">
      <div class="header-content">
        <h1 class="app-title">{title}</h1>
        
        {user && (
          <div class="user-menu">
            <span class="username">{user.id}</span>
            <button 
              onClick={handleLogout}
              class="logout-button"
              type="button"
            >
              Logout
            </button>
          </div>
        )}
      </div>
    </header>
  );
}