package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

// Message represents a Claude JSON message
type Message struct {
	Type    string          `json:"type"`
	Subtype string          `json:"subtype"`
	Message json.RawMessage `json:"message"`
	Model   string          `json:"model"`
	CWD     string          `json:"cwd"`
	IsError bool            `json:"is_error"`
	Result  string          `json:"result"`

	// Result fields
	NumTurns     int     `json:"num_turns"`
	DurationMS   int     `json:"duration_ms"`
	TotalCostUSD float64 `json:"total_cost_usd"`
}

// AssistantMessage represents an assistant message
type AssistantMessage struct {
	Content []ContentBlock `json:"content"`
}

// UserMessage represents a user message
type UserMessage struct {
	Content []ToolResult `json:"content"`
}

// ContentBlock represents a content block in an assistant message
type ContentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolResult represents a tool result in a user message
type ToolResult struct {
	Type    string `json:"type"`
	Content string `json:"content"`
	IsError bool   `json:"is_error"`
}

var (
	// Global counter for unique IDs
	idCounter int
	// Markdown renderer
	md goldmark.Markdown
)

func init() {
	// Initialize markdown renderer with common extensions
	md = goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Table,
			extension.Strikethrough,
			extension.TaskList,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			gmhtml.WithUnsafe(), // Allow raw HTML in markdown
		),
	)
}

// renderMarkdown converts markdown text to HTML
func renderMarkdown(text string) string {
	var buf bytes.Buffer
	if err := md.Convert([]byte(text), &buf); err != nil {
		// If markdown parsing fails, fall back to escaped HTML with <br> for newlines
		escaped := html.EscapeString(text)
		return strings.ReplaceAll(escaped, "\n", "<br>\n")
	}
	return buf.String()
}

// nextID generates a unique ID for collapsible elements
func nextID() string {
	idCounter++
	return fmt.Sprintf("tool-%d", idCounter)
}

// formatToolParams formats tool parameters compactly
func formatToolParams(input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}

	var params map[string]interface{}
	if err := json.Unmarshal(input, &params); err != nil {
		return ""
	}

	if len(params) == 0 {
		return ""
	}

	// Common parameter names to extract
	keyParams := []string{"command", "file_path", "pattern", "path", "message", "query", "description"}

	for _, key := range keyParams {
		if val, ok := params[key]; ok {
			valStr := fmt.Sprintf("%v", val)
			// Truncate if too long
			if len(valStr) > 60 {
				valStr = valStr[:57] + "..."
			}
			return html.EscapeString(valStr)
		}
	}

	// If no key params found, show count
	return fmt.Sprintf("%d params", len(params))
}

