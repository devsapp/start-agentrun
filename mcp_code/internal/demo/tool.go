package demo

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	agentrun "github.com/alibabacloud-go/agentrun-20250910/v5/client"
	openapiutil "github.com/alibabacloud-go/darabonba-openapi/v2/utils"
	"github.com/alibabacloud-go/tea/dara"
	"github.com/alibabacloud-go/tea/tea"
)

// codePackageLocation 表示上传到 OSS 的代码包位置。
type codePackageLocation struct {
	OSSBucketName string
	OSSObjectName string
}

// validate 检查代码包 OSS 地址是否完整。
func (l codePackageLocation) validate() error {
	if strings.TrimSpace(l.OSSBucketName) == "" || strings.TrimSpace(l.OSSObjectName) == "" {
		return errors.New("代码包 OSS 地址不能为空")
	}
	return nil
}

// createCodeTool 创建 CODE_PACKAGE 工具。
// 参数 sdkClient 是 AgentRun 客户端；toolName 是工具名称；codePackage 是 OSS 代码包地址；hookURL 是 MCP hook 地址。
func createCodeTool(sdkClient *agentrun.Client, toolName string, codePackage codePackageLocation, hookURL string) (string, error) {
	if err := codePackage.validate(); err != nil {
		return "", err
	}
	slog.Info("创建 AgentRun CODE_PACKAGE 工具", "tool", toolName, "ossBucketName", codePackage.OSSBucketName, "ossObjectName", codePackage.OSSObjectName)
	req := &agentrun.CreateToolRequest{
		Body: newCodeToolInput(toolName, codePackage, hookURL),
	}
	logCreateToolRequestShape(toolName, req.Body)
	resp, err := sdkClient.CreateToolWithOptions(req, map[string]*string{}, &dara.RuntimeOptions{})
	if err != nil {
		slog.Error("AgentRun CreateTool 失败", "tool", toolName, "agentrunRequestId", extractRequestID(err), "error", formatSDKError(err))
		return "", fmt.Errorf("CreateTool: %s", formatSDKError(err))
	}
	requestID := extractRequestIDFromHeaders(resp.GetHeaders())
	slog.Info("AgentRun CreateTool 已返回", "tool", toolName, "agentrunRequestId", requestID)
	return requestID, nil
}

// logCreateToolRequestShape 打印 CreateTool 请求体中 OSS 代码包地址。
// 参数 toolName 是工具名称；body 是 AgentRun CreateTool 请求体。
func logCreateToolRequestShape(toolName string, body *agentrun.CreateToolInputV2) {
	codePackage, ok := createToolRequestCodeLocation(body)
	slog.Info("AgentRun CreateTool 请求体检查", "tool", toolName, "hasCodeConfigurationOSSLocation", ok, "ossBucketName", codePackage.OSSBucketName, "ossObjectName", codePackage.OSSObjectName)
}

// createToolRequestCodeLocation 从 SDK 序列化映射中读取 OSS 代码包地址。
// 参数 body 是 AgentRun CreateTool 请求体。
func createToolRequestCodeLocation(body *agentrun.CreateToolInputV2) (codePackageLocation, bool) {
	payload := openapiutil.ParseToMap(body)
	codeConfig, ok := payload["codeConfiguration"].(map[string]any)
	if !ok {
		return codePackageLocation{}, false
	}
	bucketName, bucketOK := codeConfig["ossBucketName"].(string)
	objectName, objectOK := codeConfig["ossObjectName"].(string)
	if !bucketOK || !objectOK {
		return codePackageLocation{}, false
	}
	return codePackageLocation{OSSBucketName: bucketName, OSSObjectName: objectName}, true
}

