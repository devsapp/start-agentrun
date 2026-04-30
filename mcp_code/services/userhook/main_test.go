package main

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

// TestMaskAndAuditResponseMasksSensitiveFields 验证 Hook 会脱敏敏感字段并写入 audit_id。
// 参数 t 是测试上下文。
func TestMaskAndAuditResponseMasksSensitiveFields(t *testing.T) {
	response := maskAndAuditResponse(buildPostCallPayload(t))
	textData, structuredData := decodeHookResponse(t, response)

	assertMaskedOrder(t, textData)
	assertMaskedOrder(t, structuredData)
	if textData["audit_id"] != structuredData["audit_id"] {
		t.Fatalf("文本结果和结构化结果 audit_id 不一致: text=%v structured=%v", textData["audit_id"], structuredData["audit_id"])
	}
}

// buildPostCallPayload 构造 POST_CALL_TOOL Hook 负载。
// 参数 t 是测试上下文。
func buildPostCallPayload(t *testing.T) hookPayload {
	t.Helper()

	order := map[string]any{
		"order_id":         "ORDER-1001",
		"customer_name":    "张三",
		"phone":            "13812345678",
		"email":            "zhangsan@example.com",
		"shipping_address": "浙江省杭州市西湖区文三路 100 号",
		"status":           "PAID",
		"items":            []any{map[string]any{"name": "人体工学鼠标"}},
	}
	orderText, err := json.Marshal(order)
	if err != nil {
		t.Fatalf("构造订单文本失败: %v", err)
	}

	rpc := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"result": map[string]any{
			"content": []any{
				map[string]any{"type": "text", "text": string(orderText)},
			},
			"structuredContent": order,
		},
	}
	body, err := json.Marshal(rpc)
	if err != nil {
		t.Fatalf("构造 Hook 响应体失败: %v", err)
	}

	return hookPayload{
		Event: postCallToolEvent,
		Response: hookMessage{
			Headers: map[string]string{},
			Body:    base64.StdEncoding.EncodeToString(body),
		},
	}
}

// decodeHookResponse 解码 Hook 改写后的文本结果和结构化结果。
// 参数 t 是测试上下文；参数 response 是 Hook 响应。
func decodeHookResponse(t *testing.T, response hookPayload) (map[string]any, map[string]any) {
	t.Helper()

	body, err := base64.StdEncoding.DecodeString(response.Response.Body)
	if err != nil {
		t.Fatalf("解码 Hook 响应失败: %v", err)
	}
	var rpc map[string]any
	if err := json.Unmarshal(body, &rpc); err != nil {
		t.Fatalf("解析 Hook 响应失败: %v", err)
	}
	result, ok := rpc["result"].(map[string]any)
	if !ok {
		t.Fatalf("Hook 响应缺少 result: %#v", rpc)
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("Hook 响应缺少 content: %#v", result)
	}
	textContent, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("Hook 响应 content 格式错误: %#v", content[0])
	}
	text, ok := textContent["text"].(string)
	if !ok {
		t.Fatalf("Hook 响应缺少 text: %#v", textContent)
	}
	var textData map[string]any
	if err := json.Unmarshal([]byte(text), &textData); err != nil {
		t.Fatalf("解析文本 JSON 失败: %v", err)
	}
	structuredData, ok := result["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("Hook 响应缺少 structuredContent: %#v", result)
	}
	return textData, structuredData
}

// assertMaskedOrder 断言订单结果已经完成脱敏和审计编号注入。
// 参数 t 是测试上下文；参数 data 是订单结果。
func assertMaskedOrder(t *testing.T, data map[string]any) {
	t.Helper()

	if data["phone"] != "138****5678" {
		t.Fatalf("手机号未脱敏: %#v", data["phone"])
	}
	if data["email"] != "zh***@example.com" {
		t.Fatalf("邮箱未脱敏: %#v", data["email"])
	}
	if data["shipping_address"] != "浙江省杭州市****" {
		t.Fatalf("地址未脱敏: %#v", data["shipping_address"])
	}
	if data["customer_name"] != "张*" {
		t.Fatalf("客户姓名未脱敏: %#v", data["customer_name"])
	}
	auditID, ok := data["audit_id"].(string)
	if !ok || !strings.HasPrefix(auditID, "audit_") {
		t.Fatalf("audit_id 未注入: %#v", data["audit_id"])
	}
	items, ok := data["items"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("订单商品缺失: %#v", data["items"])
	}
	firstItem, ok := items[0].(map[string]any)
	if !ok || firstItem["name"] != "人体工学鼠标" {
		t.Fatalf("商品名称不应被脱敏: %#v", data["items"])
	}
}
