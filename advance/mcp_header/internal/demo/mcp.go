package demo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	debugHeadersToolName     = "debug_headers"
	debugRequestHeaderKey    = "X-Debug-Request-Header"
	debugRequestHeaderValue  = "pre-call-tool-upstream"
	debugResponseHeaderKey   = "X-Debug-Response-Header"
	debugResponseHeaderValue = "post-call-tool-client"
	debugClientHeaderValue   = "client-before-hook"
	debugToolCallBody        = `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"debug_headers","arguments":{}}}`
)

// rawMCPResponse 表示原始 MCP HTTP 调用结果。
type rawMCPResponse struct {
	StatusCode int
	Header     http.Header
	Body       string
}

// verifyHeaderEffects 重试验证数据面上的 header hook 效果。
// 参数 maxRetries 是最大重试次数。
func (d *demoContext) verifyHeaderEffects(maxRetries int) (headerObservation, string, error) {
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

		obs, err := verifyDebugHeaders(d.ctx, mcpURL, product, d.ak, d.sk)
		if err != nil {
			lastErr = err
			slog.Warn("验证 header hook 失败", "attempt", attempt, "error", err)
			continue
		}
		return obs, mcpURL, nil
	}
	return headerObservation{}, mcpURL, lastErr
}

// verifyDebugHeaders 验证 PRE 能改写上游请求头且 POST 能改写客户端响应头。
// 参数 ctx 是调用上下文；参数 mcpURL 是数据面 MCP 地址；参数 product 是签名产品名；参数 ak 是 AccessKey ID；参数 sk 是 AccessKey Secret。
func verifyDebugHeaders(ctx context.Context, mcpURL, product, ak, sk string) (headerObservation, error) {
	initResp, err := sendRawMCPRequest(ctx, mcpURL, product, ak, sk, initializeBody, nil)
	if err != nil {
		return headerObservation{}, fmt.Errorf("initialize 失败: %w", err)
	}

	extraHeaders := map[string]string{
		"Mcp-Protocol-Version": mcp.LATEST_PROTOCOL_VERSION,
		debugRequestHeaderKey:  debugClientHeaderValue,
	}
	if sessionID := initResp.Header.Get("Mcp-Session-Id"); sessionID != "" {
		extraHeaders["Mcp-Session-Id"] = sessionID
	}

	callResp, err := sendRawMCPRequest(ctx, mcpURL, product, ak, sk, debugToolCallBody, extraHeaders)
	if err != nil {
		return headerObservation{}, fmt.Errorf("debug_headers 调用失败: %w", err)
	}

	requestHeader, err := extractDebugRequestHeader(callResp.Body)
	if err != nil {
		return headerObservation{}, err
	}
	if requestHeader != debugRequestHeaderValue {
		return headerObservation{}, fmt.Errorf("PRE_CALL_TOOL 未改写上游请求头: got=%q want=%q", requestHeader, debugRequestHeaderValue)
	}

	responseHeader := callResp.Header.Get(debugResponseHeaderKey)
	if responseHeader != debugResponseHeaderValue {
		return headerObservation{}, fmt.Errorf("POST_CALL_TOOL 未改写客户端响应头: got=%q want=%q", responseHeader, debugResponseHeaderValue)
	}

	return headerObservation{
		DebugRequestHeader:  requestHeader,
		DebugResponseHeader: responseHeader,
		RawBody:             callResp.Body,
	}, nil
}

// sendRawMCPRequest 发送原始 MCP HTTP 请求并保留响应头。
// 参数 ctx 是调用上下文；参数 mcpURL 是数据面 MCP 地址；参数 product 是签名产品名；参数 ak 是 AccessKey ID；参数 sk 是 AccessKey Secret；参数 body 是 JSON-RPC 请求体；参数 extraHeaders 是额外请求头。
func sendRawMCPRequest(ctx context.Context, mcpURL, product, ak, sk, body string, extraHeaders map[string]string) (rawMCPResponse, error) {
	headers := signRequest(mcpURL, http.MethodPost, ak, sk, region, product)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mcpURL, strings.NewReader(body))
	if err != nil {
		return rawMCPResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	for key, value := range extraHeaders {
		if value != "" {
			req.Header.Set(key, value)
		}
	}

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return rawMCPResponse{}, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	rawResp := rawMCPResponse{StatusCode: resp.StatusCode, Header: resp.Header.Clone(), Body: string(respBody)}
	if resp.StatusCode >= 400 {
		return rawResp, fmt.Errorf("数据面返回 %d: %s", resp.StatusCode, rawResp.Body)
	}
	return rawResp, nil
}

// extractDebugRequestHeader 从 debug_headers 的 JSON-RPC 响应中读取上游请求头值。
// 参数 body 是原始 JSON-RPC 或 SSE 响应体。
func extractDebugRequestHeader(body string) (string, error) {
	payload := jsonRPCPayload(body)
	var rpc struct {
		Result struct {
			StructuredContent map[string]any `json:"structuredContent"`
			Content           []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(payload), &rpc); err != nil {
		return "", fmt.Errorf("解析 debug_headers 响应失败: %w body=%s", err, body)
	}
	if value, ok := rpc.Result.StructuredContent["header_value"].(string); ok {
		return value, nil
	}
	for _, item := range rpc.Result.Content {
		var parsed struct {
			HeaderValue string `json:"header_value"`
		}
		if json.Unmarshal([]byte(item.Text), &parsed) == nil && parsed.HeaderValue != "" {
			return parsed.HeaderValue, nil
		}
	}
	return "", fmt.Errorf("debug_headers 响应缺少 header_value: %s", body)
}

// jsonRPCPayload 从普通 JSON 或 SSE 响应中提取 JSON-RPC 载荷。
// 参数 body 是原始响应体。
func jsonRPCPayload(body string) string {
	payload := ""
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if data, ok := strings.CutPrefix(line, "data:"); ok {
			payload = strings.TrimSpace(data)
		}
	}
	if payload != "" {
		return payload
	}
	return body
}

// probeDataPlane 探测数据面 MCP 地址是否可用。
// 参数 ctx 是调用上下文；参数 mcpURL 是数据面 MCP 地址；参数 ak 是 AccessKey ID；参数 sk 是 AccessKey Secret；参数 region 是地域；参数 product 是签名产品名。
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
