package main

import (
	"fmt"
	"os"

	"hookquickstart/mcpheader/internal/demo"
)

// main 运行 header hook 验证示例。
func main() {
	if err := demo.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "mcp_header quickstart 失败: %v\n", err)
		os.Exit(1)
	}
}
