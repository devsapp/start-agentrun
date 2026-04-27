package demo

import (
	"fmt"
	"net/url"
	"strings"
)

func dataPlaneMCPURL(proto, dataEndpoint, toolName string) (string, string) {
	return fmt.Sprintf("%s/tools/%s/mcp", endpointBaseURL(proto, dataEndpoint), toolName), dataPlaneProduct(dataEndpoint)
}

func defaultDataEndpoint(uid string) string {
	return fmt.Sprintf("%s.agentrun-data.%s.aliyuncs.com", uid, region)
}

func dataPlaneProduct(dataEndpoint string) string {
	if strings.Contains(dataEndpoint, "funagent") {
		return "funagent"
	}
	return "agentrun"
}

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

func endpointBaseURL(proto, endpoint string) string {
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return strings.TrimRight(endpoint, "/")
	}
	return proto + "://" + strings.TrimRight(endpoint, "/")
}
