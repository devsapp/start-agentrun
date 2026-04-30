package main

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const defaultPort = "9000"

// orderItem 表示订单中的单个商品。
type orderItem struct {
	SKU      string  `json:"sku"`
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

// orderRecord 表示订单查询返回的数据。
type orderRecord struct {
	OrderID         string      `json:"order_id"`
	CustomerName    string      `json:"customer_name"`
	Phone           string      `json:"phone"`
	Email           string      `json:"email"`
	ShippingAddress string      `json:"shipping_address"`
	Status          string      `json:"status"`
	Amount          float64     `json:"amount"`
	Items           []orderItem `json:"items"`
}

var sampleOrders = map[string]orderRecord{
	"ORDER-1001": {
		OrderID:         "ORDER-1001",
		CustomerName:    "张三",
		Phone:           "13812345678",
		Email:           "zhangsan@example.com",
		ShippingAddress: "浙江省杭州市西湖区文三路 100 号",
		Status:          "PAID",
		Amount:          259.8,
		Items: []orderItem{
			{SKU: "SKU-AX100", Name: "人体工学鼠标", Quantity: 1, Price: 129.9},
			{SKU: "SKU-KB200", Name: "机械键盘键帽", Quantity: 1, Price: 129.9},
		},
	},
	"ORDER-1002": {
		OrderID:         "ORDER-1002",
		CustomerName:    "李四",
		Phone:           "13987654321",
		Email:           "lisi@example.com",
		ShippingAddress: "上海市浦东新区世纪大道 88 号",
		Status:          "SHIPPED",
		Amount:          88.5,
		Items: []orderItem{
			{SKU: "SKU-CB300", Name: "USB-C 数据线", Quantity: 3, Price: 29.5},
		},
	},
}

// main 启动订单查询 MCP 服务。
func main() {
	mcpServer := newOrderServer()

	port := os.Getenv("FC_SERVER_PORT")
	if port == "" {
		port = defaultPort
	}

	httpServer := server.NewStreamableHTTPServer(mcpServer)
	slog.Info("orderdesk 服务启动", "port", port)
	if err := httpServer.Start(":" + port); err != nil {
		slog.Error("orderdesk 服务异常", "error", err)
		os.Exit(1)
	}
}

// newOrderServer 创建订单查询 MCP 服务。
func newOrderServer() *server.MCPServer {
	mcpServer := server.NewMCPServer("hook-quickstart-orderdesk", "1.0.0", server.WithToolCapabilities(false))
	mcpServer.AddTool(
		mcp.NewTool(
			"get_order",
			mcp.WithDescription("按订单号查询订单详情"),
			mcp.WithString("order_id", mcp.Required(), mcp.Description("订单编号，例如 ORDER-1001")),
		),
		handleGetOrder,
	)
	return mcpServer
}

// handleGetOrder 处理订单详情查询。
// 参数 ctx 是调用上下文；参数 req 是 MCP 工具调用请求。
func handleGetOrder(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	orderID, err := req.RequireString("order_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	order, ok := lookupOrder(orderID)
	if !ok {
		return mcp.NewToolResultError("订单不存在: " + strings.TrimSpace(orderID)), nil
	}

	result, err := mcp.NewToolResultJSON(order)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// lookupOrder 按订单号查找示例订单。
// 参数 orderID 是订单编号。
func lookupOrder(orderID string) (orderRecord, bool) {
	order, ok := sampleOrders[strings.ToUpper(strings.TrimSpace(orderID))]
	return order, ok
}
