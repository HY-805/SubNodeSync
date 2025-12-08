# SubNodeSync

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/yourusername/subnodesync)](https://goreportcard.com/report/github.com/yourusername/subnodesync)

SubNodeSync 是一个轻量级的分布式节点同步框架，提供节点注册、心跳管理、命令控制等功能。适用于需要集中管理多个分布式应用实例的场景。

## 特性

- 🚀 **轻量级集成** - 一行代码即可完成节点注册
- 📡 **MQTT通信** - 基于MQTT协议的可靠消息传输
- 💓 **心跳管理** - 自动发送心跳，监控节点存活状态
- 🎮 **命令控制** - 支持远程停止、重启、状态查询等命令
- 🔄 **自动重连** - 网络中断后自动重连
- 📊 **监控指标** - 自动上报CPU、内存、Goroutine等指标
- 🔌 **可扩展** - 支持自定义命令处理器
- 🔒 **单实例锁** - 文件锁机制防止多实例运行

## 安装

```bash
go get github.com/HY-805/subnodesync
```

## 快速开始

### 基础用法

```go
package main

import (
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/yourusername/subnodesync/pkg/node"
)

func main() {
    // 注册节点（默认使用 tcp://127.0.0.1:1883 作为MQTT broker）
    if err := node.Register("my-app"); err != nil {
        log.Printf("节点注册失败: %v", err)
    }

    // 等待退出信号
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan

    // 优雅关闭
    node.Shutdown()
}
```

### 启用文件锁（防止多实例运行）

```go
package main

import (
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/yourusername/subnodesync/pkg/node"
)

func main() {
    // 方式1: 使用 RegisterWithLock（推荐）
    // 如果已有实例运行，返回错误
    if err := node.RegisterWithLock("my-app"); err != nil {
        log.Fatalf("启动失败: %v", err)
    }
    defer node.Shutdown() // 确保退出时释放锁

    // 方式2: 使用 MustRegisterWithLock（失败时自动退出）
    // node.MustRegisterWithLock("my-app")
    // defer node.Shutdown()

    // 等待退出信号
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan
}
```

### 自定义配置

```go
package main

import (
    "time"

    "github.com/yourusername/subnodesync/pkg/node"
)

func main() {
    config := &node.Config{
        MQTTBroker:        "tcp://your-mqtt-broker:1883",
        MQTTUsername:      "username",
        MQTTPassword:      "password",
        HeartbeatInterval: 30 * time.Second,
        Metadata: map[string]string{
            "version":     "1.0.0",
            "environment": "production",
        },
    }

    if err := node.RegisterWithConfig("my-app", config); err != nil {
        // 处理错误
    }

    // ... 应用逻辑
}
```

### 自定义命令处理

```go
package main

import (
    "context"

    "github.com/yourusername/subnodesync/pkg/sync"
)

// 实现 CommandHandler 接口
type MyHandler struct{}

func (h *MyHandler) Handle(ctx context.Context, cmd *sync.Command) (*sync.CommandResult, error) {
    // 处理自定义命令
    return &sync.CommandResult{
        Success: true,
        Message: "Command executed",
    }, nil
}

func (h *MyHandler) GetCommandName() string {
    return "my_command"
}

func main() {
    receiver := sync.NewCommandReceiver("my-app", "tcp://127.0.0.1:1883")
    receiver.RegisterHandler("my_command", &MyHandler{})
    
    ctx := context.Background()
    receiver.Start(ctx)
    
    // ... 应用逻辑
}
```

## 架构

```
┌─────────────────────────────────────────────────────────────┐
│                    SubNodeSync 框架                          │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │    node     │  │    sync     │  │  transport  │         │
│  │  节点管理   │  │  命令同步   │  │   传输层    │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
│         │                │                │                 │
│         └────────────────┼────────────────┘                 │
│                          │                                   │
│                   ┌──────┴──────┐                           │
│                   │  MQTT Broker │                          │
│                   └─────────────┘                           │
└─────────────────────────────────────────────────────────────┘
```

## MQTT 主题结构

| 主题 | 用途 | 示例 |
|------|------|------|
| `v1/node/sync/{node_name}/register` | 注册消息 | `v1/node/sync/my-app/register` |
| `v1/node/sync/{node_name}/heartbeat` | 心跳消息 | `v1/node/sync/my-app/heartbeat` |
| `v1/node/sync/{node_name}/control` | 控制命令 | `v1/node/sync/my-app/control` |
| `v1/node/sync/{node_name}/status` | 状态消息 | `v1/node/sync/my-app/status` |

## 内置命令

| 命令 | 描述 |
|------|------|
| `stop` | 停止节点 |
| `restart` | 重启节点 |
| `status` | 查询节点状态 |
| `query` | 查询节点信息 |

## 心跳消息格式

```json
{
  "timestamp": "2024-01-01T12:00:00Z",
  "node_name": "my-app",
  "instance_id": "my-app-hostname-12345",
  "status": "running",
  "pid": 12345,
  "uptime": 3600,
  "version": "1.0.0",
  "hostname": "hostname",
  "metrics": {
    "process_cpu_usage_percent": "2.50",
    "process_memory_usage_mb": 128,
    "process_goroutine_count": 42
  }
}
```

## 环境变量

| 变量 | 描述 | 默认值 |
|------|------|--------|
| `MQTT_BROKER_URL` | MQTT Broker 地址 | `tcp://127.0.0.1:1883` |
| `MQTT_USERNAME` | MQTT 用户名 | 空 |
| `MQTT_PASSWORD` | MQTT 密码 | 空 |
| `NODE_ENGINE_URL` | 管理引擎地址 | `http://localhost:9957` |
| `APP_BUILD_ID` | 构建ID | 空 |
| `APP_BUILD_TIME` | 构建时间 | 空 |

## 文件锁机制

文件锁用于防止同一应用的多个实例同时运行，实现原理如下：

1. **锁文件创建**：在系统临时目录创建锁文件（如：`/tmp/my-app.lock`）
2. **PID记录**：锁文件内容为当前进程的PID
3. **存活检测**：启动时检查锁文件，如果记录的进程仍在运行则拒绝启动
4. **陈旧锁清理**：如果记录的进程已终止，自动清理旧锁并创建新锁
5. **自动释放**：进程正常退出时通过 `defer node.Shutdown()` 释放锁

```go
// 检查是否有另一个实例运行
if locked, pid := node.IsAnotherInstanceRunning("my-app"); locked {
    log.Printf("另一个实例正在运行，PID: %d", pid)
}

// 获取锁文件路径
lockPath := node.GetLockFilePath("my-app")
// 输出: /tmp/my-app.lock
```

## 目录结构

```
SubNodeSync/
├── pkg/
│   ├── node/          # 节点管理模块
│   │   └── node.go    # 节点注册和管理
│   ├── sync/          # 命令同步模块
│   │   ├── command.go # 命令接收器
│   │   ├── context.go # 上下文管理
│   │   └── handlers.go# 内置处理器
│   ├── transport/     # 传输层模块
│   │   └── mqtt.go    # MQTT客户端
│   ├── util/          # 工具模块
│   │   └── filelock.go# 文件锁实现
│   └── log/           # 日志模块
│       └── log.go     # 日志封装
├── examples/          # 示例代码
│   ├── basic/         # 基础示例
│   ├── advanced/      # 高级示例
│   ├── with_filelock/ # 文件锁示例
│   └── with_custom_handler/ # 自定义处理器示例
├── docs/              # 文档
│   ├── API.md         # API文档
│   └── ARCHITECTURE.md# 架构文档
├── go.mod
├── go.sum
├── README.md
└── LICENSE
```

## 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件。

## 贡献

欢迎提交 Issue 和 Pull Request！

## 更新日志

### v1.1.0 (2025-12-08)
- 新增文件锁机制，防止多实例运行
- 新增 `RegisterWithLock` 和 `MustRegisterWithLock` 函数
- 新增 `IsAnotherInstanceRunning` 检查函数
- 新增 `pkg/util` 工具模块
- 新增文件锁使用示例

### v1.0.0 (2025-12-08)
- 初始版本
- 支持节点注册和心跳
- 支持MQTT命令控制
- 支持自定义命令处理器

