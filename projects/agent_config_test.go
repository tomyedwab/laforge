package projects

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestAgentConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  AgentConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: AgentConfig{
				Name:  "test-agent",
				Image: "test:latest",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			config: AgentConfig{
				Image: "test:latest",
			},
			wantErr: true,
			errMsg:  "agent name is required",
		},
		{
			name: "missing image",
			config: AgentConfig{
				Name: "test-agent",
			},
			wantErr: true,
			errMsg:  "agent image is required",
		},
		{
			name: "invalid memory format",
			config: AgentConfig{
				Name:  "test-agent",
				Image: "test:latest",
				Resources: ResourceConfig{
					Memory: "invalid",
				},
			},
			wantErr: true,
			errMsg:  "invalid memory format",
		},
		{
			name: "invalid timeout format",
			config: AgentConfig{
				Name:  "test-agent",
				Image: "test:latest",
				Runtime: RuntimeConfig{
					Timeout: "invalid",
				},
			},
			wantErr: true,
			errMsg:  "invalid timeout format",
		},
		{
			name: "invalid network mode",
			config: AgentConfig{
				Name:  "test-agent",
				Image: "test:latest",
				Runtime: RuntimeConfig{
					NetworkMode: "invalid",
				},
			},
			wantErr: true,
			errMsg:  "invalid network mode",
		},
		{
			name: "invalid volume mount",
			config: AgentConfig{
				Name:    "test-agent",
				Image:   "test:latest",
				Volumes: []string{"invalid-format"},
			},
			wantErr: true,
			errMsg:  "invalid volume mount",
		},
		{
			name: "valid complex config",
			config: AgentConfig{
				Name:        "complex-agent",
				Image:       "laforge:latest",
				Description: "A complex agent configuration",
				Environment: map[string]string{
					"KEY1": "value1",
					"KEY2": "value2",
				},
				Volumes: []string{
					"/host/path:/container/path:ro",
					"/another/host:/another/container",
				},
				Resources: ResourceConfig{
					Memory:    "1g",
					CPUShares: 1024,
					CPULimit:  "1.0",
				},
				Runtime: RuntimeConfig{
					AutoRemove:  true,
					Timeout:     "1h",
					NetworkMode: "bridge",
				},
				Command:    []string{"/bin/bash", "-c", "echo hello"},
				WorkingDir: "/workspace",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want error containing %v", err, tt.errMsg)
				}
			}
		})
	}
}

func TestResourceConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  ResourceConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty config",
			config:  ResourceConfig{},
			wantErr: false,
		},
		{
			name: "valid memory formats",
			config: ResourceConfig{
				Memory: "512m",
			},
			wantErr: false,
		},
		{
			name: "invalid memory format",
			config: ResourceConfig{
				Memory: "512x",
			},
			wantErr: true,
			errMsg:  "invalid memory format",
		},
		{
			name: "valid CPU limit",
			config: ResourceConfig{
				CPULimit: "1.0",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want error containing %v", err, tt.errMsg)
				}
			}
		})
	}
}

func TestRuntimeConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  RuntimeConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty config",
			config:  RuntimeConfig{},
			wantErr: false,
		},
		{
			name: "valid timeout",
			config: RuntimeConfig{
				Timeout: "30m",
			},
			wantErr: false,
		},
		{
			name: "invalid timeout",
			config: RuntimeConfig{
				Timeout: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid timeout format",
		},
		{
			name: "valid network modes",
			config: RuntimeConfig{
				NetworkMode: "bridge",
			},
			wantErr: false,
		},
		{
			name: "invalid network mode",
			config: RuntimeConfig{
				NetworkMode: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid network mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want error containing %v", err, tt.errMsg)
				}
			}
		})
	}
}

func TestAgentsConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  AgentsConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: AgentsConfig{
				Version: "1.0",
				Agents: map[string]AgentConfig{
					"test": {
						Name:  "test",
						Image: "test:latest",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing version",
			config: AgentsConfig{
				Agents: map[string]AgentConfig{
					"test": {
						Name:  "test",
						Image: "test:latest",
					},
				},
			},
			wantErr: true,
			errMsg:  "configuration version is required",
		},
		{
			name: "no agents",
			config: AgentsConfig{
				Version: "1.0",
				Agents:  map[string]AgentConfig{},
			},
			wantErr: true,
			errMsg:  "at least one agent configuration is required",
		},
		{
			name: "invalid default agent",
			config: AgentsConfig{
				Version: "1.0",
				Default: "nonexistent",
				Agents: map[string]AgentConfig{
					"test": {
						Name:  "test",
						Image: "test:latest",
					},
				},
			},
			wantErr: true,
			errMsg:  "default agent 'nonexistent' not found",
		},
		{
			name: "invalid agent configuration",
			config: AgentsConfig{
				Version: "1.0",
				Agents: map[string]AgentConfig{
					"test": {
						Name:  "test",
						Image: "", // Invalid: missing image
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid agent configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want error containing %v", err, tt.errMsg)
				}
			}
		})
	}
}

func TestAgentsConfig_GetAgent(t *testing.T) {
	config := &AgentsConfig{
		Version: "1.0",
		Agents: map[string]AgentConfig{
			"agent1": {
				Name:  "agent1",
				Image: "image1:latest",
			},
			"agent2": {
				Name:  "agent2",
				Image: "image2:latest",
			},
		},
	}

	tests := []struct {
		name      string
		agentName string
		wantFound bool
		wantAgent AgentConfig
	}{
		{
			name:      "existing agent",
			agentName: "agent1",
			wantFound: true,
			wantAgent: AgentConfig{Name: "agent1", Image: "image1:latest"},
		},
		{
			name:      "non-existing agent",
			agentName: "agent3",
			wantFound: false,
			wantAgent: AgentConfig{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, found := config.GetAgent(tt.agentName)
			if found != tt.wantFound {
				t.Errorf("GetAgent() found = %v, want %v", found, tt.wantFound)
			}
			if agent.Name != tt.wantAgent.Name || agent.Image != tt.wantAgent.Image {
				t.Errorf("GetAgent() agent = %v, want %v", agent, tt.wantAgent)
			}
		})
	}
}

func TestAgentsConfig_GetDefaultAgent(t *testing.T) {
	tests := []struct {
		name      string
		config    AgentsConfig
		wantFound bool
		wantAgent AgentConfig
	}{
		{
			name: "with default specified",
			config: AgentsConfig{
				Version: "1.0",
				Default: "agent1",
				Agents: map[string]AgentConfig{
					"agent1": {Name: "agent1", Image: "image1:latest"},
					"agent2": {Name: "agent2", Image: "image2:latest"},
				},
			},
			wantFound: true,
			wantAgent: AgentConfig{Name: "agent1", Image: "image1:latest"},
		},
		{
			name: "no default specified",
			config: AgentsConfig{
				Version: "1.0",
				Agents: map[string]AgentConfig{
					"agent1": {Name: "agent1", Image: "image1:latest"},
					"agent2": {Name: "agent2", Image: "image2:latest"},
				},
			},
			wantFound: true,
			wantAgent: AgentConfig{Name: "agent1", Image: "image1:latest"}, // Should return first agent
		},
		{
			name: "no agents",
			config: AgentsConfig{
				Version: "1.0",
				Agents:  map[string]AgentConfig{},
			},
			wantFound: false,
			wantAgent: AgentConfig{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, found := tt.config.GetDefaultAgent()
			if found != tt.wantFound {
				t.Errorf("GetDefaultAgent() found = %v, want %v", found, tt.wantFound)
			}
			if agent.Name != tt.wantAgent.Name || agent.Image != tt.wantAgent.Image {
				t.Errorf("GetDefaultAgent() agent = %v, want %v", agent, tt.wantAgent)
			}
		})
	}
}

func TestDefaultAgentsConfig(t *testing.T) {
	config := DefaultAgentsConfig()

	if config.Version != "1.0" {
		t.Errorf("DefaultAgentsConfig() Version = %v, want %v", config.Version, "1.0")
	}

	if config.Default != "default" {
		t.Errorf("DefaultAgentsConfig() Default = %v, want %v", config.Default, "default")
	}

	if len(config.Agents) != 1 {
		t.Errorf("DefaultAgentsConfig() len(Agents) = %v, want %v", len(config.Agents), 1)
	}

	defaultAgent, exists := config.Agents["default"]
	if !exists {
		t.Error("DefaultAgentsConfig() default agent not found")
	}

	if defaultAgent.Image != "laforge-agent:latest" {
		t.Errorf("DefaultAgentsConfig() default agent Image = %v, want %v", defaultAgent.Image, "laforge-agent:latest")
	}
}

