package mcp

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/ellistarn/shade/internal/shade"
	"github.com/ellistarn/shade/internal/source"
)

// NewServer creates an MCP server backed by the given Shade instance.
func NewServer(s *shade.Shade) *server.MCPServer {
	srv := server.NewMCPServer("shade", "0.1.0", server.WithToolCapabilities(false))

	srv.AddTool(
		mcp.NewTool("upload",
			mcp.WithDescription("Sync local agent conversations to S3. Scans OpenCode and Claude Code conversation history, compares against what's already backed up, and uploads new or updated sessions."),
		),
		uploadHandler(s),
	)

	srv.AddTool(
		mcp.NewTool("ls",
			mcp.WithDescription("List all backed-up conversation sessions in S3."),
			mcp.WithString("source", mcp.Description("Filter by source: opencode or claude-code")),
		),
		lsHandler(s),
	)

	srv.AddTool(
		mcp.NewTool("show",
			mcp.WithDescription("Display a specific backed-up conversation session."),
			mcp.WithString("session_id", mcp.Required(), mcp.Description("The session ID to display")),
			mcp.WithString("source", mcp.Description("Filter by source: opencode or claude-code")),
		),
		showHandler(s),
	)

	return srv
}

func uploadHandler(s *shade.Shade) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, err := s.Upload(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to upload: %w", err)
		}
		var b strings.Builder
		for _, w := range result.Warnings {
			fmt.Fprintf(&b, "warning: %s\n", w)
		}
		fmt.Fprintf(&b, "Backed up %d sessions (%d new/updated, %d unchanged)", result.Total, result.Uploaded, result.Skipped)
		return mcp.NewToolResultText(b.String()), nil
	}
}

func lsHandler(s *shade.Shade) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		entries, err := s.Ls(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list sessions: %w", err)
		}
		srcFilter := req.GetString("source", "")

		var b strings.Builder
		w := tabwriter.NewWriter(&b, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "SOURCE\tSESSION ID\tLAST MODIFIED")
		for _, e := range entries {
			if srcFilter != "" && e.Source != srcFilter {
				continue
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", e.Source, e.SessionID, e.LastModified.Format("2006-01-02 15:04"))
		}
		w.Flush()
		return mcp.NewToolResultText(b.String()), nil
	}
}

func showHandler(s *shade.Shade) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sessionID, err := req.RequireString("session_id")
		if err != nil {
			return nil, err
		}
		src := req.GetString("source", "")

		session, err := s.Show(ctx, sessionID, src)
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResultText(formatSession(session)), nil
	}
}

func formatSession(s *source.Session) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", s.Title)
	fmt.Fprintf(&b, "Source:  %s\n", s.Source)
	fmt.Fprintf(&b, "Project: %s\n", s.Project)
	fmt.Fprintf(&b, "Created: %s\n", s.CreatedAt.Format("2006-01-02 15:04"))
	fmt.Fprintf(&b, "Updated: %s\n\n", s.UpdatedAt.Format("2006-01-02 15:04"))
	for _, m := range s.Messages {
		role := strings.ToUpper(m.Role)
		fmt.Fprintf(&b, "--- %s [%s] ---\n", role, m.Timestamp.Format("15:04:05"))
		if m.Content != "" {
			fmt.Fprintln(&b, m.Content)
		}
		for _, tc := range m.ToolCalls {
			fmt.Fprintf(&b, "[tool: %s]\n", tc.Name)
		}
		fmt.Fprintln(&b)
	}
	return b.String()
}
