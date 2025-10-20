import { render, screen, fireEvent, waitFor } from '@testing-library/preact';
import { Autocomplete } from './Autocomplete';

describe('Autocomplete Component', () => {
  const mockLoadOptions = jest.fn();
  const mockOnChange = jest.fn();

  beforeEach(() => {
    jest.clearAllMocks();
    mockLoadOptions.mockResolvedValue([
      { id: 1, label: 'Task 1 (FEAT)' },
      { id: 2, label: 'Task 2 (BUG)' },
      { id: 3, label: 'Task 3 (EPIC)' },
    ]);
  });

  it('renders input with placeholder', () => {
    render(
      <Autocomplete
        value={null}
        onChange={mockOnChange}
        placeholder="Search for tasks..."
        loadOptions={mockLoadOptions}
      />
    );

    const input = screen.getByPlaceholderText('Search for tasks...');
    expect(input).toBeInTheDocument();
  });

  it('shows dropdown when typing with minimum 2 characters', async () => {
    render(
      <Autocomplete
        value={null}
        onChange={mockOnChange}
        placeholder="Search for tasks..."
        loadOptions={mockLoadOptions}
      />
    );

    const input = screen.getByPlaceholderText('Search for tasks...');
    fireEvent.input(input, { target: { value: 'task' } });

    await waitFor(() => {
      expect(mockLoadOptions).toHaveBeenCalledWith('task');
    });

    await waitFor(() => {
      expect(screen.getByText('Task 1 (FEAT)')).toBeInTheDocument();
      expect(screen.getByText('Task 2 (BUG)')).toBeInTheDocument();
      expect(screen.getByText('Task 3 (EPIC)')).toBeInTheDocument();
    });
  });

  it('does not search with less than 2 characters', async () => {
    render(
      <Autocomplete
        value={null}
        onChange={mockOnChange}
        placeholder="Search for tasks..."
        loadOptions={mockLoadOptions}
      />
    );

    const input = screen.getByPlaceholderText('Search for tasks...');
    fireEvent.input(input, { target: { value: 't' } });

    await waitFor(() => {
      expect(mockLoadOptions).not.toHaveBeenCalled();
    });
  });

  it('selects an option when clicked', async () => {
    render(
      <Autocomplete
        value={null}
        onChange={mockOnChange}
        placeholder="Search for tasks..."
        loadOptions={mockLoadOptions}
      />
    );

    const input = screen.getByPlaceholderText('Search for tasks...');
    fireEvent.input(input, { target: { value: 'task' } });

    await waitFor(() => {
      expect(screen.getByText('Task 1 (FEAT)')).toBeInTheDocument();
    });

    const option = screen.getByText('Task 1 (FEAT)');
    fireEvent.click(option);

    expect(mockOnChange).toHaveBeenCalledWith(1);
    expect(input).toHaveValue('Task 1 (FEAT)');
  });

  it('clears selection when clear button is clicked', async () => {
    render(
      <Autocomplete
        value={1}
        onChange={mockOnChange}
        placeholder="Search for tasks..."
        loadOptions={mockLoadOptions}
      />
    );

    const input = screen.getByPlaceholderText('Search for tasks...');
    fireEvent.input(input, { target: { value: 'task' } });

    await waitFor(() => {
      expect(screen.getByText('Task 1 (FEAT)')).toBeInTheDocument();
    });

    const option = screen.getByText('Task 1 (FEAT)');
    fireEvent.click(option);

    const clearButton = screen.getByLabelText('Clear selection');
    fireEvent.click(clearButton);

    expect(mockOnChange).toHaveBeenCalledWith(null);
    expect(input).toHaveValue('');
  });

  it('handles keyboard navigation', async () => {
    render(
      <Autocomplete
        value={null}
        onChange={mockOnChange}
        placeholder="Search for tasks..."
        loadOptions={mockLoadOptions}
      />
    );

    const input = screen.getByPlaceholderText('Search for tasks...');
    fireEvent.input(input, { target: { value: 'task' } });

    await waitFor(() => {
      expect(screen.getByText('Task 1 (FEAT)')).toBeInTheDocument();
    });

    // Press arrow down to highlight first option
    fireEvent.keyDown(input, { key: 'ArrowDown' });
    
    // Press enter to select highlighted option
    fireEvent.keyDown(input, { key: 'Enter' });

    expect(mockOnChange).toHaveBeenCalledWith(1);
  });

  it('closes dropdown on escape key', async () => {
    render(
      <Autocomplete
        value={null}
        onChange={mockOnChange}
        placeholder="Search for tasks..."
        loadOptions={mockLoadOptions}
      />
    );

    const input = screen.getByPlaceholderText('Search for tasks...');
    fireEvent.input(input, { target: { value: 'task' } });

    await waitFor(() => {
      expect(screen.getByText('Task 1 (FEAT)')).toBeInTheDocument();
    });

    fireEvent.keyDown(input, { key: 'Escape' });

    await waitFor(() => {
      expect(screen.queryByText('Task 1 (FEAT)')).not.toBeInTheDocument();
    });
  });

  it('shows loading state during search', async () => {
    mockLoadOptions.mockImplementation(() => 
      new Promise(resolve => setTimeout(() => resolve([
        { id: 1, label: 'Task 1 (FEAT)' }
      ]), 100))
    );

    render(
      <Autocomplete
        value={null}
        onChange={mockOnChange}
        placeholder="Search for tasks..."
        loadOptions={mockLoadOptions}
      />
    );

    const input = screen.getByPlaceholderText('Search for tasks...');
    fireEvent.input(input, { target: { value: 'task' } });

    // Should show loading spinner immediately
    await waitFor(() => {
      expect(screen.getByLabelText('Clear selection')).toBeInTheDocument();
    });
  });

  it('shows error state when search fails', async () => {
    mockLoadOptions.mockRejectedValue(new Error('Search failed'));

    render(
      <Autocomplete
        value={null}
        onChange={mockOnChange}
        placeholder="Search for tasks..."
        loadOptions={mockLoadOptions}
      />
    );

    const input = screen.getByPlaceholderText('Search for tasks...');
    fireEvent.input(input, { target: { value: 'task' } });

    await waitFor(() => {
      expect(screen.getByText('Search failed')).toBeInTheDocument();
    });
  });

  it('shows no results message when no options found', async () => {
    mockLoadOptions.mockResolvedValue([]);

    render(
      <Autocomplete
        value={null}
        onChange={mockOnChange}
        placeholder="Search for tasks..."
        loadOptions={mockLoadOptions}
      />
    );

    const input = screen.getByPlaceholderText('Search for tasks...');
    fireEvent.input(input, { target: { value: 'nonexistent' } });

    await waitFor(() => {
      expect(screen.getByText('No results found for "nonexistent"')).toBeInTheDocument();
    });
  });

  it('handles disabled state', () => {
    render(
      <Autocomplete
        value={null}
        onChange={mockOnChange}
        placeholder="Search for tasks..."
        loadOptions={mockLoadOptions}
        disabled={true}
      />
    );

    const input = screen.getByPlaceholderText('Search for tasks...');
    expect(input).toBeDisabled();
  });

  it('displays initial value for edit mode', () => {
    render(
      <Autocomplete
        value={123}
        onChange={mockOnChange}
        placeholder="Search for tasks..."
        loadOptions={mockLoadOptions}
      />
    );

    const input = screen.getByPlaceholderText('Search for tasks...');
    expect(input).toHaveValue('Task #123');
  });
});