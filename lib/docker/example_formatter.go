//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"os"

	"github.com/tomyedwab/laforge/lib/docker"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run example_formatter.go <path-to-claude-output.json>")
		os.Exit(1)
	}

	// Read the file
	content, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	// Check if it's Claude JSON output
	fmt.Println("=" + string(make([]byte, 78)) + "=")
	fmt.Println()

	// Format and print
	formatted := docker.FormatClaudeOutput(string(content))
	fmt.Println(formatted)
}
