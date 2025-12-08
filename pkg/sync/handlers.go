/*
 * SubNodeSync - 分布式节点同步框架
 * pkg/sync/handlers.go
 * 内置命令处理器
 *
 * Copyright (c) 2024. All Rights Reserved.
 * Licensed under the MIT License.
 */

package sync

import (
	"context"
	"fmt"
	"log"
	"os"
)

// StopHandler 停止命令处理器
type StopHandler struct {
	cancelFunc context.CancelFunc
}

// NewStopHandler 创建停止命令处理器
func NewStopHandler(cancel context.CancelFunc) *StopHandler {
	return &StopHandler{cancelFunc: cancel}
}

// Handle 处理停止命令
func (h *StopHandler) Handle(ctx context.Context, cmd *Command) (*CommandResult, error) {
	log.Println("[SubNodeSync] 收到停止命令，准备优雅退出...")

	// 获取节点上下文
	nodeCtx := GetNodeContextFromContext(ctx)
	if nodeCtx != nil {
		nodeCtx.SetStatus(StatusStopping)
	}

	// 触发取消
	if h.cancelFunc != nil {
		h.cancelFunc()
	}

	return &CommandResult{
		Success:   true,
		Message:   "Shutdown signal sent",
		RequestID: cmd.RequestID,
	}, nil
}

// GetCommandName 获取命令名称
func (h *StopHandler) GetCommandName() string {
	return "stop"
}

// StatusHandler 状态查询命令处理器
type StatusHandler struct{}

// NewStatusHandler 创建状态查询处理器
func NewStatusHandler() *StatusHandler {
	return &StatusHandler{}
}

// Handle 处理状态查询命令
func (h *StatusHandler) Handle(ctx context.Context, cmd *Command) (*CommandResult, error) {
	nodeCtx := GetNodeContextFromContext(ctx)
	status := StatusUnknown
	if nodeCtx != nil {
		status = nodeCtx.GetStatus()
	}

	return &CommandResult{
		Success:   true,
		Message:   string(status),
		RequestID: cmd.RequestID,
	}, nil
}

// GetCommandName 获取命令名称
func (h *StatusHandler) GetCommandName() string {
	return "status"
}

// RestartHandler 重启命令处理器
type RestartHandler struct {
	nodeCtx *NodeContext
}

// NewRestartHandler 创建重启命令处理器
func NewRestartHandler(nodeCtx *NodeContext) *RestartHandler {
	return &RestartHandler{nodeCtx: nodeCtx}
}

// Handle 处理重启命令
func (h *RestartHandler) Handle(ctx context.Context, cmd *Command) (*CommandResult, error) {
	log.Println("[SubNodeSync] 收到重启命令...")

	if h.nodeCtx != nil {
		h.nodeCtx.SetStatus(StatusStopping)
		h.nodeCtx.Cancel()
	}

	return &CommandResult{
		Success:   true,
		Message:   "Restart initiated",
		RequestID: cmd.RequestID,
	}, nil
}

// GetCommandName 获取命令名称
func (h *RestartHandler) GetCommandName() string {
	return "restart"
}

// QueryHandler 查询命令处理器
type QueryHandler struct{}

// NewQueryHandler 创建查询命令处理器
func NewQueryHandler() *QueryHandler {
	return &QueryHandler{}
}

// Handle 处理查询命令
func (h *QueryHandler) Handle(ctx context.Context, cmd *Command) (*CommandResult, error) {
	nodeCtx := GetNodeContextFromContext(ctx)
	if nodeCtx == nil {
		return &CommandResult{
			Success:   false,
			Message:   "Node context not available",
			RequestID: cmd.RequestID,
		}, nil
	}

	return &CommandResult{
		Success:   true,
		Message:   formatNodeInfo(nodeCtx),
		RequestID: cmd.RequestID,
	}, nil
}

// GetCommandName 获取命令名称
func (h *QueryHandler) GetCommandName() string {
	return "query"
}

// formatNodeInfo 格式化节点信息
func formatNodeInfo(nodeCtx *NodeContext) string {
	return fmt.Sprintf("node_name=%s,version=%s,status=%s,pid=%d,uptime=%d",
		nodeCtx.GetNodeName(),
		nodeCtx.GetVersion(),
		string(nodeCtx.GetStatus()),
		os.Getpid(),
		nodeCtx.GetUptime(),
	)
}

// CustomHandler 自定义命令处理器
type CustomHandler struct {
	name    string
	handler func(ctx context.Context, cmd *Command) (*CommandResult, error)
}

// NewCustomHandler 创建自定义命令处理器
func NewCustomHandler(name string, handler func(ctx context.Context, cmd *Command) (*CommandResult, error)) *CustomHandler {
	return &CustomHandler{
		name:    name,
		handler: handler,
	}
}

// Handle 处理自定义命令
func (h *CustomHandler) Handle(ctx context.Context, cmd *Command) (*CommandResult, error) {
	return h.handler(ctx, cmd)
}

// GetCommandName 获取命令名称
func (h *CustomHandler) GetCommandName() string {
	return h.name
}

