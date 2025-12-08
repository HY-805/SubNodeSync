/*
 * SubNodeSync - 分布式节点同步框架
 * examples/basic/main.go
 * 基础使用示例 - 展示如何注册节点并接收命令
 *
 * Copyright (c) 2024. All Rights Reserved.
 * Licensed under the MIT License.
 */

package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/HY-805/SubNodeSync/pkg/node"
)

func main() {
	// 应用名称
	appName := "my-basic-app"

	log.Printf("启动应用: %s", appName)

	// 方式1: 使用默认配置注册节点（不启用文件锁）
	// 默认使用 tcp://127.0.0.1:1883 作为MQTT broker
	// if err := node.Register(appName); err != nil {
	// 	log.Printf("节点注册失败: %v", err)
	// }

	// 方式2: 启用文件锁，防止多实例运行（推荐）
	if err := node.RegisterWithLock(appName); err != nil {
		log.Fatalf("节点注册失败: %v", err)
	}

	// 确保退出时释放资源（包括文件锁）
	defer node.Shutdown()

	log.Printf("应用已启动，等待信号...")

	// 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Printf("收到退出信号，正在关闭...")
	// defer 会自动调用 node.Shutdown()

	log.Printf("应用已退出")
}
