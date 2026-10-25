package main

import (
	"context"
	"fmt"
	"os"

	"github.com/furaxdev/runyx/internal/mcp"
	"github.com/furaxdev/runyx/internal/websearch"
)

// cmdMCPServe exposes Runyx as an MCP tool server over stdio (item 10's
// "mode serveur"), so any MCP-aware application (an IDE, another
// agent...) can use Runyx's tools. Only tools safe to expose without a
// live approval UI are included here (web search); the shell agent and
// code interpreter are deliberately NOT exposed this way, since MCP's
// stdio transport has no notion of Runyx's own approval flow — exposing
// them here would mean "any MCP client can run arbitrary commands with
// no confirmation," which the project's security rules (§7) forbid.
func cmdMCPServe(args []string) error {
	srv := &mcp.Server{
		Tools: []mcp.ServerTool{
			{
				Name:        "web_search",
				Description: "Search the web via DuckDuckGo and return titled, cited results.",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query":       map[string]any{"type": "string"},
						"max_results": map[string]any{"type": "integer"},
					},
					"required": []string{"query"},
				},
				Call: func(ctx context.Context, callArgs map[string]any) (string, error) {
					query, _ := callArgs["query"].(string)
					maxResults := 5
					if mr, ok := callArgs["max_results"].(float64); ok {
						maxResults = int(mr)
					}
					client := &websearch.Client{}
					results, err := client.Search(ctx, query, maxResults)
					if err != nil {
						return "", err
					}
					return websearch.AsContextBlock(results), nil
				},
			},
		},
	}

	fmt.Fprintln(os.Stderr, "runyx: serving MCP tools over stdio (Ctrl+C or EOF on stdin to stop)")
	return srv.Serve(context.Background(), os.Stdout, os.Stdin)
}
