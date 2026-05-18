package demo

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	agentrun "github.com/alibabacloud-go/agentrun-20250910/v5/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	fc "github.com/alibabacloud-go/fc-20230330/v4/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/joho/godotenv"
)

// locateDirs 定位当前模块目录和仓库根目录。
func locateDirs() (moduleDir, rootDir string, err error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", "", errors.New("runtime.Caller 失败")
	}
	moduleDir = filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	rootDir = filepath.Clean(filepath.Join(moduleDir, ".."))
	return moduleDir, rootDir, nil
}

// loadEnv 加载仓库根目录和当前模块的环境变量文件。
// 参数 rootDir 是仓库根目录；参数 moduleDir 是当前模块目录。
func loadEnv(rootDir, moduleDir string) {
	_ = godotenv.Load(filepath.Join(rootDir, ".env"))
	_ = godotenv.Overload(filepath.Join(moduleDir, ".env"))
}

// newSDKClient 创建 AgentRun 控制面客户端。
// 参数 controlEndpoint 是控制面 endpoint；参数 proto 是访问协议；参数 ak 是 AccessKey ID；参数 sk 是 AccessKey Secret。
func newSDKClient(controlEndpoint, proto, ak, sk string) (*agentrun.Client, error) {
	config := &openapi.Config{
		AccessKeyId:     tea.String(ak),
		AccessKeySecret: tea.String(sk),
		Endpoint:        tea.String(controlEndpoint),
		Protocol:        tea.String(strings.ToUpper(proto)),
	}
	return agentrun.NewClient(config)
}

// newFCClient 创建 FC 客户端。
// 参数 proto 是访问协议；参数 uid 是阿里云账号 UID；参数 ak 是 AccessKey ID；参数 sk 是 AccessKey Secret。
func newFCClient(proto, uid, ak, sk string) (*fc.Client, error) {
	config := &openapi.Config{
		AccessKeyId:     tea.String(ak),
		AccessKeySecret: tea.String(sk),
		Endpoint:        tea.String(fmt.Sprintf("%s.%s.fc.aliyuncs.com", uid, region)),
		Protocol:        tea.String(strings.ToUpper(proto)),
		RegionId:        tea.String(region),
		ConnectTimeout:  tea.Int(10000),
		ReadTimeout:     tea.Int(60000),
	}
	return fc.NewClient(config)
}

// configureNoProxy 在可直连时为 FC 和数据面补充 NO_PROXY。
// 参数 uid 是阿里云账号 UID；参数 proto 是访问协议；参数 dataEndpoint 是数据面 endpoint。
func configureNoProxy(uid, proto, dataEndpoint string) {
	var entries []string
	fcHost := fmt.Sprintf("%s.%s.fc.aliyuncs.com", uid, region)
	if shouldPreferDirectConnection(fcHost, proto) {
		entries = append(entries, fcHost, ".fc.aliyuncs.com", ".fcapp.run")
	}
	if shouldPreferDirectConnection(dataEndpoint, proto) {
		entries = append(entries, dataEndpoint, ".agentrun-data."+region+".aliyuncs.com", ".funagent-data-pre."+region+".aliyuncs.com")
	}
	merged := mergeNoProxyEntries(strings.Join(entries, ","), os.Getenv("NO_PROXY"), os.Getenv("no_proxy"))
	if merged == "" {
		return
	}
	_ = os.Setenv("NO_PROXY", merged)
	_ = os.Setenv("no_proxy", merged)
}

// shouldPreferDirectConnection 检查目标 host 是否可以直连。
// 参数 host 是目标主机；参数 proto 是访问协议。
func shouldPreferDirectConnection(host, proto string) bool {
	client := &http.Client{
		Transport: &http.Transport{Proxy: nil},
		Timeout:   5 * time.Second,
	}
	req, err := http.NewRequest(http.MethodHead, fmt.Sprintf("%s://%s/", proto, host), nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

// mergeNoProxyEntries 合并多个 NO_PROXY 配置。
// 参数 values 是多个逗号分隔的 NO_PROXY 配置。
func mergeNoProxyEntries(values ...string) string {
	var merged []string
	seen := map[string]struct{}{}
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if _, ok := seen[part]; ok {
				continue
			}
			seen[part] = struct{}{}
			merged = append(merged, part)
		}
	}
	return strings.Join(merged, ",")
}

// marshalJSON 把对象序列化为 JSON 字符串。
// 参数 v 是待序列化对象。
func marshalJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}

// formatSDKError 提取 SDK 错误中的可读消息。
// 参数 err 是原始错误。
func formatSDKError(err error) string {
	if sdkErr, ok := err.(*tea.SDKError); ok {
		return tea.StringValue(sdkErr.Message)
	}
	return err.Error()
}
