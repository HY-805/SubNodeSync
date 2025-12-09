/*
 * SubNodeSync - 分布式节点同步框架
 * pkg/sync/command.go
 * MQTT命令接收器 - 负责处理来自管理引擎的命令
 *
 * Copyright (c) 2024. All Rights Reserved.
 * Licensed under the MIT License.
 */

package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/shirou/gopsutil/v4/process"
)

// MQTT 主题格式常量
const (
	TopicHeartbeat = "v1/subapp/pcs/%s/heartbeat" // 心跳主题
	TopicRegister  = "v1/subapp/pcs/%s/register"  // 注册主题
	TopicStatus    = "v1/subapp/pcs/%s/status"    // 状态主题
	TopicControl   = "v1/subapp/pcs/%s/control"   // 控制主题
	TopicConfig    = "v1/subapp/pcs/%s/config"    // 配置主题
)

// Command 命令结构
type Command struct {
	Command    string                 `json:"command"`
	Timestamp  string                 `json:"timestamp"`
	RequestID  string                 `json:"request_id"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// CommandResult 命令执行结果
type CommandResult struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

// CommandHandler 命令处理器接口
type CommandHandler interface {
	Handle(ctx context.Context, cmd *Command) (*CommandResult, error)
	GetCommandName() string
}

// ReceiverStatus 接收器状态
type ReceiverStatus string

const (
	ReceiverStatusStopped ReceiverStatus = "stopped"
	ReceiverStatusRunning ReceiverStatus = "running"
	ReceiverStatusError   ReceiverStatus = "error"
)

// CommandReceiver MQTT命令接收器
type CommandReceiver struct {
	nodeName   string
	instanceID string
	brokerURL  string
	client     mqtt.Client
	handlers   map[string]CommandHandler
	status     ReceiverStatus
	nodeCtx    *NodeContext
	cancelFunc context.CancelFunc
}

// NewCommandReceiver 创建命令接收器
func NewCommandReceiver(nodeName, brokerURL string) *CommandReceiver {
	return &CommandReceiver{
		nodeName:   nodeName,
		instanceID: nodeName,
		brokerURL:  brokerURL,
		handlers:   make(map[string]CommandHandler),
		status:     ReceiverStatusStopped,
	}
}

// NewCommandReceiverWithInstanceID 创建带实例ID的命令接收器
func NewCommandReceiverWithInstanceID(nodeName, instanceID, brokerURL string) *CommandReceiver {
	return &CommandReceiver{
		nodeName:   nodeName,
		instanceID: instanceID,
		brokerURL:  brokerURL,
		handlers:   make(map[string]CommandHandler),
		status:     ReceiverStatusStopped,
	}
}

// Start 启动命令接收器
func (r *CommandReceiver) Start(ctx context.Context) error {
	// 获取或创建NodeContext
	r.nodeCtx = GetNodeContextFromContext(ctx)
	if r.nodeCtx == nil {
		r.nodeCtx = NewNodeContext(ctx, r.nodeName, "")
	}

	// 创建可取消的上下文
	ctx, r.cancelFunc = context.WithCancel(ctx)

	// 配置MQTT客户端
	opts := mqtt.NewClientOptions()
	opts.AddBroker(r.brokerURL)
	opts.SetClientID(fmt.Sprintf("%s-receiver", r.instanceID))
	opts.SetAutoReconnect(true)
	opts.SetCleanSession(false)
	opts.SetKeepAlive(60 * time.Second)

	// 连接成功回调
	opts.OnConnect = func(client mqtt.Client) {
		log.Printf("[%s] MQTT命令接收器已连接", r.instanceID)
		// 订阅控制主题
		controlTopic := fmt.Sprintf(TopicControl, r.nodeName)
		if token := client.Subscribe(controlTopic, 1, r.handleControlMessage); token.Wait() && token.Error() != nil {
			log.Printf("[%s] 订阅控制主题失败: %v", r.instanceID, token.Error())
		} else {
			log.Printf("[%s] 已订阅控制主题: %s", r.instanceID, controlTopic)
		}
		// 发送注册消息
		r.sendRegisterMessage()
	}

	// 连接丢失回调
	opts.OnConnectionLost = func(client mqtt.Client, err error) {
		log.Printf("[%s] MQTT连接丢失: %v", r.instanceID, err)
	}

	// 创建并连接客户端
	r.client = mqtt.NewClient(opts)
	if token := r.client.Connect(); token.Wait() && token.Error() != nil {
		r.status = ReceiverStatusError
		return fmt.Errorf("MQTT连接失败: %w", token.Error())
	}

	r.status = ReceiverStatusRunning
	r.nodeCtx.SetStatus(StatusRunning)

	// 启动心跳发送
	go r.heartbeatLoop(ctx)

	return nil
}

// Stop 停止命令接收器
func (r *CommandReceiver) Stop() error {
	if r.cancelFunc != nil {
		r.cancelFunc()
	}
	if r.client != nil && r.client.IsConnected() {
		controlTopic := fmt.Sprintf(TopicControl, r.nodeName)
		r.client.Unsubscribe(controlTopic)
		r.client.Disconnect(250)
	}
	r.status = ReceiverStatusStopped
	return nil
}

// RegisterHandler 注册命令处理器
func (r *CommandReceiver) RegisterHandler(command string, handler CommandHandler) error {
	r.handlers[command] = handler
	return nil
}

// GetStatus 获取接收器状态
func (r *CommandReceiver) GetStatus() ReceiverStatus {
	return r.status
}

// handleControlMessage 处理控制消息
func (r *CommandReceiver) handleControlMessage(client mqtt.Client, msg mqtt.Message) {
	var cmd Command
	if err := json.Unmarshal(msg.Payload(), &cmd); err != nil {
		log.Printf("[%s] 解析控制消息失败: %v", r.nodeName, err)
		return
	}

	log.Printf("[%s] 收到控制命令: %s", r.nodeName, cmd.Command)

	// 查找并执行处理器
	if handler, ok := r.handlers[cmd.Command]; ok {
		ctx := WithNodeContext(context.Background(), r.nodeCtx)
		result, err := handler.Handle(ctx, &cmd)
		if err != nil {
			log.Printf("[%s] 命令执行失败: %v", r.nodeName, err)
		} else {
			log.Printf("[%s] 命令执行结果: %+v", r.nodeName, result)
		}
	} else {
		log.Printf("[%s] 未找到命令处理器: %s", r.nodeName, cmd.Command)
	}
}

// sendRegisterMessage 发送注册消息
func (r *CommandReceiver) sendRegisterMessage() {
	registerMsg := map[string]interface{}{
		"timestamp":   time.Now().Format(time.RFC3339),
		"app_name":    r.nodeName,
		"instance_id": r.instanceID,
		"version":     r.nodeCtx.GetVersion(),
		"pid":         os.Getpid(),
		"start_time":  r.nodeCtx.GetStartTime().Format(time.RFC3339),
		"capabilities": []string{
			"mqtt_control",
			"heartbeat",
		},
		"metadata": map[string]string{
			"hostname": getHostname(),
		},
	}

	// 如果有完整的版本信息，则添加到消息中
	if nodeVersion := r.nodeCtx.GetNodeVersion(); nodeVersion != nil {
		registerMsg["app_version"] = map[string]interface{}{
			"git_version":    nodeVersion.GitVersion,
			"git_commit":     nodeVersion.GitCommit,
			"git_tree_state": nodeVersion.GitTreeState,
			"build_date":     nodeVersion.BuildDate,
			"go_version":     nodeVersion.GoVersion,
			"compiler":       nodeVersion.Compiler,
			"platform":       nodeVersion.Platform,
		}
	}

	payload, _ := json.Marshal(registerMsg)
	topic := fmt.Sprintf(TopicRegister, r.nodeName)
	if token := r.client.Publish(topic, 1, false, payload); token.Wait() && token.Error() != nil {
		log.Printf("[%s] 发送注册消息失败: %v", r.instanceID, token.Error())
	} else {
		log.Printf("[%s] 已发送注册消息", r.instanceID)
	}
}

// heartbeatLoop 心跳发送循环
func (r *CommandReceiver) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// 立即发送一次心跳
	time.Sleep(1 * time.Second)
	r.sendHeartbeat()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[%s] 心跳循环退出", r.nodeName)
			return
		case <-ticker.C:
			r.sendHeartbeat()
		}
	}
}

// sendHeartbeat 发送心跳消息
func (r *CommandReceiver) sendHeartbeat() {
	if r.client == nil || !r.client.IsConnected() {
		return
	}

	// 获取系统监控数据
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	goroutineCount := runtime.NumGoroutine()

	// 获取CPU使用率
	cpuUsage := r.getCPUUsage()

	// 获取进程内存信息
	var memoryUsageMB uint64 = 0
	if p, err := process.NewProcess(int32(os.Getpid())); err == nil {
		if memInfo, err := p.MemoryInfo(); err == nil {
			memoryUsageMB = memInfo.RSS / 1024 / 1024
		} else {
			memoryUsageMB = uint64(m.Alloc / 1024 / 1024)
		}
	}

	// 构建心跳消息
	heartbeatMsg := map[string]interface{}{
		"timestamp":   time.Now().Format(time.RFC3339),
		"app_name":    r.nodeName,
		"instance_id": r.instanceID,
		"status":      string(r.nodeCtx.GetStatus()),
		"pid":         os.Getpid(),
		"uptime":      r.nodeCtx.GetUptime(),
		"version":     r.nodeCtx.GetVersion(),
		"hostname":    getHostname(),
		"metrics": map[string]interface{}{
			"process_cpu_usage_percent": strconv.FormatFloat(cpuUsage, 'f', 2, 64),
			"process_memory_usage_mb":   int(memoryUsageMB),
			"process_goroutine_count":   goroutineCount,
		},
	}

	// 添加版本信息
	if nodeVersion := r.nodeCtx.GetNodeVersion(); nodeVersion != nil {
		heartbeatMsg["app_version"] = map[string]interface{}{
			"git_version":    nodeVersion.GitVersion,
			"git_commit":     nodeVersion.GitCommit,
			"git_tree_state": nodeVersion.GitTreeState,
			"build_date":     nodeVersion.BuildDate,
			"go_version":     nodeVersion.GoVersion,
			"compiler":       nodeVersion.Compiler,
			"platform":       nodeVersion.Platform,
		}
	}

	payload, _ := json.Marshal(heartbeatMsg)
	topic := fmt.Sprintf(TopicHeartbeat, r.nodeName)
	if token := r.client.Publish(topic, 1, false, payload); token.Wait() && token.Error() != nil {
		log.Printf("[%s] 发送心跳失败: %v", r.instanceID, token.Error())
	}
}

// getCPUUsage 获取进程CPU使用率
func (r *CommandReceiver) getCPUUsage() float64 {
	p, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		return 0.0
	}

	cpuPercent, err := p.Percent(500 * time.Millisecond)
	if err != nil {
		return 0.0
	}

	return cpuPercent
}

// getHostname 获取主机名
func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}
