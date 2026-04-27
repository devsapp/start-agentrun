package demo

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	fc "github.com/alibabacloud-go/fc-20230330/v4/client"
	fc2016 "github.com/aliyun/fc-go-sdk"
)

// buildBinary 编译服务二进制。
// 参数 name 是服务名称；srcDir 是源码目录；outputPath 是输出路径。
func buildBinary(name, srcDir, outputPath string) error {
	slog.Info("编译 Go 二进制", "name", name, "dir", srcDir)
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", outputPath, ".")
	cmd.Dir = srcDir
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("编译 %s 失败: %w", name, err)
	}
	return nil
}

// deployBinaryAsFunction 打包、上传并部署 Hook 函数。
// 参数 ctx 是调用上下文；fcClient 是 FC 2023 客户端；tempClient 是 FC 2016 客户端；uid 是阿里云账号 ID；binaryPath 是二进制路径；binaryName 是二进制文件名；functionName 是函数名。
func deployBinaryAsFunction(ctx context.Context, fcClient *fc.Client, tempClient *fc2016.Client, uid, binaryPath, binaryName, functionName string) (string, error) {
	zipPath := binaryPath + ".zip"
	if err := createZipWithBootstrapFile(zipPath, binaryPath, binaryName); err != nil {
		return "", err
	}
	codePackage, err := uploadCodePackageToFCTempBucket(tempClient, uid, zipPath)
	if err != nil {
		return "", err
	}
	if err := deployFunction(ctx, fcClient, functionName, codePackage); err != nil {
		return "", err
	}
	return getFunctionURL(fcClient, functionName, uid)
}

// createZipWithBootstrapFile 创建带 bootstrap 的 FC 函数 zip 文件。
// 参数 outputPath 是 zip 输出路径；binaryPath 是二进制路径；binaryName 是二进制文件名。
func createZipWithBootstrapFile(outputPath, binaryPath, binaryName string) error {
	zipFile, err := createZipWithBootstrap(binaryPath, binaryName)
	if err != nil {
		return err
	}
	if err := os.WriteFile(outputPath, zipFile, 0o644); err != nil {
		return fmt.Errorf("写入函数 zip 失败: %w", err)
	}
	return nil
}

// createZipWithBootstrap 创建带 bootstrap 的 FC 函数 zip 内容。
// 参数 binaryPath 是二进制路径；binaryName 是二进制文件名。
func createZipWithBootstrap(binaryPath, binaryName string) ([]byte, error) {
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)

	bootstrapHeader := &zip.FileHeader{Name: "bootstrap", Method: zip.Deflate}
	bootstrapHeader.SetMode(0o755)
	bootstrapWriter, err := writer.CreateHeader(bootstrapHeader)
	if err != nil {
		return nil, err
	}
	if _, err := bootstrapWriter.Write([]byte("#!/bin/bash\ncd /code\n./" + binaryName + "\n")); err != nil {
		return nil, err
	}

	binaryData, err := os.ReadFile(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("读取二进制失败: %w", err)
	}
	binaryHeader := &zip.FileHeader{Name: binaryName, Method: zip.Deflate}
	binaryHeader.SetMode(0o755)
	binaryWriter, err := writer.CreateHeader(binaryHeader)
	if err != nil {
		return nil, err
	}
	if _, err := binaryWriter.Write(binaryData); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// createToolCodeZipFile 创建 CODE_PACKAGE 工具 zip 文件。
// 参数 outputPath 是 zip 输出路径；binaryPath 是二进制路径；binaryName 是二进制文件名。
func createToolCodeZipFile(outputPath, binaryPath, binaryName string) error {
	zipFile, err := createToolCodeZip(binaryPath, binaryName)
	if err != nil {
		return err
	}
	if err := os.WriteFile(outputPath, zipFile, 0o644); err != nil {
		return fmt.Errorf("写入工具 zip 失败: %w", err)
	}
	return nil
}

// createToolCodeZip 创建 CODE_PACKAGE 工具 zip 内容。
// 参数 binaryPath 是二进制路径；binaryName 是二进制文件名。
func createToolCodeZip(binaryPath, binaryName string) ([]byte, error) {
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)

	binaryData, err := os.ReadFile(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("读取二进制失败: %w", err)
	}
	binaryHeader := &zip.FileHeader{Name: binaryName, Method: zip.Deflate}
	binaryHeader.SetMode(0o755)
	binaryWriter, err := writer.CreateHeader(binaryHeader)
	if err != nil {
		return nil, err
	}
	if _, err := binaryWriter.Write(binaryData); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
