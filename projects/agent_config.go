package projects

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// AgentConfig represents configuration for a LaForge agent
type AgentConfig struct {
	// Name is the identifier for this agent configuration
	Name string `yaml:"name"`

	// Image is the Docker image to use for the agent
	Image string `yaml:"image"`

	// Description provides human-readable information about the agent
	Description string `yaml:"description,omitempty"`

	// Environment variables to set in the container
	Environment map[string]string `yaml:"environment,omitempty"`

	// Volume mounts in the format "host_path:container_path[:mode]"
	Volumes []string `yaml:"volumes,omitempty"`

	// Resource limits
	Resources ResourceConfig `yaml:"resources,omitempty"`

	// Container runtime options
	Runtime RuntimeConfig `yaml:"runtime,omitempty"`

	// Command to run in the container (overrides image default)
	Command []string `yaml:"command,omitempty"`

	// Working directory in the container
	WorkingDir string `yaml:"working_dir,omitempty"`
}

// ResourceConfig represents resource limits for the agent container
type ResourceConfig struct {
	// Memory limit (e.g., "512m", "1g")
	Memory string `yaml:"memory,omitempty"`

	// CPU shares (relative weight vs other containers)
	CPUShares int64 `yaml:"cpu_shares,omitempty"`

	// CPU limit (e.g., "1.0" for one full CPU, "0.5" for half)
	CPULimit string `yaml:"cpu_limit,omitempty"`

	// Maximum number of PIDs
	PidsLimit int64 `yaml:"pids_limit,omitempty"`
}

// RuntimeConfig represents container runtime configuration
type RuntimeConfig struct {
	// Whether to automatically remove the container when it exits
	AutoRemove bool `yaml:"auto_remove,omitempty"`

	// Timeout for container execution (e.g., "30m", "1h")
	Timeout string `yaml:"timeout,omitempty"`

	// Network mode (e.g., "bridge", "host", "none")
	NetworkMode string `yaml:"network_mode,omitempty"`

	// Whether to run in privileged mode
	Privileged bool `yaml:"privileged,omitempty"`

	// Additional capabilities to add
	Capabilities []string `yaml:"capabilities,omitempty"`

	// Devices to mount (e.g., ["/dev/sda:/dev/xvda:rwm"])
	Devices []string `yaml:"devices,omitempty"`
}

// AgentsConfig represents the complete agents.yml file structure
type AgentsConfig struct {
	// Version of the configuration format
	Version string `yaml:"version"`

	// Default agent to use when none is specified
	Default string `yaml:"default,omitempty"`

	// Available agent configurations
	Agents map[string]AgentConfig `yaml:"agents"`
}

// DefaultAgentsConfig returns a default agents configuration
func DefaultAgentsConfig() *AgentsConfig {
	return &AgentsConfig{
		Version: "1.0",
		Default: "default",
		Agents: map[string]AgentConfig{
			"default": {
				Name:        "default",
				Image:       "laforge-agent:latest",
				Description: "Default LaForge agent configuration",
				Environment: map[string]string{
					"LAFORGE_AGENT": "true",
				},
				Resources: ResourceConfig{
					Memory:    "512m",
					CPUShares: 512,
				},
				Runtime: RuntimeConfig{
					AutoRemove:  true,
					Timeout:     "30m",
					NetworkMode: "bridge",
				},
				WorkingDir: "/workspace",
			},
		},
	}
}

// Validate checks if the agent configuration is valid
func (c *AgentConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("agent name is required")
	}
	if c.Image == "" {
		return fmt.Errorf("agent image is required")
	}

	// Validate resource limits
	if err := c.Resources.Validate(); err != nil {
		return fmt.Errorf("invalid resource configuration: %w", err)
	}

	// Validate runtime configuration
	if err := c.Runtime.Validate(); err != nil {
		return fmt.Errorf("invalid runtime configuration: %w", err)
	}

	// Validate volume mounts
	for i, volume := range c.Volumes {
		if err := validateVolumeMount(volume); err != nil {
			return fmt.Errorf("invalid volume mount at index %d: %w", i, err)
		}
	}

	return nil
}

