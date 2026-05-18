package demo

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

// signRequest 构造访问 AgentRun 数据面的签名请求头。
// 参数 rawURL 是请求地址；参数 method 是 HTTP 方法；参数 accessKeyID 是 AccessKey ID；参数 accessKeySecret 是 AccessKey Secret；参数 region 是地域；参数 product 是签名产品名。
func signRequest(rawURL, method, accessKeyID, accessKeySecret, region, product string) map[string]string {
	parsed, _ := url.Parse(rawURL)
	host := parsed.Host
	now := time.Now().UTC()
	date := now.Format("20060102")
	timestamp := now.Format("2006-01-02T15:04:05Z")

	headers := map[string]string{
		"host":                 host,
		"x-acs-date":           timestamp,
		"x-acs-content-sha256": "UNSIGNED-PAYLOAD",
	}

	signingKey := getSigningKey(accessKeySecret, product, region, date)
	canonicalURI := parsed.Path
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalizedHeaders := canonicalHeaders(headers)
	signedHeaders := signedHeaderNames(headers)
	signedHeadersStr := strings.Join(signedHeaders, ";")
	canonicalizedResource := canonicalResource(parsed.Query())

	stringToSign := strings.Join([]string{
		strings.ToUpper(method),
		canonicalURI,
		canonicalizedResource,
		canonicalizedHeaders,
		signedHeadersStr,
		"UNSIGNED-PAYLOAD",
	}, "\n")

	hash := sha256.Sum256([]byte(stringToSign))
	signature := hmacSHA256(signingKey, signAlgorithm+"\n"+hex.EncodeToString(hash[:]))
	headers["Agentrun-Authorization"] = fmt.Sprintf(
		"%s Credential=%s/%s/%s/%s/%s_request,SignedHeaders=%s,Signature=%s",
		signAlgorithm, accessKeyID, date, region, product, signPrefix, signedHeadersStr, hex.EncodeToString(signature),
	)
	return headers
}

// canonicalResource 构造签名使用的规范化查询串。
// 参数 query 是 URL 查询参数。
func canonicalResource(query url.Values) string {
	if len(query) == 0 {
		return ""
	}
	var keys []string
	for key := range query {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var parts []string
	for _, key := range keys {
		parts = append(parts, url.QueryEscape(key)+"="+url.QueryEscape(query.Get(key)))
	}
	return strings.Join(parts, "&")
}

// canonicalHeaders 构造签名使用的规范化请求头。
// 参数 headers 是待签名请求头。
func canonicalHeaders(headers map[string]string) string {
	var parts []string
	for _, key := range signedHeaderNames(headers) {
		parts = append(parts, key+":"+strings.TrimSpace(headers[key]))
	}
	return strings.Join(parts, "\n") + "\n"
}

// signedHeaderNames 返回参与签名的请求头名称。
// 参数 headers 是待筛选请求头。
func signedHeaderNames(headers map[string]string) []string {
	var keys []string
	for key, value := range headers {
		lower := strings.ToLower(key)
		if value == "" {
			continue
		}
		if strings.HasPrefix(lower, "x-acs-") || lower == "host" || lower == "content-type" {
			keys = append(keys, lower)
		}
	}
	sort.Strings(keys)
	return keys
}

// hmacSHA256 计算 HMAC-SHA256。
// 参数 key 是签名密钥；参数 data 是待签名内容。
func hmacSHA256(key []byte, data string) []byte {
	sum := hmac.New(sha256.New, key)
	sum.Write([]byte(data))
	return sum.Sum(nil)
}

// getSigningKey 按 AgentRun 签名规则派生签名密钥。
// 参数 secret 是 AccessKey Secret；参数 product 是签名产品名；参数 region 是地域；参数 date 是日期。
func getSigningKey(secret, product, region, date string) []byte {
	dateKey := hmacSHA256([]byte(signPrefix+secret), date)
	regionKey := hmacSHA256(dateKey, region)
	productKey := hmacSHA256(regionKey, product)
	return hmacSHA256(productKey, signPrefix+"_request")
}
