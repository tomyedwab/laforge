import { h } from 'preact';
import { AuthProvider } from './contexts/AuthContext';
import { ProtectedRoute } from './components/ProtectedRoute';
import { Header } from './components/Header';
import { TaskDashboard } from './components/TaskDashboard';
import './app.css';

export function App() {
  return (
    <AuthProvider>
      <div class="app">
        <Header />
        <main class="main-content">
          <ProtectedRoute>
            <TaskDashboard />
          </ProtectedRoute>
        </main>
      </div>
    </AuthProvider>
  );
}