// Validate checks if the resource configuration is valid
func (r *ResourceConfig) Validate() error {
	// Validate memory format if specified
	if r.Memory != "" {
		if err := validateMemoryFormat(r.Memory); err != nil {
			return fmt.Errorf("invalid memory format: %w", err)
		}
	}

	// Validate CPU limit format if specified
	if r.CPULimit != "" {
		if err := validateCPULimitFormat(r.CPULimit); err != nil {
			return fmt.Errorf("invalid CPU limit format: %w", err)
		}
	}

	return nil
}

// Validate checks if the runtime configuration is valid
func (r *RuntimeConfig) Validate() error {
	// Validate timeout format if specified
	if r.Timeout != "" {
		if _, err := time.ParseDuration(r.Timeout); err != nil {
			return fmt.Errorf("invalid timeout format: %w", err)
		}
	}

	// Validate network mode
	validNetworkModes := []string{"", "bridge", "host", "none"}
	valid := false
	for _, mode := range validNetworkModes {
		if r.NetworkMode == mode {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid network mode: %s", r.NetworkMode)
	}

	return nil
}

// Validate checks if the agents configuration is valid
func (c *AgentsConfig) Validate() error {
	if c.Version == "" {
		return fmt.Errorf("configuration version is required")
	}

	if len(c.Agents) == 0 {
		return fmt.Errorf("at least one agent configuration is required")
	}

	// Validate each agent configuration
	for name, agent := range c.Agents {
		if err := agent.Validate(); err != nil {
			return fmt.Errorf("invalid agent configuration '%s': %w", name, err)
		}
	}

	// Validate default agent exists
	if c.Default != "" {
		if _, exists := c.Agents[c.Default]; !exists {
			return fmt.Errorf("default agent '%s' not found", c.Default)
		}
	}

	return nil
}

// GetAgent returns the agent configuration by name
func (c *AgentsConfig) GetAgent(name string) (AgentConfig, bool) {
	agent, exists := c.Agents[name]
	return agent, exists
}

// GetDefaultAgent returns the default agent configuration
func (c *AgentsConfig) GetDefaultAgent() (AgentConfig, bool) {
	if c.Default == "" {
		// Return first agent if no default is specified
		for _, agent := range c.Agents {
			return agent, true
		}
		return AgentConfig{}, false
	}
	return c.GetAgent(c.Default)
}

// MarshalYAML implements custom YAML marshaling
func (c *AgentsConfig) MarshalYAML() (interface{}, error) {
	// Create a type alias to avoid infinite recursion
	type Alias AgentsConfig
	return (*Alias)(c), nil
}

// UnmarshalYAML implements custom YAML unmarshaling
func (c *AgentsConfig) UnmarshalYAML(node *yaml.Node) error {
	// Create a type alias to avoid infinite recursion
	type Alias AgentsConfig
	var alias Alias

	if err := node.Decode(&alias); err != nil {
		return err
	}

	*c = AgentsConfig(alias)
	return nil
}

// Helper functions

func validateMemoryFormat(memory string) error {
	if memory == "" {
		return nil
	}

	// Simple validation for common memory formats
	validUnits := []string{"b", "k", "kb", "m", "mb", "g", "gb"}
	memory = strings.ToLower(strings.TrimSpace(memory))

	for _, unit := range validUnits {
		if strings.HasSuffix(memory, unit) {
			return nil
		}
	}

	return fmt.Errorf("memory must end with one of: %v", validUnits)
}

func validateCPULimitFormat(cpu string) error {
	if cpu == "" {
		return nil
	}

	// Simple validation for CPU limit format (should be a number)
	// In a real implementation, you might want more sophisticated validation
	return nil
}

func validateVolumeMount(volume string) error {
	if volume == "" {
		return fmt.Errorf("volume mount cannot be empty")
	}

	parts := strings.Split(volume, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return fmt.Errorf("volume mount must be in format 'host_path:container_path[:mode]'")
	}

	if parts[0] == "" {
		return fmt.Errorf("host path cannot be empty")
	}

	if parts[1] == "" {
		return fmt.Errorf("container path cannot be empty")
	}

	// Validate mode if specified
	if len(parts) == 3 {
		mode := parts[2]
		validModes := []string{"ro", "rw", "z", "Z"}
		valid := false
		for _, validMode := range validModes {
			if mode == validMode {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid volume mode: %s", mode)
		}
	}

	return nil
}
