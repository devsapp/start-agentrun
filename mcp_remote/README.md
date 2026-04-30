# mcp_remote

这个模块演示：

- 先把 `services/orderdesk` 部署成远程 MCP 服务
- 再把 `services/userhook` 部署成 Hook 回调服务
- 然后创建 `MCP_REMOTE + proxyEnabled + hooks`
- 最后从 AgentRun 数据面验证订单结果脱敏和 `audit_id` 注入是否生效

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

1. `tools/list` 出现 `get_order`
2. `get_order(ORDER-1001)` 返回订单详情
3. Hook 把手机号、邮箱、收货地址脱敏，并注入 `audit_id`

## 基于 services 快速上手

如果你想改成自己的最小示例，优先改这两个目录：

- `services/orderdesk`
  - 这里是远程订单查询 MCP 服务
  - 你可以直接把 `get_order` 换成自己的业务工具
  - 只要继续暴露 `/mcp`，quickstart 主流程不用改
- `services/userhook`
  - 这里是 Hook 服务
  - 当前只处理 `POST_CALL_TOOL`
  - 你可以把示例改成自己的审计或结果脱敏逻辑
