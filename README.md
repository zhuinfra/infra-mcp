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

### Run the server

```bash
./infra-mcp
```

The server runs over stdin/stdout and communicates using the MCP protocol.

## Usage

### get_server_info

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

### get_metrics

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

### For Claude Desktop

Add to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "infra-mcp": {
      "command": "/path/to/infra-mcp",
      "args": []
    }
  }
}
```

### For Cline/VS Code

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
