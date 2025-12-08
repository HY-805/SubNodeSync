/*
 * SubNodeSync - 分布式节点同步框架
 * pkg/sync/context.go
 * 应用生命周期上下文管理
 *
 * Copyright (c) 2024. All Rights Reserved.
 * Licensed under the MIT License.
 */

package sync

import (
	"context"
	gosync "sync"
	"sync/atomic"
	"time"
)

// NodeStatus 节点状态类型
type NodeStatus string

const (
	StatusDiscovered NodeStatus = "discovered" // 已发现
	StatusPending    NodeStatus = "pending"    // 待启动
	StatusStarting   NodeStatus = "starting"   // 启动中
	StatusRunning    NodeStatus = "running"    // 运行中
	StatusStopping   NodeStatus = "stopping"   // 停止中
	StatusStopped    NodeStatus = "stopped"    // 已停止
	StatusError      NodeStatus = "error"      // 错误
	StatusUnknown    NodeStatus = "unknown"    // 未知
)

// NodeVersion 节点版本信息结构
type NodeVersion struct {
	GitVersion   string `json:"gitVersion"`
	GitCommit    string `json:"gitCommit"`
	GitTreeState string `json:"gitTreeState"`
	BuildDate    string `json:"buildDate"`
	GoVersion    string `json:"goVersion"`
	Compiler     string `json:"compiler"`
	Platform     string `json:"platform"`
}

// NodeContext 节点生命周期上下文
type NodeContext struct {
	context.Context
	cancelFunc context.CancelFunc

	// 节点信息
	nodeID      string
	nodeName    string
	version     string
	nodeVersion *NodeVersion

	// 状态管理
	status    atomic.Value
	startTime time.Time

	// 优雅退出
	shutdownHooks []func()
	mu            gosync.Mutex
}

// NewNodeContext 创建节点上下文
func NewNodeContext(parent context.Context, nodeName, version string) *NodeContext {
	return NewNodeContextWithVersion(parent, nodeName, version, nil)
}

// NewNodeContextWithVersion 创建带有完整版本信息的节点上下文
func NewNodeContextWithVersion(parent context.Context, nodeName, version string, nodeVersion *NodeVersion) *NodeContext {
	ctx, cancel := context.WithCancel(parent)
	nodeCtx := &NodeContext{
		Context:       ctx,
		cancelFunc:    cancel,
		nodeName:      nodeName,
		version:       version,
		nodeVersion:   nodeVersion,
		startTime:     time.Now(),
		shutdownHooks: make([]func(), 0),
	}
	nodeCtx.status.Store(StatusStarting)
	return nodeCtx
}

// GetNodeName 获取节点名称
func (c *NodeContext) GetNodeName() string {
	return c.nodeName
}

// GetVersion 获取版本号
func (c *NodeContext) GetVersion() string {
	return c.version
}

// GetNodeVersion 获取完整的节点版本信息
func (c *NodeContext) GetNodeVersion() *NodeVersion {
	return c.nodeVersion
}

// SetNodeVersion 设置节点版本信息
func (c *NodeContext) SetNodeVersion(nodeVersion *NodeVersion) {
	c.nodeVersion = nodeVersion
}

// GetStartTime 获取启动时间
func (c *NodeContext) GetStartTime() time.Time {
	return c.startTime
}

// GetUptime 获取运行时长（秒）
func (c *NodeContext) GetUptime() int64 {
	return int64(time.Since(c.startTime).Seconds())
}

// SetStatus 设置节点状态
func (c *NodeContext) SetStatus(status NodeStatus) {
	c.status.Store(status)
}

// GetStatus 获取节点状态
func (c *NodeContext) GetStatus() NodeStatus {
	if v := c.status.Load(); v != nil {
		return v.(NodeStatus)
	}
	return StatusUnknown
}

// Cancel 取消上下文，触发优雅退出
func (c *NodeContext) Cancel() {
	c.SetStatus(StatusStopping)
	c.cancelFunc()
}

// AddShutdownHook 注册优雅退出钩子
func (c *NodeContext) AddShutdownHook(hook func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.shutdownHooks = append(c.shutdownHooks, hook)
}

// ExecuteShutdownHooks 执行所有优雅退出钩子
func (c *NodeContext) ExecuteShutdownHooks() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 逆序执行钩子（后注册的先执行）
	for i := len(c.shutdownHooks) - 1; i >= 0; i-- {
		c.shutdownHooks[i]()
	}
	c.SetStatus(StatusStopped)
}

// contextKey 用于在context中存储NodeContext
type contextKey struct{}

// WithNodeContext 将NodeContext存储到context中
func WithNodeContext(ctx context.Context, nodeCtx *NodeContext) context.Context {
	return context.WithValue(ctx, contextKey{}, nodeCtx)
}

// GetNodeContextFromContext 从context中获取NodeContext
func GetNodeContextFromContext(ctx context.Context) *NodeContext {
	if v := ctx.Value(contextKey{}); v != nil {
		return v.(*NodeContext)
	}
	return nil
}

