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

func marshalJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func formatSDKError(err error) string {
	if sdkErr, ok := err.(*tea.SDKError); ok {
		return tea.StringValue(sdkErr.Message)
	}
	return err.Error()
}
