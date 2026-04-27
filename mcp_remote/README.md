# mcp_remote

这个模块演示：

- 先把 `services/calculator` 部署成远程 MCP 服务
- 再把 `services/userhook` 部署成 Hook 回调服务
- 然后创建 `MCP_REMOTE + proxyEnabled + hooks`
- 最后从 AgentRun 数据面验证 Hook 是否生效

## 运行

先准备当前模块的 `.env`：

```bash
cp .env.example .env
go run .
```

默认使用生产 endpoint：

- `AGENTRUN_CONTROL_ENDPOINT=agentrun.cn-hangzhou.aliyuncs.com`
- `AGENTRUN_DATA_ENDPOINT=<ALIBABA_CLOUD_UID>.agentrun-data.cn-hangzhou.aliyuncs.com`

如果要验证其他环境，在 `.env` 中改这两个 endpoint。

跑通后会验证：

1. `tools/list` 出现 `hook_info`
2. `multiply(2,3)` 变成 `121`
3. `add(1,2)` 带上 `[hooked:200]`

## 基于 services 快速上手

如果你想改成自己的最小示例，优先改这两个目录：

- `services/calculator`
  - 这里是远程 MCP 服务
  - 你可以直接把 `add`、`multiply` 换成自己的工具
  - 只要继续暴露 `/mcp`，quickstart 主流程不用改
- `services/userhook`
  - 这里是 Hook 服务
  - 现在实现了 `POST_LIST_TOOLS`、`PRE_CALL_TOOL`、`POST_CALL_TOOL`
  - 你可以把示例改成自己的参数改写、审计、鉴权或结果脱敏逻辑
