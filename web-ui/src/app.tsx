import { h } from 'preact';
import { useState } from 'preact/hooks';
import { AuthProvider } from './contexts/AuthContext';
import { ProtectedRoute } from './components/ProtectedRoute';
import { Header } from './components/Header';
import { TaskDashboard } from './components/TaskDashboard';
import { StepDashboard } from './components/StepDashboard';
import { ReviewsDashboard } from './components/ReviewsDashboard';
import { TaskDetail } from './components/TaskDetail';
import './app.css';

export function App() {
  const [currentView, setCurrentView] = useState<'tasks' | 'steps' | 'reviews'>('tasks');
  const [selectedTask, setSelectedTask] = useState<any>(null);

  const handleTaskClick = (task: any) => {
    setSelectedTask(task);
  };

  const handleCloseTaskDetail = () => {
    setSelectedTask(null);
  };

  return (
    <AuthProvider>
      <div class="app">
        <Header 
          currentView={currentView}
          onViewChange={setCurrentView}
        />
        <main class="main-content">
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
              <>
                {currentView === 'tasks' && <TaskDashboard onTaskClick={handleTaskClick} />}
                {currentView === 'steps' && <StepDashboard />}
                {currentView === 'reviews' && <ReviewsDashboard onTaskClick={handleTaskClick} />}
              </>
            )}
          </ProtectedRoute>
        </main>
      </div>
    </AuthProvider>
  );
}