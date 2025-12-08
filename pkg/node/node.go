/*
 * SubNodeSync - 分布式节点同步框架
 * pkg/node/node.go
 * 节点注册管理模块 - 提供节点注册、心跳和状态同步功能
 *
 * Copyright (c) 2024. All Rights Reserved.
 * Licensed under the MIT License.
 */

package node

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	gosync "sync"
	"time"

	nodesync "github.com/yourusername/subnodesync/pkg/sync"
	"github.com/yourusername/subnodesync/pkg/transport"
	"github.com/yourusername/subnodesync/pkg/util"
)

// 配置常量
const (
	// DefaultMQTTBroker 默认MQTT broker地址
	DefaultMQTTBroker = "tcp://127.0.0.1:1883"

	// ReconnectInterval 重连间隔
	ReconnectInterval = 15 * time.Minute

	// HeartbeatInterval 心跳间隔
	HeartbeatInterval = 30 * time.Second
)

// Endpoint 管理引擎端点地址，可通过SetEndpoint或环境变量NODE_ENGINE_URL设置
var Endpoint string

// Instance 节点实例信息
type Instance struct {
	// 基本信息
	NodeName   string // 节点名称
	InstanceID string // 实例唯一标识 (nodeName-hostname-pid)
	Hostname   string // 主机名
	PID        int    // 进程ID

	// 内部组件
	mqttClient      *transport.MQTTClient
	receiver        *nodesync.CommandReceiver
	connected       bool
	mu              gosync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
	reconnectTicker *time.Ticker

	// 文件锁（用于防止多实例运行）
	fileLock *util.FileLock

	// 配置
	config *Config
}

// Config 节点配置
type Config struct {
	// MQTT配置
	MQTTBroker   string
	MQTTUsername string
	MQTTPassword string

	// 引擎配置
	EngineEndpoint string

	// 心跳配置
	HeartbeatInterval time.Duration

	// 文件锁配置
	// EnableFileLock 启用文件锁以防止多实例运行
	// 默认为false，需要显式开启
	EnableFileLock bool

	// 自定义元数据
	Metadata map[string]string
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		MQTTBroker:        getMQTTBroker(),
		HeartbeatInterval: HeartbeatInterval,
		Metadata:          make(map[string]string),
	}
}

var (
	currentInstance *Instance
	instanceMu      gosync.Mutex
)

// SetEndpoint 设置管理引擎端点地址
func SetEndpoint(e string) {
	Endpoint = e
}

// resolveEndpoint 解析引擎端点地址
func resolveEndpoint() string {
	if Endpoint != "" {
		return Endpoint
	}
	if env := os.Getenv("NODE_ENGINE_URL"); env != "" {
		return env
	}
	return "http://localhost:9957"
}

// GetInstanceID 生成唯一实例标识
// 格式: nodeName-hostname-pid
func GetInstanceID(nodeName string) string {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	pid := os.Getpid()
	return fmt.Sprintf("%s-%s-%d", nodeName, hostname, pid)
}

// GetCurrentInstance 获取当前节点实例
func GetCurrentInstance() *Instance {
	instanceMu.Lock()
	defer instanceMu.Unlock()
	return currentInstance
}

// Register 注册节点到管理引擎
//
// 设计说明：
// - 通过 MQTT 建立与引擎的连接，自动注册心跳、状态和日志消息
// - 默认使用 127.0.0.1:1883 作为 MQTT broker
// - 如果 MQTT 连接失败，不阻塞应用启动，而是启动后台重连任务
// - 使用 nodeName-hostname-pid 作为唯一实例标识，解决同名节点冲突问题
func Register(nodeName string) error {
	return RegisterWithConfig(nodeName, DefaultConfig())
}

// RegisterWithConfig 使用自定义配置注册节点
func RegisterWithConfig(nodeName string, config *Config) error {
	if nodeName == "" {
		return fmt.Errorf("nodeName is required")
	}

	instanceMu.Lock()
	defer instanceMu.Unlock()

	// 如果启用了文件锁，尝试获取锁
	var fileLock *util.FileLock
	if config.EnableFileLock {
		fileLock = util.AcquireLock(nodeName)
		if fileLock == nil {
			return fmt.Errorf("另一个 %s 实例已在运行中，无法获取文件锁", nodeName)
		}
		log.Printf("[SubNodeSync] 文件锁已获取: %s", util.GetLockFilePath(nodeName))
	}

	// 创建节点实例
	hostname, _ := os.Hostname()
	ctx, cancel := context.WithCancel(context.Background())

	instance := &Instance{
		NodeName:   nodeName,
		InstanceID: GetInstanceID(nodeName),
		Hostname:   hostname,
		PID:        os.Getpid(),
		connected:  false,
		ctx:        ctx,
		cancel:     cancel,
		fileLock:   fileLock,
		config:     config,
	}
	currentInstance = instance

	log.Printf("[SubNodeSync] 节点实例信息: name=%s, instanceID=%s, hostname=%s, pid=%d",
		nodeName, instance.InstanceID, hostname, instance.PID)

	// 尝试连接 MQTT
	if err := instance.connectMQTT(); err != nil {
		log.Printf("[SubNodeSync] MQTT 初始连接失败: %v，将在后台重试", err)
		// 启动后台重连任务
		go instance.startReconnectLoop()
	} else {
		log.Printf("[SubNodeSync] MQTT 连接成功: %s", instance.InstanceID)
	}

	// 可选：通过 HTTP 进行轻量级注册
	if err := instance.registerViaHTTP(); err != nil {
		log.Printf("[SubNodeSync] HTTP 注册失败: %v (继续运行)", err)
	}

	return nil
}