func TestAgentsConfig_MarshalYAML(t *testing.T) {
	config := &AgentsConfig{
		Version: "1.0",
		Default: "test-agent",
		Agents: map[string]AgentConfig{
			"test-agent": {
				Name:        "test-agent",
				Image:       "test:latest",
				Description: "Test agent",
				Environment: map[string]string{"KEY": "value"},
				Volumes:     []string{"/host:/container"},
				Resources: ResourceConfig{
					Memory:    "512m",
					CPUShares: 512,
				},
				Runtime: RuntimeConfig{
					AutoRemove:  true,
					Timeout:     "30m",
					NetworkMode: "bridge",
				},
			},
		},
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		t.Fatalf("MarshalYAML() error = %v", err)
	}

	// Verify some key fields are present in the output
	output := string(data)
	expectedFields := []string{"version: \"1.0\"", "default: test-agent", "agents:", "test-agent:", "image: test:latest"}

	for _, field := range expectedFields {
		if !contains(output, field) {
			t.Errorf("MarshalYAML() output missing expected field: %v", field)
		}
	}
}

func TestAgentsConfig_UnmarshalYAML(t *testing.T) {
	yamlData := `
version: "1.0"
default: test-agent
agents:
  test-agent:
    name: test-agent
    image: test:latest
    description: Test agent
    environment:
      KEY: value
    volumes:
      - /host:/container
    resources:
      memory: 512m
      cpu_shares: 512
    runtime:
      auto_remove: true
      timeout: 30m
      network_mode: bridge
`

	var config AgentsConfig
	err := yaml.Unmarshal([]byte(yamlData), &config)
	if err != nil {
		t.Fatalf("UnmarshalYAML() error = %v", err)
	}

	if config.Version != "1.0" {
		t.Errorf("UnmarshalYAML() Version = %v, want %v", config.Version, "1.0")
	}

	if config.Default != "test-agent" {
		t.Errorf("UnmarshalYAML() Default = %v, want %v", config.Default, "test-agent")
	}

	agent, exists := config.Agents["test-agent"]
	if !exists {
		t.Fatal("UnmarshalYAML() test-agent not found")
	}

	if agent.Name != "test-agent" {
		t.Errorf("UnmarshalYAML() agent Name = %v, want %v", agent.Name, "test-agent")
	}

	if agent.Image != "test:latest" {
		t.Errorf("UnmarshalYAML() agent Image = %v, want %v", agent.Image, "test:latest")
	}

	if agent.Environment["KEY"] != "value" {
		t.Errorf("UnmarshalYAML() agent Environment[KEY] = %v, want %v", agent.Environment["KEY"], "value")
	}

	if len(agent.Volumes) != 1 || agent.Volumes[0] != "/host:/container" {
		t.Errorf("UnmarshalYAML() agent Volumes = %v, want %v", agent.Volumes, []string{"/host:/container"})
	}

	if agent.Resources.Memory != "512m" {
		t.Errorf("UnmarshalYAML() agent Resources.Memory = %v, want %v", agent.Resources.Memory, "512m")
	}

	if agent.Resources.CPUShares != 512 {
		t.Errorf("UnmarshalYAML() agent Resources.CPUShares = %v, want %v", agent.Resources.CPUShares, 512)
	}

	if !agent.Runtime.AutoRemove {
		t.Errorf("UnmarshalYAML() agent Runtime.AutoRemove = %v, want %v", agent.Runtime.AutoRemove, true)
	}

	if agent.Runtime.Timeout != "30m" {
		t.Errorf("UnmarshalYAML() agent Runtime.Timeout = %v, want %v", agent.Runtime.Timeout, "30m")
	}

	if agent.Runtime.NetworkMode != "bridge" {
		t.Errorf("UnmarshalYAML() agent Runtime.NetworkMode = %v, want %v", agent.Runtime.NetworkMode, "bridge")
	}
}

