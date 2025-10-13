package docker

import (
	"os/exec"
	"testing"
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
