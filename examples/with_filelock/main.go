/*
 * SubNodeSync - 分布式节点同步框架
 * examples/with_filelock/main.go
 * 文件锁示例 - 展示如何使用文件锁防止多实例运行
 *
 * 文件锁机制说明：
 * 1. 在系统临时目录创建锁文件（如：/tmp/my-app.lock）
 * 2. 锁文件内容为当前进程PID
 * 3. 启动时检查锁文件，如果进程仍在运行则拒绝启动
 * 4. 进程退出时自动释放锁文件
 *
 * 运行测试：
 * 1. 运行第一个实例：go run main.go
 * 2. 在另一个终端运行第二个实例：go run main.go
 * 3. 第二个实例会因为文件锁而无法启动
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
	"time"

	"github.com/yourusername/subnodesync/pkg/node"
)

func main() {
	appName := "filelock-demo"

	log.Printf("启动应用: %s", appName)

	// 方式1: 使用 RegisterWithLock（推荐）
	// 如果已有实例运行，会返回错误
	if err := node.RegisterWithLock(appName); err != nil {
		log.Fatalf("启动失败: %v", err)
	}

	// 方式2: 使用 MustRegisterWithLock（失败时自动退出）
	// node.MustRegisterWithLock(appName)

	// 方式3: 使用自定义配置
	// config := &node.Config{
	// 	EnableFileLock:    true,
	// 	HeartbeatInterval: 30 * time.Second,
	// }
	// if err := node.RegisterWithConfig(appName, config); err != nil {
	// 	log.Fatalf("启动失败: %v", err)
	// }

	// 确保退出时释放锁
	defer node.Shutdown()

	// 打印锁文件位置
	log.Printf("锁文件位置: %s", node.GetLockFilePath(appName))

	// 检查当前锁状态
	if locked, pid := node.IsAnotherInstanceRunning(appName); locked {
		log.Printf("当前实例PID: %d", pid)
	}

	// 模拟业务逻辑
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			log.Printf("应用运行中... PID=%d", os.Getpid())
		}
	}()

	log.Printf("应用已启动，按 Ctrl+C 退出...")

	// 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Printf("收到退出信号，正在关闭...")
	// defer 会自动调用 node.Shutdown() 释放锁
}

