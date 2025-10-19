import { h } from 'preact';
import { useState } from 'preact/hooks';

interface MobileNavigationProps {
  currentView: 'tasks' | 'steps' | 'reviews';
  onViewChange: (view: 'tasks' | 'steps' | 'reviews') => void;
}

export function MobileNavigation({ currentView, onViewChange }: MobileNavigationProps) {
  const [isMenuOpen, setIsMenuOpen] = useState(false);

  const toggleMenu = () => {
    setIsMenuOpen(!isMenuOpen);
  };

  const handleViewChange = (view: 'tasks' | 'steps' | 'reviews') => {
    onViewChange(view);
    setIsMenuOpen(false);
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Escape') {
      setIsMenuOpen(false);
    }
  };

  return (
    <div class="mobile-navigation">
      <button
        class="mobile-menu-toggle"
        onClick={toggleMenu}
        onKeyDown={handleKeyDown}
        type="button"
        aria-expanded={isMenuOpen}
        aria-controls="mobile-menu"
        aria-label="Toggle navigation menu"
      >
        <span class="hamburger-line"></span>
        <span class="hamburger-line"></span>
        <span class="hamburger-line"></span>
      </button>

      {isMenuOpen && (
        <>
          <div
            class="mobile-menu-overlay"
            onClick={() => setIsMenuOpen(false)}
            role="presentation"
          />
          <nav
            id="mobile-menu"
            class="mobile-menu"
            role="navigation"
            aria-label="Mobile navigation"
          >
            <button
              class={`mobile-nav-item ${currentView === 'tasks' ? 'active' : ''}`}
              onClick={() => handleViewChange('tasks')}
              type="button"
              role="tab"
              aria-selected={currentView === 'tasks'}
            >
              <span class="nav-icon">📋</span>
              Tasks
            </button>
            <button
              class={`mobile-nav-item ${currentView === 'steps' ? 'active' : ''}`}
              onClick={() => handleViewChange('steps')}
              type="button"
              role="tab"
              aria-selected={currentView === 'steps'}
            >
              <span class="nav-icon">📊</span>
              Step History
            </button>
            <button
              class={`mobile-nav-item ${currentView === 'reviews' ? 'active' : ''}`}
              onClick={() => handleViewChange('reviews')}
              type="button"
              role="tab"
              aria-selected={currentView === 'reviews'}
            >
              <span class="nav-icon">👁️</span>
              Reviews
            </button>
          </nav>
        </>
      )}
    </div>
  );
}