// newCodeToolInput 构造 CODE_PACKAGE 工具请求体。
// 参数 toolName 是工具名称；codePackage 是 OSS 代码包地址；hookURL 是 MCP hook 地址。
func newCodeToolInput(toolName string, codePackage codePackageLocation, hookURL string) *agentrun.CreateToolInputV2 {
	return &agentrun.CreateToolInputV2{
		ToolName:     tea.String(toolName),
		Description:  tea.String("hook quickstart CODE_PACKAGE"),
		ToolType:     tea.String("MCP"),
		CreateMethod: tea.String("CODE_PACKAGE"),
		McpConfig: &agentrun.McpConfig{
			SessionAffinity:       tea.String("MCP_STREAMABLE"),
			SessionAffinityConfig: tea.String(`{"sessionTTLInSeconds":21600,"sessionConcurrencyPerInstance":20,"sessionIdleTimeoutInSeconds":1800}`),
			ProxyEnabled:          tea.Bool(true),
			McpProxyConfiguration: &agentrun.McpProxyConfiguration{Hooks: buildHooks(hookURL)},
		},
		ArtifactType: tea.String("Code"),
		CodeConfiguration: &agentrun.CodeConfiguration{
			OssBucketName: tea.String(codePackage.OSSBucketName),
			OssObjectName: tea.String(codePackage.OSSObjectName),
			Language:      tea.String("nodejs22"),
			Command:       []*string{tea.String("/code/calculator")},
		},
		Port: tea.Int32(9000),
	}
}

// buildHooks 构造 MCP proxy hook 配置。
// 参数 hookURL 是 hook 服务地址。
func buildHooks(hookURL string) []*agentrun.Hook {
	return []*agentrun.Hook{
		{
			Url:         tea.String(hookURL),
			Description: tea.String("inject hook_info"),
			Enabled:     tea.Bool(true),
			Timeout:     tea.Int32(5000),
			Event:       tea.String("POST_LIST_TOOLS"),
		},
		{
			Url:         tea.String(hookURL),
			Description: tea.String("rewrite multiply args"),
			Enabled:     tea.Bool(true),
			Timeout:     tea.Int32(5000),
			Event:       tea.String("PRE_CALL_TOOL"),
		},
		{
			Url:         tea.String(hookURL),
			Description: tea.String("append hooked marker"),
			Enabled:     tea.Bool(true),
			Timeout:     tea.Int32(5000),
			Event:       tea.String("POST_CALL_TOOL"),
		},
	}
}

// waitForToolReady 等待工具创建完成。
// 参数 sdkClient 是 AgentRun 客户端；toolName 是工具名称；createToolRequestID 是 CreateTool 响应 header 中的请求 ID；timeout 是等待超时时间。
func waitForToolReady(sdkClient *agentrun.Client, toolName, createToolRequestID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	delay := 3 * time.Second
	lastStatus := ""
	lastReason := ""
	for time.Now().Before(deadline) {
		tool, err := getToolDetail(sdkClient, toolName)
		if err == nil {
			lastStatus = tea.StringValue(tool.Status)
			lastReason = tea.StringValue(tool.StatusReason)
			if strings.EqualFold(lastStatus, "READY") {
				return nil
			}
			normalizedStatus := strings.ToUpper(lastStatus)
			if strings.Contains(normalizedStatus, "FAILED") || strings.Contains(normalizedStatus, "ERROR") {
				slog.Error("AgentRun 工具创建失败", "tool", toolName, "status", lastStatus, "agentrunRequestId", createToolRequestID, "fcRequestId", extractRequestIDFromText(lastReason), "reason", lastReason)
				return fmt.Errorf("tool 进入失败状态: %s reason=%s", lastStatus, lastReason)
			}
		}
		time.Sleep(delay)
		if delay < 20*time.Second {
			delay *= 2
		}
	}
	return fmt.Errorf("等待 tool READY 超时: %s status=%s reason=%s", toolName, lastStatus, lastReason)
}

// getToolDetail 查询工具详情。
// 参数 sdkClient 是 AgentRun 客户端；toolName 是工具名称。
func getToolDetail(sdkClient *agentrun.Client, toolName string) (*agentrun.Tool, error) {
	resp, err := sdkClient.GetTool(tea.String(toolName))
	if err != nil {
		return nil, fmt.Errorf("GetTool: %s", formatSDKError(err))
	}
	if resp.Body == nil || resp.Body.Data == nil {
		return nil, errors.New("GetTool 返回空响应")
	}
	return resp.Body.Data, nil
}

// deleteTool 删除工具。
// 参数 sdkClient 是 AgentRun 客户端；toolName 是工具名称。
func deleteTool(sdkClient *agentrun.Client, toolName string) error {
	_, err := sdkClient.DeleteTool(tea.String(toolName))
	if err != nil {
		return fmt.Errorf("DeleteTool: %s", formatSDKError(err))
	}
	return nil
}
