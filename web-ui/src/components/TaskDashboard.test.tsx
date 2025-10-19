import { render, screen } from '@testing-library/preact';
import { TaskDashboard } from './TaskDashboard';

// Mock the API service
vi.mock('../services/api', () => ({
  apiService: {
    getTasks: vi.fn().mockResolvedValue({
      tasks: [
        {
          id: 1,
          title: 'Test Task',
          description: 'Test description',
          type: 'FEAT',
          status: 'todo',
          parent_id: null,
          upstream_dependency_id: null,
          review_required: false,
          created_at: '2025-10-18T19:00:00Z',
          updated_at: '2025-10-18T19:00:00Z',
          completed_at: null,
          children: [],
          logs: [],
          reviews: [],
        },
      ],
      pagination: { page: 1, limit: 25, total: 1, pages: 1 },
    }),
  },
}));

describe('TaskDashboard', () => {
  it('renders task dashboard', async () => {
    render(<TaskDashboard />);
    
    // Check if the dashboard header is rendered
    expect(screen.getByText('Task Dashboard')).toBeInTheDocument();
    
    // Check if the task is rendered (after loading)
    await screen.findByText('Test Task');
    expect(screen.getByText('Test Task')).toBeInTheDocument();
  });

  it('shows loading state initially', () => {
    render(<TaskDashboard />);
    expect(screen.getByText('Loading tasks...')).toBeInTheDocument();
  });
});