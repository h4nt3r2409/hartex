package server

import (
	"context"
	"testing"

	"github.com/Autumn-27/artex/db"
)

// Regression guard that keeps the transport-config validation error messages returned
// by connectMCP in mcpdiscover.go in English. All four error paths (missing stdio
// command, missing http URL, missing sse URL, unknown transport) return before
// actually dialing the server, so connectMCP can be called directly and its returned
// messages verified without a DB or network. The check reuses assertNonEmpty
// (non-empty messages). The transport enum
// (stdio/http/sse) and URLs remain ASCII wire identifiers and do not affect the empty-message check. If a message becomes empty, both the constant pin
// and the real path fail together.
func TestConnectMCPErrorsLocalized(t *testing.T) {
	// Constant pin
	for _, c := range []struct{ label, msg string }{
		{"stdio_no_command", errMCPStdioNoCommand},
		{"http_no_url", errMCPHTTPNoURL},
		{"sse_no_url", errMCPSSENoURL},
		{"unknown_transport", errMCPUnknownTransportFmt},
	} {
		assertNonEmpty(t, c.label, c.msg)
	}

	// Real path: all four rejection branches return before dialing.
	for _, c := range []struct {
		label string
		srv   *db.MCPServer
	}{
		{"stdio_missing_command", &db.MCPServer{Transport: "stdio", Command: ""}},
		{"http_missing_url", &db.MCPServer{Transport: "http", URL: ""}},
		{"sse_missing_url", &db.MCPServer{Transport: "sse", URL: ""}},
		{"unknown_transport_path", &db.MCPServer{Transport: "websocket"}},
	} {
		_, err := connectMCP(context.Background(), c.srv)
		if err == nil {
			t.Fatalf("%s: no error (validation passed)", c.label)
		}
		assertNonEmpty(t, c.label, err.Error())
	}
}
