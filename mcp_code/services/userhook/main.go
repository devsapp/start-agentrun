package main

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
)

type hookMessage struct {
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	StatusCode int               `json:"status_code,omitempty"`
}

type hookPayload struct {
	Event    string      `json:"event"`
	Request  hookMessage `json:"request"`
	Response hookMessage `json:"response"`
}

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

	response := hookPayload{
		Request:  hookMessage{Headers: map[string]string{}},
		Response: hookMessage{Headers: map[string]string{}},
	}

	switch payload.Event {
	case "POST_LIST_TOOLS":
		response = rewriteToolList(payload)
	case "PRE_CALL_TOOL":
		response = rewriteMultiply(payload)
	case "POST_CALL_TOOL":
		response = rewriteAddResult(payload)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func rewriteToolList(payload hookPayload) hookPayload {
	body, err := base64.StdEncoding.DecodeString(payload.Response.Body)
	if err != nil {
		return emptyPayload()
	}

	var rpc map[string]any
	if json.Unmarshal(body, &rpc) != nil {
		return emptyPayload()
	}

	result, ok := rpc["result"].(map[string]any)
	if !ok {
		return emptyPayload()
	}
	tools, ok := result["tools"].([]any)
	if !ok {
		return emptyPayload()
	}

	tools = append(tools, map[string]any{
		"name":        "hook_info",
		"description": "added by hook",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
	})
	result["tools"] = tools
	rpc["result"] = result
	encoded, _ := json.Marshal(rpc)

	return hookPayload{
		Request:  hookMessage{Headers: map[string]string{}},
		Response: hookMessage{Headers: map[string]string{}, Body: base64.StdEncoding.EncodeToString(encoded)},
	}
}

func rewriteMultiply(payload hookPayload) hookPayload {
	body, err := base64.StdEncoding.DecodeString(payload.Request.Body)
	if err != nil {
		return emptyPayload()
	}

	var rpc map[string]any
	if json.Unmarshal(body, &rpc) != nil {
		return emptyPayload()
	}

	params, ok := rpc["params"].(map[string]any)
	if !ok {
		return emptyPayload()
	}
	if name, _ := params["name"].(string); name != "multiply" {
		return emptyPayload()
	}
	args, ok := params["arguments"].(map[string]any)
	if !ok {
		return emptyPayload()
	}
	args["a"] = 11
	args["b"] = 11
	params["arguments"] = args
	rpc["params"] = params
	encoded, _ := json.Marshal(rpc)

	return hookPayload{
		Request:  hookMessage{Headers: map[string]string{}, Body: base64.StdEncoding.EncodeToString(encoded)},
		Response: hookMessage{Headers: map[string]string{}},
	}
}

func rewriteAddResult(payload hookPayload) hookPayload {
	body, err := base64.StdEncoding.DecodeString(payload.Response.Body)
	if err != nil {
		return emptyPayload()
	}

	var rpc map[string]any
	if json.Unmarshal(body, &rpc) != nil {
		return emptyPayload()
	}

	result, ok := rpc["result"].(map[string]any)
	if !ok {
		return emptyPayload()
	}
	content, ok := result["content"].([]any)
	if !ok {
		return emptyPayload()
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
		textContent["text"] = text + " [hooked:200]"
		content[idx] = textContent
	}
	result["content"] = content
	rpc["result"] = result
	encoded, _ := json.Marshal(rpc)

	return hookPayload{
		Request:  hookMessage{Headers: map[string]string{}},
		Response: hookMessage{Headers: map[string]string{}, Body: base64.StdEncoding.EncodeToString(encoded)},
	}
}

func emptyPayload() hookPayload {
	return hookPayload{
		Request:  hookMessage{Headers: map[string]string{}},
		Response: hookMessage{Headers: map[string]string{}},
	}
}
