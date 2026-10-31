// Command agentsync-cli is the headless AgentSync entry point: a CLI for people and scripts and,
// with "agentsync-cli mcp", a stdio MCP server for AI agents. It shares internal/service with the GUI.
package main

import (
	"os"

	"agentsync/internal/cli"
)

func main() {
	os.Exit(cli.Main())
}
