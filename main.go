package main

import (
	"fmt"
	"os"

	"github.com/e-roux/mcp-git-ops/internal/gitops"
	"github.com/mark3labs/mcp-go/server"
)

const serverVersion = "0.3.0"

func main() {
	mcpServer := server.NewMCPServer(
		"git-ops",
		serverVersion,
	)

	gitops.RegisterAllTools(mcpServer)

	if err := server.ServeStdio(mcpServer); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %s\n", err)
		os.Exit(1)
	}
}
