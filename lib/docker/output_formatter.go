package docker

import (
	"encoding/json"
	"fmt"
	"strings"
)

// formatToolParams extracts and formats key parameters from tool input
func formatToolParams(input map[string]interface{}) string {
	if len(input) == 0 {
		return ""
	}

	// Common parameter names to extract (in order of preference)
	keyParams := []string{"command", "file_path", "pattern", "path", "message", "query", "description"}

	var params []string
	for _, key := range keyParams {
		if val, ok := input[key]; ok {
			// Format the value compactly
			valStr := fmt.Sprintf("%v", val)
			// Truncate if too long
			if len(valStr) > 60 {
				valStr = valStr[:57] + "..."
			}
			params = append(params, valStr)
			// Only show first important parameter for compactness
			break
		}
	}

	// If no key params found, just show count
	if len(params) == 0 {
		return fmt.Sprintf("%d params", len(input))
	}

	return strings.Join(params, ", ")
}

// ClaudeMessage represents different types of Claude Code JSON output messages
type ClaudeMessage struct {
	Type           string                 `json:"type"`
	Subtype        string                 `json:"subtype,omitempty"`
	Message        json.RawMessage        `json:"message,omitempty"`
	Result         string                 `json:"result,omitempty"`
	IsError        bool                   `json:"is_error,omitempty"`
	DurationMS     int                    `json:"duration_ms,omitempty"`
	DurationAPIMS  int                    `json:"duration_api_ms,omitempty"`
	NumTurns       int                    `json:"num_turns,omitempty"`
	TotalCostUSD   float64                `json:"total_cost_usd,omitempty"`
	Usage          map[string]interface{} `json:"usage,omitempty"`
	ModelUsage     map[string]interface{} `json:"modelUsage,omitempty"`
	SessionID      string                 `json:"session_id,omitempty"`
	UUID           string                 `json:"uuid,omitempty"`
	CWD            string                 `json:"cwd,omitempty"`
	Tools          []string               `json:"tools,omitempty"`
	MCPServers     []string               `json:"mcp_servers,omitempty"`
	Model          string                 `json:"model,omitempty"`
	PermissionMode string                 `json:"permission_mode,omitempty"`
	SlashCommands  []string               `json:"slash_commands,omitempty"`
	Agents         []string               `json:"agents,omitempty"`
	Skills         []string               `json:"skills,omitempty"`
}

// AssistantMessage represents the detailed structure of assistant messages
type AssistantMessage struct {
	Model        string                   `json:"model"`
	ID           string                   `json:"id"`
	Type         string                   `json:"type"`
	Role         string                   `json:"role"`
	Content      []map[string]interface{} `json:"content"`
	StopReason   *string                  `json:"stop_reason"`
	StopSequence *string                  `json:"stop_sequence"`
	Usage        map[string]interface{}   `json:"usage"`
}

