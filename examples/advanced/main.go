/*
 * SubNodeSync - 分布式节点同步框架
 * examples/advanced/main.go
 * 高级使用示例 - 展示自定义配置、命令处理和元数据
 *
 * Copyright (c) 2024. All Rights Reserved.
 * Licensed under the MIT License.
 */

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	nodepkg "github.com/yourusername/subnodesync/pkg/node"
	"github.com/yourusername/subnodesync/pkg/sync"
)

func main() {
	appName := "my-advanced-app"

	log.Printf("启动高级应用: %s", appName)

	// 方式2: 使用自定义配置注册节点
	config := &nodepkg.Config{
		// 自定义MQTT broker地址
		MQTTBroker: "tcp://127.0.0.1:1883",
		// 心跳间隔
		HeartbeatInterval: 30 * time.Second,
		// 自定义元数据
		Metadata: map[string]string{
			"version":     "1.0.0",
			"environment": "production",
			"region":      "cn-east",
		},
	}

	if err := nodepkg.RegisterWithConfig(appName, config); err != nil {
		log.Printf("节点注册失败: %v", err)
	}

	// 获取当前实例并注册自定义命令处理器
	instance := nodepkg.GetCurrentInstance()
	if instance != nil {
		// 这里可以添加自定义命令处理逻辑
		log.Printf("节点实例ID: %s", instance.InstanceID)
	}

	// 模拟业务逻辑
	go runBusinessLogic()

	log.Printf("应用已启动，等待信号...")

	// 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Printf("收到退出信号，正在关闭...")
	nodepkg.Shutdown()
	log.Printf("应用已退出")
}

// runBusinessLogic 模拟业务逻辑
func runBusinessLogic() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		log.Printf("业务逻辑运行中...")
	}
}

// CustomCommandHandler 自定义命令处理器示例
type CustomCommandHandler struct {
	name string
}

func (h *CustomCommandHandler) Handle(ctx context.Context, cmd *sync.Command) (*sync.CommandResult, error) {
	log.Printf("收到自定义命令: %s, 参数: %v", cmd.Command, cmd.Parameters)

	// 处理自定义逻辑
	result := &sync.CommandResult{
		Success:   true,
		Message:   "Custom command executed successfully",
		RequestID: cmd.RequestID,
	}

	return result, nil
}

func (h *CustomCommandHandler) GetCommandName() string {
	return h.name
}