// RenderMessage renders a Claude JSON message to HTML
func RenderMessage(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}

	var msg Message
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		// Not valid JSON, output as-is
		return fmt.Sprintf("<div class=\"raw\">%s</div>\n", html.EscapeString(line))
	}

	var output strings.Builder

	switch msg.Type {
	case "system":
		if msg.Subtype == "init" {
			output.WriteString("<div class=\"session-header\">\n")
			output.WriteString(fmt.Sprintf("<h2>Claude Code (%s)</h2>\n", html.EscapeString(msg.Model)))
			output.WriteString(fmt.Sprintf("<p><code>%s</code></p>\n", html.EscapeString(msg.CWD)))
			output.WriteString("</div>\n")
		}

	case "assistant":
		if len(msg.Message) > 0 {
			var assistantMsg AssistantMessage
			if err := json.Unmarshal(msg.Message, &assistantMsg); err == nil {
				for _, content := range assistantMsg.Content {
					switch content.Type {
					case "text":
						if content.Text != "" {
							// Render markdown to HTML
							htmlText := renderMarkdown(content.Text)
							output.WriteString(fmt.Sprintf("<div class=\"assistant-text\">%s</div>\n", htmlText))
						}

					case "tool_use":
						paramStr := formatToolParams(content.Input)
						output.WriteString("<div class=\"tool-use\">")
						if paramStr != "" {
							output.WriteString(fmt.Sprintf("&gt; <strong>%s</strong> (%s)", html.EscapeString(content.Name), paramStr))
						} else {
							output.WriteString(fmt.Sprintf("&gt; <strong>%s</strong>", html.EscapeString(content.Name)))
						}
						output.WriteString("</div>\n")
					}
				}
			}
		}

	case "user":
		if len(msg.Message) > 0 {
			var userMsg UserMessage
			if err := json.Unmarshal(msg.Message, &userMsg); err == nil {
				for _, item := range userMsg.Content {
					if item.IsError && item.Content != "" {
						// Extract error message if wrapped in XML tags
						errorMsg := item.Content
						startTag := "<tool_use_error>"
						endTag := "</tool_use_error>"
						start := strings.Index(item.Content, startTag)
						end := strings.Index(item.Content, endTag)
						if start >= 0 && end > start {
							errorMsg = item.Content[start+len(startTag) : end]
						}
						output.WriteString(fmt.Sprintf("<div class=\"tool-error\">⚠️ Tool error: %s</div>\n", html.EscapeString(errorMsg)))
					} else if item.Content != "" {
						// Non-error result - make it collapsible
						lines := strings.Split(item.Content, "\n")
						id := nextID()
						lineCount := len(lines)

						output.WriteString("<div class=\"tool-result-collapsible\">\n")
						output.WriteString(fmt.Sprintf("<div class=\"tool-result-header\" onclick=\"toggleTool('%s')\">\n", id))
						output.WriteString(fmt.Sprintf("<span class=\"toggle-icon\" id=\"icon-%s\">▶</span> ", id))
						output.WriteString(fmt.Sprintf("<strong>Tool result</strong> <em>(%d lines)</em>\n", lineCount))
						output.WriteString("</div>\n")

						output.WriteString(fmt.Sprintf("<div class=\"tool-result-content\" id=\"%s\" style=\"display: none;\">\n", id))
						if lineCount <= 100 {
							output.WriteString(fmt.Sprintf("<pre>%s</pre>\n", html.EscapeString(item.Content)))
						} else {
							// Truncate long results
							firstLines := strings.Join(lines[:50], "\n")
							lastLines := strings.Join(lines[len(lines)-50:], "\n")
							truncatedCount := lineCount - 100
							truncated := fmt.Sprintf("%s\n\n... [%d lines truncated] ...\n\n%s", firstLines, truncatedCount, lastLines)
							output.WriteString(fmt.Sprintf("<pre>%s</pre>\n", html.EscapeString(truncated)))
						}
						output.WriteString("</div>\n")
						output.WriteString("</div>\n")
					}
				}
			}
		}

	case "result":
		if msg.Subtype == "success" || msg.Subtype == "" {
			summaryParts := []string{}

			if !msg.IsError {
				summaryParts = append(summaryParts, "✅ Success")
			} else {
				summaryParts = append(summaryParts, "❌ Error")
			}

			if msg.NumTurns > 0 {
				summaryParts = append(summaryParts, fmt.Sprintf("%d turns", msg.NumTurns))
			}

			if msg.DurationMS > 0 {
				summaryParts = append(summaryParts, fmt.Sprintf("%.1fs", float64(msg.DurationMS)/1000.0))
			}

			if msg.TotalCostUSD > 0 {
				summaryParts = append(summaryParts, fmt.Sprintf("$%.4f", msg.TotalCostUSD))
			}

			output.WriteString("<div class=\"result-summary\">\n")
			output.WriteString("<hr>\n")
			output.WriteString(fmt.Sprintf("<p>%s</p>\n", strings.Join(summaryParts, " • ")))

			if msg.Result != "" {
				output.WriteString(fmt.Sprintf("<p>%s</p>\n", html.EscapeString(msg.Result)))
			}
			output.WriteString("</div>\n")
		}
	}

	return output.String()
}
