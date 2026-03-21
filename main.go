package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// QueryInput defines the input for querying an exporter
type QueryInput struct {
	Host    string `json:"host" jsonschema:"required,The IP address or hostname of the server"`
	Port    int    `json:"port" jsonschema:"The port number of the exporter (default 9100)"`
	Path    string `json:"path" jsonschema:"The exporter path (default /metrics)"`
	Metrics string `json:"metrics" jsonschema:"Comma-separated list of metric names to filter (optional)"`
}

// ServerConfig holds the server configuration
type ServerConfig struct {
	HTTPTimeout time.Duration
}

// DefaultConfig returns default server configuration
func DefaultConfig() *ServerConfig {
	return &ServerConfig{
		HTTPTimeout: 10 * time.Second,
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

// getServerInfo handles the get_server_info tool request
func getServerInfo(ctx context.Context, req *mcp.CallToolRequest, input QueryInput) (*mcp.CallToolResult, any, error) {
	config := DefaultConfig()
	resp, err := queryExporter(ctx, config, input.Host, input.Port, input.Path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error querying exporter: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	// Parse and extract useful server info
	var infoBuilder strings.Builder
	infoBuilder.WriteString(fmt.Sprintf("Server: %s:%d\n", input.Host, input.Port))
	if input.Path != "" {
		infoBuilder.WriteString(fmt.Sprintf("Path: %s\n\n", input.Path))
	} else {
		infoBuilder.WriteString(fmt.Sprintf("Path: /metrics\n\n", input.Path))
	}

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

	infoBuilder.WriteString(fmt.Sprintf("Found %d unique metrics:\n\n", len(seen)))
	count := 0
	for m := range seen {
		count++
		if count <= 100 {
			infoBuilder.WriteString(fmt.Sprintf("  - %s\n", m))
		}
	}
	if len(seen) > 100 {
		infoBuilder.WriteString(fmt.Sprintf("  ... and %d more\n", len(seen)-100))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: infoBuilder.String()},
		},
	}, nil, nil
}

// getMetrics handles the get_metrics tool request
func getMetrics(ctx context.Context, req *mcp.CallToolRequest, input QueryInput) (*mcp.CallToolResult, any, error) {
	config := DefaultConfig()
	resp, err := queryExporter(ctx, config, input.Host, input.Port, input.Path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error querying exporter: %v", err)},
			},
			IsError: true,
		}, nil, nil
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

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: resultBuilder.String()},
		},
	}, nil, nil
}

// createServer creates an MCP server with the tools
func createServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "infra-mcp",
		Version: "v0.1.0",
	}, nil)

	// Add the tools
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_server_info",
		Description: "Query node_exporter or other Prometheus exporters to get server information. Returns list of available metrics with their names.",
	}, getServerInfo)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_metrics",
		Description: "Query specific metrics from a Prometheus exporter by filtering for metric names. Returns filtered metric lines with values.",
	}, getMetrics)

	return server
}

// runStdio runs the MCP server over stdin/stdout
func runStdio() {
	server := createServer()
	log.Println("Starting infra-mcp server (stdio)...")
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

// runSSE runs the MCP server over HTTP with SSE
func runSSE(port string) {
	server := createServer()
	
	// Create SSE handler
	handler := mcp.NewSSEHandler(func(req *http.Request) *mcp.Server {
		return server
	}, nil)
	
	http.Handle("/mcp", handler)
	
	addr := ":" + port
	if port == "" {
		addr = ":8080"
	}
	
	log.Printf("Starting infra-mcp server (SSE) on http://localhost%s/mcp", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}

// runStreamableHTTP runs the MCP server with streamable HTTP transport
func runStreamableHTTP(port string) {
	server := createServer()
	
	// Create streamable HTTP handler
	handler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return server
	}, nil)
	
	http.Handle("/mcp", handler)
	
	addr := ":" + port
	if port == "" {
		addr = ":8080"
	}
	
	log.Printf("Starting infra-mcp server (streamable HTTP) on http://localhost%s/mcp", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}

func main() {
	// Define command line flags
	mode := flag.String("mode", "stdio", "Server mode: stdio, sse, or streamable-http")
	port := flag.String("port", "8080", "Port for HTTP server (used with sse or streamable-http mode)")
	flag.Parse()

	switch *mode {
	case "stdio":
		runStdio()
	case "sse":
		runSSE(*port)
	case "streamable-http":
		runStreamableHTTP(*port)
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s\n", *mode)
		fmt.Fprintf(os.Stderr, "Usage: infra-mcp --mode [stdio|sse|streamable-http] --port [port]\n")
		os.Exit(1)
	}
}
