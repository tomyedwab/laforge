import { h, createContext } from 'preact';
import { useState, useContext, useEffect } from 'preact/hooks';

export interface Project {
  id: string;
  name: string;
  description: string;
  created_at: string;
  updated_at: string;
}

interface ProjectContextType {
  selectedProject: Project | null;
  setSelectedProject: (project: Project | null) => void;
  projects: Project[];
  setProjects: (projects: Project[]) => void;
  isLoading: boolean;
  setIsLoading: (loading: boolean) => void;
}

const ProjectContext = createContext<ProjectContextType | undefined>(undefined);

const PROJECT_STORAGE_KEY = 'laforge_selected_project';

export function ProjectProvider({ children }: { children: preact.ComponentChildren }) {
  const [selectedProject, setSelectedProjectState] = useState<Project | null>(null);
  const [projects, setProjects] = useState<Project[]>([]);
  const [isLoading, setIsLoading] = useState(false);

  // Load selected project from localStorage on mount
  useEffect(() => {
    const storedProject = localStorage.getItem(PROJECT_STORAGE_KEY);
    if (storedProject) {
      try {
        const project = JSON.parse(storedProject);
        setSelectedProjectState(project);
      } catch (error) {
        console.error('Failed to parse stored project:', error);
        localStorage.removeItem(PROJECT_STORAGE_KEY);
      }
    }
  }, []);

  // Enhanced setSelectedProject that also saves to localStorage
  const setSelectedProject = (project: Project | null) => {
    setSelectedProjectState(project);
    if (project) {
      localStorage.setItem(PROJECT_STORAGE_KEY, JSON.stringify(project));
    } else {
      localStorage.removeItem(PROJECT_STORAGE_KEY);
    }
  };

  const value: ProjectContextType = {
    selectedProject,
    setSelectedProject,
    projects,
    setProjects,
    isLoading,
    setIsLoading,
  };

  return (
    <ProjectContext.Provider value={value}>
      {children}
    </ProjectContext.Provider>
  );
}

export function useProject() {
  const context = useContext(ProjectContext);
  if (context === undefined) {
    throw new Error('useProject must be used within a ProjectProvider');
  }
  return context;
}