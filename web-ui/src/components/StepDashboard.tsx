import { h } from 'preact';
import { useState, useEffect, useMemo } from 'preact/hooks';
import { apiService } from '../services/api';
import type { Step } from '../types';
import { StepCard } from './StepCard';
import { StepDetail } from './StepDetail';
import { StepTimeline } from './StepTimeline';
import { Pagination } from './Pagination';

interface StepFilterOptions {
  active?: boolean;
  dateFrom?: string;
  dateTo?: string;
  viewType?: 'list' | 'timeline';
}

export function StepDashboard() {
  const [steps, setSteps] = useState<Step[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedStep, setSelectedStep] = useState<Step | null>(null);
  const [filters, setFilters] = useState<StepFilterOptions>({
    viewType: 'list'
  });
  const [currentPage, setCurrentPage] = useState(1);
  const [itemsPerPage, setItemsPerPage] = useState(25);
  const [totalSteps, setTotalSteps] = useState(0);

  useEffect(() => {
    loadSteps();
  }, [currentPage, itemsPerPage, filters.active]);

  const loadSteps = async () => {
    try {
      setIsLoading(true);
      setError(null);
      
      const response = await apiService.getSteps({
        active: filters.active,
        page: currentPage,
        limit: itemsPerPage,
      });
      
      setSteps(response.steps);
      setTotalSteps(response.pagination.total);
    } catch (error) {
      console.error('Failed to load steps:', error);
      setError('Failed to load steps. Please try again.');
    } finally {
      setIsLoading(false);
    }
  };

  // Filter steps locally for more responsive UI
  const processedSteps = useMemo(() => {
    let filtered = steps;

    // Apply date range filter
    if (filters.dateFrom || filters.dateTo) {
      filtered = filtered.filter(step => {
        const stepDate = new Date(step.start_time);
        const fromDate = filters.dateFrom ? new Date(filters.dateFrom) : null;
        const toDate = filters.dateTo ? new Date(filters.dateTo) : null;
        
        if (fromDate && stepDate < fromDate) return false;
        if (toDate && stepDate > toDate) return false;
        return true;
      });
    }

    return filtered;
  }, [steps, filters.dateFrom, filters.dateTo]);

  const handleStepClick = (step: Step) => {
    setSelectedStep(step);
  };

  const handlePageChange = (page: number) => {
    setCurrentPage(page);
  };

  const handleItemsPerPageChange = (items: number) => {
    setItemsPerPage(items);
    setCurrentPage(1);
  };

  const handleViewTypeChange = (viewType: 'list' | 'timeline') => {
    setFilters(prev => ({ ...prev, viewType }));
  };

  const formatDuration = (durationMs: number) => {
    if (durationMs < 1000) return `${durationMs}ms`;
    if (durationMs < 60000) return `${(durationMs / 1000).toFixed(1)}s`;
    return `${(durationMs / 60000).toFixed(1)}m`;
  };

  const formatCost = (cost: number) => {
    return `$${cost.toFixed(4)}`;
  };

  if (isLoading) {
    return (
      <div class="loading-container">
        <div class="loading-spinner">Loading steps...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div class="error-container">
        <div class="error-message">{error}</div>
        <button onClick={loadSteps}>Retry</button>
      </div>
    );
  }

  return (
    <div class="step-dashboard">
      <div class="dashboard-header">
        <h2>Step History</h2>
        <div class="dashboard-actions">
          <div class="view-toggle">
            <button
              class={`view-button ${filters.viewType === 'list' ? 'active' : ''}`}
              onClick={() => handleViewTypeChange('list')}
            >
              List View
            </button>
            <button
              class={`view-button ${filters.viewType === 'timeline' ? 'active' : ''}`}
              onClick={() => handleViewTypeChange('timeline')}
            >
              Timeline View
            </button>
          </div>
        </div>
      </div>
      
      <div class="step-filters">
        <div class="filter-group">
          <label>
            <input
              type="checkbox"
              checked={filters.active === true}
              onChange={(e) => setFilters(prev => ({ 
                ...prev, 
                active: (e.target as HTMLInputElement).checked ? true : undefined 
              }))}
            />
            Active steps only
          </label>
        </div>
        
        <div class="filter-group">
          <label>
            From:
            <input
              type="date"
              value={filters.dateFrom || ''}
              onChange={(e) => setFilters(prev => ({ 
                ...prev, 
                dateFrom: (e.target as HTMLInputElement).value || undefined 
              }))}
            />
          </label>
          <label>
            To:
            <input
              type="date"
              value={filters.dateTo || ''}
              onChange={(e) => setFilters(prev => ({ 
                ...prev, 
                dateTo: (e.target as HTMLInputElement).value || undefined 
              }))}
            />
          </label>
        </div>
      </div>
      
      {processedSteps.length === 0 ? (
        <div class="empty-state">
          <p>No steps found.</p>
          <p>Try adjusting your filters or check back later.</p>
        </div>
      ) : (
        <>
          {filters.viewType === 'list' ? (
            <div class="step-list">
              {processedSteps.map(step => (
                <StepCard
                  key={step.id}
                  step={step}
                  onClick={handleStepClick}
                  formatDuration={formatDuration}
                  formatCost={formatCost}
                />
              ))}
            </div>
          ) : (
            <StepTimeline
              steps={processedSteps}
              onStepClick={handleStepClick}
              formatDuration={formatDuration}
              formatCost={formatCost}
            />
          )}
          
          <Pagination
            currentPage={currentPage}
            totalPages={Math.ceil(totalSteps / itemsPerPage)}
            totalItems={totalSteps}
            itemsPerPage={itemsPerPage}
            onPageChange={handlePageChange}
            onItemsPerPageChange={handleItemsPerPageChange}
          />
        </>
      )}
      
      {selectedStep && (
        <StepDetail
          step={selectedStep}
          onClose={() => setSelectedStep(null)}
          formatDuration={formatDuration}
          formatCost={formatCost}
        />
      )}
    </div>
  );
}