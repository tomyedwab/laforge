import { h } from 'preact';
import { useState, useEffect } from 'preact/hooks';
import { AuthProvider } from './contexts/AuthContext';
import { ProjectProvider, useProject } from './contexts/ProjectContext';
import { ProtectedRoute } from './components/ProtectedRoute';
import { Header } from './components/Header';
import { MobileNavigation } from './components/MobileNavigation';
import { TaskDashboard } from './components/TaskDashboard';
import { StepDashboard } from './components/StepDashboard';
import { ReviewsDashboard } from './components/ReviewsDashboard';
import { TaskDetail } from './components/TaskDetail';
import { AppContent } from './components/AppContent';
import { websocketService } from './services/websocket';
import './app.css';

export function App() {
  const [currentView, setCurrentView] = useState<'tasks' | 'steps' | 'reviews'>('tasks');

  const handleViewChange = (view: 'tasks' | 'steps' | 'reviews') => {
    setCurrentView(view);
  };

  return (
    <AuthProvider>
      <ProtectedRoute>
        <ProjectProvider>
          <AppContent currentView={currentView} onViewChange={handleViewChange} />
        </ProjectProvider>
      </ProtectedRoute>
    </AuthProvider>
  );
}