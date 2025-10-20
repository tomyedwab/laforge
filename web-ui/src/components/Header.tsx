import { h } from 'preact';
import { useAuth, useUser } from '../hooks/useAuth';
import { ProjectSelector } from './ProjectSelector';

interface HeaderProps {
  title?: string;
  currentView?: 'tasks' | 'steps' | 'reviews';
  onViewChange?: (view: 'tasks' | 'steps' | 'reviews') => void;
}

export function Header({ title = 'LaForge', currentView = 'tasks', onViewChange }: HeaderProps) {
  const { logout } = useAuth();
  const user = useUser();

  const handleLogout = () => {
    if (confirm('Are you sure you want to logout?')) {
      logout();
    }
  };

  const handleKeyDown = (e: KeyboardEvent, view: 'tasks' | 'steps' | 'reviews') => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      onViewChange?.(view);
    }
  };

  return (
    <header class="app-header" role="banner">
      <div class="header-content">
        <h1 class="app-title">{title}</h1>
         
        {user && (
          <div class="header-middle">
            <ProjectSelector />
          </div>
        )}
        
        {user && (
          <div class="header-actions">
             <nav class="view-navigation" role="navigation" aria-label="Main navigation">
              <button
                class={`nav-button ${currentView === 'tasks' ? 'active' : ''}`}
                onClick={() => onViewChange?.('tasks')}
                onKeyDown={(e) => handleKeyDown(e, 'tasks')}
                type="button"
                role="tab"
                aria-selected={currentView === 'tasks'}
                aria-controls="tasks-panel"
                tabIndex={currentView === 'tasks' ? 0 : -1}
              >
                Tasks
              </button>
              <button
                class={`nav-button ${currentView === 'steps' ? 'active' : ''}`}
                onClick={() => onViewChange?.('steps')}
                onKeyDown={(e) => handleKeyDown(e, 'steps')}
                type="button"
                role="tab"
                aria-selected={currentView === 'steps'}
                aria-controls="steps-panel"
                tabIndex={currentView === 'steps' ? 0 : -1}
              >
                Step History
              </button>
              <button
                class={`nav-button ${currentView === 'reviews' ? 'active' : ''}`}
                onClick={() => onViewChange?.('reviews')}
                onKeyDown={(e) => handleKeyDown(e, 'reviews')}
                type="button"
                role="tab"
                aria-selected={currentView === 'reviews'}
                aria-controls="reviews-panel"
                tabIndex={currentView === 'reviews' ? 0 : -1}
              >
                Reviews
              </button>
            </nav>
            
            <div class="user-menu" role="region" aria-label="User menu">
              <span class="username" aria-label={`Logged in as ${user.id}`}>{user.id}</span>
              <button 
                onClick={handleLogout}
                class="logout-button"
                type="button"
                aria-label="Logout from LaForge"
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