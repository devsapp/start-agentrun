package demo

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	fc "github.com/alibabacloud-go/fc-20230330/v4/client"
)

// buildBinary 编译 Go 服务为 Linux 二进制。
// 参数 name 是服务名；参数 srcDir 是源码目录；参数 outputPath 是输出路径。
func buildBinary(name, srcDir, outputPath string) error {
	slog.Info("编译 Go 二进制", "name", name, "dir", srcDir)
	cmd := exec.Command("go", "build", "-o", outputPath, ".")
	cmd.Dir = srcDir
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("编译 %s 失败: %w", name, err)
	}
	return nil
}

// deployBinaryAsFunction 把二进制打包并部署为 FC 函数。
// 参数 ctx 是调用上下文；参数 fcClient 是 FC 客户端；参数 uid 是阿里云账号 UID；参数 binaryPath 是二进制路径；参数 binaryName 是二进制文件名；参数 functionName 是函数名。
func deployBinaryAsFunction(ctx context.Context, fcClient *fc.Client, uid, binaryPath, binaryName, functionName string) (string, error) {
	zipFile, err := createZipWithBootstrap(binaryPath, binaryName)
	if err != nil {
		return "", err
	}
	if err := deployFunction(ctx, fcClient, functionName, zipFile); err != nil {
		return "", err
	}
	return getFunctionURL(fcClient, functionName, uid)
}

// createZipWithBootstrap 创建包含 bootstrap 和二进制的 FC 代码包。
// 参数 binaryPath 是二进制路径；参数 binaryName 是二进制文件名。
func createZipWithBootstrap(binaryPath, binaryName string) (string, error) {
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)

	bootstrapHeader := &zip.FileHeader{Name: "bootstrap", Method: zip.Deflate}
	bootstrapHeader.SetMode(0o755)
	bootstrapWriter, err := writer.CreateHeader(bootstrapHeader)
	if err != nil {
		return "", err
	}
	if _, err := bootstrapWriter.Write([]byte("#!/bin/bash\ncd /code\n./" + binaryName + "\n")); err != nil {
		return "", err
	}

	binaryData, err := os.ReadFile(binaryPath)
	if err != nil {
		return "", fmt.Errorf("读取二进制失败: %w", err)
	}
	binaryHeader := &zip.FileHeader{Name: binaryName, Method: zip.Deflate}
	binaryHeader.SetMode(0o755)
	binaryWriter, err := writer.CreateHeader(binaryHeader)
	if err != nil {
		return "", err
	}
	if _, err := binaryWriter.Write(binaryData); err != nil {
		return "", err
	}

	if err := writer.Close(); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
