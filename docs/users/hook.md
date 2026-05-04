# User Hook 开发指南

agentrun 支持在 MCP 请求的各个阶段注入自定义逻辑（User Hook）。通过 Hook，你可以在不修改 MCP 服务本身的情况下实现：

- 修改工具列表（添加/删除/重命名工具）
- 拦截和修改工具调用的请求参数
- 修改工具调用的返回结果
- 实现凭证转换、审计日志等横切关注点

## 快速开始

### 1. 编写 Hook 服务

Hook 服务是一个普通的 HTTP 服务，接收 POST 请求，返回 JSON 响应。

以下示例在 `tools/list` 的响应中追加一个自定义工具：

```go
package main

import (
    "encoding/base64"
    "encoding/json"
    "net/http"
)

type HookMessage struct {
    Headers    map[string]string `json:"headers"`
    Body       string            `json:"body"`       // Base64 编码
    StatusCode int               `json:"status_code,omitempty"` // HTTP 状态码
}

type HookPayload struct {
    Event    string      `json:"event"`
    ToolName string      `json:"toolname"`
    Region   string      `json:"region"`
    Request  HookMessage `json:"request"`
    Response HookMessage `json:"response"`
}

func main() {
    http.HandleFunc("/hook", func(w http.ResponseWriter, r *http.Request) {
        var payload HookPayload
        json.NewDecoder(r.Body).Decode(&payload)

        resp := HookPayload{
            Request:  HookMessage{Headers: map[string]string{}, Body: ""},
            Response: HookMessage{Headers: map[string]string{}, Body: ""},
        }

        if payload.Event == "POST_LIST_TOOLS" {
            // 解码上游响应
            respBody, _ := base64.StdEncoding.DecodeString(payload.Response.Body)

            var rpcResp map[string]interface{}
            json.Unmarshal(respBody, &rpcResp)

            // 添加自定义工具
            result := rpcResp["result"].(map[string]interface{})
            tools := result["tools"].([]interface{})
            tools = append(tools, map[string]interface{}{
                "name":        "my_tool",
                "description": "我的自定义工具",
                "inputSchema": map[string]interface{}{
                    "type":       "object",
                    "properties": map[string]interface{}{},
                },
            })
            result["tools"] = tools
            rpcResp["result"] = result

            modified, _ := json.Marshal(rpcResp)
            resp.Response.Body = base64.StdEncoding.EncodeToString(modified)
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(resp)
    })
    http.ListenAndServe(":9000", nil)
}
```

### 2. 部署 Hook 服务

将 Hook 服务部署为可通过 HTTP 访问的服务（如 FC 函数、ECS、K8s 等），获取其公网 URL。

### 3. 通过 agentrun API 配置 Hook

通过 agentrun API 创建或更新工具时，在 `mcpConfig.mcpProxyConfiguration.hooks` 中配置 Hook，同时需要将 `mcpConfig.proxyEnabled` 设置为 `true`。

**创建工具时配置 Hook：**

```http
POST /2025-09-10/agents/tools HTTP/1.1
Host: agentrun.cn-hangzhou.aliyuncs.com
Content-Type: application/json

{
  "toolName": "my-mcp-tool",
  "workspaceId": "ws-xxxxxxxxxxxx",
  "description": "带 Hook 的 MCP 工具",
  "toolType": "MCP",
  "createMethod": "MCP_REMOTE",
  "mcpConfig": {
    "sessionAffinity": "MCP_STREAMABLE",
    "sessionAffinityConfig": "{\"sessionTTLInSeconds\":21600,\"sessionConcurrencyPerInstance\":20,\"sessionIdleTimeoutInSeconds\":1800}",
    "proxyEnabled": true,
    "mcpProxyConfiguration": {
      "hooks": [
        {
          "url": "https://your-hook-service.example.com/hook",
          "description": "追加自定义工具",
          "event": "POST_LIST_TOOLS",
          "enabled": true,
          "timeout": 5000
        }
      ]
    }
  },
  "artifactType": "Code",
  "codeConfiguration": {
    "ossBucketName": "my-tool-bucket",
    "ossObjectName": "tool-code-v1.0.zip",
    "language": "python3.12",
    "command": ["python", "main.py"]
  },
  "cpu": 0.5,
  "memory": 512,
  "port": 8080
}
```

