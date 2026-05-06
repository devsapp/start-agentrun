package demo

import (
	"fmt"
	"net/url"
	"strings"
)

// dataPlaneMCPURL 构造访问 AgentRun 数据平面的 MCP 地址。
// 参数 proto 是访问协议；参数 dataEndpoint 是数据面 endpoint；参数 toolName 是工具名称。
func dataPlaneMCPURL(proto, dataEndpoint, toolName string) (string, string) {
	return fmt.Sprintf("%s/tools/%s/mcp", endpointBaseURL(proto, dataEndpoint), toolName), dataPlaneProduct(dataEndpoint)
}

// remoteMCPConfigURL 构造写入 MCP_REMOTE 控制面配置的远程 MCP 端点。
// 参数 baseURL 是部署出的远程 MCP 函数基地址。
func remoteMCPConfigURL(baseURL string) string {
	baseURL = strings.TrimSuffix(baseURL, "/")
	if strings.HasSuffix(baseURL, "/mcp") {
		return baseURL
	}
	return baseURL + "/mcp"
}

// defaultDataEndpoint 构造默认数据面 endpoint。
// 参数 uid 是阿里云账号 UID。
func defaultDataEndpoint(uid string) string {
	return fmt.Sprintf("%s.agentrun-data.%s.aliyuncs.com", uid, region)
}

// dataPlaneProduct 根据数据面 endpoint 判断签名 product。
// 参数 dataEndpoint 是数据面 endpoint。
func dataPlaneProduct(dataEndpoint string) string {
	if strings.Contains(dataEndpoint, "funagent") {
		return "funagent"
	}
	return "agentrun"
}

// endpointHost 从 endpoint 配置中提取 host。
// 参数 value 是用户配置值；参数 fallback 是默认 endpoint。
func endpointHost(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	if parsed, err := url.Parse(value); err == nil && parsed.Host != "" {
		return parsed.Host
	}
	return strings.Trim(strings.SplitN(value, "/", 2)[0], "/")
}

// endpointBaseURL 构造带协议的 endpoint 基地址。
// 参数 proto 是默认访问协议；参数 endpoint 是 endpoint 配置。
func endpointBaseURL(proto, endpoint string) string {
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return strings.TrimRight(endpoint, "/")
	}
	return proto + "://" + strings.TrimRight(endpoint, "/")
}
