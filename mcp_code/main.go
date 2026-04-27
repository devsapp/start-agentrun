package main

import (
	"fmt"
	"os"

	"hookquickstart/mcpcode/internal/demo"
)

func main() {
	if err := demo.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "mcp_code quickstart 失败: %v\n", err)
		os.Exit(1)
	}
}
