import { h } from 'preact';
import { useState, useEffect } from 'preact/hooks';
import { useProject } from '../contexts/ProjectContext';
import { apiService } from '../services/api';
import { websocketService } from '../services/websocket';
import './ProjectSelector.css';

export function ProjectSelector() {
  const { selectedProject, setSelectedProject, projects, setProjects, isLoading, setIsLoading } = useProject();
  const [isOpen, setIsOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchProjects();
  }, []);

  const fetchProjects = async () => {
    setIsLoading(true);
    setError(null);

    try {
      const response = await apiService.getProjects();
      setProjects(response.projects);
    } catch (err) {
      console.error('Failed to fetch projects:', err);
      setError('Failed to load projects');
    } finally {
      setIsLoading(false);
    }
  };

  const handleProjectSelect = (project: any) => {
    setSelectedProject(project);
    apiService.setProjectId(project.id);
    websocketService.setProjectId(project.id);
    setIsOpen(false);
  };

  const handleToggleDropdown = () => {
    setIsOpen(!isOpen);
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Escape') {
      setIsOpen(false);
    } else if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      handleToggleDropdown();
    }
  };

  if (isLoading && projects.length === 0) {
    return (
      <div class="project-selector">
        <div class="project-selector-loading">
          <span class="loading-spinner" aria-hidden="true"></span>
          <span>Loading projects...</span>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div class="project-selector">
        <div class="project-selector-error" role="alert">
          <span class="error-icon" aria-hidden="true">⚠️</span>
          <span>{error}</span>
          <button 
            onClick={fetchProjects}
            class="retry-button"
            type="button"
            aria-label="Retry loading projects"
          >
            Retry
          </button>
        </div>
      </div>
    );
  }

  if (projects.length === 0) {
    return (
      <div class="project-selector">
        <div class="project-selector-empty">
          <span>No projects available</span>
        </div>
      </div>
    );
  }

  return (
    <div class="project-selector">
      <div class="dropdown-container">
        <button
          class="project-selector-button"
          onClick={handleToggleDropdown}
          onKeyDown={handleKeyDown}
          type="button"
          aria-expanded={isOpen}
          aria-haspopup="listbox"
          aria-label="Select project"
          disabled={isLoading}
        >
          <span class="project-name">
            {selectedProject?.name || 'No project selected'}
          </span>
          <span class={`dropdown-arrow ${isOpen ? 'open' : ''}`} aria-hidden="true">
            ▼
          </span>
        </button>

        {isOpen && (
          <ul class="project-dropdown" role="listbox" aria-label="Projects">
            {projects.map((project) => (
              <li
                key={project.id}
                class={`project-option ${selectedProject?.id === project.id ? 'selected' : ''}`}
                role="option"
                aria-selected={selectedProject?.id === project.id}
              >
                <button
                  onClick={() => handleProjectSelect(project)}
                  class="project-option-button"
                  type="button"
                >
                  <div class="project-option-content">
                    <span class="project-option-name">{project.name}</span>
                    {project.description && (
                      <span class="project-option-description">{project.description}</span>
                    )}
                  </div>
                  {selectedProject?.id === project.id && (
                    <span class="selected-indicator" aria-hidden="true">✓</span>
                  )}
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>

      {/* Click outside to close */}
      {isOpen && (
        <div
          class="dropdown-backdrop"
          onClick={() => setIsOpen(false)}
          aria-hidden="true"
        />
      )}
    </div>
  );
}