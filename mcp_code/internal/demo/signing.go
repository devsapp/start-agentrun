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

func canonicalHeaders(headers map[string]string) string {
	var parts []string
	for _, key := range signedHeaderNames(headers) {
		parts = append(parts, key+":"+strings.TrimSpace(headers[key]))
	}
	return strings.Join(parts, "\n") + "\n"
}

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

func hmacSHA256(key []byte, data string) []byte {
	sum := hmac.New(sha256.New, key)
	sum.Write([]byte(data))
	return sum.Sum(nil)
}

func getSigningKey(secret, product, region, date string) []byte {
	dateKey := hmacSHA256([]byte(signPrefix+secret), date)
	regionKey := hmacSHA256(dateKey, region)
	productKey := hmacSHA256(regionKey, product)
	return hmacSHA256(productKey, signPrefix+"_request")
}
