# infra-mcp

A Model Context Protocol (MCP) server for infrastructure operations, designed to query Prometheus exporters (like node_exporter) to retrieve server information and metrics.

## Features

- **get_server_info**: Query node_exporter to get server information including CPU, memory, load, disk, and network metrics
- **get_metrics**: Query specific metrics from Prometheus exporters by filtering for metric names

## Installation

### Prerequisites

- Go 1.21 or later

### Build from source

```bash
git clone https://github.com/zhuinfra/infra-mcp.git
cd infra-mcp
go build -o infra-mcp .
```

## Usage

### Server Modes

The server supports three transport modes:

#### Stdio Mode (Default)

For local MCP clients like Claude Desktop or Cline:

```bash
./infra-mcp
```

#### SSE Mode

For HTTP-based MCP clients with Server-Sent Events:

```bash
./infra-mcp --mode sse --port 8080
```

#### Streamable HTTP Mode

For modern HTTP-based MCP clients:

```bash
./infra-mcp --mode streamable-http --port 8080
```

### Tool Input

#### get_server_info

Query server information from a node_exporter endpoint.

**Input:**
- `host` (required): The IP address or hostname of the server
- `port` (optional): The port number of the exporter (default: 9100)
- `path` (optional): The exporter path (default: /metrics)

**Example:**
```json
{
  "host": "192.168.1.100",
  "port": 9100
}
```

#### get_metrics

Query specific metrics from a Prometheus exporter.

**Input:**
- `host` (required): The IP address or hostname of the server
- `port` (optional): The port number of the exporter (default: 9100)
- `path` (optional): The exporter path (default: /metrics)
- `metrics` (optional): Comma-separated list of metric names to retrieve

**Example:**
```json
{
  "host": "192.168.1.100",
  "port": 9100,
  "metrics": "cpu,memory,load"
}
```

## Configuration

### For Cline/VS Code (Stdio Mode)

Add to your `cline_mcp_settings.json`:

```json
{
  "mcpServers": {
    "infra-mcp": {
      "command": "/path/to/infra-mcp"
    }
  }
}
```

### For HTTP Clients (SSE or Streamable HTTP)

Start the server in SSE or streamable-http mode, then configure your client:

```json
{
  "mcpServers": {
    "infra-mcp": {
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Claude/Cline   │────▶│   infra-mcp     │────▶│ node_exporter   │
│                 │     │                 │     │ (port 9100)    │
│  MCP Client     │◀────│  MCP Server     │◀────│                 │
└─────────────────┘     └─────────────────┘     └─────────────────┘
```

## License

MIT