func TestValidateVolumeMount(t *testing.T) {
	tests := []struct {
		name    string
		volume  string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid simple mount",
			volume:  "/host:/container",
			wantErr: false,
		},
		{
			name:    "valid mount with mode",
			volume:  "/host:/container:ro",
			wantErr: false,
		},
		{
			name:    "empty volume",
			volume:  "",
			wantErr: true,
			errMsg:  "volume mount cannot be empty",
		},
		{
			name:    "single part",
			volume:  "/host",
			wantErr: true,
			errMsg:  "volume mount must be in format",
		},
		{
			name:    "empty host path",
			volume:  ":/container",
			wantErr: true,
			errMsg:  "host path cannot be empty",
		},
		{
			name:    "empty container path",
			volume:  "/host:",
			wantErr: true,
			errMsg:  "container path cannot be empty",
		},
		{
			name:    "invalid mode",
			volume:  "/host:/container:invalid",
			wantErr: true,
			errMsg:  "invalid volume mode",
		},
		{
			name:    "valid modes",
			volume:  "/host:/container:rw",
			wantErr: false,
		},
		{
			name:    "valid mode z",
			volume:  "/host:/container:z",
			wantErr: false,
		},
		{
			name:    "valid mode Z",
			volume:  "/host:/container:Z",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVolumeMount(tt.volume)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateVolumeMount() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("validateVolumeMount() error = %v, want error containing %v", err, tt.errMsg)
				}
			}
		})
	}
}

func TestValidateMemoryFormat(t *testing.T) {
	tests := []struct {
		name    string
		memory  string
		wantErr bool
	}{
		{
			name:    "empty memory",
			memory:  "",
			wantErr: false,
		},
		{
			name:    "valid bytes",
			memory:  "512b",
			wantErr: false,
		},
		{
			name:    "valid kilobytes",
			memory:  "512k",
			wantErr: false,
		},
		{
			name:    "valid kilobytes KB",
			memory:  "512kb",
			wantErr: false,
		},
		{
			name:    "valid megabytes",
			memory:  "512m",
			wantErr: false,
		},
		{
			name:    "valid megabytes MB",
			memory:  "512mb",
			wantErr: false,
		},
		{
			name:    "valid gigabytes",
			memory:  "1g",
			wantErr: false,
		},
		{
			name:    "valid gigabytes GB",
			memory:  "1gb",
			wantErr: false,
		},
		{
			name:    "invalid unit",
			memory:  "512x",
			wantErr: true,
		},
		{
			name:    "no unit",
			memory:  "512",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMemoryFormat(tt.memory)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateMemoryFormat() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Test file management functions
func TestSaveAndLoadAgentsConfig(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "agents.yml")

	// Test configuration
	config := &AgentsConfig{
		Version: "1.0",
		Default: "test-agent",
		Agents: map[string]AgentConfig{
			"test-agent": {
				Name:  "test-agent",
				Image: "test:latest",
			},
		},
	}

	// Test saving configuration
	data, err := yaml.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal test config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// Test loading configuration
	loadedData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var loadedConfig AgentsConfig
	if err := yaml.Unmarshal(loadedData, &loadedConfig); err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	if err := loadedConfig.Validate(); err != nil {
		t.Fatalf("Loaded config validation failed: %v", err)
	}

	if loadedConfig.Version != config.Version {
		t.Errorf("Loaded config Version = %v, want %v", loadedConfig.Version, config.Version)
	}

	if loadedConfig.Default != config.Default {
		t.Errorf("Loaded config Default = %v, want %v", loadedConfig.Default, config.Default)
	}
}

func TestCreateDefaultAgentsConfigFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "agents.yml")

	// Create default configuration
	config := DefaultAgentsConfig()

	// Save it to file
	data, err := yaml.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal default config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to write default config file: %v", err)
	}

	// Load and verify
	loadedData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var loadedConfig AgentsConfig
	if err := yaml.Unmarshal(loadedData, &loadedConfig); err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	if err := loadedConfig.Validate(); err != nil {
		t.Fatalf("Loaded default config validation failed: %v", err)
	}

	if loadedConfig.Version != "1.0" {
		t.Errorf("Default config Version = %v, want %v", loadedConfig.Version, "1.0")
	}

	if loadedConfig.Default != "default" {
		t.Errorf("Default config Default = %v, want %v", loadedConfig.Default, "default")
	}

	defaultAgent, exists := loadedConfig.Agents["default"]
	if !exists {
		t.Error("Default agent not found in loaded config")
	}

	if defaultAgent.Image != "laforge-agent:latest" {
		t.Errorf("Default agent Image = %v, want %v", defaultAgent.Image, "laforge-agent:latest")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsAt(s, substr)
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
