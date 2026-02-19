package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the complete orchestrator configuration
type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Redis         RedisConfig         `yaml:"redis"`
	Gitea         GiteaConfig         `yaml:"gitea"`
	Worker        WorkerConfig        `yaml:"worker"`
	Bot           BotConfig           `yaml:"bot"`
	Docker        DockerConfig        `yaml:"docker"`
	Notifications NotificationsConfig `yaml:"notifications"`
	Anthropic     AnthropicConfig     `yaml:"anthropic"`
	Models        map[string]Model    `yaml:"models"`
	Prompts       PromptsConfig       `yaml:"prompts"`
	Repositories  RepositoriesConfig  `yaml:"repositories"`
}

// ServerConfig contains server-related configuration
type ServerConfig struct {
	Port          string `yaml:"port"`
	WebhookSecret string `yaml:"webhook_secret"`
}

// RedisConfig contains Redis connection configuration
type RedisConfig struct {
	Address string `yaml:"address"`
}

// GiteaConfig contains Gitea-related configuration
type GiteaConfig struct {
	URL         string `yaml:"url"`
	ExternalURL string `yaml:"external_url"`
	Token       string `yaml:"token"`
}

// WorkerConfig contains worker-related configuration
type WorkerConfig struct {
	Concurrency int `yaml:"concurrency"`
}

// BotConfig contains bot user configuration
type BotConfig struct {
	Username string `yaml:"username"`
	Email    string `yaml:"email"`
}

// DockerConfig contains Docker image configuration
type DockerConfig struct {
	GitImage              string `yaml:"git_image"`
	NetworkName           string `yaml:"network_name"`
	LogsVolumeName        string `yaml:"logs_volume_name"`
	DefaultDevcontainerImage string `yaml:"default_devcontainer_image"`
}

// RepositoryConfig contains per-repository configuration
type RepositoryConfig struct {
	DevcontainerImage string `yaml:"devcontainer_image"`
}

// RepositoriesConfig maps repository names to their configurations
type RepositoriesConfig map[string]RepositoryConfig

// NotificationsConfig contains notification settings
type NotificationsConfig struct {
	NtfyEndpoint string `yaml:"ntfy_endpoint"`
}

// AnthropicConfig contains Anthropic API proxy configuration
type AnthropicConfig struct {
	BaseUrl    string `yaml:"base_url"`
	APIKey     string `yaml:"api_key"`
	OAuthToken string `yaml:"oauth_token"`
	Port       string `yaml:"port"`
}

// Model represents a model configuration with its ID and container image
type Model struct {
	ModelID string `yaml:"model_id"`
	Image   string `yaml:"image"`
}

// PromptsConfig contains prompt-related configuration
type PromptsConfig struct {
	DefaultType  string   `yaml:"default_type"`
	DefaultModel string   `yaml:"default_model"`
	ValidTypes   []string `yaml:"valid_types"`
}

// Load reads and parses the YAML configuration file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults
	if cfg.Server.Port == "" {
		cfg.Server.Port = "8080"
	}
	if cfg.Worker.Concurrency == 0 {
		cfg.Worker.Concurrency = 5
	}
	if cfg.Bot.Username == "" {
		cfg.Bot.Username = "laforge"
	}
	if cfg.Bot.Email == "" {
		cfg.Bot.Email = "laforge@example.com"
	}
	if cfg.Docker.GitImage == "" {
		cfg.Docker.GitImage = "alpine/git:latest"
	}
	if cfg.Docker.NetworkName == "" {
		cfg.Docker.NetworkName = "laforge_gitea"
	}
	if cfg.Docker.LogsVolumeName == "" {
		cfg.Docker.LogsVolumeName = "laforge_logs"
	}
	if cfg.Notifications.NtfyEndpoint == "" {
		cfg.Notifications.NtfyEndpoint = "http://ntfy:80"
	}
	if cfg.Prompts.DefaultType == "" {
		cfg.Prompts.DefaultType = "implement"
	}
	if cfg.Prompts.DefaultModel == "" {
		cfg.Prompts.DefaultModel = "sonnet"
	}
	if len(cfg.Prompts.ValidTypes) == 0 {
		cfg.Prompts.ValidTypes = []string{"implement", "plan", "critique"}
	}

	// Set default for Gitea external URL
	if cfg.Gitea.ExternalURL == "" {
		cfg.Gitea.ExternalURL = cfg.Gitea.URL
	}

	// Set default for Anthropic API url
	if cfg.Anthropic.BaseUrl == "" {
		cfg.Anthropic.BaseUrl = "https://api.anthropic.com"
	}
	// Set default for Anthropic proxy port
	if cfg.Anthropic.Port == "" {
		cfg.Anthropic.Port = "8081"
	}

	// Set default devcontainer image
	if cfg.Docker.DefaultDevcontainerImage == "" {
		cfg.Docker.DefaultDevcontainerImage = "python:3.11-slim"
	}

	// Apply default devcontainer image to all repositories that don't have one configured
	for repoName, repoConfig := range cfg.Repositories {
		if repoConfig.DevcontainerImage == "" {
			repoConfig.DevcontainerImage = cfg.Docker.DefaultDevcontainerImage
			cfg.Repositories[repoName] = repoConfig
		}
	}

	// Validate required fields
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate checks that required configuration fields are set
func (c *Config) Validate() error {
	if c.Redis.Address == "" {
		return fmt.Errorf("redis.address is required")
	}
	if c.Gitea.URL == "" {
		return fmt.Errorf("gitea.url is required")
	}
	if len(c.Models) == 0 {
		return fmt.Errorf("at least one model must be configured")
	}

	// Validate that default model exists
	if _, ok := c.Models[c.Prompts.DefaultModel]; !ok {
		return fmt.Errorf("prompts.default_model '%s' is not defined in models", c.Prompts.DefaultModel)
	}

	// Validate that all models have required fields
	for name, model := range c.Models {
		if model.ModelID == "" {
			return fmt.Errorf("model '%s' missing model_id", name)
		}
		if model.Image == "" {
			return fmt.Errorf("model '%s' missing image", name)
		}
	}

	return nil
}

// GetModel returns the model configuration for a given model name
// Returns the default model if the name is not found
func (c *Config) GetModel(name string) Model {
	if model, ok := c.Models[name]; ok {
		return model
	}
	// Return default model if not found
	return c.Models[c.Prompts.DefaultModel]
}

// IsValidPromptType checks if a prompt type is valid
func (c *Config) IsValidPromptType(promptType string) bool {
	for _, valid := range c.Prompts.ValidTypes {
		if valid == promptType {
			return true
		}
	}
	return false
}

// GetRepositoryConfig returns the repository configuration for a given repository
// Returns a config with the default devcontainer image if the repository is not found
func (c *Config) GetRepositoryConfig(repository string) RepositoryConfig {
	if config, ok := c.Repositories[repository]; ok {
		return config
	}
	// Return config with default devcontainer image
	return RepositoryConfig{
		DevcontainerImage: c.Docker.DefaultDevcontainerImage,
	}
}
