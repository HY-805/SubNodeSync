# SubNodeSync API 文档

## 目录

- [节点管理 (pkg/node)](#节点管理-pkgnode)
- [工具函数 (pkg/util)](#工具函数-pkgutil)
- [命令同步 (pkg/sync)](#命令同步-pkgsync)
- [传输层 (pkg/transport)](#传输层-pkgtransport)
- [日志 (pkg/log)](#日志-pkglog)

---

## 节点管理 (pkg/node)

### 函数

#### Register

```go
func Register(nodeName string) error
```

使用默认配置注册节点到管理引擎。

**参数:**
- `nodeName`: 节点名称，不能为空

**返回:**
- `error`: 如果注册失败返回错误，否则返回nil

**示例:**
```go
if err := node.Register("my-app"); err != nil {
    log.Printf("注册失败: %v", err)
}
```

---

#### RegisterWithConfig

```go
func RegisterWithConfig(nodeName string, config *Config) error
```

使用自定义配置注册节点。

**参数:**
- `nodeName`: 节点名称
- `config`: 自定义配置

**返回:**
- `error`: 如果注册失败返回错误

---

#### GetCurrentInstance

```go
func GetCurrentInstance() *Instance
```

获取当前节点实例。

**返回:**
- `*Instance`: 当前节点实例，如果未注册返回nil

---

#### GetInstanceID

```go
func GetInstanceID(nodeName string) string
```

生成唯一实例标识符。

**参数:**
- `nodeName`: 节点名称

**返回:**
- `string`: 格式为 `nodeName-hostname-pid` 的实例ID

---

#### SetEndpoint

```go
func SetEndpoint(e string)
```

设置管理引擎端点地址。

**参数:**
- `e`: 端点地址，如 `http://localhost:9957`

---

#### Shutdown

```go
func Shutdown()
```

优雅关闭当前节点实例。

---

#### RegisterWithLock

```go
func RegisterWithLock(nodeName string) error
```

注册节点并启用文件锁，防止多实例运行。

**参数:**
- `nodeName`: 节点名称

**返回:**
- `error`: 如果注册失败或另一个实例已运行，返回错误

**示例:**
```go
if err := node.RegisterWithLock("my-app"); err != nil {
    log.Fatalf("启动失败: %v", err)
}
defer node.Shutdown()
```

---

#### MustRegisterWithLock

```go
func MustRegisterWithLock(nodeName string)
```

注册节点并启用文件锁，如果失败则调用 `log.Fatal`。

**参数:**
- `nodeName`: 节点名称

**示例:**
```go
node.MustRegisterWithLock("my-app")
defer node.Shutdown()
```

---

#### IsAnotherInstanceRunning

```go
func IsAnotherInstanceRunning(nodeName string) (bool, int)
```

检查是否有另一个实例正在运行。

**参数:**
- `nodeName`: 节点名称

**返回:**
- `bool`: 是否有另一个实例运行
- `int`: 运行中实例的PID，如果没有则为0

---

#### GetLockFilePath

```go
func GetLockFilePath(nodeName string) string
```

获取锁文件路径。

**参数:**
- `nodeName`: 节点名称

**返回:**
- `string`: 锁文件的完整路径

---

### 类型

#### Config

```go
type Config struct {
    MQTTBroker        string            // MQTT broker地址
    MQTTUsername      string            // MQTT用户名
    MQTTPassword      string            // MQTT密码
    EngineEndpoint    string            // 管理引擎端点
    HeartbeatInterval time.Duration     // 心跳间隔
    EnableFileLock    bool              // 启用文件锁
    Metadata          map[string]string // 自定义元数据
}
```

节点配置结构。

| 字段 | 类型 | 描述 | 默认值 |
|------|------|------|--------|
| MQTTBroker | string | MQTT broker地址 | `tcp://127.0.0.1:1883` |
| MQTTUsername | string | MQTT用户名 | 空 |
| MQTTPassword | string | MQTT密码 | 空 |
| EngineEndpoint | string | 管理引擎端点 | `http://localhost:9957` |
| HeartbeatInterval | time.Duration | 心跳间隔 | 30秒 |
| EnableFileLock | bool | 启用文件锁防止多实例 | false |
| Metadata | map[string]string | 自定义元数据 | 空 |

---

#### Instance

```go
type Instance struct {
    NodeName   string // 节点名称
    InstanceID string // 实例唯一标识
    Hostname   string // 主机名
    PID        int    // 进程ID
}
```

节点实例信息。

**方法:**

| 方法 | 描述 |
|------|------|
| `Stop()` | 停止节点实例 |
| `IsConnected() bool` | 检查MQTT连接状态 |
| `GetMQTTClient() *transport.MQTTClient` | 获取MQTT客户端 |

---

## 工具函数 (pkg/util)

### 文件锁

#### AcquireApplicationLock

```go
func AcquireApplicationLock(appName string) (*os.File, string)
```

获取应用程序锁，防止多实例运行。

**参数:**
- `appName`: 应用程序名称

**返回:**
- `*os.File`: 锁文件句柄，获取失败时为nil
- `string`: 锁文件路径，获取失败时为空字符串

**示例:**
```go
lockFile, lockPath := util.AcquireApplicationLock("my-app")
if lockFile == nil {
    log.Fatal("另一个实例正在运行")
}
defer util.ReleaseFileLock(lockFile, lockPath)
```

---

#### AcquireLock

```go
func AcquireLock(appName string) *FileLock
```

获取应用程序锁（返回FileLock结构体）。

**参数:**
- `appName`: 应用程序名称

**返回:**
- `*FileLock`: 文件锁对象，获取失败时为nil

**示例:**
```go
lock := util.AcquireLock("my-app")
if lock == nil {
    log.Fatal("另一个实例正在运行")
}
defer lock.Release()
```

---

#### ReleaseFileLock

```go
func ReleaseFileLock(lockFile *os.File, lockPath string)
```

释放文件锁。

---

#### IsLocked

```go
func IsLocked(appName string) (bool, int)
```

检查应用是否已被锁定。

**参数:**
- `appName`: 应用程序名称

**返回:**
- `bool`: 是否已被锁定
- `int`: 持有锁的进程PID

---

#### GetLockFilePath

```go
func GetLockFilePath(appName string) string
```

获取锁文件的路径。

---

### 类型

#### FileLock

```go
type FileLock struct {
    File *os.File // 锁文件句柄
    Path string   // 锁文件路径
}
```

文件锁结构体。

**方法:**

| 方法 | 描述 |
|------|------|
| `Release()` | 释放文件锁 |

---

## 命令同步 (pkg/sync)

### 函数

#### NewCommandReceiver

```go
func NewCommandReceiver(nodeName, brokerURL string) *CommandReceiver
```

创建命令接收器。

---

#### NewCommandReceiverWithInstanceID

```go
func NewCommandReceiverWithInstanceID(nodeName, instanceID, brokerURL string) *CommandReceiver
```

创建带实例ID的命令接收器。

---

#### NewNodeContext

```go
func NewNodeContext(parent context.Context, nodeName, version string) *NodeContext
```

创建节点上下文。

---

### 类型

#### CommandReceiver

```go
type CommandReceiver struct {
    // 内部字段
}
```

MQTT命令接收器。

**方法:**

| 方法 | 描述 |
|------|------|
| `Start(ctx context.Context) error` | 启动命令接收器 |
| `Stop() error` | 停止命令接收器 |
| `RegisterHandler(command string, handler CommandHandler) error` | 注册命令处理器 |
| `GetStatus() ReceiverStatus` | 获取接收器状态 |

---

#### Command

```go
type Command struct {
    Command    string                 `json:"command"`
    Timestamp  string                 `json:"timestamp"`
    RequestID  string                 `json:"request_id"`
    Parameters map[string]interface{} `json:"parameters,omitempty"`
}
```

命令结构。

---

#### CommandResult

```go
type CommandResult struct {
    Success   bool   `json:"success"`
    Message   string `json:"message"`
    RequestID string `json:"request_id,omitempty"`
}
```

命令执行结果。

---

#### CommandHandler

```go
type CommandHandler interface {
    Handle(ctx context.Context, cmd *Command) (*CommandResult, error)
    GetCommandName() string
}
```

命令处理器接口。

---

#### NodeContext

```go
type NodeContext struct {
    context.Context
    // 内部字段
}
```

节点生命周期上下文。

**方法:**

| 方法 | 描述 |
|------|------|
| `GetNodeName() string` | 获取节点名称 |
| `GetVersion() string` | 获取版本号 |
| `GetStartTime() time.Time` | 获取启动时间 |
| `GetUptime() int64` | 获取运行时长(秒) |
| `SetStatus(status NodeStatus)` | 设置节点状态 |
| `GetStatus() NodeStatus` | 获取节点状态 |
| `Cancel()` | 取消上下文 |
| `AddShutdownHook(hook func())` | 添加关闭钩子 |
| `ExecuteShutdownHooks()` | 执行关闭钩子 |

---

#### NodeStatus

```go
type NodeStatus string

const (
    StatusDiscovered NodeStatus = "discovered"
    StatusPending    NodeStatus = "pending"
    StatusStarting   NodeStatus = "starting"
    StatusRunning    NodeStatus = "running"
    StatusStopping   NodeStatus = "stopping"
    StatusStopped    NodeStatus = "stopped"
    StatusError      NodeStatus = "error"
    StatusUnknown    NodeStatus = "unknown"
)
```

节点状态枚举。

---

### 内置处理器

#### NewStopHandler

```go
func NewStopHandler(cancel context.CancelFunc) *StopHandler
```

创建停止命令处理器。

---

#### NewStatusHandler

```go
func NewStatusHandler() *StatusHandler
```

创建状态查询处理器。

---

#### NewRestartHandler

```go
func NewRestartHandler(nodeCtx *NodeContext) *RestartHandler
```

创建重启命令处理器。

---

#### NewQueryHandler

```go
func NewQueryHandler() *QueryHandler
```

创建查询命令处理器。

---

#### NewCustomHandler

```go
func NewCustomHandler(name string, handler func(ctx context.Context, cmd *Command) (*CommandResult, error)) *CustomHandler
```

创建自定义命令处理器。

---

## 传输层 (pkg/transport)

### 函数

#### NewMQTTClient

```go
func NewMQTTClient(nodeName string, config *MQTTConfig) (*MQTTClient, error)
```

创建MQTT客户端。

---

#### NewMQTTClientWithID

```go
func NewMQTTClientWithID(nodeName, clientID, brokerURL, username, password string) (*MQTTClient, error)
```

创建带自定义客户端ID的MQTT客户端。

---

### 类型

#### MQTTClient

```go
type MQTTClient struct {
    NodeName string
    // 内部字段
}
```

MQTT客户端。

**方法:**

| 方法 | 描述 |
|------|------|
| `Connect() error` | 连接MQTT broker |
| `Disconnect()` | 断开连接 |
| `IsConnected() bool` | 检查连接状态 |
| `SetControlHandler(fn func(action string))` | 设置控制消息处理器 |
| `Publish(topic string, qos byte, retained bool, payload interface{}) error` | 发布消息 |
| `Subscribe(topic string, qos byte, callback mqtt.MessageHandler) error` | 订阅主题 |
| `Unsubscribe(topics ...string) error` | 取消订阅 |
| `SendHeartbeat() error` | 发送心跳 |
| `SendStatus(status string, details map[string]string) error` | 发送状态 |
| `SendLog(level, message string) error` | 发送日志 |

---

#### MQTTConfig

```go
type MQTTConfig struct {
    BrokerURL string
    ClientID  string
    Username  string
    Password  string
    KeepAlive time.Duration
}
```

MQTT配置。

---

## 日志 (pkg/log)

### 函数

#### Init

```go
func Init(opts *Options)
```

初始化全局日志。

---

#### Debug/Info/Warn/Error/Fatal

```go
func Debug(msg string, fields ...zap.Field)
func Info(msg string, fields ...zap.Field)
func Warn(msg string, fields ...zap.Field)
func Error(msg string, fields ...zap.Field)
func Fatal(msg string, fields ...zap.Field)
```

输出不同级别的日志。

---

#### Debugf/Infof/Warnf/Errorf/Fatalf

```go
func Debugf(format string, args ...interface{})
func Infof(format string, args ...interface{})
func Warnf(format string, args ...interface{})
func Errorf(format string, args ...interface{})
func Fatalf(format string, args ...interface{})
```

格式化输出日志。

---

### 字段构造函数

```go
func String(key, val string) zap.Field
func Int(key string, val int) zap.Field
func Int64(key string, val int64) zap.Field
func Float64(key string, val float64) zap.Field
func Bool(key string, val bool) zap.Field
func Any(key string, val interface{}) zap.Field
func Err(err error) zap.Field
```

---

### 类型

#### Options

```go
type Options struct {
    Level       string   // 日志级别: debug, info, warn, error
    Format      string   // 输出格式: console, json
    OutputPaths []string // 输出路径
    Development bool     // 开发模式
    EnableColor bool     // 启用颜色
}
```

日志配置选项。

---

## MQTT 主题

| 常量 | 主题格式 | 用途 |
|------|----------|------|
| `TopicRegister` | `v1/node/sync/{node_name}/register` | 注册消息 |
| `TopicHeartbeat` | `v1/node/sync/{node_name}/heartbeat` | 心跳消息 |
| `TopicControl` | `v1/node/sync/{node_name}/control` | 控制命令 |
| `TopicStatus` | `v1/node/sync/{node_name}/status` | 状态消息 |
| `TopicConfig` | `v1/node/sync/{node_name}/config` | 配置消息 |

