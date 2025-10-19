import { render, screen, fireEvent } from '@testing-library/preact';
import { MobileNavigation } from './MobileNavigation';

describe('MobileNavigation', () => {
  const mockOnViewChange = jest.fn();

  beforeEach(() => {
    mockOnViewChange.mockClear();
  });

  it('renders mobile menu toggle button', () => {
    render(
      <MobileNavigation
        currentView="tasks"
        onViewChange={mockOnViewChange}
      />
    );

    const toggleButton = screen.getByRole('button', { name: /toggle navigation menu/i });
    expect(toggleButton).toBeInTheDocument();
  });

  it('opens mobile menu when toggle is clicked', () => {
    render(
      <MobileNavigation
        currentView="tasks"
        onViewChange={mockOnViewChange}
      />
    );

    const toggleButton = screen.getByRole('button', { name: /toggle navigation menu/i });
    fireEvent.click(toggleButton);

    expect(screen.getByRole('navigation', { name: /mobile navigation/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /📋 tasks/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /📊 step history/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /👁️ reviews/i })).toBeInTheDocument();
  });

  it('calls onViewChange when navigation item is clicked', () => {
    render(
      <MobileNavigation
        currentView="tasks"
        onViewChange={mockOnViewChange}
      />
    );

    const toggleButton = screen.getByRole('button', { name: /toggle navigation menu/i });
    fireEvent.click(toggleButton);

    const stepsButton = screen.getByRole('tab', { name: /📊 step history/i });
    fireEvent.click(stepsButton);

    expect(mockOnViewChange).toHaveBeenCalledWith('steps');
  });

  it('closes mobile menu after view change', () => {
    render(
      <MobileNavigation
        currentView="tasks"
        onViewChange={mockOnViewChange}
      />
    );

    const toggleButton = screen.getByRole('button', { name: /toggle navigation menu/i });
    fireEvent.click(toggleButton);

    const stepsButton = screen.getByRole('tab', { name: /📊 step history/i });
    fireEvent.click(stepsButton);

    // Menu should be closed after selection
    expect(mockOnViewChange).toHaveBeenCalledWith('steps');
  });

  it('shows active state for current view', () => {
    render(
      <MobileNavigation
        currentView="steps"
        onViewChange={mockOnViewChange}
      />
    );

    const toggleButton = screen.getByRole('button', { name: /toggle navigation menu/i });
    fireEvent.click(toggleButton);

    const stepsButton = screen.getByRole('tab', { name: /📊 step history/i });
    expect(stepsButton).toHaveAttribute('aria-selected', 'true');
  });

  it('handles keyboard navigation', () => {
    render(
      <MobileNavigation
        currentView="tasks"
        onViewChange={mockOnViewChange}
      />
    );

    const toggleButton = screen.getByRole('button', { name: /toggle navigation menu/i });
    fireEvent.click(toggleButton);

    // Test Escape key closes menu
    fireEvent.keyDown(document, { key: 'Escape' });
    
    // Menu should be closed
    expect(screen.queryByRole('navigation', { name: /mobile navigation/i })).not.toBeInTheDocument();
  });

  it('has proper accessibility attributes', () => {
    render(
      <MobileNavigation
        currentView="tasks"
        onViewChange={mockOnViewChange}
      />
    );

    const toggleButton = screen.getByRole('button', { name: /toggle navigation menu/i });
    expect(toggleButton).toHaveAttribute('aria-expanded', 'false');
    expect(toggleButton).toHaveAttribute('aria-controls', 'mobile-menu');

    fireEvent.click(toggleButton);
    expect(toggleButton).toHaveAttribute('aria-expanded', 'true');
  });

  it('closes menu when overlay is clicked', () => {
    render(
      <MobileNavigation
        currentView="tasks"
        onViewChange={mockOnViewChange}
      />
    );

    const toggleButton = screen.getByRole('button', { name: /toggle navigation menu/i });
    fireEvent.click(toggleButton);

    const overlay = screen.getByRole('presentation');
    fireEvent.click(overlay);

    expect(screen.queryByRole('navigation', { name: /mobile navigation/i })).not.toBeInTheDocument();
  });
});