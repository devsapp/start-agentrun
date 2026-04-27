package demo

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	fc2016 "github.com/aliyun/fc-go-sdk"
)

// fcTempBucketCredentials 表示 FC TempBucket 返回的 OSS 临时凭证。
type fcTempBucketCredentials struct {
	AccessKeyID     string `json:"AccessKeyId"`
	AccessKeySecret string `json:"AccessKeySecret"`
	SecurityToken   string `json:"SecurityToken"`
}

// fcTempBucketToken 表示 FC TempBucket 上传信息。
type fcTempBucketToken struct {
	OSSRegion     string                  `json:"ossRegion"`
	Credentials   fcTempBucketCredentials `json:"credentials"`
	OSSBucket     string                  `json:"ossBucket"`
	ObjectName    string                  `json:"objectName"`
	CodeSizeLimit int64                   `json:"codeSizeLimit,omitempty"`
}

// getFCTempBucketToken 获取 FC 临时 OSS 上传桶和 STS 凭证。
// 参数 client 是 FC 2016 客户端。
func getFCTempBucketToken(client *fc2016.Client) (fcTempBucketToken, error) {
	output, err := client.GetTempBucketToken()
	if err != nil {
		return fcTempBucketToken{}, fmt.Errorf("获取 FC TempBucket 失败: %w", err)
	}
	if output == nil {
		return fcTempBucketToken{}, fmt.Errorf("FC TempBucket 返回空响应")
	}
	token := fcTempBucketToken{
		OSSRegion:  output.OssRegion,
		OSSBucket:  output.OssBucket,
		ObjectName: output.ObjectName,
		Credentials: fcTempBucketCredentials{
			AccessKeyID:     output.Credentials.AccessKeyID,
			AccessKeySecret: output.Credentials.AccessKeySecret,
			SecurityToken:   output.Credentials.SecurityToken,
		},
	}
	if err := token.validate(); err != nil {
		return fcTempBucketToken{}, err
	}
	return token, nil
}

// validate 检查 FC TempBucket 响应是否包含上传必需字段。
func (t fcTempBucketToken) validate() error {
	if strings.TrimSpace(t.OSSRegion) == "" {
		return fmt.Errorf("FC TempBucket 响应缺少 ossRegion")
	}
	if strings.TrimSpace(t.OSSBucket) == "" {
		return fmt.Errorf("FC TempBucket 响应缺少 ossBucket")
	}
	if strings.TrimSpace(t.ObjectName) == "" {
		return fmt.Errorf("FC TempBucket 响应缺少 objectName")
	}
	if strings.TrimSpace(t.Credentials.AccessKeyID) == "" || strings.TrimSpace(t.Credentials.AccessKeySecret) == "" || strings.TrimSpace(t.Credentials.SecurityToken) == "" {
		return fmt.Errorf("FC TempBucket 响应缺少 OSS 临时凭证")
	}
	return nil
}

// uploadCodePackageToFCTempBucket 上传代码包到 FC TempBucket。
// 参数 client 是 FC 2016 客户端；uid 是阿里云账号 ID；zipPath 是本地 zip 包路径。
func uploadCodePackageToFCTempBucket(client *fc2016.Client, uid, zipPath string) (codePackageLocation, error) {
	info, err := os.Stat(zipPath)
	if err != nil {
		return codePackageLocation{}, fmt.Errorf("读取代码包失败: %w", err)
	}
	token, err := getFCTempBucketToken(client)
	if err != nil {
		return codePackageLocation{}, err
	}
	if token.CodeSizeLimit > 0 && info.Size() > token.CodeSizeLimit {
		return codePackageLocation{}, fmt.Errorf("代码包大小 %d 超过 FC 限制 %d", info.Size(), token.CodeSizeLimit)
	}

	objectName := tempBucketObjectName(uid, token.ObjectName)
	endpoint := ossEndpointFromRegion(token.OSSRegion)
	slog.Info("上传代码包到 FC TempBucket", "zip", zipPath, "size", info.Size(), "ossEndpoint", endpoint, "ossBucketName", token.OSSBucket, "ossObjectName", objectName)

	ossClient, err := oss.New(endpoint, token.Credentials.AccessKeyID, token.Credentials.AccessKeySecret, oss.SecurityToken(token.Credentials.SecurityToken))
	if err != nil {
		return codePackageLocation{}, fmt.Errorf("创建 OSS 客户端失败: %w", err)
	}
	bucket, err := ossClient.Bucket(token.OSSBucket)
	if err != nil {
		return codePackageLocation{}, fmt.Errorf("打开 OSS bucket 失败: %w", err)
	}
	if err := bucket.PutObjectFromFile(objectName, zipPath); err != nil {
		return codePackageLocation{}, fmt.Errorf("上传代码包到 OSS 失败: %w", err)
	}
	return codePackageLocation{OSSBucketName: token.OSSBucket, OSSObjectName: objectName}, nil
}

// tempBucketObjectName 生成 FC TempBucket 中最终传给 FC 或 AgentRun 的对象名。
// 参数 uid 是阿里云账号 ID；objectName 是 FC TempBucket 返回的对象名。
func tempBucketObjectName(uid, objectName string) string {
	uid = strings.Trim(strings.TrimSpace(uid), "/")
	objectName = strings.TrimLeft(strings.TrimSpace(objectName), "/")
	return uid + "/" + objectName
}

// ossEndpointFromRegion 根据 FC TempBucket 返回的 OSS region 生成 OSS endpoint。
// 参数 ossRegion 是 FC TempBucket 返回的 ossRegion，也可以是完整 endpoint。
func ossEndpointFromRegion(ossRegion string) string {
	value := strings.TrimSpace(ossRegion)
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	if strings.Contains(value, ".") {
		return "https://" + value
	}
	if strings.HasPrefix(value, "oss-") {
		return "https://" + value + ".aliyuncs.com"
	}
	return "https://oss-" + value + ".aliyuncs.com"
}
