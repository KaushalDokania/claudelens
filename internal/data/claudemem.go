package data

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

const defaultMemURL = "http://localhost:37777"

// ClaudeMemClient provides access to the claude-mem HTTP API.
type ClaudeMemClient struct {
	baseURL    string
	httpClient *http.Client
}

// SessionSummary represents a session summary from /api/summaries.
type SessionSummary struct {
	ID           int    `json:"id"`
	SessionID    string `json:"session_id"`
	Project      string `json:"project"`
	Request      string `json:"request"`
	Investigated string `json:"investigated"`
	Learned      string `json:"learned"`
	Completed    string `json:"completed"`
	NextSteps    string `json:"next_steps"`
	CreatedAt    string `json:"created_at"`
}

type summariesResponse struct {
	Items   []SessionSummary `json:"items"`
	HasMore bool             `json:"hasMore"`
	Offset  int              `json:"offset"`
	Limit   int              `json:"limit"`
}

type projectsResponse struct {
	Projects []string `json:"projects"`
}

type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// mcpResponse is the common wrapper for search endpoints that return
// MCP-formatted content (markdown text with embedded IDs).
type mcpResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// SessionSearchResult represents a session found via semantic search.
type SessionSearchResult struct {
	ID    int    // claude-mem internal ID
	Title string // extracted from search result
}

// NewClaudeMemClient creates a new client with the given base URL.
// If baseURL is empty, it defaults to http://localhost:37777.
func NewClaudeMemClient(baseURL string) *ClaudeMemClient {
	if baseURL == "" {
		baseURL = defaultMemURL
	}
	return &ClaudeMemClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// IsAvailable checks if the claude-mem worker is running and healthy.
func (c *ClaudeMemClient) IsAvailable() bool {
	resp, err := c.httpClient.Get(c.baseURL + "/api/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	var health healthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return false
	}
	return health.Status == "ok"
}

// ListSummaries returns paginated session summaries.
func (c *ClaudeMemClient) ListSummaries(limit, offset int) ([]SessionSummary, bool, error) {
	u := fmt.Sprintf("%s/api/summaries?limit=%d&offset=%d", c.baseURL, limit, offset)
	resp, err := c.httpClient.Get(u)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("summaries returned status %d", resp.StatusCode)
	}

	var result summariesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, false, err
	}
	return result.Items, result.HasMore, nil
}

// GetSession returns a single session summary by its claude-mem ID.
func (c *ClaudeMemClient) GetSession(id int) (*SessionSummary, error) {
	u := fmt.Sprintf("%s/api/session/%d", c.baseURL, id)
	resp, err := c.httpClient.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("session %d returned status %d", id, resp.StatusCode)
	}

	var summary SessionSummary
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		return nil, err
	}
	return &summary, nil
}

// ListProjects returns all indexed project names.
func (c *ClaudeMemClient) ListProjects() ([]string, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/api/projects")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("projects returned status %d", resp.StatusCode)
	}

	var result projectsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Projects, nil
}

// searchSessionIDRegex matches session IDs like "#S1132" in MCP markdown output.
var searchSessionIDRegex = regexp.MustCompile(`#S(\d+)`)

// SearchSessions performs a semantic search and returns matching session IDs.
// The search endpoint returns MCP-formatted markdown; we parse the IDs from it.
func (c *ClaudeMemClient) SearchSessions(query string, limit int) ([]SessionSearchResult, error) {
	u := fmt.Sprintf("%s/api/search/sessions?query=%s&limit=%d",
		c.baseURL, url.QueryEscape(query), limit)

	resp, err := c.httpClient.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search returned status %d: %s", resp.StatusCode, string(body))
	}

	var mcp mcpResponse
	if err := json.NewDecoder(resp.Body).Decode(&mcp); err != nil {
		return nil, err
	}

	var results []SessionSearchResult
	for _, content := range mcp.Content {
		if content.Type != "text" {
			continue
		}
		matches := searchSessionIDRegex.FindAllStringSubmatch(content.Text, -1)
		for _, match := range matches {
			id, err := strconv.Atoi(match[1])
			if err != nil {
				continue
			}
			title := extractTitleFromLine(content.Text, match[0])
			results = append(results, SessionSearchResult{ID: id, Title: title})
		}
	}

	return results, nil
}

// extractTitleFromLine attempts to extract the title from the markdown table
// row that contains the given ID marker.
func extractTitleFromLine(text, idMarker string) string {
	lines := splitLines(text)
	for _, line := range lines {
		if !containsString(line, idMarker) {
			continue
		}
		// Table format: | #S1132 | 3:18 AM | emoji | Title text | ... |
		parts := splitPipe(line)
		if len(parts) >= 5 {
			return trimString(parts[4])
		}
	}
	return ""
}

// SummaryToSession converts a claude-mem SessionSummary to a Session.
func SummaryToSession(s SessionSummary) Session {
	return Session{
		SessionID:   s.SessionID,
		Summary:     s.Request,
		FirstPrompt: s.Completed,
		Project:     s.Project,
		Source:      "claudemem",
		Created:     parseTime(s.CreatedAt),
		Modified:    parseTime(s.CreatedAt),
	}
}

// Helper functions to avoid importing strings for simple operations
// (these are small enough to inline).

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func splitPipe(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '|' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr) >= 0
}

func searchString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func trimString(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
