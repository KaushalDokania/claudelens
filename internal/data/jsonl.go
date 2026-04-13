package data

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// ConversationMessage represents a single parsed message from a JSONL file.
type ConversationMessage struct {
	Role      string // "user" or "assistant"
	Content   string // Extracted text content
	Timestamp time.Time
	ToolCalls []string // Brief tool call summaries
}

// jsonlLine represents a single line in a Claude Code JSONL file.
type jsonlLine struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Message   json.RawMessage `json:"message"`
}

type messagePayload struct {
	Role    string            `json:"role"`
	Content json.RawMessage   `json:"content"`
}

type contentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ParseConversation reads a JSONL file and returns parsed conversation messages.
// limit controls how many messages to return (0 = unlimited).
func ParseConversation(jsonlPath string, limit int) ([]ConversationMessage, error) {
	f, err := os.Open(jsonlPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var messages []ConversationMessage
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10MB max line

	for scanner.Scan() {
		if limit > 0 && len(messages) >= limit {
			return messages, nil
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry jsonlLine
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		if entry.Message == nil {
			continue
		}

		var msg messagePayload
		if err := json.Unmarshal(entry.Message, &msg); err != nil {
			continue
		}

		if msg.Role != "user" && msg.Role != "assistant" {
			continue
		}

		text, tools := extractContent(msg.Content)
		if text == "" && len(tools) == 0 {
			continue
		}

		messages = append(messages, ConversationMessage{
			Role:      msg.Role,
			Content:   text,
			Timestamp: parseTime(entry.Timestamp),
			ToolCalls: tools,
		})
	}

	return messages, scanner.Err()
}

// extractContent parses the content field which can be a string or an array
// of content blocks.
func extractContent(raw json.RawMessage) (string, []string) {
	if len(raw) == 0 {
		return "", nil
	}

	// Try as a plain string first
	var plainText string
	if err := json.Unmarshal(raw, &plainText); err == nil {
		return plainText, nil
	}

	// Try as an array of content blocks
	var blocks []contentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return "", nil
	}

	var textParts []string
	var toolCalls []string

	for _, block := range blocks {
		switch block.Type {
		case "text":
			if block.Text != "" {
				textParts = append(textParts, block.Text)
			}
		case "tool_use":
			summary := formatToolCall(block.Name, block.Input)
			toolCalls = append(toolCalls, summary)
		}
	}

	return strings.Join(textParts, "\n"), toolCalls
}

// formatToolCall creates a brief summary of a tool call.
func formatToolCall(name string, input json.RawMessage) string {
	if name == "" {
		return "[tool call]"
	}

	var params map[string]interface{}
	if err := json.Unmarshal(input, &params); err != nil {
		return fmt.Sprintf("[%s]", name)
	}

	// Extract a useful preview based on common tool patterns
	if path, ok := params["file_path"].(string); ok {
		return fmt.Sprintf("[%s %s]", name, shortPath(path))
	}
	if cmd, ok := params["command"].(string); ok {
		if len(cmd) > 40 {
			cmd = cmd[:40] + "..."
		}
		return fmt.Sprintf("[%s: %s]", name, cmd)
	}
	if pattern, ok := params["pattern"].(string); ok {
		return fmt.Sprintf("[%s %s]", name, pattern)
	}

	return fmt.Sprintf("[%s]", name)
}

// shortPath returns the last two components of a file path.
func shortPath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) <= 2 {
		return path
	}
	return strings.Join(parts[len(parts)-2:], "/")
}
