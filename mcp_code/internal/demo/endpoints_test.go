package demo

import "testing"

// TestDataPlaneMCPURLAppendsMCP 验证 CODE_PACKAGE 数据平面访问地址拼接 /mcp。
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