// connectMQTT 连接MQTT broker
func (inst *Instance) connectMQTT() error {
	brokerURL := inst.config.MQTTBroker
	if brokerURL == "" {
		brokerURL = getMQTTBroker()
	}

	// 使用实例ID作为客户端ID，确保唯一性
	mqttClient, err := transport.NewMQTTClientWithID(
		inst.NodeName,
		inst.InstanceID,
		brokerURL,
		inst.config.MQTTUsername,
		inst.config.MQTTPassword,
	)
	if err != nil {
		return err
	}

	if err := mqttClient.Connect(); err != nil {
		return err
	}

	inst.mu.Lock()
	inst.mqttClient = mqttClient
	inst.connected = true
	inst.mu.Unlock()

	// 设置控制消息处理
	mqttClient.SetControlHandler(inst.handleControl)

	// 启动命令接收和心跳机制
	go inst.startCommandReceiver(brokerURL)

	return nil
}

// handleControl 处理控制消息
func (inst *Instance) handleControl(action string) {
	log.Printf("[SubNodeSync] 收到控制命令: %s", action)
	switch action {
	case "stop":
		log.Printf("[SubNodeSync] 收到停止命令，准备退出...")
		os.Exit(0)
	case "restart":
		log.Printf("[SubNodeSync] 收到重启命令...")
		inst.Stop()
		os.Exit(0)
	}
}

// startReconnectLoop 启动后台重连循环
func (inst *Instance) startReconnectLoop() {
	inst.reconnectTicker = time.NewTicker(ReconnectInterval)
	defer inst.reconnectTicker.Stop()

	log.Printf("[SubNodeSync] 启动 MQTT 后台重连任务，间隔: %v", ReconnectInterval)

	for {
		select {
		case <-inst.ctx.Done():
			log.Printf("[SubNodeSync] MQTT 重连任务已停止")
			return
		case <-inst.reconnectTicker.C:
			inst.mu.RLock()
			connected := inst.connected
			inst.mu.RUnlock()

			if connected {
				if inst.mqttClient != nil && inst.mqttClient.IsConnected() {
					continue
				}
				inst.mu.Lock()
				inst.connected = false
				inst.mu.Unlock()
			}

			log.Printf("[SubNodeSync] 尝试重新连接 MQTT...")
			if err := inst.connectMQTT(); err != nil {
				log.Printf("[SubNodeSync] MQTT 重连失败: %v，将在 %v 后重试", err, ReconnectInterval)
			} else {
				log.Printf("[SubNodeSync] MQTT 重连成功: %s", inst.InstanceID)
			}
		}
	}
}

// startCommandReceiver 启动命令接收器
func (inst *Instance) startCommandReceiver(brokerURL string) {
	// 创建命令接收器
	receiver := nodesync.NewCommandReceiverWithInstanceID(inst.NodeName, inst.InstanceID, brokerURL)

	inst.mu.Lock()
	inst.receiver = receiver
	inst.mu.Unlock()

	// 注册默认命令处理器
	receiver.RegisterHandler("stop", nodesync.NewStopHandler(func() {
		log.Printf("[%s] 收到停止命令，准备退出...", inst.InstanceID)
		os.Exit(0)
	}))
	receiver.RegisterHandler("status", nodesync.NewStatusHandler())
	receiver.RegisterHandler("query", nodesync.NewQueryHandler())

	// 启动命令接收器（包含心跳发送）
	if err := receiver.Start(inst.ctx); err != nil {
		log.Printf("[%s] 启动命令接收器失败: %v", inst.InstanceID, err)
		return
	}

	log.Printf("[%s] 命令接收器已启动, broker=%s", inst.InstanceID, brokerURL)
}

