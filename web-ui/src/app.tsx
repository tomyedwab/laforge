import { h } from 'preact';
import { useState, useEffect } from 'preact/hooks';
import { AuthProvider } from './contexts/AuthContext';
import { ProtectedRoute } from './components/ProtectedRoute';
import { Header } from './components/Header';
import { MobileNavigation } from './components/MobileNavigation';
import { TaskDashboard } from './components/TaskDashboard';
import { StepDashboard } from './components/StepDashboard';
import { ReviewsDashboard } from './components/ReviewsDashboard';
import { TaskDetail } from './components/TaskDetail';
import './app.css';

export function App() {
  const [currentView, setCurrentView] = useState<'tasks' | 'steps' | 'reviews'>('tasks');
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

  const handleTaskClick = (task: any) => {
    setSelectedTask(task);
  };

  const handleCloseTaskDetail = () => {
    setSelectedTask(null);
  };

  const handleViewChange = (view: 'tasks' | 'steps' | 'reviews') => {
    setCurrentView(view);
    setSelectedTask(null); // Clear selected task when changing views
  };

  return (
    <AuthProvider>
      <div class="app">
        <Header 
          currentView={currentView}
          onViewChange={handleViewChange}
        />
        {isMobile && (
          <MobileNavigation
            currentView={currentView}
            onViewChange={handleViewChange}
          />
        )}
        <main class="main-content" role="main">
          <ProtectedRoute>
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
          </ProtectedRoute>
        </main>
      </div>
    </AuthProvider>
  );
}