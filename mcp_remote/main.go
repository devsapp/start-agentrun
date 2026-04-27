package main

import (
	"fmt"
	"os"

	"hookquickstart/mcpremote/internal/demo"
)

func main() {
	if err := demo.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "mcp_remote quickstart 失败: %v\n", err)
		os.Exit(1)
	}
}
