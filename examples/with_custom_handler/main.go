/*
 * SubNodeSync - 分布式节点同步框架
 * examples/with_custom_handler/main.go
 * 自定义命令处理器示例 - 展示如何注册和处理自定义命令
 *
 * Copyright (c) 2024. All Rights Reserved.
 * Licensed under the MIT License.
 */

package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/HY-805/SubNodeSync/pkg/node"
	"github.com/HY-805/SubNodeSync/pkg/sync"
)

func main() {
	appName := "custom-handler-app"

	log.Printf("启动自定义命令处理应用: %s", appName)

	// 注册节点
	if err := node.Register(appName); err != nil {
		log.Printf("节点注册失败: %v", err)
	}

	// 创建命令接收器并注册自定义处理器
	receiver := sync.NewCommandReceiver(appName, "tcp://127.0.0.1:1883")

	// 注册配置更新命令处理器
	receiver.RegisterHandler("config_update", &ConfigUpdateHandler{})

	// 注册数据查询命令处理器
	receiver.RegisterHandler("data_query", &DataQueryHandler{})

	// 注册健康检查命令处理器
	receiver.RegisterHandler("health_check", sync.NewCustomHandler("health_check", func(ctx context.Context, cmd *sync.Command) (*sync.CommandResult, error) {
		return &sync.CommandResult{
			Success: true,
			Message: "healthy",
		}, nil
	}))

	// 启动命令接收器
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := receiver.Start(ctx); err != nil {
		log.Printf("启动命令接收器失败: %v", err)
	}

	log.Printf("应用已启动，等待命令...")

	// 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Printf("收到退出信号，正在关闭...")
	receiver.Stop()
	node.Shutdown()
	log.Printf("应用已退出")
}

// ConfigUpdateHandler 配置更新命令处理器
type ConfigUpdateHandler struct{}

func (h *ConfigUpdateHandler) Handle(ctx context.Context, cmd *sync.Command) (*sync.CommandResult, error) {
	log.Printf("收到配置更新命令")

	// 解析配置参数
	if configData, ok := cmd.Parameters["config"]; ok {
		configJSON, _ := json.Marshal(configData)
		log.Printf("新配置: %s", string(configJSON))

		// TODO: 应用新配置
	}

	return &sync.CommandResult{
		Success:   true,
		Message:   "Configuration updated successfully",
		RequestID: cmd.RequestID,
	}, nil
}

func (h *ConfigUpdateHandler) GetCommandName() string {
	return "config_update"
}

// DataQueryHandler 数据查询命令处理器
type DataQueryHandler struct{}

func (h *DataQueryHandler) Handle(ctx context.Context, cmd *sync.Command) (*sync.CommandResult, error) {
	log.Printf("收到数据查询命令")

	// 获取查询参数
	queryType, _ := cmd.Parameters["type"].(string)

	var result string
	switch queryType {
	case "metrics":
		result = `{"cpu": 25.5, "memory": 512, "goroutines": 42}`
	case "status":
		result = `{"status": "healthy", "uptime": 3600}`
	default:
		result = `{"error": "unknown query type"}`
	}

	return &sync.CommandResult{
		Success:   true,
		Message:   result,
		RequestID: cmd.RequestID,
	}, nil
}

func (h *DataQueryHandler) GetCommandName() string {
	return "data_query"
}