## 请求流程

```
客户端                    agentrun 代理                上游 MCP 服务           Hook 服务
  │                        │                           │                      │
  │── POST tools/list ────>│                           │                      │
  │                        │                           │                      │
  │                        │── PRE_LIST_TOOLS ──────────────────────────────>│
  │                        │   (如有 PRE hook)         │                      │
  │                        │<────────────────────────────────────────────────│
  │                        │                           │                      │
  │                        │── 转发请求 ──────────────>│                      │
  │                        │<── 返回响应 ─────────────│                      │
  │                        │                           │                      │
  │                        │── POST_LIST_TOOLS ─────────────────────────────>│
  │                        │   (如有 POST hook)        │                      │
  │                        │<───────────────────────────────────────────────│
  │                        │                           │                      │
  │<── 最终响应 ───────────│                           │                      │
```

agentrun 对两种 MCP 方法触发 Hook：

| MCP 方法 | PRE 事件 | POST 事件 |
|----------|----------|-----------|
| `tools/list` | `PRE_LIST_TOOLS` | `POST_LIST_TOOLS` |
| `tools/call` | `PRE_CALL_TOOL` | `POST_CALL_TOOL` |

- **PRE 阶段**：在转发到上游之前执行，可以修改**请求**（body 和 headers），或通过返回 `status_code` **终止请求**
- **POST 阶段**：在收到上游响应之后执行，可以修改**响应**（body、headers 和 status_code）
- 其他 MCP 方法（如 `initialize`）直接透传，不触发 Hook

## 协议参考

### Hook 请求（agentrun → Hook 服务）

agentrun 向 Hook 服务发送 HTTP POST 请求，Content-Type 为 `application/json`。

请求时会携带 Hook 配置中的 `headers` 字段（用于认证等场景）。

**请求体结构：**

