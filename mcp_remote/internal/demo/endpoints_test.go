package demo

import "testing"

// TestRemoteMCPConfigURLDoesNotAppendMCP 验证远程 MCP 控制面配置不拼接 /mcp。
// 参数 t 是 Go 测试上下文。
func TestRemoteMCPConfigURLDoesNotAppendMCP(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{
			name:    "函数基地址",
			baseURL: "https://example.com/runtime/invoke/",
			want:    "https://example.com/runtime/invoke",
		},
		{
			name:    "误带 mcp 后缀",
			baseURL: "https://example.com/runtime/invoke/mcp/",
			want:    "https://example.com/runtime/invoke",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := remoteMCPConfigURL(tt.baseURL)
			if got != tt.want {
				t.Fatalf("remoteMCPConfigURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestDataPlaneMCPURLAppendsMCP 验证数据平面访问地址拼接 /mcp。
// 参数 t 是 Go 测试上下文。
func TestDataPlaneMCPURLAppendsMCP(t *testing.T) {
	got, product := dataPlaneMCPURL("https", "123456.agentrun-data.cn-hangzhou.aliyuncs.com", "demo-tool")
	want := "https://123456.agentrun-data.cn-hangzhou.aliyuncs.com/tools/demo-tool/mcp"
	if got != want {
		t.Fatalf("dataPlaneMCPURL() = %q, want %q", got, want)
	}
	if product != "agentrun" {
		t.Fatalf("dataPlaneMCPURL() product = %q, want %q", product, "agentrun")
	}
}
