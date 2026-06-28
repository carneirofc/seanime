package mcp

import (
	"encoding/json"
	"seanime/internal/platforms/platform"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultPerPage = 10
	maxPerPage     = 50
)

// platform returns the active AniList platform.
func (s *Server) platform() platform.Platform {
	return s.app.AnilistPlatformRef.Get()
}

func clampPerPage(n int) int {
	if n <= 0 {
		return defaultPerPage
	}
	if n > maxPerPage {
		return maxPerPage
	}
	return n
}

// jsonResult marshals v to indented JSON and wraps it as a text tool result.
func jsonResult(v any) (*mcp.CallToolResult, any, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}, nil, nil
}

func ptr[T any](v T) *T { return &v }

func deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

// derefEnum dereferences a string-backed enum pointer (MediaFormat, MediaStatus,
// MediaListStatus, ...) to its string value, or "" when nil.
func derefEnum[T ~string](p *T) string {
	if p == nil {
		return ""
	}
	return string(*p)
}
