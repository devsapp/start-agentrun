package main

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
)

const (
	preCallToolEvent         = "PRE_CALL_TOOL"
	postCallToolEvent        = "POST_CALL_TOOL"
	debugHeadersToolName     = "debug_headers"
	debugRequestHeaderKey    = "X-Debug-Request-Header"
	debugRequestHeaderValue  = "pre-call-tool-upstream"
	debugResponseHeaderKey   = "X-Debug-Response-Header"
	debugResponseHeaderValue = "post-call-tool-client"
)

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
	switch payload.Event {
	case preCallToolEvent:
		response = rewriteDebugRequestHeaders(payload)
	case postCallToolEvent:
		response = rewriteDebugResponseHeaders(payload)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// rewriteDebugRequestHeaders 在 PRE 阶段改写发往上游 MCP 服务的请求头。
// 参数 payload 是 AgentRun Hook 事件负载。
func rewriteDebugRequestHeaders(payload hookPayload) hookPayload {
	if hookToolName(payload.Request.Body) != debugHeadersToolName {
		return emptyPayload()
	}
	headers := cloneHeaders(payload.Request.Headers)
	headers[debugRequestHeaderKey] = debugRequestHeaderValue
	return hookPayload{
		Request:  hookMessage{Headers: headers},
		Response: hookMessage{Headers: map[string]string{}},
	}
}

// rewriteDebugResponseHeaders 在 POST 阶段改写返回给客户端的响应头。
// 参数 payload 是 AgentRun Hook 事件负载。
func rewriteDebugResponseHeaders(payload hookPayload) hookPayload {
	if hookToolName(payload.Request.Body) != debugHeadersToolName {
		return emptyPayload()
	}
	return hookPayload{
		Request: hookMessage{Headers: map[string]string{}},
		Response: hookMessage{
			Headers: map[string]string{
				"Content-Type":         "application/json",
				debugResponseHeaderKey: debugResponseHeaderValue,
			},
			StatusCode: http.StatusOK,
		},
	}
}

// hookToolName 从 Base64 编码的 JSON-RPC 请求体中读取工具名称。
// 参数 encodedBody 是 Hook 请求中的 request.body。
func hookToolName(encodedBody string) string {
	body, err := base64.StdEncoding.DecodeString(encodedBody)
	if err != nil {
		return ""
	}
	var rpc struct {
		Params struct {
			Name string `json:"name"`
		} `json:"params"`
	}
	if json.Unmarshal(body, &rpc) != nil {
		return ""
	}
	return rpc.Params.Name
}

// cloneHeaders 复制 Hook 请求头，避免直接修改入参。
// 参数 headers 是待复制的请求头。
func cloneHeaders(headers map[string]string) map[string]string {
	cloned := make(map[string]string, len(headers)+1)
	for key, value := range headers {
		cloned[key] = value
	}
	return cloned
}

// emptyPayload 返回空 Hook 改写结果。
func emptyPayload() hookPayload {
	return hookPayload{
		Request:  hookMessage{Headers: map[string]string{}},
		Response: hookMessage{Headers: map[string]string{}},
	}
}
