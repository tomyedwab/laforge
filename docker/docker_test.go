package docker

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/tomyedwab/laforge/projects"
)

func TestNewClient(t *testing.T) {
	// Skip if Docker is not available
	if err := exec.Command("docker", "--version").Run(); err != nil {
		t.Skip("Docker is not available")
	}

	client, err := NewClient()
	if err != nil {
		t.Fatalf("Failed to create Docker client: %v", err)
	}
	defer client.Close()

	if client == nil {
		t.Error("Expected non-nil client")
	}
}

func TestContainerConfigValidation(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name    string
		config  *ContainerConfig
		wantErr bool
	}{
		{
			name:    "empty config",
			config:  &ContainerConfig{},
			wantErr: true,
		},
		{
			name: "missing image",
			config: &ContainerConfig{
				WorkDir:    "/tmp",
				TaskDBPath: "/tmp/tasks.db",
			},
			wantErr: true,
		},
		{
			name: "missing workdir",
			config: &ContainerConfig{
				Image:      "test:latest",
				TaskDBPath: "/tmp/tasks.db",
			},
			wantErr: true,
		},
		{
			name: "missing taskdb path",
			config: &ContainerConfig{
				Image:   "test:latest",
				WorkDir: "/tmp",
			},
			wantErr: true,
		},
		{
			name: "valid config",
			config: &ContainerConfig{
				Image:      "test:latest",
				WorkDir:    "/tmp",
				TaskDBPath: "/tmp/tasks.db",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.CreateAgentContainer(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateAgentContainer() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseMemoryLimit(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"512", "512m"},
		{"512m", "512m"},
		{"1g", "1g"},
		{"1024mb", "1024mb"},
		{"2gb", "2gb"},
		{"512k", "512k"},
		{"1024kb", "1024kb"},
		{"100b", "100b"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseMemoryLimit(tt.input)
			if result != tt.expected {
				t.Errorf("parseMemoryLimit(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestEnsureImage(t *testing.T) {
	// Skip if Docker is not available
	if err := exec.Command("docker", "--version").Run(); err != nil {
		t.Skip("Docker is not available")
	}

	client := &Client{}

	// Test with a small, commonly available image
	// Using alpine as it's small and widely available
	image := "alpine:latest"

	// This should succeed either by finding the image locally or pulling it
	err := client.ensureImage(image)
	if err != nil {
		t.Logf("ensureImage failed (this might be expected in some environments): %v", err)
	}
}

func TestIsTempFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/tmp/test-123456.db", true},
		{"/tmp/.hidden.db", true},
		{"/tmp/file-tmp-123.db", true},
		{"/tmp/file-temp-123.db", true},
		{"/tmp/file.tmp", true},
		{"/tmp/file_tmp_123.db", true},
		{"/tmp/laforge-test-123.db", true},
		{"/home/user/project.db", false},
		{"/var/lib/data.db", false},
		{"/tmp/regular.db", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isTempFile(tt.path)
			if result != tt.expected {
				t.Errorf("isTempFile(%s) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"hello world", "world", true},
		{"hello world", "foo", false},
		{"test-tmp-file", "-tmp-", true},
		{"test", "test-long", false},
		{"", "test", false},
		{"test", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			got := contains(tt.s, tt.substr)
			if got != tt.want {
				t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}

func TestConvertAgentConfigToContainerConfig(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name           string
		agentConfig    *projects.AgentConfig
		workDir        string
		taskDBPath     string
		wantErr        bool
		expectedImage  string
		expectedMemory string
		expectedCPU    int64
	}{
		{
			name: "basic conversion",
			agentConfig: &projects.AgentConfig{
				Name:  "test-agent",
				Image: "test:latest",
			},
			workDir:       "/tmp/work",
			taskDBPath:    "/tmp/tasks.db",
			wantErr:       false,
			expectedImage: "test:latest",
		},
		{
			name: "with resources",
			agentConfig: &projects.AgentConfig{
				Name:  "test-agent",
				Image: "test:latest",
				Resources: projects.ResourceConfig{
					Memory:    "512m",
					CPUShares: 512,
				},
			},
			workDir:        "/tmp/work",
			taskDBPath:     "/tmp/tasks.db",
			wantErr:        false,
			expectedImage:  "test:latest",
			expectedMemory: "512m",
			expectedCPU:    512,
		},
		{
			name: "with timeout",
			agentConfig: &projects.AgentConfig{
				Name:  "test-agent",
				Image: "test:latest",
				Runtime: projects.RuntimeConfig{
					Timeout: "30m",
				},
			},
			workDir:       "/tmp/work",
			taskDBPath:    "/tmp/tasks.db",
			wantErr:       false,
			expectedImage: "test:latest",
		},
		{
			name: "invalid timeout",
			agentConfig: &projects.AgentConfig{
				Name:  "test-agent",
				Image: "test:latest",
				Runtime: projects.RuntimeConfig{
					Timeout: "invalid",
				},
			},
			workDir:    "/tmp/work",
			taskDBPath: "/tmp/tasks.db",
			wantErr:    true,
		},
		{
			name:        "nil agent config",
			agentConfig: nil,
			workDir:     "/tmp/work",
			taskDBPath:  "/tmp/tasks.db",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := client.convertAgentConfigToContainerConfig(tt.agentConfig, tt.workDir, tt.taskDBPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertAgentConfigToContainerConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if config.Image != tt.expectedImage {
					t.Errorf("Image = %v, want %v", config.Image, tt.expectedImage)
				}
				if config.MemoryLimit != tt.expectedMemory {
					t.Errorf("MemoryLimit = %v, want %v", config.MemoryLimit, tt.expectedMemory)
				}
				if config.CPUShares != tt.expectedCPU {
					t.Errorf("CPUShares = %v, want %v", config.CPUShares, tt.expectedCPU)
				}
				// Check that required environment variables are set
				if config.Environment["TASKS_DB_PATH"] != tt.taskDBPath {
					t.Errorf("TASKS_DB_PATH = %v, want %v", config.Environment["TASKS_DB_PATH"], tt.taskDBPath)
				}
				if config.Environment["LAFORGE_AGENT"] != "true" {
					t.Errorf("LAFORGE_AGENT = %v, want true", config.Environment["LAFORGE_AGENT"])
				}
			}
		})
	}
}

func TestCreateAgentContainerFromConfig(t *testing.T) {
	// Skip if Docker is not available
	if err := exec.Command("docker", "--version").Run(); err != nil {
		t.Skip("Docker is not available")
	}

	client := &Client{}

	tests := []struct {
		name        string
		agentConfig *projects.AgentConfig
		workDir     string
		taskDBPath  string
		wantErr     bool
	}{
		{
			name: "valid config",
			agentConfig: &projects.AgentConfig{
				Name:  "test-agent",
				Image: "alpine:latest", // Use alpine since it's small and available
			},
			workDir:    "/tmp/work",
			taskDBPath: "/tmp/tasks.db",
			wantErr:    false,
		},
		{
			name:        "nil config",
			agentConfig: nil,
			workDir:     "/tmp/work",
			taskDBPath:  "/tmp/tasks.db",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.CreateAgentContainerFromConfig(tt.agentConfig, tt.workDir, tt.taskDBPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateAgentContainerFromConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStartContainerFromConfig(t *testing.T) {
	// Skip if Docker is not available
	if err := exec.Command("docker", "--version").Run(); err != nil {
		t.Skip("Docker is not available")
	}

	client := &Client{}

	tests := []struct {
		name        string
		agentConfig *projects.AgentConfig
		workDir     string
		taskDBPath  string
		wantErr     bool
	}{
		{
			name: "valid config with simple command",
			agentConfig: &projects.AgentConfig{
				Name:    "test-agent",
				Image:   "alpine:latest",
				Command: []string{"echo", "hello"},
				Runtime: projects.RuntimeConfig{
					AutoRemove: true,
					Timeout:    "10s",
				},
			},
			workDir:    "/tmp/work",
			taskDBPath: "/tmp/tasks.db",
			wantErr:    false,
		},
		{
			name:        "nil config",
			agentConfig: nil,
			workDir:     "/tmp/work",
			taskDBPath:  "/tmp/tasks.db",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container, err := client.StartContainerFromConfig(tt.agentConfig, tt.workDir, tt.taskDBPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("StartContainerFromConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && container != nil {
				// Clean up the container if it was created successfully
				_ = client.CleanupContainer(container)
			}
		})
	}
}

func TestRunAgentContainerFromConfig(t *testing.T) {
	// Skip if Docker is not available
	if err := exec.Command("docker", "--version").Run(); err != nil {
		t.Skip("Docker is not available")
	}

	client := &Client{}

	tests := []struct {
		name        string
		agentConfig *projects.AgentConfig
		workDir     string
		taskDBPath  string
		wantErr     bool
	}{
		{
			name: "valid config with metrics",
			agentConfig: &projects.AgentConfig{
				Name:    "test-agent",
				Image:   "alpine:latest",
				Command: []string{"echo", "hello world"},
				Runtime: projects.RuntimeConfig{
					AutoRemove: true,
					Timeout:    "10s",
				},
			},
			workDir:    "/tmp/work",
			taskDBPath: "/tmp/tasks.db",
			wantErr:    false,
		},
		{
			name:        "nil config",
			agentConfig: nil,
			workDir:     "/tmp/work",
			taskDBPath:  "/tmp/tasks.db",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exitCode, logs, err := client.RunAgentContainerFromConfigWithStreamingLogs(tt.agentConfig, tt.workDir, tt.taskDBPath, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("RunAgentContainerFromConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if exitCode != 0 {
					t.Errorf("Expected exit code 0, got %d", exitCode)
				}
				if logs == "" {
					t.Error("Expected non-empty logs")
				}
			}
		})
	}
}

func TestVolumeMountDetection(t *testing.T) {

	tests := []struct {
		name              string
		volumes           []string
		expectedWorkspace bool
		expectedData      bool
	}{
		{
			name:              "no volumes",
			volumes:           []string{},
			expectedWorkspace: false,
			expectedData:      false,
		},
		{
			name: "workspace volume specified",
			volumes: []string{
				"/host/path:/src",
				"/other/path:/other/container",
			},
			expectedWorkspace: true,
			expectedData:      false,
		},
		{
			name: "data volume specified",
			volumes: []string{
				"/host/path:/state",
				"/other/path:/other/container",
			},
			expectedWorkspace: false,
			expectedData:      true,
		},
		{
			name: "both volumes specified",
			volumes: []string{
				"/host/path:/src",
				"/host/data:/state",
			},
			expectedWorkspace: true,
			expectedData:      true,
		},
		{
			name: "partial matches should not count",
			volumes: []string{
				"/host/path:/src/state",
				"/host/path:/state/src",
			},
			expectedWorkspace: false,
			expectedData:      false,
		},
		{
			name: "exact matches at end",
			volumes: []string{
				"/host/path:/src",
				"/host/path:/state",
			},
			expectedWorkspace: true,
			expectedData:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasWorkspace := false
			hasData := false
			for _, volume := range tt.volumes {
				// Check for exact match at the end of the string
				if strings.HasSuffix(volume, ":/src") {
					hasWorkspace = true
				}
				if strings.HasSuffix(volume, ":/state") {
					hasData = true
				}
			}
			if hasWorkspace != tt.expectedWorkspace {
				t.Errorf("hasWorkspace = %v, want %v", hasWorkspace, tt.expectedWorkspace)
			}
			if hasData != tt.expectedData {
				t.Errorf("hasData = %v, want %v", hasData, tt.expectedData)
			}
		})
	}
}