// registerViaHTTP 通过HTTP注册节点
func (inst *Instance) registerViaHTTP() error {
	payload := map[string]interface{}{
		"node_name":   inst.NodeName,
		"instance_id": inst.InstanceID,
		"hostname":    inst.Hostname,
		"pid":         inst.PID,
		"metadata":    inst.config.Metadata,
	}
	if v := os.Getenv("APP_BUILD_ID"); v != "" {
		payload["build_id"] = v
	}
	if v := os.Getenv("APP_BUILD_TIME"); v != "" {
		payload["build_time"] = v
	}

	httpClient := &http.Client{Timeout: 5 * time.Second}
	data, _ := json.Marshal(payload)
	url := resolveEndpoint() + "/api/nodes/register"

	resp, err := httpClient.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("post register: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("register failed: status=%d", resp.StatusCode)
	}

	return nil
}

// Stop 停止节点实例
//
// 释放所有资源，包括：
// - 取消上下文
// - 断开MQTT连接
// - 停止命令接收器
// - 释放文件锁（如果启用）
func (inst *Instance) Stop() {
	if inst.cancel != nil {
		inst.cancel()
	}
	if inst.mqttClient != nil {
		inst.mqttClient.Disconnect()
	}
	if inst.receiver != nil {
		inst.receiver.Stop()
	}
	// 释放文件锁
	if inst.fileLock != nil {
		inst.fileLock.Release()
		log.Printf("[SubNodeSync] 文件锁已释放: %s", inst.NodeName)
	}
}

// IsConnected 检查MQTT是否已连接
func (inst *Instance) IsConnected() bool {
	inst.mu.RLock()
	defer inst.mu.RUnlock()
	return inst.connected && inst.mqttClient != nil && inst.mqttClient.IsConnected()
}

// GetMQTTClient 获取MQTT客户端
func (inst *Instance) GetMQTTClient() *transport.MQTTClient {
	inst.mu.RLock()
	defer inst.mu.RUnlock()
	return inst.mqttClient
}

// getMQTTBroker 获取MQTT broker地址
func getMQTTBroker() string {
	// 根据操作系统选择默认broker
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		// 开发环境可能使用远程broker
		if env := os.Getenv("MQTT_BROKER_URL"); env != "" {
			return env
		}
	}
	return getEnvOrDefault("MQTT_BROKER_URL", DefaultMQTTBroker)
}

// getEnvOrDefault 获取环境变量或返回默认值
func getEnvOrDefault(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

// Shutdown 优雅关闭当前节点实例
func Shutdown() {
	instanceMu.Lock()
	defer instanceMu.Unlock()

	if currentInstance != nil {
		currentInstance.Stop()
		currentInstance = nil
	}
}

// RegisterWithLock 注册节点并启用文件锁
//
// 这是 Register 的便捷版本，自动启用文件锁以防止多实例运行。
// 如果已有另一个实例运行，将返回错误。
//
// 参数:
//   - nodeName: 节点名称
//
// 返回:
//   - error: 如果注册失败或另一个实例已运行，返回错误
//
// 示例:
//
//	if err := node.RegisterWithLock("my-app"); err != nil {
//	    log.Fatalf("启动失败: %v", err)
//	}
//	defer node.Shutdown()
func RegisterWithLock(nodeName string) error {
	config := DefaultConfig()
	config.EnableFileLock = true
	return RegisterWithConfig(nodeName, config)
}

// MustRegisterWithLock 注册节点并启用文件锁（失败时panic）
//
// 这是 RegisterWithLock 的便捷版本，如果注册失败会调用 log.Fatal。
// 适用于应用程序启动时，如果无法注册则直接退出的场景。
//
// 参数:
//   - nodeName: 节点名称
//
// 示例:
//
//	func main() {
//	    node.MustRegisterWithLock("my-app")
//	    defer node.Shutdown()
//	    // ... 应用逻辑
//	}
func MustRegisterWithLock(nodeName string) {
	if err := RegisterWithLock(nodeName); err != nil {
		log.Fatalf("[SubNodeSync] 节点注册失败: %v", err)
	}
}

// IsAnotherInstanceRunning 检查是否有另一个实例正在运行
//
// 这是一个只读检查函数，不会修改任何锁状态。
//
// 参数:
//   - nodeName: 节点名称
//
// 返回:
//   - bool: 是否有另一个实例运行
//   - int: 运行中实例的PID，如果没有则为0
func IsAnotherInstanceRunning(nodeName string) (bool, int) {
	return util.IsLocked(nodeName)
}

// GetLockFilePath 获取锁文件路径
//
// 用于调试或检查锁文件位置。
//
// 参数:
//   - nodeName: 节点名称
//
// 返回:
//   - string: 锁文件的完整路径
func GetLockFilePath(nodeName string) string {
	return util.GetLockFilePath(nodeName)
}

