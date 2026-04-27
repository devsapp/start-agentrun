package demo

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

func (d *demoContext) verifyHookEffects(maxRetries int) (hookObservation, string, error) {
	mcpURL, product := dataPlaneMCPURL(d.proto, d.dataEndpoint, d.toolName)
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			time.Sleep(15 * time.Second)
		}

		status, body := probeDataPlane(d.ctx, mcpURL, d.ak, d.sk, region, product)
		if status >= 400 {
			lastErr = fmt.Errorf("数据面返回 %d: %s", status, body)
			slog.Warn("数据面探测失败", "attempt", attempt, "status", status, "body", body)
			continue
		}

		client, cleanup, err := connectMCP(d.ctx, mcpURL, product, d.ak, d.sk)
		if err != nil {
			lastErr = err
			slog.Warn("连接 MCP 失败", "attempt", attempt, "error", err)
			continue
		}

		obs, err := collectHookObservation(d.ctx, client)
		cleanup()
		if err != nil {
			lastErr = err
			slog.Warn("验证 Hook 效果失败", "attempt", attempt, "error", err)
			continue
		}
		return obs, mcpURL, nil
	}
	return hookObservation{}, mcpURL, lastErr
}

func collectHookObservation(ctx context.Context, client *mcpclient.Client) (hookObservation, error) {
	var obs hookObservation

	toolsResult, err := client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return obs, fmt.Errorf("tools/list 失败: %w", err)
	}
	for _, tool := range toolsResult.Tools {
		obs.toolNames = append(obs.toolNames, tool.Name)
		if tool.Name == "hook_info" {
			obs.hookInfoFound = true
		}
	}
	if !obs.hookInfoFound {
		return obs, fmt.Errorf("tools/list 未出现 hook_info: %v", obs.toolNames)
	}

	multiplyResult, err := client.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "multiply",
			Arguments: map[string]any{"a": float64(2), "b": float64(3)},
		},
	})
	if err != nil {
		return obs, fmt.Errorf("multiply 调用失败: %w", err)
	}
	obs.multiplyResult = extractText(multiplyResult)
	obs.multiplyHooked = strings.Contains(obs.multiplyResult, "121")
	if !obs.multiplyHooked {
		return obs, fmt.Errorf("multiply 未被 hook 改写，实际结果: %q", obs.multiplyResult)
	}

	addResult, err := client.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "add",
			Arguments: map[string]any{"a": float64(1), "b": float64(2)},
		},
	})
	if err != nil {
		return obs, fmt.Errorf("add 调用失败: %w", err)
	}
	obs.addResult = extractText(addResult)
	obs.addHooked = strings.Contains(obs.addResult, "[hooked:200]")
	if !obs.addHooked {
		return obs, fmt.Errorf("add 未被 hook 改写，实际结果: %q", obs.addResult)
	}

	return obs, nil
}

func extractText(result *mcp.CallToolResult) string {
	for _, content := range result.Content {
		if text, ok := content.(mcp.TextContent); ok {
			return text.Text
		}
	}
	return ""
}

func connectMCP(ctx context.Context, mcpURL, product, ak, sk string) (*mcpclient.Client, func(), error) {
	tp, err := transport.NewStreamableHTTP(
		mcpURL,
		transport.WithHTTPHeaderFunc(func(context.Context) map[string]string {
			return signRequest(mcpURL, http.MethodPost, ak, sk, region, product)
		}),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("创建 MCP transport 失败: %w", err)
	}

	client := mcpclient.NewClient(tp)
	if err := client.Start(ctx); err != nil {
		return nil, nil, fmt.Errorf("启动 MCP client 失败: %w", err)
	}
	if _, err := client.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo:      mcp.Implementation{Name: "hook-quickstart-remote", Version: "1.0.0"},
		},
	}); err != nil {
		client.Close()
		return nil, nil, fmt.Errorf("初始化 MCP client 失败: %w", err)
	}
	return client, func() { client.Close() }, nil
}

func probeDataPlane(ctx context.Context, mcpURL, ak, sk, region, product string) (int, string) {
	headers := signRequest(mcpURL, http.MethodPost, ak, sk, region, product)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, mcpURL, strings.NewReader(initializeBody))
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}
