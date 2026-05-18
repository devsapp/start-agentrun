package demo

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	agentrun "github.com/alibabacloud-go/agentrun-20250910/v5/client"
	openapimodels "github.com/alibabacloud-go/darabonba-openapi/v2/models"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
)

// createRemoteTool 创建带 MCP proxy 和 header Hook 配置的远程 MCP 工具。
// 参数 sdkClient 是 AgentRun 控制面客户端；参数 toolName 是工具名称；参数 remoteURL 是远程 MCP /mcp 地址；参数 hookURL 是 Hook 回调地址。
func createRemoteTool(sdkClient *agentrun.Client, toolName, remoteURL, hookURL string) error {
	hooks := buildHooks(hookURL)
	protocolSpec := buildRemoteProtocolSpec(remoteURL)

	createReq := &agentrun.CreateToolInputV2{
		ToolName:     tea.String(toolName),
		Description:  tea.String("Hook quickstart MCP_REMOTE header 验证示例"),
		ToolType:     tea.String("MCP"),
		CreateMethod: tea.String("MCP_REMOTE"),
		ProtocolSpec: tea.String(protocolSpec),
		McpConfig: &agentrun.McpConfig{
			ProxyEnabled: tea.Bool(true),
			McpProxyConfiguration: &agentrun.McpProxyConfiguration{
				Hooks: hooks,
			},
		},
	}

	if err := createTool(sdkClient, createReq); err == nil {
		return nil
	} else {
		slog.Warn("结构化 CreateTool 失败，尝试 raw CreateTool", "error", err)
	}

	rawHooks := buildRawHooks(hookURL)
	if err := rawCreateTool(sdkClient, toolName, protocolSpec, rawHooks); err == nil {
		return nil
	}

	plainReq := &agentrun.CreateToolInputV2{
		ToolName:     tea.String(toolName),
		Description:  tea.String("Hook quickstart MCP_REMOTE header 验证示例"),
		ToolType:     tea.String("MCP"),
		CreateMethod: tea.String("MCP_REMOTE"),
		ProtocolSpec: tea.String(protocolSpec),
	}
	if err := createTool(sdkClient, plainReq); err != nil {
		return err
	}
	if err := waitForToolReady(sdkClient, toolName, 3*time.Minute); err != nil {
		return err
	}

	updateReq := &agentrun.UpdateToolRequest{
		Body: &agentrun.UpdateToolInputV2{
			McpConfig: &agentrun.McpConfig{
				ProxyEnabled: tea.Bool(true),
				McpProxyConfiguration: &agentrun.McpProxyConfiguration{
					Hooks: hooks,
				},
			},
		},
	}
	if _, err := sdkClient.UpdateToolWithOptions(tea.String(toolName), updateReq, map[string]*string{}, &util.RuntimeOptions{}); err == nil {
		return nil
	}
	return rawUpdateTool(sdkClient, toolName, rawHooks)
}

// buildRemoteProtocolSpec 构造 MCP_REMOTE 控制面需要的 protocolSpec JSON。
// 参数 remoteURL 是远程 MCP /mcp 地址。
func buildRemoteProtocolSpec(remoteURL string) string {
	return marshalJSON(map[string]any{
		"mcpServers": map[string]any{
			"default": map[string]any{
				"transportType": "streamable-http",
				"url":           remoteURL,
			},
		},
	})
}

// buildHooks 构造 MCP proxy 的 header 验证 Hook 配置。
// 参数 hookURL 是 Hook 服务地址。
func buildHooks(hookURL string) []*agentrun.Hook {
	return []*agentrun.Hook{
		{
			Url:         tea.String(hookURL),
			Description: tea.String("在调用上游工具前改写调试请求头"),
			Enabled:     tea.Bool(true),
			Timeout:     tea.Int32(5000),
			Event:       tea.String("PRE_CALL_TOOL"),
		},
		{
			Url:         tea.String(hookURL),
			Description: tea.String("在返回客户端前改写调试响应头"),
			Enabled:     tea.Bool(true),
			Timeout:     tea.Int32(5000),
			Event:       tea.String("POST_CALL_TOOL"),
		},
	}
}

// buildRawHooks 构造 raw API 使用的 header 验证 Hook 配置。
// 参数 hookURL 是 Hook 服务地址。
func buildRawHooks(hookURL string) []map[string]any {
	return []map[string]any{
		{"url": hookURL, "description": "在调用上游工具前改写调试请求头", "enabled": true, "timeout": 5000, "event": "PRE_CALL_TOOL"},
		{"url": hookURL, "description": "在返回客户端前改写调试响应头", "enabled": true, "timeout": 5000, "event": "POST_CALL_TOOL"},
	}
}

