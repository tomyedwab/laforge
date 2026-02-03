package types

// FileComment represents a file-level comment in a PR review
type FileComment struct {
	File    string `yaml:"file"`
	Line    int    `yaml:"line"`
	Comment string `yaml:"comment"`
}
