import { h } from 'preact';
import { render, screen, fireEvent, waitFor } from '@testing-library/preact';
import { TaskForm } from './TaskForm';
import { apiService } from '../services/api';

// Mock the API service
jest.mock('../services/api');

const mockApiService = apiService as jest.Mocked<typeof apiService>;

describe('TaskForm', () => {
  const mockOnSave = jest.fn();
  const mockOnCancel = jest.fn();

  beforeEach(() => {
    jest.clearAllMocks();
    // Mock getTasks for loading available parents/dependencies
    mockApiService.getTasks.mockResolvedValue({
      tasks: [
        {
          id: 1,
          title: 'Parent Task 1',
          type: 'EPIC',
          status: 'in-progress',
          description: '',
          parent_id: null,
          upstream_dependency_id: null,
          review_required: false,
          created_at: '2025-10-18T19:00:00Z',
          updated_at: '2025-10-18T19:00:00Z',
          completed_at: null,
        },
        {
          id: 2,
          title: 'Parent Task 2',
          type: 'FEAT',
          status: 'todo',
          description: '',
          parent_id: null,
          upstream_dependency_id: null,
          review_required: false,
          created_at: '2025-10-18T19:00:00Z',
          updated_at: '2025-10-18T19:00:00Z',
          completed_at: null,
        },
      ],
      pagination: { page: 1, limit: 100, total: 2, pages: 1 },
    });
  });

  describe('Create New Task', () => {
    it('renders the create task form', async () => {
      render(
        <TaskForm onSave={mockOnSave} onCancel={mockOnCancel} />
      );

      expect(screen.getByText('Create New Task')).toBeInTheDocument();
      expect(screen.getByLabelText('Title *')).toBeInTheDocument();
      expect(screen.getByLabelText('Description')).toBeInTheDocument();
      expect(screen.getByLabelText('Type *')).toBeInTheDocument();
      expect(screen.getByLabelText('Status *')).toBeInTheDocument();
      expect(screen.getByLabelText('Parent Task')).toBeInTheDocument();
      expect(screen.getByLabelText('Depends On')).toBeInTheDocument();
      expect(screen.getByText('Review Required')).toBeInTheDocument();
    });

    it('validates required fields', async () => {
      render(
        <TaskForm onSave={mockOnSave} onCancel={mockOnCancel} />
      );

      const submitButton = screen.getByText('Create Task');
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(screen.getByText('Title is required')).toBeInTheDocument();
      });

      expect(mockOnSave).not.toHaveBeenCalled();
    });

    it('validates title length', async () => {
      render(
        <TaskForm onSave={mockOnSave} onCancel={mockOnCancel} />
      );

      const titleInput = screen.getByLabelText('Title *');
      fireEvent.change(titleInput, { target: { value: 'ab' } });

      const submitButton = screen.getByText('Create Task');
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(screen.getByText('Title must be at least 3 characters')).toBeInTheDocument();
      });

      expect(mockOnSave).not.toHaveBeenCalled();
    });

    it('creates a new task successfully', async () => {
      const newTask = {
        id: 3,
        title: '[FEAT] New Feature',
        description: 'New feature description',
        type: 'FEAT' as const,
        status: 'todo' as const,
        parent_id: null,
        upstream_dependency_id: null,
        review_required: false,
        created_at: '2025-10-18T20:00:00Z',
        updated_at: '2025-10-18T20:00:00Z',
        completed_at: null,
      };

      mockApiService.createTask.mockResolvedValueOnce({ task: newTask });

      render(
        <TaskForm onSave={mockOnSave} onCancel={mockOnCancel} />
      );

      const titleInput = screen.getByLabelText('Title *');
      const descriptionInput = screen.getByLabelText('Description');
      const typeSelect = screen.getByLabelText('Type *');
      const statusSelect = screen.getByLabelText('Status *');

      fireEvent.change(titleInput, { target: { value: '[FEAT] New Feature' } });
      fireEvent.change(descriptionInput, { target: { value: 'New feature description' } });
      fireEvent.change(typeSelect, { target: { value: 'FEAT' } });
      fireEvent.change(statusSelect, { target: { value: 'todo' } });

      const submitButton = screen.getByText('Create Task');
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(mockApiService.createTask).toHaveBeenCalledWith({
          title: '[FEAT] New Feature',
          description: 'New feature description',
          type: 'FEAT',
          status: 'todo',
          parent_id: null,
          upstream_dependency_id: null,
          review_required: false,
        });
      });

      expect(mockOnSave).toHaveBeenCalledWith(newTask);
    });

    it('handles API errors', async () => {
      mockApiService.createTask.mockRejectedValueOnce(new Error('API Error'));

      render(
        <TaskForm onSave={mockOnSave} onCancel={mockOnCancel} />
      );

      const titleInput = screen.getByLabelText('Title *');
      fireEvent.change(titleInput, { target: { value: '[FEAT] New Feature' } });

      const submitButton = screen.getByText('Create Task');
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(screen.getByText('API Error')).toBeInTheDocument();
      });

      expect(mockOnSave).not.toHaveBeenCalled();
    });

    it('calls onCancel when cancel button is clicked', () => {
      render(
        <TaskForm onSave={mockOnSave} onCancel={mockOnCancel} />
      );

      const cancelButton = screen.getByText('Cancel');
      fireEvent.click(cancelButton);

      expect(mockOnCancel).toHaveBeenCalled();
    });
  });

  describe('Edit Existing Task', () => {
    const existingTask = {
      id: 1,
      title: '[FEAT] Existing Feature',
      description: 'Existing feature description',
      type: 'FEAT' as const,
      status: 'in-progress' as const,
      parent_id: null,
      upstream_dependency_id: null,
      review_required: true,
      created_at: '2025-10-18T19:00:00Z',
      updated_at: '2025-10-18T19:30:00Z',
      completed_at: null,
    };

    it('renders the edit task form with existing data', async () => {
      render(
        <TaskForm task={existingTask} onSave={mockOnSave} onCancel={mockOnCancel} />
      );

      expect(screen.getByText('Edit Task')).toBeInTheDocument();
      expect(screen.getByLabelText('Title *')).toHaveValue('[FEAT] Existing Feature');
      expect(screen.getByLabelText('Description')).toHaveValue('Existing feature description');
      expect(screen.getByLabelText('Type *')).toHaveValue('FEAT');
      expect(screen.getByLabelText('Status *')).toHaveValue('in-progress');
      expect(screen.getByLabelText('Review Required')).toBeChecked();
    });

    it('updates an existing task successfully', async () => {
      const updatedTask = {
        ...existingTask,
        title: '[FEAT] Updated Feature',
        description: 'Updated description',
        status: 'completed' as const,
      };

      mockApiService.updateTask.mockResolvedValueOnce({ task: updatedTask });

      render(
        <TaskForm task={existingTask} onSave={mockOnSave} onCancel={mockOnCancel} />
      );

      const titleInput = screen.getByLabelText('Title *');
      const descriptionInput = screen.getByLabelText('Description');
      const statusSelect = screen.getByLabelText('Status *');

      fireEvent.change(titleInput, { target: { value: '[FEAT] Updated Feature' } });
      fireEvent.change(descriptionInput, { target: { value: 'Updated description' } });
      fireEvent.change(statusSelect, { target: { value: 'completed' } });

      const submitButton = screen.getByText('Update Task');
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(mockApiService.updateTask).toHaveBeenCalledWith(1, {
          title: '[FEAT] Updated Feature',
          description: 'Updated description',
          type: 'FEAT',
          status: 'completed',
          parent_id: null,
          upstream_dependency_id: null,
          review_required: true,
        });
      });

      expect(mockOnSave).toHaveBeenCalledWith(updatedTask);
    });

    it('filters out the current task from parent/dependency options', async () => {
      render(
        <TaskForm task={existingTask} onSave={mockOnSave} onCancel={mockOnCancel} />
      );

      await waitFor(() => {
        expect(mockApiService.getTasks).toHaveBeenCalled();
      });

      const parentSelect = screen.getByLabelText('Parent Task');
      const dependencySelect = screen.getByLabelText('Depends On');

      // Should not include the current task (id: 1) in the options
      expect(parentSelect).not.toHaveTextContent('Existing Feature');
      expect(dependencySelect).not.toHaveTextContent('Existing Feature');
    });
  });

  describe('Form Validation', () => {
    it('validates description length', async () => {
      render(
        <TaskForm onSave={mockOnSave} onCancel={mockOnCancel} />
      );

      const descriptionInput = screen.getByLabelText('Description');
      const longDescription = 'a'.repeat(2001);
      
      fireEvent.change(descriptionInput, { target: { value: longDescription } });

      const submitButton = screen.getByText('Create Task');
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(screen.getByText('Description must be less than 2000 characters')).toBeInTheDocument();
      });
    });

    it('clears field errors when user starts typing', async () => {
      render(
        <TaskForm onSave={mockOnSave} onCancel={mockOnCancel} />
      );

      const submitButton = screen.getByText('Create Task');
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(screen.getByText('Title is required')).toBeInTheDocument();
      });

      const titleInput = screen.getByLabelText('Title *');
      fireEvent.change(titleInput, { target: { value: 'New Title' } });

      await waitFor(() => {
        expect(screen.queryByText('Title is required')).not.toBeInTheDocument();
      });
    });
  });

  describe('Loading States', () => {
    it('disables form while submitting', async () => {
      mockApiService.createTask.mockImplementationOnce(() => 
        new Promise(resolve => setTimeout(resolve, 100))
      );

      render(
        <TaskForm onSave={mockOnSave} onCancel={mockOnCancel} />
      );

      const titleInput = screen.getByLabelText('Title *');
      fireEvent.change(titleInput, { target: { value: '[FEAT] New Feature' } });

      const submitButton = screen.getByText('Create Task');
      fireEvent.click(submitButton);

      expect(submitButton).toBeDisabled();
      expect(submitButton).toHaveTextContent('Saving...');
      expect(screen.getByLabelText('Title *')).toBeDisabled();
      expect(screen.getByLabelText('Description')).toBeDisabled();
      expect(screen.getByLabelText('Type *')).toBeDisabled();
      expect(screen.getByLabelText('Status *')).toBeDisabled();
    });
  });
});