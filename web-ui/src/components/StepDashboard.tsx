import { h } from 'preact';
import { useState, useEffect } from 'preact/hooks';
import { apiService } from '../services/api';
import type { Step } from '../types';
import { StepDetail } from './StepDetail';
import { StepTimeline } from './StepTimeline';
import { Pagination } from './Pagination';

export function StepDashboard() {
  const [steps, setSteps] = useState<Step[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedStep, setSelectedStep] = useState<Step | null>(null);
  const [currentPage, setCurrentPage] = useState(1);
  const [itemsPerPage, setItemsPerPage] = useState(25);
  const [totalSteps, setTotalSteps] = useState(0);

  useEffect(() => {
    loadSteps();
  }, [currentPage, itemsPerPage]);

  const loadSteps = async () => {
    try {
      setIsLoading(true);
      setError(null);

      const response = await apiService.getSteps({
        active: false,
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
      </div>

      {steps.length === 0 ? (
        <div class="empty-state">
          <p>No steps found.</p>
          <p>Try adjusting your filters or check back later.</p>
        </div>
      ) : (
        <>
          <StepTimeline
            steps={steps}
            onStepClick={handleStepClick}
            formatDuration={formatDuration}
            formatCost={formatCost}
          />

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
