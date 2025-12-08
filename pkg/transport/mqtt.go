/*
 * SubNodeSync - 分布式节点同步框架
 * pkg/transport/mqtt.go
 * MQTT传输层 - 提供MQTT客户端连接和消息传输功能
 *
 * Copyright (c) 2024. All Rights Reserved.
 * Licensed under the MIT License.
 */

package transport

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// MQTT主题常量
const (
	ControlTopic   = "v1/node/sync/%s/control"   // 控制命令主题
	HeartbeatTopic = "v1/node/sync/%s/heartbeat" // 心跳主题
	StatusTopic    = "v1/node/sync/%s/status"    // 状态主题
	RegisterTopic  = "v1/node/sync/%s/register"  // 注册主题
	LogTopic       = "v1/node/sync/%s/log"       // 日志主题
)

// MQTTClient MQTT客户端结构体
type MQTTClient struct {
	NodeName     string
	client       mqtt.Client
	connected    bool
	controlTopic string
	statusTopic  string
	logTopic     string
	onControl    func(action string)
}

// MQTTConfig MQTT配置
type MQTTConfig struct {
	BrokerURL string
	ClientID  string
	Username  string
	Password  string
	KeepAlive time.Duration
}

// DefaultMQTTConfig 默认MQTT配置
func DefaultMQTTConfig() *MQTTConfig {
	return &MQTTConfig{
		BrokerURL: "tcp://127.0.0.1:1883",
		KeepAlive: 60 * time.Second,
	}
}

// NewMQTTClient 创建新的MQTT客户端
func NewMQTTClient(nodeName string, config *MQTTConfig) (*MQTTClient, error) {
	if config == nil {
		config = DefaultMQTTConfig()
	}

	clientID := config.ClientID
	if clientID == "" {
		clientID = nodeName + "-client"
	}

	mqttClient := &MQTTClient{
		NodeName:     nodeName,
		connected:    false,
		controlTopic: fmt.Sprintf(ControlTopic, nodeName),
		statusTopic:  fmt.Sprintf(HeartbeatTopic, nodeName),
		logTopic:     fmt.Sprintf(LogTopic, nodeName),
	}

	// 配置MQTT客户端选项
	opts := mqtt.NewClientOptions()
	opts.AddBroker(config.BrokerURL)
	opts.SetClientID(clientID)
	opts.SetUsername(config.Username)
	opts.SetPassword(config.Password)
	opts.SetKeepAlive(config.KeepAlive)
	opts.SetAutoReconnect(true)
	opts.OnConnect = mqttClient.onConnect
	opts.OnConnectionLost = mqttClient.onConnectionLost

	mqttClient.client = mqtt.NewClient(opts)
	return mqttClient, nil
}

// NewMQTTClientWithID 创建带自定义客户端ID的MQTT客户端
func NewMQTTClientWithID(nodeName, clientID, brokerURL, username, password string) (*MQTTClient, error) {
	config := &MQTTConfig{
		BrokerURL: brokerURL,
		ClientID:  clientID,
		Username:  username,
		Password:  password,
		KeepAlive: 60 * time.Second,
	}
	return NewMQTTClient(nodeName, config)
}

