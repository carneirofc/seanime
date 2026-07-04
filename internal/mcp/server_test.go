package mcp

import (
	"context"
	"seanime/internal/core"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog"
)

// connectTestClient wires an in-memory client to a freshly built server.
func connectTestClient(t *testing.T) (*mcp.ClientSession, context.Context) {
	t.Helper()
	logger := zerolog.Nop()
	app := &core.App{Logger: &logger, Version: "test"}
	srv := NewServer(app).newMCPServer()

	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	ctx := context.Background()
	if _, err := srv.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session, ctx
}

func TestServerBuildsAndInitializes(t *testing.T) {
	session, _ := connectTestClient(t)
	if session == nil {
		t.Fatal("nil session")
	}
}

func TestToolsListed(t *testing.T) {
	session, ctx := connectTestClient(t)

	res, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	want := map[string]bool{
		"search_anime":         false,
		"search_manga":         false,
		"get_anime":            false,
		"get_anime_details":    false,
		"get_anime_collection": false,
		"get_viewer_stats":     false,
		"get_library_files":    false,
	}
	for _, tool := range res.Tools {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
		if tool.Description == "" {
			t.Errorf("tool %q missing description", tool.Name)
		}
		if tool.InputSchema == nil {
			t.Errorf("tool %q missing input schema", tool.Name)
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("expected tool %q to be registered", name)
		}
	}
}

// TestSearchAnimeRejectsMissingQuery verifies a call without the required
// "query" is reported as a tool error (MCP reports tool failures via the
// result's IsError flag, not as a transport-level error).
func TestSearchAnimeRejectsMissingQuery(t *testing.T) {
	session, ctx := connectTestClient(t)

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "search_anime",
		Arguments: map[string]any{}, // missing required "query"
	})
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected tool error result for missing query")
	}
}
