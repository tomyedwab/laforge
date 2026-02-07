package types

// FileComment represents a file-level comment in a PR review
type FileComment struct {
	File    string `yaml:"file"`
	Line    int    `yaml:"line"`
	Comment string `yaml:"comment"`
}

// BashRequest represents a bash command execution request
type BashRequest struct {
	Command string `json:"command"`
}

// BashResponse represents a bash command execution response
type BashResponse struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// BashJobContext holds the context for a bash command execution
type BashJobContext struct {
	VolumeName           string
	DevcontainerImage    string
	DevcontainerID       string // ID of the running devcontainer
	Repository           string
	CommandTimeout       int // timeout in seconds
}
