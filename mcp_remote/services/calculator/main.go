package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	mcpServer := server.NewMCPServer("hook-quickstart-calculator", "1.0.0", server.WithToolCapabilities(false))

	mcpServer.AddTool(
		mcp.NewTool(
			"add",
			mcp.WithDescription("两个数字相加"),
			mcp.WithNumber("a", mcp.Required(), mcp.Description("第一个数字")),
			mcp.WithNumber("b", mcp.Required(), mcp.Description("第二个数字")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			a, err := req.RequireFloat("a")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			b, err := req.RequireFloat("b")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("%g", a+b)), nil
		},
	)

	mcpServer.AddTool(
		mcp.NewTool(
			"multiply",
			mcp.WithDescription("两个数字相乘"),
			mcp.WithNumber("a", mcp.Required(), mcp.Description("第一个数字")),
			mcp.WithNumber("b", mcp.Required(), mcp.Description("第二个数字")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			a, err := req.RequireFloat("a")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			b, err := req.RequireFloat("b")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("%g", a*b)), nil
		},
	)

	port := os.Getenv("FC_SERVER_PORT")
	if port == "" {
		port = "9000"
	}

	httpServer := server.NewStreamableHTTPServer(mcpServer)
	slog.Info("calculator 服务启动", "port", port)
	if err := httpServer.Start(":" + port); err != nil {
		slog.Error("calculator 服务异常", "error", err)
		os.Exit(1)
	}
}
