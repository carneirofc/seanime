// Package mcp exposes read-only Seanime data (AniList search, collection,
// media details, viewer stats) to AI clients via the Model Context Protocol.
//
// It is built on the official Go MCP SDK
// (github.com/modelcontextprotocol/go-sdk) and served over the Streamable HTTP
// transport. All tools are read-only.
package mcp

import (
	"net/http"
	"seanime/internal/core"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const serverName = "seanime"

// Server holds the dependencies the MCP tools need.
type Server struct {
	app *core.App
}

// NewServer creates an MCP server bound to the given app.
func NewServer(app *core.App) *Server {
	return &Server{app: app}
}

// newMCPServer builds the underlying SDK server with all tools registered.
func (s *Server) newMCPServer() *mcp.Server {
	impl := &mcp.Implementation{Name: serverName, Version: s.app.Version}
	srv := mcp.NewServer(impl, nil)
	s.registerTools(srv)
	return srv
}

// Handler returns the Streamable HTTP handler for the MCP server, suitable for
// mounting on an HTTP route. The SDK handler implements the full transport
// (POST for requests, GET for the optional SSE stream, DELETE for session
// termination).
func (s *Server) Handler() http.Handler {
	srv := s.newMCPServer()
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return srv
	}, nil)
}
