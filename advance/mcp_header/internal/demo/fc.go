package demo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	fc "github.com/alibabacloud-go/fc-20230330/v4/client"
	"github.com/alibabacloud-go/tea/tea"
)

// deployFunction 创建或更新 FC 函数并确保 HTTP 触发器存在。
// 参数 ctx 是调用上下文；参数 client 是 FC 客户端；参数 funcName 是函数名；参数 zipBase64 是 Base64 编码的代码包。
func deployFunction(ctx context.Context, client *fc.Client, funcName, zipBase64 string) error {
	code := &fc.InputCodeLocation{}
	code.SetZipFile(zipBase64)

	_, err := client.GetFunction(tea.String(funcName), &fc.GetFunctionRequest{})
	if err == nil {
		input := &fc.UpdateFunctionInput{}
		input.SetCode(code)
		_, err = client.UpdateFunction(tea.String(funcName), &fc.UpdateFunctionRequest{Body: input})
		if err != nil {
			return fmt.Errorf("更新函数失败: %w", err)
		}
		return ensureHTTPTrigger(client, funcName)
	}

	input := &fc.CreateFunctionInput{}
	input.SetFunctionName(funcName)
	input.SetRuntime("custom.debian11")
	input.SetHandler("index.handler")
	input.SetCpu(0.35)
	input.SetMemorySize(512)
	input.SetDiskSize(512)
	input.SetTimeout(300)
	input.SetInstanceConcurrency(20)
	input.SetInternetAccess(true)
	input.SetCode(code)
	_, err = client.CreateFunction(&fc.CreateFunctionRequest{Body: input})
	if err != nil {
		return fmt.Errorf("创建函数失败: %w", err)
	}
	return ensureHTTPTrigger(client, funcName)
}

// getFunctionURL 获取 FC 函数公网访问地址。
// 参数 client 是 FC 客户端；参数 funcName 是函数名；参数 uid 是阿里云账号 UID。
func getFunctionURL(client *fc.Client, funcName, uid string) (string, error) {
	resp, err := client.ListTriggers(tea.String(funcName), &fc.ListTriggersRequest{})
	if err == nil && resp.Body != nil {
		for _, trigger := range resp.Body.Triggers {
			if trigger.HttpTrigger != nil && trigger.HttpTrigger.UrlInternet != nil {
				return tea.StringValue(trigger.HttpTrigger.UrlInternet), nil
			}
		}
	}
	return fmt.Sprintf("%s://%s-%s.%s.fcapp.run", defaultProto, funcName, uid, region), nil
}

// ensureHTTPTrigger 确保函数存在匿名 HTTP 触发器。
// 参数 client 是 FC 客户端；参数 funcName 是函数名。
func ensureHTTPTrigger(client *fc.Client, funcName string) error {
	const triggerName = "http-trigger"

	listResp, listErr := client.ListTriggers(tea.String(funcName), &fc.ListTriggersRequest{})
	if listErr == nil && listResp.Body != nil {
		for _, trigger := range listResp.Body.Triggers {
			if tea.StringValue(trigger.TriggerName) == triggerName {
				return nil
			}
		}
	}

	triggerInput := &fc.CreateTriggerInput{}
	triggerInput.SetTriggerName(triggerName)
	triggerInput.SetTriggerType("http")
	triggerInput.SetTriggerConfig(`{"authType":"anonymous","methods":["GET","POST","PUT","DELETE","PATCH","HEAD","OPTIONS"],"disableURLInternet":false}`)
	if _, err := client.CreateTrigger(tea.String(funcName), &fc.CreateTriggerRequest{Body: triggerInput}); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already exist") {
			return nil
		}
		return fmt.Errorf("创建 HTTP trigger 失败: %w", err)
	}
	return nil
}

// waitForFunction 等待 FC 函数可查询。
// 参数 client 是 FC 客户端；参数 funcName 是函数名；参数 timeout 是等待超时时间。
func waitForFunction(client *fc.Client, funcName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := client.GetFunction(tea.String(funcName), &fc.GetFunctionRequest{}); err == nil {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("函数未创建: %s", funcName)
}

// warmupFunction 预热远程 MCP 函数。
// 参数 ctx 是调用上下文；参数 baseURL 是函数基地址；参数 timeout 是等待超时时间。
func warmupFunction(ctx context.Context, baseURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	warmupURL := strings.TrimSuffix(baseURL, "/") + "/mcp"
	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy:               nil,
			DisableKeepAlives:   true,
			ForceAttemptHTTP2:   false,
			TLSHandshakeTimeout: 10 * time.Second,
		},
		Timeout: 20 * time.Second,
	}
	var lastErr error
	for time.Now().Before(deadline) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, warmupURL, strings.NewReader(warmupBody))
		req.Header.Set("Content-Type", "application/json")
		resp, err := httpClient.Do(req)
		if err == nil {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		} else {
			lastErr = err
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("函数预热超时: %s，最后错误: %v", baseURL, lastErr)
}
