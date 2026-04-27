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
