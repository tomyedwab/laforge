import { render, screen, fireEvent } from '@testing-library/preact';
import { TaskCard } from './TaskCard';
import type { Task } from '../types';

const mockTask: Task = {
  id: 1,
  title: 'Test Task',
  description: 'Test description',
  type: 'FEAT',
  status: 'todo',
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
  children: [],
};

const mockTaskWithChildren: Task = {
  ...mockTask,
  children: [
    { ...mockTask, id: 2, title: 'Child Task 1' },
    { ...mockTask, id: 3, title: 'Child Task 2' },
  ],
};

const mockTaskOverdue: Task = {
  ...mockTask,
  status: 'in-progress',
  completed_at: '2023-12-31T23:59:59Z', // Past date
};

describe('TaskCard', () => {
  const mockOnClick = jest.fn();
  const mockOnStatusChange = jest.fn();

  beforeEach(() => {
    mockOnClick.mockClear();
    mockOnStatusChange.mockClear();
  });

  it('renders task information correctly', () => {
    render(
      <TaskCard
        task={mockTask}
        onClick={mockOnClick}
        onStatusChange={mockOnStatusChange}
      />
    );

    expect(screen.getByText('Test Task')).toBeInTheDocument();
    expect(screen.getByText('Test description')).toBeInTheDocument();
    expect(screen.getByText('FEAT')).toBeInTheDocument();
    expect(screen.getByText('○ todo')).toBeInTheDocument();
  });

  it('shows type icon based on task type', () => {
    render(<TaskCard task={mockTask} />);
    
    const typeIcon = screen.getByTitle('FEAT');
    expect(typeIcon).toHaveTextContent('✨');
  });

  it('shows status icon based on task status', () => {
    render(<TaskCard task={mockTask} />);
    
    const statusBadge = screen.getByText('○ todo');
    expect(statusBadge).toBeInTheDocument();
  });

  it('shows review required badge when review_required is true', () => {
    const taskWithReview = { ...mockTask, review_required: true };
    render(<TaskCard task={taskWithReview} />);
    
    const reviewBadge = screen.getByTitle('Review required');
    expect(reviewBadge).toHaveTextContent('👁️');
  });

  it('shows overdue badge when task is overdue', () => {
    render(<TaskCard task={mockTaskOverdue} />);
    
    const overdueBadge = screen.getByTitle('Overdue');
    expect(overdueBadge).toHaveTextContent('⚠️');
  });

  it('shows child count when task has children', () => {
    render(<TaskCard task={mockTaskWithChildren} />);
    
    const childCount = screen.getByTitle('2 subtasks');
    expect(childCount).toHaveTextContent('📂 2');
  });

  it('formats dates correctly', () => {
    render(<TaskCard task={mockTask} />);
    
    const dateElement = screen.getByText(/Created/);
    expect(dateElement).toHaveTextContent('Created Today');
  });

  it('calls onClick when card is clicked', () => {
    render(
      <TaskCard
        task={mockTask}
        onClick={mockOnClick}
        onStatusChange={mockOnStatusChange}
      />
    );

    const card = screen.getByRole('article');
    fireEvent.click(card);

    expect(mockOnClick).toHaveBeenCalledWith(mockTask);
  });

  it('shows status select when onStatusChange is provided and showActions is true', () => {
    render(
      <TaskCard
        task={mockTask}
        onClick={mockOnClick}
        onStatusChange={mockOnStatusChange}
        showActions={true}
      />
    );

    const select = screen.getByRole('combobox');
    expect(select).toBeInTheDocument();
    expect(select).toHaveValue('todo');
  });

  it('calls onStatusChange when status is changed', () => {
    render(
      <TaskCard
        task={mockTask}
        onClick={mockOnClick}
        onStatusChange={mockOnStatusChange}
        showActions={true}
      />
    );

    const select = screen.getByRole('combobox');
    fireEvent.change(select, { target: { value: 'in-progress' } });

    expect(mockOnStatusChange).toHaveBeenCalledWith(1, 'in-progress');
  });

  it('shows action buttons when showActions is true', () => {
    render(
      <TaskCard
        task={mockTask}
        onClick={mockOnClick}
        onStatusChange={mockOnStatusChange}
        showActions={true}
      />
    );

    expect(screen.getByRole('button', { name: /👁️ view/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /✏️ edit/i })).toBeInTheDocument();
  });

  it('applies selected class when isSelected is true', () => {
    const { container } = render(
      <TaskCard
        task={mockTask}
        isSelected={true}
      />
    );

    const card = container.querySelector('.task-card');
    expect(card).toHaveClass('selected');
  });

  it('applies child-task class and margin when depth is greater than 0', () => {
    const { container } = render(
      <TaskCard
        task={mockTask}
        depth={2}
      />
    );

    const card = container.querySelector('.task-card');
    expect(card).toHaveClass('child-task');
    expect(card).toHaveStyle({ marginLeft: '48px' });
  });

  it('has proper accessibility attributes', () => {
    render(
      <TaskCard
        task={mockTask}
        onClick={mockOnClick}
        isSelected={false}
      />
    );

    const card = screen.getByRole('article');
    expect(card).toHaveAttribute('tabIndex', '0');
  });

  it('prevents event propagation when status select is clicked', () => {
    render(
      <TaskCard
        task={mockTask}
        onClick={mockOnClick}
        onStatusChange={mockOnStatusChange}
        showActions={true}
      />
    );

    const select = screen.getByRole('combobox');
    const clickEvent = new MouseEvent('click', { bubbles: true });
    Object.defineProperty(clickEvent, 'stopPropagation', { value: jest.fn() });
    
    fireEvent(select, clickEvent);
    
    // The select should have stopPropagation called
    expect(clickEvent.stopPropagation).toHaveBeenCalled();
  });

  it('prevents event propagation when action buttons are clicked', () => {
    render(
      <TaskCard
        task={mockTask}
        onClick={mockOnClick}
        onStatusChange={mockOnStatusChange}
        showActions={true}
      />
    );

    const viewButton = screen.getByRole('button', { name: /👁️ view/i });
    const clickEvent = new MouseEvent('click', { bubbles: true });
    Object.defineProperty(clickEvent, 'stopPropagation', { value: jest.fn() });
    
    fireEvent(viewButton, clickEvent);
    
    // The button should have stopPropagation called
    expect(clickEvent.stopPropagation).toHaveBeenCalled();
  });
});