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

func createRemoteTool(sdkClient *agentrun.Client, toolName, remoteURL, hookURL string) error {
	hooks := buildHooks(hookURL)
	protocolSpec := marshalJSON(map[string]any{
		"mcpServers": map[string]any{
			"default": map[string]any{
				"url": remoteURL,
			},
		},
	})

	createReq := &agentrun.CreateToolInputV2{
		ToolName:     tea.String(toolName),
		Description:  tea.String("Hook quickstart MCP_REMOTE 订单查询示例"),
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
		Description:  tea.String("Hook quickstart MCP_REMOTE 订单查询示例"),
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

func buildHooks(hookURL string) []*agentrun.Hook {
	return []*agentrun.Hook{
		{
			Url:         tea.String(hookURL),
			Description: tea.String("脱敏订单结果并注入 audit_id"),
			Enabled:     tea.Bool(true),
			Timeout:     tea.Int32(5000),
			Event:       tea.String("POST_CALL_TOOL"),
		},
	}
}

func buildRawHooks(hookURL string) []map[string]any {
	return []map[string]any{
		{"url": hookURL, "description": "脱敏订单结果并注入 audit_id", "enabled": true, "timeout": 5000, "event": "POST_CALL_TOOL"},
	}
}

func createTool(sdkClient *agentrun.Client, body *agentrun.CreateToolInputV2) error {
	_, err := sdkClient.CreateTool(&agentrun.CreateToolRequest{Body: body})
	if err != nil {
		return fmt.Errorf("CreateTool: %s", formatSDKError(err))
	}
	return nil
}

func rawCreateTool(sdkClient *agentrun.Client, toolName, protocolSpec string, hooks []map[string]any) error {
	body := map[string]any{
		"toolName":     toolName,
		"description":  "Hook quickstart MCP_REMOTE 订单查询示例",
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

func rawUpdateTool(sdkClient *agentrun.Client, toolName string, hooks []map[string]any) error {
	body := map[string]any{
		"proxyEnabled": true,
		"mcpProxyConfiguration": map[string]any{
			"hooks": hooks,
		},
	}
	return rawToolAPICall(sdkClient, http.MethodPatch, "/2025-09-10/agents/tools/"+toolName, body)
}

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

func deleteTool(sdkClient *agentrun.Client, toolName string) error {
	_, err := sdkClient.DeleteToolWithOptions(tea.String(toolName), &agentrun.DeleteToolRequest{}, map[string]*string{}, &util.RuntimeOptions{})
	if err != nil {
		return fmt.Errorf("DeleteTool: %s", formatSDKError(err))
	}
	return nil
}