// Connect 连接MQTT broker
func (m *MQTTClient) Connect() error {
	if token := m.client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	// 订阅控制主题
	if token := m.client.Subscribe(m.controlTopic, 1, m.onControlMessage); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

// Disconnect 断开MQTT连接
func (m *MQTTClient) Disconnect() {
	if m.client != nil && m.connected {
		m.client.Unsubscribe(m.controlTopic)
		m.client.Disconnect(250)
	}
}

// onConnect 连接成功回调
func (m *MQTTClient) onConnect(client mqtt.Client) {
	m.connected = true
	log.Printf("[SubNodeSync] MQTT客户端 %s 已连接到broker", m.NodeName)
}

// onConnectionLost 连接丢失回调
func (m *MQTTClient) onConnectionLost(client mqtt.Client, err error) {
	m.connected = false
	log.Printf("[SubNodeSync] MQTT客户端 %s 连接丢失: %v", m.NodeName, err)
}

// SetControlHandler 设置控制消息处理回调
func (m *MQTTClient) SetControlHandler(fn func(action string)) {
	m.onControl = fn
}

// onControlMessage 控制消息处理
func (m *MQTTClient) onControlMessage(client mqtt.Client, msg mqtt.Message) {
	var controlData struct {
		Action string                 `json:"action"`
		Params map[string]interface{} `json:"params,omitempty"`
	}

	if err := json.Unmarshal(msg.Payload(), &controlData); err != nil {
		log.Printf("[SubNodeSync] 解析控制消息失败: %v", err)
		return
	}

	if m.onControl != nil {
		m.onControl(controlData.Action)
	}
}

// IsConnected 检查MQTT连接状态
func (m *MQTTClient) IsConnected() bool {
	return m.connected && m.client.IsConnected()
}

// Publish 发布消息
func (m *MQTTClient) Publish(topic string, qos byte, retained bool, payload interface{}) error {
	if !m.IsConnected() {
		return fmt.Errorf("MQTT client not connected")
	}

	var data []byte
	var err error

	switch v := payload.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		data, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %w", err)
		}
	}

	token := m.client.Publish(topic, qos, retained, data)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

// Subscribe 订阅主题
func (m *MQTTClient) Subscribe(topic string, qos byte, callback mqtt.MessageHandler) error {
	if !m.IsConnected() {
		return fmt.Errorf("MQTT client not connected")
	}

	token := m.client.Subscribe(topic, qos, callback)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

// Unsubscribe 取消订阅
func (m *MQTTClient) Unsubscribe(topics ...string) error {
	if !m.IsConnected() {
		return fmt.Errorf("MQTT client not connected")
	}

	token := m.client.Unsubscribe(topics...)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

// SendHeartbeat 发送心跳消息
func (m *MQTTClient) SendHeartbeat() error {
	if !m.IsConnected() {
		return fmt.Errorf("MQTT client not connected")
	}

	heartbeatData := struct {
		Status   string            `json:"status"`
		PID      int               `json:"pid"`
		Memory   int64             `json:"memory"`
		CPU      float64           `json:"cpu"`
		Metadata map[string]string `json:"metadata,omitempty"`
	}{
		Status: "running",
		PID:    os.Getpid(),
	}

	return m.Publish(m.statusTopic, 1, false, heartbeatData)
}

// SendStatus 发送状态消息
func (m *MQTTClient) SendStatus(status string, details map[string]string) error {
	if !m.IsConnected() {
		return fmt.Errorf("MQTT client not connected")
	}

	pid := os.Getpid()
	statusData := struct {
		Status  string            `json:"status"`
		PID     *int              `json:"pid,omitempty"`
		Details map[string]string `json:"details,omitempty"`
	}{
		Status:  status,
		PID:     &pid,
		Details: details,
	}

	return m.Publish(m.statusTopic, 1, false, statusData)
}

// SendLog 发送日志消息
func (m *MQTTClient) SendLog(level, message string) error {
	if !m.IsConnected() {
		return fmt.Errorf("MQTT client not connected")
	}

	logData := struct {
		Level   string `json:"level"`
		Message string `json:"message"`
		Source  string `json:"source,omitempty"`
	}{
		Level:   level,
		Message: message,
		Source:  m.NodeName,
	}

	return m.Publish(m.logTopic, 1, false, logData)
}

// GetControlTopic 获取控制主题
func (m *MQTTClient) GetControlTopic() string {
	return m.controlTopic
}

// GetStatusTopic 获取状态主题
func (m *MQTTClient) GetStatusTopic() string {
	return m.statusTopic
}

// GetLogTopic 获取日志主题
func (m *MQTTClient) GetLogTopic() string {
	return m.logTopic
}

