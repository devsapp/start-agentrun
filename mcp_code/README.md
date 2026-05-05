# mcp_code：给代码包 MCP 工具添加 Hook

这个示例演示 MCP 服务需要托管在 AgentRun 代码包中时，如何同时开启 MCP proxy 并配置 Hook。

运行后会创建一个 `CODE_PACKAGE` 工具，并配置：

- `proxyEnabled=true`
- `mcpProxyConfiguration.hooks[0].event=POST_CALL_TOOL`
- Hook URL 指向示例的 `services/userhook` 服务
- 代码包来自 `services/orderdesk` 编译后的可执行文件

客户端仍然访问 AgentRun 数据面地址。AgentRun 会先调用代码包 MCP 服务，再把 `tools/call` 的上游响应交给 Hook 服务改写。

## 示例链路

```text
客户端
  -> AgentRun 数据面 /tools/<tool>/mcp
  -> AgentRun MCP proxy
  -> AgentRun 托管的代码包 MCP 服务 services/orderdesk
  -> AgentRun 调用 Hook 服务 services/userhook
  -> AgentRun 返回脱敏后的订单结果
```

## 运行前准备

需要本机已安装 Go，并准备有权限调用 AgentRun、FC 和 FC TempBucket 的阿里云账号凭证。

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

1. 构建 `services/orderdesk`。
2. 将 `orderdesk` 打包为代码包并上传到 FC TempBucket。
3. 构建并部署 `services/userhook` 作为 Hook 服务。
4. 创建 `CODE_PACKAGE` AgentRun 工具，并写入 Hook 配置。
5. 等待主函数和 proxy 函数就绪。
6. 访问 AgentRun 数据面验证 Hook 效果。

成功后会输出类似信息：

```text
tool=hook-quickstart-code-...
tool_id=...
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

## 改成自己的代码包 MCP Hook

按这个顺序改：

1. 把 `services/orderdesk` 换成你的 MCP 服务，保持监听 `9000` 并暴露 `/mcp`。
2. 在 `services/userhook` 中实现自己的 Hook 逻辑。
3. 在 `internal/demo/tool.go` 的 `newCodeToolInput` 中确认代码包启动命令、运行端口和语言配置。
4. 在 `internal/demo/tool.go` 的 `buildHooks` 中调整 Hook 事件、描述、超时和认证 headers。
5. 重新运行 `go run .`，通过 `data_plane` 地址验证最终效果。

更完整的 Hook 协议和配置字段见 [../docs/users/hook.md](../docs/users/hook.md)。