```json
{
  "event": "POST_LIST_TOOLS",
  "toolname": "calculator",
  "region": "cn-hangzhou",
  "request": {
    "headers": {
      "Content-Type": "application/json",
      "Mcp-Session-Id": "xxx"
    },
    "body": "<Base64 编码的原始请求 body>"
  },
  "response": {
    "headers": {
      "Content-Type": "application/json"
    },
    "body": "<Base64 编码的上游响应 body>",
    "status_code": 200
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `event` | string | 事件名称（见上表） |
| `toolname` | string | 工具名称（创建工具时指定的 `toolName`） |
| `region` | string | 工具所在区域 |
| `request.headers` | object | 客户端请求的 HTTP 头 |
| `request.body` | string | 客户端请求体的 Base64 编码 |
| `response.headers` | object | 上游响应的 HTTP 头（仅 POST 事件有值） |
| `response.body` | string | 上游响应体的 Base64 编码（仅 POST 事件有值） |
| `response.status_code` | int | 上游响应的 HTTP 状态码（仅 POST 事件有值） |

> PRE 事件中 `response` 字段为空。POST 事件中 `response` 包含上游的实际返回（含 `status_code`）。
>
> 若上游 MCP 返回 `text/event-stream`，agentrun/midgo 会先读取整条 SSE 流，提取最终 JSON-RPC response，再将这个最终 response 写入 `response.body`。客户端看到的仍是普通 HTTP JSON 响应，不保留 SSE 恢复语义。

### Hook 响应（Hook 服务 → agentrun）

Hook 服务必须返回 HTTP 200 和 JSON 响应。返回非 200 状态码会导致请求失败（fail-close 策略）。

**响应体结构：**

```json
{
  "request": {
    "headers": {},
    "body": ""
  },
  "response": {
    "headers": {},
    "body": "",
    "status_code": 0
  }
}
```

**返回值的影响：**

| 阶段 | 返回字段 | 非零/非空时的效果 | 为空/为零时 |
|------|----------|------------------|------------|
| PRE | `request.body` | 用解码后的内容**替换**请求 body，转发给上游 | 保持原始请求不变 |
| PRE | `request.headers` | 用返回的 headers **替换**转发时的 HTTP 头 | 保持原始 headers |
| PRE | `response.status_code` | **终止请求**，不转发到上游，直接用该状态码返回给客户端 | 正常转发到上游 |
| PRE | `response.body` | 终止时返回给客户端的响应 body（需配合 `status_code` 使用） | — |
| PRE | `response.headers` | 终止时返回给客户端的响应 headers（需配合 `status_code` 使用） | — |
| POST | `response.body` | 用解码后的内容**替换**返回给客户端的响应 body | 保持上游原始响应 |
| POST | `response.headers` | 用返回的 headers **替换**返回给客户端的响应 headers | 保持上游原始 headers |
| POST | `response.status_code` | 用该状态码**替换**返回给客户端的 HTTP 状态码 | 保持默认状态码 |

> `body` 字段始终使用 **Base64 编码**。空字符串 `""`、`null` 或省略字段均表示不修改。
>
> `status_code` 为 `0`、`null` 或省略均表示不修改。PRE 阶段设置 `status_code` 会终止请求链路。

## Hook 配置参考

Hook 通过 agentrun API 的 `mcpConfig.mcpProxyConfiguration.hooks` 数组配置，每个元素对应一个 Hook 规则：

```json
{
  "url": "https://hook-service.example.com/hook",
  "description": "Hook 描述",
  "headers": {
    "X-Auth-Token": "your-secret-token"
  },
  "enabled": true,
  "timeout": 5000,
  "event": "POST_LIST_TOOLS"
}
```

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `url` | string | 是 | Hook 服务 URL |
| `event` | string | 是 | 触发事件：`PRE_LIST_TOOLS` / `POST_LIST_TOOLS` / `PRE_CALL_TOOL` / `POST_CALL_TOOL` |
| `enabled` | bool | 否 | 是否启用 |
| `timeout` | int32 | 否 | 超时时间（毫秒），默认 5000 |
| `description` | string | 否 | 描述信息 |
| `headers` | map<string, string> | 否 | 调用 Hook 时附加的 HTTP 头（可用于认证） |

同一事件可以配置多个 Hook，按数组顺序依次执行，前一个 Hook 的修改会传递给下一个。

## 常见场景

### 修改工具列表

注册 `POST_LIST_TOOLS` 事件，解码 `response.body`，修改 JSON-RPC 响应中的 `result.tools` 数组，编码后写入返回的 `response.body`。

### 修改工具调用结果

注册 `POST_CALL_TOOL` 事件，解码 `response.body`，修改 `result.content` 中的文本内容。

### 拦截和修改请求参数

注册 `PRE_CALL_TOOL` 事件，解码 `request.body`，修改 JSON-RPC 请求中的 `params`，编码后写入返回的 `request.body`。

### 凭证转换

注册 `PRE_CALL_TOOL` 事件，从 `request.headers` 中读取客户端凭证，转换为后端凭证后写入返回的 `request.headers`。agentrun 会使用返回的 headers 替代原始 headers 转发给上游。

### 在 PRE 阶段终止请求

注册 `PRE_CALL_TOOL` 事件，根据业务逻辑决定是否拦截请求。返回 `response.status_code`（非零值）即可终止请求，agentrun 不会将请求转发到上游，而是直接用 `response.body` 和 `response.status_code` 返回给客户端。

```go
// 示例：拦截特定工具调用，直接返回结果
if toolName == "blocked_tool" {
    rpcResp := map[string]interface{}{
        "jsonrpc": "2.0",
        "id":      rpcID,
        "result": map[string]interface{}{
            "content": []interface{}{
                map[string]interface{}{"type": "text", "text": "此工具已被拦截"},
            },
        },
    }
    respBytes, _ := json.Marshal(rpcResp)
    return HookPayload{
        Response: HookMessage{
            Body:       base64.StdEncoding.EncodeToString(respBytes),
            StatusCode: 200,
        },
    }
}
```

### 在 POST 阶段修改状态码

注册 `POST_CALL_TOOL` 事件，Hook 服务可以通过 `payload.Response.StatusCode` 获取上游的 HTTP 状态码，并通过在返回中设置 `response.status_code` 来修改返回给客户端的状态码。

## 错误处理

agentrun 采用 **fail-close** 策略：

- Hook 服务返回非 200 状态码 → 请求失败，返回 502
- Hook 服务超时 → 请求失败，返回 502
- Hook 服务不可达 → 请求失败，返回 502

确保 Hook 服务的高可用性，否则会影响所有经过代理的 MCP 请求。
