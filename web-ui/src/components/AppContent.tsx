import { h } from 'preact';
import { useState, useEffect } from 'preact/hooks';
import { useProject } from '../contexts/ProjectContext';
import { websocketService } from '../services/websocket';
import { Header } from './Header';
import { MobileNavigation } from './MobileNavigation';
import { TaskDashboard } from './TaskDashboard';
import { StepDashboard } from './StepDashboard';
import { ReviewsDashboard } from './ReviewsDashboard';
import { TaskDetail } from './TaskDetail';
import { LoadingSpinner } from './LoadingStates';

interface AppContentProps {
  currentView: 'tasks' | 'steps' | 'reviews';
  onViewChange: (view: 'tasks' | 'steps' | 'reviews') => void;
}

export function AppContent({ currentView, onViewChange }: AppContentProps) {
  const { selectedProject, isLoading } = useProject();
  const [selectedTask, setSelectedTask] = useState<any>(null);
  const [isMobile, setIsMobile] = useState(false);

  // Handle responsive behavior
  useEffect(() => {
    const checkMobile = () => {
      setIsMobile(window.innerWidth <= 768);
    };

    checkMobile();
    window.addEventListener('resize', checkMobile);

    return () => window.removeEventListener('resize', checkMobile);
  }, []);

  // Sync selected project with WebSocket service
  useEffect(() => {
    if (selectedProject) {
      websocketService.setProjectId(selectedProject.id);
    }
  }, [selectedProject?.id]);

  const handleTaskClick = (task: any) => {
    setSelectedTask(task);
  };

  const handleCloseTaskDetail = () => {
    setSelectedTask(null);
  };

  if (isLoading) {
    return (
      <div class="app">
        <Header 
          currentView={currentView}
          onViewChange={onViewChange}
        />
        <main class="main-content" role="main">
          <div class="loading-container">
            <LoadingSpinner size="large" message="Loading projects..." />
          </div>
        </main>
      </div>
    );
  }

  if (!selectedProject) {
    return (
      <div class="app">
        <Header 
          currentView={currentView}
          onViewChange={onViewChange}
        />
        <main class="main-content" role="main">
          <div class="error-container">
            <h2>No Project Selected</h2>
            <p>Please select a project from the dropdown in the header to continue.</p>
          </div>
        </main>
      </div>
    );
  }

  return (
    <div class="app">
      <Header 
        currentView={currentView}
        onViewChange={onViewChange}
      />
      {isMobile && (
        <MobileNavigation
          currentView={currentView}
          onViewChange={onViewChange}
        />
      )}
      <main class="main-content" role="main">
        {selectedTask ? (
          <TaskDetail
            task={selectedTask}
            onClose={handleCloseTaskDetail}
            onStatusChange={(taskId, status) => {
              // Handle status change if needed
              console.log(`Task ${taskId} status changed to ${status}`);
            }}
            onTaskUpdate={(updatedTask) => {
              // Handle task update if needed
              setSelectedTask(updatedTask);
            }}
          />
        ) : (
          <div id={`${currentView}-panel`} role="tabpanel">
            {currentView === 'tasks' && <TaskDashboard onTaskClick={handleTaskClick} />}
            {currentView === 'steps' && <StepDashboard />}
            {currentView === 'reviews' && <ReviewsDashboard onTaskClick={handleTaskClick} />}
          </div>
        )}
      </main>
    </div>
  );
}