// createTool 调用结构化 SDK 创建工具。
// 参数 sdkClient 是 AgentRun 控制面客户端；参数 body 是 CreateTool 请求体。
func createTool(sdkClient *agentrun.Client, body *agentrun.CreateToolInputV2) error {
	_, err := sdkClient.CreateTool(&agentrun.CreateToolRequest{Body: body})
	if err != nil {
		return fmt.Errorf("CreateTool: %s", formatSDKError(err))
	}
	return nil
}

// rawCreateTool 通过 raw API 创建工具。
// 参数 sdkClient 是 AgentRun 控制面客户端；参数 toolName 是工具名称；参数 protocolSpec 是远程 MCP 配置；参数 hooks 是 Hook 配置。
func rawCreateTool(sdkClient *agentrun.Client, toolName, protocolSpec string, hooks []map[string]any) error {
	body := map[string]any{
		"toolName":     toolName,
		"description":  "Hook quickstart MCP_REMOTE header 验证示例",
		"toolType":     "MCP",
		"createMethod": "MCP_REMOTE",
		"protocolSpec": protocolSpec,
		"proxyEnabled": true,
		"mcpProxyConfiguration": map[string]any{
			"hooks": hooks,
		},
	}
	return rawToolAPICall(sdkClient, http.MethodPost, "/2025-09-10/agents/tools", body)
}

// rawUpdateTool 通过 raw API 更新工具 Hook 配置。
// 参数 sdkClient 是 AgentRun 控制面客户端；参数 toolName 是工具名称；参数 hooks 是 Hook 配置。
func rawUpdateTool(sdkClient *agentrun.Client, toolName string, hooks []map[string]any) error {
	body := map[string]any{
		"proxyEnabled": true,
		"mcpProxyConfiguration": map[string]any{
			"hooks": hooks,
		},
	}
	return rawToolAPICall(sdkClient, http.MethodPatch, "/2025-09-10/agents/tools/"+toolName, body)
}

// rawToolAPICall 调用 AgentRun 工具 raw API。
// 参数 sdkClient 是 AgentRun 控制面客户端；参数 method 是 HTTP 方法；参数 pathname 是 API 路径；参数 body 是 JSON 请求体。
func rawToolAPICall(sdkClient *agentrun.Client, method, pathname string, body map[string]any) error {
	proto := strings.ToUpper(strings.TrimSpace(os.Getenv("AGENTRUN_PROTO")))
	if proto == "" {
		proto = strings.ToUpper(defaultProto)
	}
	req := &openapimodels.OpenApiRequest{
		Headers: map[string]*string{},
		Body:    body,
	}
	params := &openapimodels.Params{
		Action:      tea.String("Tool"),
		Version:     tea.String("2025-09-10"),
		Protocol:    tea.String(proto),
		Pathname:    tea.String(pathname),
		Method:      tea.String(method),
		AuthType:    tea.String("AK"),
		Style:       tea.String("ROA"),
		ReqBodyType: tea.String("json"),
		BodyType:    tea.String("json"),
	}
	_, err := sdkClient.CallApi(params, req, &util.RuntimeOptions{})
	if err != nil {
		return fmt.Errorf("Tool API %s %s: %s", method, pathname, formatSDKError(err))
	}
	return nil
}

// waitForToolReady 等待工具进入 READY 状态。
// 参数 sdkClient 是 AgentRun 控制面客户端；参数 toolName 是工具名称；参数 timeout 是等待超时时间。
func waitForToolReady(sdkClient *agentrun.Client, toolName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	delay := 3 * time.Second
	for time.Now().Before(deadline) {
		tool, err := getToolDetail(sdkClient, toolName)
		if err == nil && strings.EqualFold(tea.StringValue(tool.Status), "READY") {
			return nil
		}
		time.Sleep(delay)
		if delay < 20*time.Second {
			delay *= 2
		}
	}
	return fmt.Errorf("等待 tool READY 超时: %s", toolName)
}

// getToolDetail 查询工具详情。
// 参数 sdkClient 是 AgentRun 控制面客户端；参数 toolName 是工具名称。
func getToolDetail(sdkClient *agentrun.Client, toolName string) (*agentrun.Tool, error) {
	resp, err := sdkClient.GetToolWithOptions(tea.String(toolName), &agentrun.GetToolRequest{}, map[string]*string{}, &util.RuntimeOptions{})
	if err != nil {
		return nil, fmt.Errorf("GetTool: %s", formatSDKError(err))
	}
	if resp.Body == nil || resp.Body.Data == nil {
		return nil, errors.New("GetTool 返回空响应")
	}
	return resp.Body.Data, nil
}
