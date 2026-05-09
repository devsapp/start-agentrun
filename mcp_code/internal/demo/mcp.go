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

		client, closeClient, err := connectMCP(d.ctx, mcpURL, product, d.ak, d.sk)
		if err != nil {
			lastErr = err
			slog.Warn("连接 MCP 失败", "attempt", attempt, "error", err)
			continue
		}

		obs, err := collectHookObservation(d.ctx, client)
		closeClient()
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
	orderToolFound := false
	for _, tool := range toolsResult.Tools {
		obs.toolNames = append(obs.toolNames, tool.Name)
		if tool.Name == "get_order" {
			orderToolFound = true
		}
	}
	if !orderToolFound {
		return obs, fmt.Errorf("tools/list 未出现 get_order: %v", obs.toolNames)
	}

	orderResult, err := client.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "get_order",
			Arguments: map[string]any{"order_id": "ORDER-1001"},
		},
	})
	if err != nil {
		return obs, fmt.Errorf("get_order 调用失败: %w", err)
	}
	obs.orderResult = extractText(orderResult)
	obs.orderMasked = strings.Contains(obs.orderResult, "138****5678") &&
		strings.Contains(obs.orderResult, "zh***@example.com") &&
		strings.Contains(obs.orderResult, "浙江省杭州市****") &&
		!strings.Contains(obs.orderResult, "13812345678") &&
		!strings.Contains(obs.orderResult, "zhangsan@example.com") &&
		!strings.Contains(obs.orderResult, "浙江省杭州市西湖区")
	obs.auditIDFound = strings.Contains(obs.orderResult, `"audit_id":"audit_`)
	if !obs.orderMasked || !obs.auditIDFound {
		return obs, fmt.Errorf("get_order 结果未完成脱敏或缺少 audit_id，实际结果: %q", obs.orderResult)
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
			ClientInfo:      mcp.Implementation{Name: "hook-quickstart-code", Version: "1.0.0"},
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
