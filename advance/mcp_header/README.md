# advance/mcp_header：独立验证 MCP Hook 改写 header

这个 case 只验证 header，不验证订单脱敏。

运行后会创建一个 `MCP_REMOTE` 工具，并配置：

- `proxyEnabled=true`
- `mcpProxyConfiguration.hooks[0].event=PRE_CALL_TOOL`
- `mcpProxyConfiguration.hooks[1].event=POST_CALL_TOOL`
- Hook URL 指向示例的 `services/userhook` 服务

验证目标：

1. `PRE_CALL_TOOL` Hook 可以修改发送给上游 MCP tool 的请求头。
2. `POST_CALL_TOOL` Hook 可以修改最终返回给客户端的响应头。

## 示例链路

```text
客户端
  -> AgentRun 数据面 /tools/<tool>/mcp
  -> AgentRun MCP proxy
  -> PRE_CALL_TOOL Hook 改写请求头
  -> 远程 header 调试 MCP 服务 services/headerdesk
  -> POST_CALL_TOOL Hook 改写响应头
  -> AgentRun 返回带调试响应头的结果
```

`services/headerdesk` 只提供一个 `debug_headers` 工具。工具会返回自己实际收到的 `X-Debug-Request-Header`，用于确认 PRE Hook 是否真的改写了上游请求头。

## 运行前准备

需要本机已安装 Go，并准备有权限调用 AgentRun 和 FC 的阿里云账号凭证。

创建 `.env`：

```bash
cp .env.example .env
```

填写：

```dotenv
ALIBABA_CLOUD_UID=你的阿里云账号 UID
ALIBABA_CLOUD_ACCESS_KEY_ID=你的 AccessKey ID
ALIBABA_CLOUD_ACCESS_KEY_SECRET=你的 AccessKey Secret
```

## 运行

```bash
go run .
```

成功后会输出类似信息：

```text
tool=hook-quickstart-header-...
tool_id=...
remote_mcp=https://...
hook=https://.../hook
data_plane=https://.../tools/<tool>/mcp
debug_request_header=pre-call-tool-upstream
debug_response_header=post-call-tool-client
```

## 验证内容

示例会自动完成：

1. 构建并部署 `services/headerdesk`。
2. 构建并部署 `services/userhook`。
3. 创建 `MCP_REMOTE + proxyEnabled + hooks` 工具。
4. 发起原始 MCP HTTP 调用，并在客户端请求中带上 `X-Debug-Request-Header=client-before-hook`。
5. 验证上游工具实际收到 `X-Debug-Request-Header=pre-call-tool-upstream`。
6. 验证客户端响应头包含 `X-Debug-Response-Header=post-call-tool-client`。

这里使用原始 HTTP 调用做验证，是因为标准 MCP client 通常不会暴露传输层响应头；订单脱敏示例仍使用标准 MCP client 验证工具调用结果。

示例不会自动清理测试资源。创建的 AgentRun Tool、远程 MCP FC 函数和 Hook FC 函数会保留，便于继续调试；不再需要时请到控制台手动删除。
