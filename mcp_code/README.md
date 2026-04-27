# mcp_code

这个模块演示：

- 先把 `services/calculator` 编译并打成 `CODE_PACKAGE`
- 把 `services/userhook` 部署成 Hook 回调服务
- 通过 FC TempBucket 上传代码包到 OSS，再用 `ossBucketName + ossObjectName` 创建 `CODE_PACKAGE + proxyEnabled + hooks`
- 从 AgentRun 数据面验证主函数和 proxy 函数都已创建，且 Hook 生效

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

如果你想基于这个模块改自己的代码包工具，优先改：

- `services/calculator`
  - 这里就是会被打包上传的 MCP 服务
  - 直接替换成你的工具实现即可
  - 只要继续监听 `9000` 并提供 `/mcp`，主流程基本不用改
- `services/userhook`
  - 这里是 Hook 回调服务
  - 可以继续保留现在的三类事件，也可以只保留你需要的 Hook
