import { useState, useEffect } from 'preact/hooks';

interface ArtifactViewerProps {
  artifactPath: string;
  onClose: () => void;
}

export function ArtifactViewer({ artifactPath, onClose }: ArtifactViewerProps) {
  const [content, setContent] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [artifactType, setArtifactType] = useState<'markdown' | 'image' | 'text' | 'unknown'>('unknown');

  useEffect(() => {
    loadArtifact();
  }, [artifactPath]);

  useEffect(() => {
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose();
      }
    };

    document.addEventListener('keydown', handleEscape);
    return () => document.removeEventListener('keydown', handleEscape);
  }, [onClose]);

  const loadArtifact = async () => {
    try {
      setIsLoading(true);
      setError(null);

      // Determine artifact type based on file extension
      const extension = artifactPath.split('.').pop()?.toLowerCase();
      const isImage = ['jpg', 'jpeg', 'png', 'gif', 'svg', 'webp'].includes(extension || '');
      const isMarkdown = ['md', 'markdown'].includes(extension || '');
      
      if (isImage) {
        setArtifactType('image');
        setContent(artifactPath); // For images, we just store the path
        setIsLoading(false);
        return;
      }

      if (isMarkdown) {
        setArtifactType('markdown');
      } else {
        setArtifactType('text');
      }

      // Try to fetch the artifact content
      // For now, we'll try to fetch from the repository
      // In a real implementation, this would be an API endpoint
      const response = await fetch(`/api/v1/projects/laforge-main/artifacts/${encodeURIComponent(artifactPath)}`);
      
      if (!response.ok) {
        throw new Error(`Failed to load artifact: ${response.statusText}`);
      }

      const textContent = await response.text();
      setContent(textContent);
    } catch (error) {
      console.error('Failed to load artifact:', error);
      setError('Failed to load artifact. The file may not exist or you may not have permission to view it.');
    } finally {
      setIsLoading(false);
    }
  };

  const renderMarkdown = (markdownContent: string) => {
    // Simple markdown to HTML conversion
    // In a real implementation, you'd use a proper markdown library like marked
    const html = markdownContent
      .replace(/^### (.*$)/gim, '<h3>$1</h3>')
      .replace(/^## (.*$)/gim, '<h2>$1</h2>')
      .replace(/^# (.*$)/gim, '<h1>$1</h1>')
      .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
      .replace(/\*(.+?)\*/g, '<em>$1</em>')
      .replace(/\`(.+?)\`/g, '<code>$1</code>')
      .replace(/```([\s\S]*?)```/g, '<pre><code>$1</code></pre>')
      .replace(/\n/g, '<br />');

    return <div class="markdown-content" dangerouslySetInnerHTML={{ __html: html }} />;
  };

  const renderContent = () => {
    if (isLoading) {
      return (
        <div class="artifact-loading">
          <div class="loading-spinner"></div>
          <p>Loading artifact...</p>
        </div>
      );
    }

    if (error) {
      return (
        <div class="artifact-error">
          <div class="error-icon">⚠️</div>
          <p>{error}</p>
          <button class="retry-button" onClick={loadArtifact}>
            Retry
          </button>
        </div>
      );
    }

    if (!content) {
      return (
        <div class="artifact-empty">
          <p>No content available</p>
        </div>
      );
    }

    switch (artifactType) {
      case 'image':
        return (
          <div class="artifact-image">
            <img 
              src={content} 
              alt={artifactPath}
              onError={(e) => {
                setError('Failed to load image');
                setArtifactType('unknown');
              }}
            />
          </div>
        );
      
      case 'markdown':
        return (
          <div class="artifact-markdown">
            {renderMarkdown(content)}
          </div>
        );
      
      case 'text':
        return (
          <div class="artifact-text">
            <pre>{content}</pre>
          </div>
        );
      
      default:
        return (
          <div class="artifact-unknown">
            <p>Unable to display this type of artifact</p>
            <p class="artifact-path">{artifactPath}</p>
          </div>
        );
    }
  };

  const getFileName = () => {
    return artifactPath.split('/').pop() || artifactPath;
  };

  return (
    <div class="artifact-viewer-overlay" onClick={onClose} role="dialog" aria-modal="true" aria-labelledby="artifact-viewer-title">
      <div class="artifact-viewer-modal" onClick={(e) => e.stopPropagation()}>
        <div class="artifact-viewer-header">
          <div class="artifact-info">
            <h3 id="artifact-viewer-title">{getFileName()}</h3>
            <span class="artifact-path">{artifactPath}</span>
          </div>
          <div class="artifact-actions">
            <button 
              class="refresh-button"
              onClick={loadArtifact}
              disabled={isLoading}
              title="Refresh content"
              aria-label="Refresh artifact content"
            >
              🔄
            </button>
            <button class="close-button" onClick={onClose} aria-label="Close artifact viewer">×</button>
          </div>
        </div>

        <div class="artifact-viewer-content">
          {renderContent()}
        </div>
      </div>
    </div>
  );
}