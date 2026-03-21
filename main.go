package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/server"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
)

// QueryInput defines the input for querying an exporter
type QueryInput struct {
	Host    string `json:"host" description:"The IP address or hostname of the server" required:"true"`
	Port    int    `json:"port" description:"The port number of the exporter (default 9100)"`
	Path    string `json:"path" description:"The exporter path (default /metrics)"`
	Metrics string `json:"metrics" description:"Comma-separated list of metric names to retrieve (optional)"`
}

// ServerConfig holds the server configuration
type ServerConfig struct {
	HTTPTimeout time.Duration
	RetryCount  int
}

// DefaultConfig returns default server configuration
func DefaultConfig() *ServerConfig {
	return &ServerConfig{
		HTTPTimeout: 10 * time.Second,
		RetryCount:  3,
	}
}

// queryExporter queries an exporter endpoint and returns the response
func queryExporter(ctx context.Context, config *ServerConfig, host string, port int, path string) (string, error) {
	if port == 0 {
		port = 9100
	}
	if path == "" {
		path = "/metrics"
	}

	baseURL := fmt.Sprintf("http://%s:%d%s", host, port, path)

	client := &http.Client{
		Timeout: config.HTTPTimeout,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to query exporter: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("exporter returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(body), nil
}

// handleGetServerInfo handles the get_server_info tool request
func handleGetServerInfo(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	var input QueryInput
	if err := protocol.VerifyAndUnmarshal(req.RawArguments, &input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	config := DefaultConfig()
	resp, err := queryExporter(ctx, config, input.Host, input.Port, input.Path)
	if err != nil {
		return &protocol.CallToolResult{
			Content: []protocol.Content{
				&protocol.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error querying exporter: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Parse and extract useful server info
	var infoBuilder strings.Builder
	infoBuilder.WriteString(fmt.Sprintf("Server: %s:%d\n", input.Host, input.Port))
	infoBuilder.WriteString(fmt.Sprintf("Path: %s\n\n", input.Path))

	lines := strings.Split(resp, "\n")
	var metrics []string

	for _, line := range lines {
		if strings.HasPrefix(line, "#") {
			continue
		}
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 2 {
			metricName := parts[0]
			if strings.Contains(metricName, "{") {
				metricName = strings.Split(metricName, "{")[0]
			}
			metrics = append(metrics, metricName)
		}
	}

	// Remove duplicates
	seen := make(map[string]bool)
	for _, m := range metrics {
		if !seen[m] {
			seen[m] = true
		}
	}

	infoBuilder.WriteString(fmt.Sprintf("Found %d unique metrics:\n", len(seen)))
	count := 0
	for m := range seen {
		count++
		if count <= 50 {
			infoBuilder.WriteString(fmt.Sprintf("  - %s\n", m))
		}
	}
	if len(seen) > 50 {
		infoBuilder.WriteString(fmt.Sprintf("  ... and %d more\n", len(seen)-50))
	}

	return &protocol.CallToolResult{
		Content: []protocol.Content{
			&protocol.TextContent{
				Type: "text",
				Text: infoBuilder.String(),
			},
		},
	}, nil
}

// handleGetMetrics handles the get_metrics tool request
func handleGetMetrics(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	var input QueryInput
	if err := protocol.VerifyAndUnmarshal(req.RawArguments, &input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	config := DefaultConfig()
	resp, err := queryExporter(ctx, config, input.Host, input.Port, input.Path)
	if err != nil {
		return &protocol.CallToolResult{
			Content: []protocol.Content{
				&protocol.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error querying exporter: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	lines := strings.Split(resp, "\n")
	var filteredLines []string

	// Parse requested metrics filter
	var requestedMetrics map[string]bool
	if input.Metrics != "" {
		requestedMetrics = make(map[string]bool)
		for _, m := range strings.Split(input.Metrics, ",") {
			requestedMetrics[strings.TrimSpace(m)] = true
		}
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "#") {
			continue
		}
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 2 {
			metricName := parts[0]
			if strings.Contains(metricName, "{") {
				metricName = strings.Split(metricName, "{")[0]
			}

			// Filter by requested metrics if specified
			if requestedMetrics != nil {
				include := false
				for reqM := range requestedMetrics {
					if strings.Contains(metricName, reqM) {
						include = true
						break
					}
				}
				if include {
					filteredLines = append(filteredLines, line)
				}
			} else {
				filteredLines = append(filteredLines, line)
			}
		}
	}

	var resultBuilder strings.Builder
	resultBuilder.WriteString(fmt.Sprintf("Metrics from %s:%d%s:\n\n", input.Host, input.Port, input.Path))

	if input.Metrics != "" {
		resultBuilder.WriteString(fmt.Sprintf("Filtered by: %s\n\n", input.Metrics))
	}

	resultBuilder.WriteString(fmt.Sprintf("Found %d metric lines:\n\n", len(filteredLines)))
	resultBuilder.WriteString(strings.Join(filteredLines, "\n"))

	return &protocol.CallToolResult{
		Content: []protocol.Content{
			&protocol.TextContent{
				Type: "text",
				Text: resultBuilder.String(),
			},
		},
	}, nil
}

func main() {
	// Create Stdio transport server (for local MCP clients like Claude/Cline)
	transportServer := transport.NewStdioServerTransport()

	// Initialize MCP server
	mcpServer, err := server.NewServer(transportServer)
	if err != nil {
		log.Fatalf("Failed to create MCP server: %v", err)
	}

	// Create tools
	getServerInfoTool, err := protocol.NewTool(
		"get_server_info",
		"Query node_exporter or other Prometheus exporters to get server information. Returns list of available metrics with their current values.",
		QueryInput{},
	)
	if err != nil {
		log.Fatalf("Failed to create get_server_info tool: %v", err)
	}

	getMetricsTool, err := protocol.NewTool(
		"get_metrics",
		"Query specific metrics from a Prometheus exporter by filtering for metric names. Returns filtered metric lines.",
		QueryInput{},
	)
	if err != nil {
		log.Fatalf("Failed to create get_metrics tool: %v", err)
	}

	// Register tools
	mcpServer.RegisterTool(getServerInfoTool, handleGetServerInfo)
	mcpServer.RegisterTool(getMetricsTool, handleGetMetrics)

	// Start server
	log.Println("Starting infra-mcp server...")
	if err = mcpServer.Run(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
