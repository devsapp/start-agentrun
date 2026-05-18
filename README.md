# AgentRun MCP Hook 快速入门

这个仓库演示 AgentRun MCP Tool Hook 的完整使用方式：不修改原 MCP 服务，通过 AgentRun MCP 代理在 `tools/list`、`tools/call` 前后调用你的 Hook 服务，从而改写工具列表、工具调用请求或工具调用结果。

示例使用一个订单查询 MCP 服务和一个 Hook 服务。客户端调用 `get_order` 后，上游 MCP 服务返回原始订单信息，Hook 服务再对手机号、邮箱、收货地址做脱敏，并注入 `audit_id`。

## 你会了解什么

- AgentRun MCP Hook 是什么，适合解决哪些问题。
- 如何编写一个 HTTP Hook 服务。
- 如何在 AgentRun MCP 工具中开启 `proxyEnabled` 并配置 `hooks`。
- 如何通过 AgentRun 数据面验证 Hook 是否生效。

## 先读哪份文档

| 文档 | 内容 |
|------|------|
| [docs/users/hook.md](docs/users/hook.md) | Hook 概念、事件、协议、配置字段和改造步骤 |
| [mcp_remote/README.md](mcp_remote/README.md) | 已有远程 MCP 服务时，如何加 Hook |
| [mcp_code/README.md](mcp_code/README.md) | MCP 服务作为代码包托管时，如何加 Hook |
| [advance/mcp_header/README.md](advance/mcp_header/README.md) | 独立验证 PRE/POST Hook 改写 header |

初次使用建议先跑 `mcp_remote`。它更接近“已有远程 MCP 服务，只想通过 AgentRun 加一层 Hook”的场景。

## 示例目录

| 目录 | 用途 |
|------|------|
| `mcp_remote/` | 部署远程 MCP 服务和 Hook 服务，创建 `MCP_REMOTE + proxyEnabled + hooks` 工具 |
| `mcp_code/` | 打包 MCP 服务为代码包，创建 `CODE_PACKAGE + proxyEnabled + hooks` 工具 |
| `advance/mcp_header/` | 单独验证 `PRE_CALL_TOOL` 改写上游请求头、`POST_CALL_TOOL` 改写客户端响应头 |

`mcp_remote` 和 `mcp_code` 两个基础示例里的核心服务一致：

- `services/orderdesk`：订单查询 MCP 服务，提供 `get_order`。
- `services/userhook`：Hook 回调服务，处理 `POST_CALL_TOOL`，对订单结果脱敏并注入审计编号。

`advance/mcp_header` 是独立调试 case，只提供 `debug_headers` 工具，用于验证请求头和响应头改写。

## 快速运行

准备当前示例目录的 `.env`：

```bash
cd mcp_remote
cp .env.example .env
```

填写阿里云账号信息：

```dotenv
ALIBABA_CLOUD_UID=你的阿里云账号 UID
ALIBABA_CLOUD_ACCESS_KEY_ID=你的 AccessKey ID
ALIBABA_CLOUD_ACCESS_KEY_SECRET=你的 AccessKey Secret
```

运行示例：

```bash
go run .
```

成功后会输出 `tool`、`hook`、`data_plane`、`tools`、`order` 等信息。其中 `order` 应包含脱敏后的订单信息和 `audit_id`。

## 运行结果说明

示例会自动完成以下动作：

1. 构建并部署示例 MCP 服务。
2. 构建并部署 Hook 服务。
3. 创建开启 MCP proxy 的 AgentRun MCP 工具。
4. 在 `mcpProxyConfiguration.hooks` 中配置 `POST_CALL_TOOL` Hook。
5. 通过 AgentRun 数据面调用 `tools/list` 和 `get_order(ORDER-1001)`。
6. 验证结果已脱敏，并包含 `audit_id`。

示例不会自动清理测试资源。运行完成后，创建的 AgentRun Tool、FC 函数和代码包等测试资源会保留，便于继续调试；不再需要时请到控制台手动删除。

## 密钥和生成文件

- 密钥只放在 `.env` 中，不要提交到代码仓库。
- `.env` 和 `.bin/` 已通过 `.gitignore` 排除。
- 示例会优先加载仓库根目录 `.env`，再加载当前模块 `.env`；当前模块配置会覆盖根目录配置。
