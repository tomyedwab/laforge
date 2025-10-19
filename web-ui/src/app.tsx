import { h } from 'preact';
import { useState } from 'preact/hooks';
import { AuthProvider } from './contexts/AuthContext';
import { ProtectedRoute } from './components/ProtectedRoute';
import { Header } from './components/Header';
import { TaskDashboard } from './components/TaskDashboard';
import { StepDashboard } from './components/StepDashboard';
import './app.css';

export function App() {
  const [currentView, setCurrentView] = useState<'tasks' | 'steps'>('tasks');

  return (
    <AuthProvider>
      <div class="app">
        <Header 
          currentView={currentView}
          onViewChange={setCurrentView}
        />
        <main class="main-content">
          <ProtectedRoute>
            {currentView === 'tasks' ? (
              <TaskDashboard />
            ) : (
              <StepDashboard />
            )}
          </ProtectedRoute>
        </main>
      </div>
    </AuthProvider>
  );
}