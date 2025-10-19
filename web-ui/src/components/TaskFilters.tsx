// Preact JSX doesn't require h import
import type { TaskStatus, TaskType } from '../types';

export interface TaskFilterOptions {
  status?: TaskStatus;
  type?: TaskType;
  search?: string;
  sortBy?: 'created_at' | 'updated_at' | 'title' | 'status' | 'type';
  sortOrder?: 'asc' | 'desc';
}

interface TaskFiltersProps {
  filters: TaskFilterOptions;
  onFiltersChange: (filters: TaskFilterOptions) => void;
}

export function TaskFilters({ filters, onFiltersChange }: TaskFiltersProps) {
  const handleStatusChange = (status: TaskStatus | '') => {
    onFiltersChange({
      ...filters,
      status: status === '' ? undefined : status,
    });
  };

  const handleTypeChange = (type: TaskType | '') => {
    onFiltersChange({
      ...filters,
      type: type === '' ? undefined : type,
    });
  };

  const handleSearchChange = (search: string) => {
    onFiltersChange({
      ...filters,
      search: search || undefined,
    });
  };

  const handleSortChange = (sortBy: TaskFilterOptions['sortBy']) => {
    onFiltersChange({
      ...filters,
      sortBy,
    });
  };

  const handleSortOrderChange = () => {
    onFiltersChange({
      ...filters,
      sortOrder: filters.sortOrder === 'asc' ? 'desc' : 'asc',
    });
  };

  return (
    <div class="task-filters">
      <div class="filter-row">
        <div class="filter-group">
          <label for="status-filter">Status:</label>
          <select
            id="status-filter"
            value={filters.status || ''}
            onChange={(e) => handleStatusChange((e.target as HTMLSelectElement).value as TaskStatus | '')}
          >
            <option value="">All Statuses</option>
            <option value="todo">Todo</option>
            <option value="in-progress">In Progress</option>
            <option value="in-review">In Review</option>
            <option value="completed">Completed</option>
          </select>
        </div>

        <div class="filter-group">
          <label for="type-filter">Type:</label>
          <select
            id="type-filter"
            value={filters.type || ''}
            onChange={(e) => handleTypeChange((e.target as HTMLSelectElement).value as TaskType | '')}
          >
            <option value="">All Types</option>
            <option value="EPIC">Epic</option>
            <option value="FEAT">Feature</option>
            <option value="BUG">Bug</option>
            <option value="PLAN">Plan</option>
            <option value="DOC">Documentation</option>
            <option value="ARCH">Architecture</option>
            <option value="DESIGN">Design</option>
            <option value="TEST">Test</option>
          </select>
        </div>

        <div class="filter-group search-group">
          <label for="search-filter">Search:</label>
          <input
            id="search-filter"
            type="text"
            placeholder="Search tasks..."
            value={filters.search || ''}
            onInput={(e) => handleSearchChange((e.target as HTMLInputElement).value)}
          />
        </div>
      </div>

      <div class="filter-row">
        <div class="filter-group">
          <label for="sort-filter">Sort by:</label>
          <select
            id="sort-filter"
            value={filters.sortBy || 'created_at'}
            onChange={(e) => handleSortChange((e.target as HTMLSelectElement).value as TaskFilterOptions['sortBy'])}
          >
            <option value="created_at">Created Date</option>
            <option value="updated_at">Updated Date</option>
            <option value="title">Title</option>
            <option value="status">Status</option>
            <option value="type">Type</option>
          </select>
        </div>

        <button
          class="sort-order-button"
          onClick={handleSortOrderChange}
          title={`Sort ${filters.sortOrder === 'asc' ? 'Descending' : 'Ascending'}`}
        >
          {filters.sortOrder === 'asc' ? '↑' : '↓'}
        </button>

        <button
          class="clear-filters-button"
          onClick={() => onFiltersChange({})}
          disabled={!filters.status && !filters.type && !filters.search && !filters.sortBy}
        >
          Clear Filters
        </button>
      </div>
    </div>
  );
}