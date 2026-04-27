package demo

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	agentrun "github.com/alibabacloud-go/agentrun-20250910/v5/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	fc "github.com/alibabacloud-go/fc-20230330/v4/client"
	"github.com/alibabacloud-go/tea/tea"
	fc2016 "github.com/aliyun/fc-go-sdk"
	"github.com/joho/godotenv"
)

func locateDirs() (moduleDir, rootDir string, err error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", "", errors.New("runtime.Caller 失败")
	}
	moduleDir = filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	rootDir = filepath.Clean(filepath.Join(moduleDir, ".."))
	return moduleDir, rootDir, nil
}

func loadEnv(rootDir, moduleDir string) {
	_ = godotenv.Load(filepath.Join(rootDir, ".env"))
	_ = godotenv.Overload(filepath.Join(moduleDir, ".env"))
}

func newSDKClient(controlEndpoint, proto, ak, sk string) (*agentrun.Client, error) {
	config := &openapi.Config{
		AccessKeyId:     tea.String(ak),
		AccessKeySecret: tea.String(sk),
		Endpoint:        tea.String(controlEndpoint),
		Protocol:        tea.String(strings.ToUpper(proto)),
	}
	return agentrun.NewClient(config)
}

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

// newFCTempBucketClient 创建用于获取 TempBucket 的 FC 2016 客户端。
// 参数 proto 是协议；uid 是阿里云账号 ID；ak 是 AccessKey ID；sk 是 AccessKey Secret。
func newFCTempBucketClient(proto, uid, ak, sk string) (*fc2016.Client, error) {
	endpoint := fmt.Sprintf("%s://%s.%s.fc.aliyuncs.com", proto, uid, region)
	return fc2016.NewClient(endpoint, "2016-08-15", ak, sk, fc2016.WithTimeout(60))
}

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

func formatSDKError(err error) string {
	if sdkErr, ok := err.(*tea.SDKError); ok {
		return tea.StringValue(sdkErr.Message)
	}
	return err.Error()
}

// requestIDError 表示带 request id 的云端错误。
type requestIDError interface {
	GetRequestId() *string
}

// extractRequestID 从 SDK 错误对象中提取 request id。
func extractRequestID(err error) string {
	if err == nil {
		return ""
	}
	if requestErr, ok := err.(requestIDError); ok {
		if requestID := tea.StringValue(requestErr.GetRequestId()); requestID != "" {
			return requestID
		}
	}
	if sdkErr, ok := err.(*tea.SDKError); ok {
		if requestID := extractRequestIDFromJSON(tea.StringValue(sdkErr.Data)); requestID != "" {
			return requestID
		}
		if requestID := extractRequestIDFromText(tea.StringValue(sdkErr.Message)); requestID != "" {
			return requestID
		}
	}
	return extractRequestIDFromText(err.Error())
}

// extractRequestIDFromHeaders 从 AgentRun 响应 header 中提取 request id。
func extractRequestIDFromHeaders(headers map[string]*string) string {
	if len(headers) == 0 {
		return ""
	}
	keys := []string{
		"x-acs-request-id",
		"x-request-id",
		"request-id",
		"requestid",
		"requestId",
	}
	for _, want := range keys {
		for got, value := range headers {
			if strings.EqualFold(got, want) {
				return tea.StringValue(value)
			}
		}
	}
	return ""
}

// extractRequestIDFromJSON 从 JSON 字符串中提取 request id。
func extractRequestIDFromJSON(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return ""
	}
	for _, key := range []string{"RequestId", "requestId", "request_id"} {
		if requestID, ok := payload[key].(string); ok && requestID != "" {
			return requestID
		}
	}
	return ""
}

var requestIDPattern = regexp.MustCompile(`(?i)request id:\s*([^\s,]+)`)

// extractRequestIDFromText 从错误文本中提取 request id。
func extractRequestIDFromText(value string) string {
	match := requestIDPattern.FindStringSubmatch(value)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}
