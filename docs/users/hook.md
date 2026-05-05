# AgentRun MCP Tool Hook 使用指南

AgentRun MCP Tool Hook 是 AgentRun MCP 代理上的 HTTP 回调机制。开启 MCP proxy 后，客户端仍然访问 AgentRun 数据面；AgentRun 会在 `tools/list` 或 `tools/call` 的前后调用你的 Hook 服务，让你有机会改写请求、改写响应或直接拦截调用。

Hook 不是一个新工具，也不是替代 MCP 服务的业务实现。它适合处理横切逻辑，例如权限校验、凭证转换、审计编号、结果脱敏、工具列表调整等。真正的业务能力仍应放在 MCP 服务、Function Call 工具或 Skill 中。

## 一句话流程

```text
客户端 -> AgentRun 数据面 -> AgentRun MCP proxy -> 上游 MCP 服务 -> AgentRun 调用 Hook 服务 -> AgentRun 返回客户端
```

PRE Hook 在请求转发到上游 MCP 服务前执行，POST Hook 在上游 MCP 服务返回后执行。Hook 服务只返回需要改写的字段；空字段表示不修改。

## 什么时候用 Hook

| 需求 | 推荐事件 | 常改写字段 |
|------|----------|------------|
| 添加、删除或重命名工具 | `POST_LIST_TOOLS` | `response.body` 中的 `result.tools` |
| 补充或校验工具调用参数 | `PRE_CALL_TOOL` | `request.body` |
| 把客户端凭证换成后端凭证 | `PRE_CALL_TOOL` | `request.headers` |
| 按业务规则拦截某次调用 | `PRE_CALL_TOOL` | `response.status_code`、`response.body` |
| 对工具返回结果做脱敏 | `POST_CALL_TOOL` | `response.body` |
| 给结果追加审计编号 | `POST_CALL_TOOL` | `response.body` 或 `response.headers` |

如果只是想给 Agent 增加新能力，优先使用 Skills、MCP 工具、Function Call 工具或工具市场。只有需要“拦截并改写已有 MCP 请求或响应”时，才需要 Hook。

## 先跑通仓库示例

这个仓库提供两个示例：

| 示例 | 适合场景 | 说明 |
|------|----------|------|
| [mcp_remote](../../mcp_remote/README.md) | 已有远程 MCP 服务 | 创建 `MCP_REMOTE + proxyEnabled + hooks` 工具 |
| [mcp_code](../../mcp_code/README.md) | MCP 服务作为代码包托管 | 创建 `CODE_PACKAGE + proxyEnabled + hooks` 工具 |

初次使用建议先跑 `mcp_remote`：

```bash
cd mcp_remote
cp .env.example .env
go run .
```

`.env` 中填写：

```dotenv
ALIBABA_CLOUD_UID=你的阿里云账号 UID
ALIBABA_CLOUD_ACCESS_KEY_ID=你的 AccessKey ID
ALIBABA_CLOUD_ACCESS_KEY_SECRET=你的 AccessKey Secret
```

成功后会输出 `tool`、`hook`、`data_plane`、`tools`、`order` 等信息。示例会验证：

1. `tools/list` 能看到 `get_order`。
2. `get_order(ORDER-1001)` 能返回订单详情。
3. `POST_CALL_TOOL` Hook 会把手机号、邮箱、收货地址脱敏，并注入 `audit_id`。

## 接入步骤

1. 选择 Hook 事件：修改请求用 PRE，修改响应用 POST。
2. 编写 Hook 服务：接收 AgentRun 的 JSON 回调，解码 Base64 body，结构化解析 JSON-RPC，再返回改写字段。
3. 部署 Hook 服务：确保 AgentRun 可以访问 Hook URL。
4. 创建或更新 MCP 工具：开启 `proxyEnabled`，并配置 `mcpProxyConfiguration.hooks`。
5. 通过 AgentRun 数据面调用工具，确认客户端看到的是 Hook 改写后的结果。

## Hook 事件

AgentRun 只对两类 MCP 方法触发 Hook：

| MCP 方法 | PRE 事件 | POST 事件 |
|----------|----------|-----------|
| `tools/list` | `PRE_LIST_TOOLS` | `POST_LIST_TOOLS` |
| `tools/call` | `PRE_CALL_TOOL` | `POST_CALL_TOOL` |

- PRE 阶段：上游 MCP 服务调用前执行，可以改写请求，也可以直接终止请求。
- POST 阶段：上游 MCP 服务返回后执行，可以改写最终返回给客户端的响应。
- 其他 MCP 方法，如 `initialize`，直接透传，不触发 Hook。

## Hook 服务协议

Hook 服务是一个普通 HTTP 服务：

