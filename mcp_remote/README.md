# mcp_remote：给远程 MCP 服务添加 Hook

这个示例演示已有远程 MCP 服务时，如何通过 AgentRun MCP proxy 添加 Hook。

运行后会创建一个 `MCP_REMOTE` 工具，并配置：

- `proxyEnabled=true`
- `mcpProxyConfiguration.hooks[0].event=POST_CALL_TOOL`
- Hook URL 指向示例的 `services/userhook` 服务

客户端仍然访问 AgentRun 数据面地址。AgentRun 会把 `tools/call` 的上游响应发给 Hook 服务，Hook 服务改写响应后再返回给客户端。

## 示例链路

```text
客户端
  -> AgentRun 数据面 /tools/<tool>/mcp
  -> AgentRun MCP proxy
  -> 远程订单查询 MCP 服务 services/orderdesk
  -> AgentRun 调用 Hook 服务 services/userhook
  -> AgentRun 返回脱敏后的订单结果
```

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

示例会自动：

1. 构建 `services/orderdesk`，部署为远程 MCP 服务。
2. 构建 `services/userhook`，部署为 Hook 服务。
3. 创建 `MCP_REMOTE` AgentRun 工具，并写入 Hook 配置。
4. 等待工具和 proxy 函数就绪。
5. 访问 AgentRun 数据面验证 Hook 效果。

成功后会输出类似信息：

```text
tool=hook-quickstart-remote-...
tool_id=...
remote_mcp=https://...
hook=https://.../hook
data_plane=https://.../tools/<tool>/mcp
tools=get_order
order={"audit_id":"audit_...","phone":"138****5678",...}
```

## 验证内容

示例会验证三件事：

1. `tools/list` 能看到 `get_order`。
2. `get_order` 能查询 `ORDER-1001`。
3. `POST_CALL_TOOL` Hook 已把手机号、邮箱、收货地址脱敏，并注入 `audit_id`。

## 改成自己的远程 MCP Hook

按这个顺序改：

1. 把 `services/orderdesk` 换成你的 MCP 服务，继续暴露 `/mcp`。
2. 在 `services/userhook` 中实现自己的 Hook 逻辑。
3. 在 `internal/demo/tool.go` 的 `buildHooks` 中调整 Hook 事件、描述、超时和认证 headers。
4. 重新运行 `go run .`，通过 `data_plane` 地址验证最终效果。

创建远程 MCP 工具时，写入控制面的 `protocolSpec` 使用 `mcpServers.default.transportType=streamable-http`，并将 `mcpServers.default.url` 指向远程服务的 `/mcp` 端点；访问数据面时使用 AgentRun 生成的 `.../tools/<tool>/mcp`。

更完整的 Hook 协议和配置字段见 [../docs/users/hook.md](../docs/users/hook.md)。
