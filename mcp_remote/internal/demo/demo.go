package demo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	agentrun "github.com/alibabacloud-go/agentrun-20250910/v5/client"
	fc "github.com/alibabacloud-go/fc-20230330/v4/client"
	"github.com/alibabacloud-go/tea/tea"
)

const (
	region                 = "cn-hangzhou"
	defaultProto           = "https"
	defaultControlEndpoint = "agentrun.cn-hangzhou.aliyuncs.com"
	signPrefix             = "aliyun_v4"
	signAlgorithm          = "AGENTRUN4-HMAC-SHA256"
	warmupBody             = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","clientInfo":{"name":"warmup","version":"1.0.0"}}}`
	initializeBody         = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","clientInfo":{"name":"probe","version":"1.0.0"}}}`
)

type demoContext struct {
	ctx          context.Context
	rootDir      string
	moduleDir    string
	binDir       string
	proto        string
	dataEndpoint string
	uid          string
	ak           string
	sk           string
	sdkClient    *agentrun.Client
	fcClient     *fc.Client
	toolName     string
	mcpFuncName  string
	hookFuncName string
}

type hookObservation struct {
	toolNames    []string
	orderResult  string
	orderMasked  bool
	auditIDFound bool
}

func Run() error {
	ctx := context.Background()
	moduleDir, rootDir, err := locateDirs()
	if err != nil {
		return err
	}
	loadEnv(rootDir, moduleDir)

	proto := strings.TrimSpace(strings.ToLower(os.Getenv("AGENTRUN_PROTO")))
	if proto == "" {
		proto = defaultProto
	}
	if proto != "http" && proto != "https" {
		return fmt.Errorf("AGENTRUN_PROTO 仅支持 http/https，实际为 %q", proto)
	}

	uid := strings.TrimSpace(os.Getenv("ALIBABA_CLOUD_UID"))
	ak := strings.TrimSpace(os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_ID"))
	sk := strings.TrimSpace(os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET"))
	if uid == "" || ak == "" || sk == "" {
		return errors.New("缺少阿里云凭证，请先配置当前模块的 .env 或环境变量")
	}

	controlEndpoint := endpointHost(os.Getenv("AGENTRUN_CONTROL_ENDPOINT"), defaultControlEndpoint)
	dataEndpoint := endpointHost(os.Getenv("AGENTRUN_DATA_ENDPOINT"), defaultDataEndpoint(uid))
	configureNoProxy(uid, proto, dataEndpoint)

	sdkClient, err := newSDKClient(controlEndpoint, proto, ak, sk)
	if err != nil {
		return err
	}
	fcClient, err := newFCClient(proto, uid, ak, sk)
	if err != nil {
		return err
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	binDir := filepath.Join(moduleDir, ".bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("创建 bin 目录失败: %w", err)
	}

	dctx := &demoContext{
		ctx:          ctx,
		rootDir:      rootDir,
		moduleDir:    moduleDir,
		binDir:       binDir,
		proto:        proto,
		dataEndpoint: dataEndpoint,
		uid:          uid,
		ak:           ak,
		sk:           sk,
		sdkClient:    sdkClient,
		fcClient:     fcClient,
		toolName:     "hook-quickstart-remote-" + suffix,
		mcpFuncName:  "hook-qs-remote-mcp-" + suffix,
		hookFuncName: "hook-qs-remote-hook-" + suffix,
	}

	return dctx.run()
}

func (d *demoContext) run() error {
	slog.Info("开始运行 MCP_REMOTE Hook quickstart", "tool", d.toolName, "dataEndpoint", d.dataEndpoint)

	var createdTool bool
	defer func() {
		if createdTool {
			if err := deleteTool(d.sdkClient, d.toolName); err != nil {
				slog.Warn("清理 tool 失败", "tool", d.toolName, "error", err)
			}
		}
	}()

	createdFunctions := []string{}
	defer func() {
		for i := len(createdFunctions) - 1; i >= 0; i-- {
			if err := deleteFunction(d.fcClient, createdFunctions[i]); err != nil {
				slog.Warn("清理 FC 函数失败", "function", createdFunctions[i], "error", err)
			}
		}
	}()

	if err := buildBinary("orderdesk", filepath.Join(d.moduleDir, "services", "orderdesk"), filepath.Join(d.binDir, "orderdesk")); err != nil {
		return err
	}
	if err := buildBinary("userhook", filepath.Join(d.moduleDir, "services", "userhook"), filepath.Join(d.binDir, "userhook")); err != nil {
		return err
	}

	mcpURL, err := deployBinaryAsFunction(d.ctx, d.fcClient, d.uid, filepath.Join(d.binDir, "orderdesk"), "orderdesk", d.mcpFuncName)
	if err != nil {
		return err
	}
	createdFunctions = append(createdFunctions, d.mcpFuncName)
	slog.Info("远程 MCP 函数已部署", "url", mcpURL)
	if err := warmupFunction(d.ctx, mcpURL, 2*time.Minute); err != nil {
		return fmt.Errorf("预热远程 MCP 失败: %w", err)
	}

	hookURL, err := deployBinaryAsFunction(d.ctx, d.fcClient, d.uid, filepath.Join(d.binDir, "userhook"), "userhook", d.hookFuncName)
	if err != nil {
		return err
	}
	createdFunctions = append(createdFunctions, d.hookFuncName)
	slog.Info("Hook 函数已部署", "url", hookURL)

	remoteURL := remoteMCPConfigURL(mcpURL)
	hookEndpoint := strings.TrimSuffix(hookURL, "/") + "/hook"
	if err := createRemoteTool(d.sdkClient, d.toolName, remoteURL, hookEndpoint); err != nil {
		return err
	}
	createdTool = true

	if err := waitForToolReady(d.sdkClient, d.toolName, 5*time.Minute); err != nil {
		return err
	}

	toolID, err := d.assertRemoteFunctions()
	if err != nil {
		return err
	}

	obs, mcpEndpoint, err := d.verifyHookEffects(3)
	if err != nil {
		return err
	}

	slog.Info("MCP_REMOTE Hook quickstart 通过", "tool", d.toolName, "toolId", toolID)
	fmt.Printf("tool=%s\n", d.toolName)
	fmt.Printf("tool_id=%s\n", toolID)
	fmt.Printf("remote_mcp=%s\n", remoteURL)
	fmt.Printf("hook=%s\n", hookEndpoint)
	fmt.Printf("data_plane=%s\n", mcpEndpoint)
	fmt.Printf("tools=%s\n", strings.Join(obs.toolNames, ","))
	fmt.Printf("order=%s\n", obs.orderResult)
	return nil
}

func (d *demoContext) assertRemoteFunctions() (string, error) {
	toolDetail, err := getToolDetail(d.sdkClient, d.toolName)
	if err != nil {
		return "", err
	}
	toolID := tea.StringValue(toolDetail.ToolId)
	if toolID == "" {
		toolID = d.toolName
	}

	storedProxy := false
	storedHooks := 0
	if toolDetail.McpConfig != nil {
		storedProxy = tea.BoolValue(toolDetail.McpConfig.ProxyEnabled)
		if cfg := toolDetail.McpConfig.McpProxyConfiguration; cfg != nil && cfg.Hooks != nil {
			storedHooks = len(cfg.Hooks)
		}
	}
	if !storedProxy || storedHooks != 1 {
		return "", fmt.Errorf("后端存储的 hook 配置不符合预期: proxyEnabled=%v hooks=%d", storedProxy, storedHooks)
	}

	proxyFunc := "agentrun-proxy-" + toolID
	if err := waitForFunction(d.fcClient, proxyFunc, 5*time.Minute); err != nil {
		return "", fmt.Errorf("等待 proxy 函数失败: %w", err)
	}
	return toolID, nil
}