- 接收 `POST` 请求。
- 请求和响应都是 JSON。
- `request.body` 和 `response.body` 都是 Base64 编码的原始 JSON-RPC 内容。
- Hook 服务应返回 HTTP 200。返回非 200、超时或不可达时，本次 MCP 请求会失败。

AgentRun 发给 Hook 服务的请求示例：

```json
{
  "event": "POST_CALL_TOOL",
  "toolname": "orderdesk",
  "region": "cn-hangzhou",
  "request": {
    "headers": {
      "Content-Type": "application/json",
      "Mcp-Session-Id": "xxx"
    },
    "body": "<Base64 编码的客户端请求 body>"
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

字段说明：

| 字段 | 说明 |
|------|------|
| `event` | 当前 Hook 事件 |
| `toolname` | AgentRun 工具名称 |
| `region` | 工具所在地域 |
| `request.headers` | 客户端请求头 |
| `request.body` | 客户端 JSON-RPC 请求体的 Base64 编码 |
| `response.headers` | 上游响应头，POST 事件有值 |
| `response.body` | 上游 JSON-RPC 响应体的 Base64 编码，POST 事件有值 |
| `response.status_code` | 上游 HTTP 状态码，POST 事件有值 |

Hook 服务返回示例：

```json
{
  "request": {
    "headers": {},
    "body": ""
  },
  "response": {
    "headers": {},
    "body": "<Base64 编码的改写后响应 body>",
    "status_code": 0
  }
}
```

返回规则：

| 阶段 | 返回字段 | 非空或非零时 | 为空或为零时 |
|------|----------|--------------|--------------|
| PRE | `request.body` | 替换发往上游的请求 body | 保持原请求 body |
| PRE | `request.headers` | 替换发往上游的请求 headers | 保持原请求 headers |
| PRE | `response.status_code` | 终止请求，不再调用上游 | 继续调用上游 |
| PRE | `response.body` | 终止时返回给客户端的 body | 不单独生效 |
| PRE | `response.headers` | 终止时返回给客户端的 headers | 不单独生效 |
| POST | `response.body` | 替换返回给客户端的响应 body | 保持上游响应 body |
| POST | `response.headers` | 替换返回给客户端的响应 headers | 保持上游响应 headers |
| POST | `response.status_code` | 替换返回给客户端的 HTTP 状态码 | 保持默认状态码 |

`body` 字段必须是 Base64 编码。空字符串、`null` 或省略字段都表示不修改。没有改写需求时，可以返回空 `request` 和空 `response`。

## Hook 配置

创建或更新 MCP 工具时，在 `mcpConfig` 中开启 proxy 并配置 Hook：

```json
{
  "proxyEnabled": true,
  "mcpProxyConfiguration": {
    "hooks": [
      {
        "url": "https://your-hook-service.example.com/hook",
        "description": "脱敏工具调用结果并注入审计编号",
        "event": "POST_CALL_TOOL",
        "enabled": true,
        "timeout": 5000,
        "headers": {
          "X-Hook-Token": "your-secret-token"
        }
      }
    ]
  }
}
```

字段说明：

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `url` | string | 是 | Hook 服务 URL |
| `event` | string | 是 | `PRE_LIST_TOOLS`、`POST_LIST_TOOLS`、`PRE_CALL_TOOL` 或 `POST_CALL_TOOL` |
| `enabled` | bool | 否 | 是否启用 |
| `timeout` | int32 | 否 | 超时时间，单位毫秒，默认 5000 |
| `description` | string | 否 | Hook 描述 |
| `headers` | map<string, string> | 否 | AgentRun 调用 Hook 服务时附加的请求头，常用于认证 |

同一事件可以配置多个 Hook，按数组顺序依次执行。前一个 Hook 的改写结果会传递给下一个 Hook。

## 本仓库示例代码

两个快速入门示例都包含这两个服务：

- `services/orderdesk`：示例 MCP 服务，提供 `get_order`。
- `services/userhook`：示例 Hook 服务，只处理 `POST_CALL_TOOL`，对订单结果脱敏并注入 `audit_id`。

把示例改成自己的 Hook 时，通常只需要改：

1. `services/orderdesk`：替换为你的业务 MCP 工具。
2. `services/userhook`：替换为你的审计、脱敏、拦截或凭证转换逻辑。
3. `internal/demo/tool.go` 的 `buildHooks`：调整事件、URL、超时、headers。

## 注意事项

- Hook 服务失败会影响本次 MCP 请求，生产环境要保证 Hook 服务可用。
- 只返回需要改写的字段，避免无意义地重写完整请求或响应。
- 对 Base64 解码后的 JSON-RPC 内容做结构化解析，不要依赖字符串替换。
- 如果 Hook URL 需要认证，使用 `headers` 配置固定密钥，并在 Hook 服务端校验。
- 密钥只放在 `.env` 中，不要提交到代码仓库。
