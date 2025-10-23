import { useState } from 'preact/hooks';
import { AuthProvider } from './contexts/AuthContext';
import { ProjectProvider } from './contexts/ProjectContext';
import { ProtectedRoute } from './components/ProtectedRoute';
import { AppContent } from './components/AppContent';
import './app.css';

export function App() {
  const [currentView, setCurrentView] = useState<'tasks' | 'steps' | 'reviews'>(
    'tasks'
  );

  const handleViewChange = (view: 'tasks' | 'steps' | 'reviews') => {
    setCurrentView(view);
  };

  return (
    <AuthProvider>
      <ProtectedRoute>
        <ProjectProvider>
          <AppContent
            currentView={currentView}
            onViewChange={handleViewChange}
          />
        </ProjectProvider>
      </ProtectedRoute>
    </AuthProvider>
  );
}
