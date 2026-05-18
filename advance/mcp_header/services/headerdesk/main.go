package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	defaultPort           = "9000"
	debugHeadersToolName  = "debug_headers"
	debugRequestHeaderKey = "X-Debug-Request-Header"
)

// headerRecord 表示调试工具观测到的请求头。
type headerRecord struct {
	HeaderName  string `json:"header_name"`
	HeaderValue string `json:"header_value"`
}

// main 启动 header 调试 MCP 服务。
func main() {
	mcpServer := newHeaderServer()

	port := os.Getenv("FC_SERVER_PORT")
	if port == "" {
		port = defaultPort
	}

	httpServer := server.NewStreamableHTTPServer(mcpServer)
	slog.Info("headerdesk 服务启动", "port", port)
	if err := httpServer.Start(":" + port); err != nil {
		slog.Error("headerdesk 服务异常", "error", err)
		os.Exit(1)
	}
}

// newHeaderServer 创建只包含 header 调试工具的 MCP 服务。
func newHeaderServer() *server.MCPServer {
	mcpServer := server.NewMCPServer("hook-quickstart-headerdesk", "1.0.0", server.WithToolCapabilities(false))
	mcpServer.AddTool(
		mcp.NewTool(
			debugHeadersToolName,
			mcp.WithDescription("返回 MCP 工具实际收到的调试请求头"),
		),
		handleDebugHeaders,
	)
	return mcpServer
}

// handleDebugHeaders 返回当前工具调用实际收到的调试请求头。
// 参数 ctx 是调用上下文；参数 req 是 MCP 工具调用请求。
func handleDebugHeaders(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	result := headerRecord{
		HeaderName:  debugRequestHeaderKey,
		HeaderValue: req.Header.Get(debugRequestHeaderKey),
	}
	return mcp.NewToolResultJSON(result)
}
