# mcp_code

这个模块演示：

- 先把 `services/orderdesk` 编译并打成 `CODE_PACKAGE`
- 把 `services/userhook` 部署成 Hook 回调服务
- 通过 FC TempBucket 上传代码包到 OSS，再用 `ossBucketName + ossObjectName` 创建 `CODE_PACKAGE + proxyEnabled + hooks`
- 从 AgentRun 数据面验证主函数和 proxy 函数都已创建，且订单结果脱敏和 `audit_id` 注入生效

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

如果你想基于这个模块改自己的代码包工具，优先改：

- `services/orderdesk`
  - 这里就是会被打包上传的订单查询 MCP 服务
  - 直接替换成你的业务工具实现即可
  - 只要继续监听 `9000` 并提供 `/mcp`，主流程基本不用改
- `services/userhook`
  - 这里是 Hook 回调服务
  - 当前只处理 `POST_CALL_TOOL`，用于结果脱敏和 `audit_id` 注入
