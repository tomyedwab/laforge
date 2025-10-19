import { h } from 'preact';
import { useAuth, useUser } from '../hooks/useAuth';

interface HeaderProps {
  title?: string;
  currentView?: 'tasks' | 'steps';
  onViewChange?: (view: 'tasks' | 'steps') => void;
}

export function Header({ title = 'LaForge', currentView = 'tasks', onViewChange }: HeaderProps) {
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
          <div class="header-actions">
            <nav class="view-navigation">
              <button
                class={`nav-button ${currentView === 'tasks' ? 'active' : ''}`}
                onClick={() => onViewChange?.('tasks')}
                type="button"
              >
                Tasks
              </button>
              <button
                class={`nav-button ${currentView === 'steps' ? 'active' : ''}`}
                onClick={() => onViewChange?.('steps')}
                type="button"
              >
                Step History
              </button>
            </nav>
            
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
          </div>
        )}
      </div>
    </header>
  );
}