// FormatClaudeOutput transforms Claude Code JSON output into human-readable markdown
// If the input is not valid JSON lines, it returns the raw text as-is
func FormatClaudeOutput(logs string) string {
	lines := strings.Split(logs, "\n")
	var output strings.Builder
	var sessionInfo *ClaudeMessage
	var messageCount int
	var hasResult bool
	var hasUserMessage bool

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Try to parse as JSON
		var msg ClaudeMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			// Not valid JSON, return raw text for all logs
			return logs
		}

		// Process different message types
		switch msg.Type {
		case "system":
			if msg.Subtype == "init" {
				sessionInfo = &msg
				// Compact header
				output.WriteString(fmt.Sprintf("# Claude Code (%s)\n", msg.Model))
				output.WriteString(fmt.Sprintf("`%s` • Session: `%s`\n\n", msg.CWD, msg.SessionID))
			}

		case "assistant":
			messageCount++
			var assistantMsg AssistantMessage
			if err := json.Unmarshal(msg.Message, &assistantMsg); err == nil {
				// Process content items - more compact format
				for i, content := range assistantMsg.Content {
					contentType, _ := content["type"].(string)

					switch contentType {
					case "text":
						if text, ok := content["text"].(string); ok {
							// Only add newline between items, not before first
							if i > 0 {
								output.WriteString("\n")
							}
							output.WriteString(text)
							output.WriteString("\n")
						}

					case "tool_use":
						toolName, _ := content["name"].(string)
						toolInput, _ := content["input"].(map[string]interface{})

						// Compact one-line format for tool use
						if i > 0 {
							output.WriteString("\n")
						}

						// Format tool input compactly - extract key parameters
						paramStr := formatToolParams(toolInput)
						if paramStr != "" {
							output.WriteString(fmt.Sprintf("> %s (%s)\n", toolName, paramStr))
						} else {
							output.WriteString(fmt.Sprintf("> %s\n", toolName))
						}
					}
				}

				output.WriteString("\n")
			}

		case "user":
			hasUserMessage = true
			// User messages contain tool results - show them (with length limits)
			var userMsg map[string]interface{}
			if err := json.Unmarshal(msg.Message, &userMsg); err == nil {
				if content, ok := userMsg["content"].([]interface{}); ok {
					for _, item := range content {
						if itemMap, ok := item.(map[string]interface{}); ok {
							// Check if this is an error result
							if isError, ok := itemMap["is_error"].(bool); ok && isError {
								if contentStr, ok := itemMap["content"].(string); ok {
									// Extract error message if it's wrapped in XML tags
									errorMsg := contentStr
									if strings.Contains(contentStr, "<tool_use_error>") {
										start := strings.Index(contentStr, "<tool_use_error>") + len("<tool_use_error>")
										end := strings.Index(contentStr, "</tool_use_error>")
										if start > 0 && end > start {
											errorMsg = contentStr[start:end]
										}
									}
									output.WriteString(fmt.Sprintf("⚠️  Tool error: %s\n\n", errorMsg))
								}
							} else if contentStr, ok := itemMap["content"].(string); ok {
								// Non-error result - show if not too long
								lines := strings.Split(contentStr, "\n")
								if len(lines) <= 100 {
									output.WriteString(fmt.Sprintf("**Tool result:**\n```\n%s\n```\n\n", contentStr))
								} else {
									// Truncate long results
									truncated := strings.Join(lines[:50], "\n") + "\n\n... [" + fmt.Sprintf("%d", len(lines)-100) + " lines truncated] ...\n\n" + strings.Join(lines[len(lines)-50:], "\n")
									output.WriteString(fmt.Sprintf("**Tool result** _(truncated, %d lines total)_:\n```\n%s\n```\n\n", len(lines), truncated))
								}
							}
						}
					}
				}
			}
			continue

		case "result":
			if msg.Subtype == "success" || msg.Subtype == "" {
				hasResult = true
				// Compact summary on one or two lines
				var summaryParts []string

				if !msg.IsError {
					summaryParts = append(summaryParts, "✅ Success")
				} else {
					summaryParts = append(summaryParts, "❌ Error")
				}

				if msg.NumTurns > 0 {
					summaryParts = append(summaryParts, fmt.Sprintf("%d turns", msg.NumTurns))
				}

				if msg.DurationMS > 0 {
					summaryParts = append(summaryParts, fmt.Sprintf("%.1fs", float64(msg.DurationMS)/1000))
				}

				if msg.TotalCostUSD > 0 {
					summaryParts = append(summaryParts, fmt.Sprintf("$%.4f", msg.TotalCostUSD))
				}

				output.WriteString("---\n")
				output.WriteString(strings.Join(summaryParts, " • "))
				output.WriteString("\n")

				// Show result text if present
				if msg.Result != "" {
					output.WriteString("\n")
					output.WriteString(msg.Result)
					output.WriteString("\n")
				}
			}
		}
	}

	// If we parsed at least some JSON, return the formatted output
	if sessionInfo != nil || messageCount > 0 || hasResult || hasUserMessage {
		result := output.String()
		// If we only have user messages (no assistant or result), add a note about incomplete log
		if hasUserMessage && sessionInfo == nil && messageCount == 0 && !hasResult && strings.TrimSpace(result) == "" {
			return "_(Incomplete log - contains only tool results with no assistant messages or results)_\n"
		}
		return result
	}

	// Otherwise, return raw logs
	return logs
}
