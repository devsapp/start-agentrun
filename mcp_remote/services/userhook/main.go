package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
)

const postCallToolEvent = "POST_CALL_TOOL"

// hookMessage 表示 Hook 请求或响应中的 HTTP 消息。
type hookMessage struct {
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	StatusCode int               `json:"status_code,omitempty"`
}

// hookPayload 表示 AgentRun 发送给 Hook 服务的事件负载。
type hookPayload struct {
	Event    string      `json:"event"`
	Request  hookMessage `json:"request"`
	Response hookMessage `json:"response"`
}

// main 启动 Hook 回调服务。
func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/hook", handleHook)

	port := os.Getenv("FC_SERVER_PORT")
	if port == "" {
		port = "9000"
	}

	slog.Info("userhook 服务启动", "port", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		slog.Error("userhook 服务异常", "error", err)
		os.Exit(1)
	}
}

// handleHook 处理 AgentRun Hook 回调。
// 参数 w 是 HTTP 响应写入器；参数 r 是 HTTP 请求。
func handleHook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload hookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	response := emptyPayload()
	if payload.Event == postCallToolEvent {
		response = maskAndAuditResponse(payload)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// maskAndAuditResponse 对 MCP 工具结果做脱敏，并写入审计编号。
// 参数 payload 是 AgentRun Hook 事件负载。
func maskAndAuditResponse(payload hookPayload) hookPayload {
	body, err := base64.StdEncoding.DecodeString(payload.Response.Body)
	if err != nil {
		return emptyPayload()
	}

	var rpc map[string]any
	if json.Unmarshal(body, &rpc) != nil {
		return emptyPayload()
	}

	auditID := auditIDFromBytes(body)
	result, ok := rpc["result"].(map[string]any)
	if !ok {
		return emptyPayload()
	}
	maskAndAuditResult(result, auditID)
	rpc["result"] = result

	encoded, err := json.Marshal(rpc)
	if err != nil {
		return emptyPayload()
	}

	return hookPayload{
		Request:  hookMessage{Headers: map[string]string{}},
		Response: hookMessage{Headers: map[string]string{}, Body: base64.StdEncoding.EncodeToString(encoded)},
	}
}

// maskAndAuditResult 处理 MCP result 中的结构化结果和文本结果。
// 参数 result 是 MCP 工具调用结果；参数 auditID 是审计编号。
func maskAndAuditResult(result map[string]any, auditID string) {
	if structured, ok := result["structuredContent"].(map[string]any); ok {
		maskSensitiveMap(structured)
		structured["audit_id"] = auditID
		result["structuredContent"] = structured
	}

	content, ok := result["content"].([]any)
	if !ok {
		return
	}
	for idx, item := range content {
		textContent, ok := item.(map[string]any)
		if !ok {
			continue
		}
		text, ok := textContent["text"].(string)
		if !ok {
			continue
		}
		rewrittenText, rewritten := rewriteJSONText(text, auditID)
		if !rewritten {
			continue
		}
		textContent["text"] = rewrittenText
		content[idx] = textContent
	}
	result["content"] = content
}

// rewriteJSONText 重写文本中的 JSON 订单结果。
// 参数 text 是 MCP 文本结果；参数 auditID 是审计编号。
func rewriteJSONText(text string, auditID string) (string, bool) {
	var data any
	if json.Unmarshal([]byte(text), &data) != nil {
		return "", false
	}
	data = maskSensitiveValue(data)
	if dataMap, ok := data.(map[string]any); ok {
		dataMap["audit_id"] = auditID
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return "", false
	}
	return string(encoded), true
}

// maskSensitiveValue 递归处理任意 JSON 值中的敏感字段。
// 参数 value 是待处理的 JSON 值。
func maskSensitiveValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		maskSensitiveMap(typed)
		return typed
	case []any:
		for idx, item := range typed {
			typed[idx] = maskSensitiveValue(item)
		}
		return typed
	default:
		return value
	}
}

// maskSensitiveMap 脱敏订单结果中的敏感字段。
// 参数 data 是待处理的 JSON 对象。
func maskSensitiveMap(data map[string]any) {
	for key, value := range data {
		lowerKey := strings.ToLower(key)
		text, isString := value.(string)
		switch {
		case isString && (strings.Contains(lowerKey, "phone") || strings.Contains(lowerKey, "mobile")):
			data[key] = maskPhone(text)
		case isString && strings.Contains(lowerKey, "email"):
			data[key] = maskEmail(text)
		case isString && strings.Contains(lowerKey, "address"):
			data[key] = maskAddress(text)
		case isString && (lowerKey == "customer_name" || lowerKey == "receiver_name"):
			data[key] = maskName(text)
		default:
			data[key] = maskSensitiveValue(value)
		}
	}
}

// maskPhone 脱敏手机号。
// 参数 phone 是原始手机号。
func maskPhone(phone string) string {
	runes := []rune(strings.TrimSpace(phone))
	if len(runes) <= 7 {
		return "***"
	}
	return string(runes[:3]) + "****" + string(runes[len(runes)-4:])
}

// maskEmail 脱敏邮箱地址。
// 参数 email 是原始邮箱地址。
func maskEmail(email string) string {
	parts := strings.SplitN(strings.TrimSpace(email), "@", 2)
	if len(parts) != 2 {
		return "***"
	}
	name := []rune(parts[0])
	if len(name) == 0 {
		return "***@" + parts[1]
	}
	keep := 1
	if len(name) > 2 {
		keep = 2
	}
	return string(name[:keep]) + "***@" + parts[1]
}

// maskAddress 脱敏收货地址。
// 参数 address 是原始收货地址。
func maskAddress(address string) string {
	runes := []rune(strings.TrimSpace(address))
	if len(runes) <= 6 {
		return "***"
	}
	return string(runes[:6]) + "****"
}

// maskName 脱敏客户姓名。
// 参数 name 是原始客户姓名。
func maskName(name string) string {
	runes := []rune(strings.TrimSpace(name))
	if len(runes) == 0 {
		return "***"
	}
	return string(runes[:1]) + "*"
}

// auditIDFromBytes 根据原始响应生成稳定的审计编号。
// 参数 body 是解码后的 MCP 响应体。
func auditIDFromBytes(body []byte) string {
	sum := sha256.Sum256(body)
	return "audit_" + hex.EncodeToString(sum[:])[:16]
}

// emptyPayload 返回空 Hook 改写结果。
func emptyPayload() hookPayload {
	return hookPayload{
		Request:  hookMessage{Headers: map[string]string{}},
		Response: hookMessage{Headers: map[string]string{}},
	}
}
