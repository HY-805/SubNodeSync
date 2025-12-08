/*
 * SubNodeSync - 分布式节点同步框架
 * pkg/util/filelock.go
 * 文件锁模块 - 用于防止同一应用的多个实例同时运行
 *
 * 实现原理：
 * 1. 在系统临时目录创建一个以应用名称命名的锁文件（如：/tmp/myapp.lock）
 * 2. 锁文件内容为当前进程的PID
 * 3. 启动时检查锁文件是否存在：
 *    - 如果不存在，创建锁文件并写入当前PID
 *    - 如果存在，读取PID并检查该进程是否仍在运行
 *      - 进程仍在运行：拒绝获取锁，返回nil
 *      - 进程已终止（陈旧锁）：删除旧锁文件，创建新锁
 * 4. 进程正常退出时，释放锁文件（关闭并删除）
 *
 * 跨平台支持：
 * - Unix/Linux/macOS: 使用 syscall.Kill(pid, 0) 发送信号0检测进程存活
 * - Windows: 采用保守策略，假设进程运行中以避免误判
 *
 * 使用场景：
 * - 守护进程/服务程序，确保单实例运行
 * - 定时任务，避免任务重叠执行
 * - 资源独占型应用，防止竞争条件
 *
 * Copyright (c) 2024. All Rights Reserved.
 * Licensed under the MIT License.
 */

package util

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
)

// FileLock 文件锁结构体
// 封装了锁文件的文件句柄和路径，便于管理和释放
type FileLock struct {
	File *os.File // 锁文件句柄
	Path string   // 锁文件路径
}

// AcquireApplicationLock 获取应用程序锁
//
// 在系统临时目录创建一个锁文件，防止同一应用的多个实例同时运行。
// 如果已有实例正在运行，返回 nil。
// 如果发现陈旧的锁文件（进程已终止），会自动清理并重新获取锁。
//
// 参数:
//   - appName: 应用程序名称，用于生成锁文件名
//
// 返回:
//   - *os.File: 锁文件句柄，获取失败时为 nil
//   - string: 锁文件路径，获取失败时为空字符串
//
// 示例:
//
//	lockFile, lockPath := util.AcquireApplicationLock("my-app")
//	if lockFile == nil {
//	    log.Fatal("另一个实例正在运行")
//	}
//	defer util.ReleaseFileLock(lockFile, lockPath)
func AcquireApplicationLock(appName string) (*os.File, string) {
	// 构建锁文件路径：系统临时目录 + 应用名称 + .lock 后缀
	// 例如：/tmp/my-app.lock (Unix) 或 C:\Users\xxx\AppData\Local\Temp\my-app.lock (Windows)
	lockPath := filepath.Join(os.TempDir(), fmt.Sprintf("%s.lock", appName))

	// 检查锁文件是否已存在
	if _, err := os.Stat(lockPath); err == nil {
		// 锁文件存在，尝试读取其中的PID
		pidBytes, readErr := os.ReadFile(lockPath)
		if readErr == nil {
			// 解析PID
			if pid, convErr := strconv.Atoi(string(pidBytes)); convErr == nil {
				// 检查该PID对应的进程是否仍在运行
				if isProcessRunning(pid) {
					// 进程仍在运行，另一个实例正在使用此锁
					// 拒绝获取锁，返回nil表示失败
					return nil, ""
				}
			}
		}
		// 锁文件存在但进程已终止（陈旧锁），或PID无法解析
		// 删除旧的锁文件，准备创建新锁
		_ = os.Remove(lockPath)
	}

	// 以独占方式创建新的锁文件
	// O_CREATE: 如果文件不存在则创建
	// O_EXCL: 与 O_CREATE 配合使用，如果文件已存在则失败（原子操作）
	// O_WRONLY: 只写模式
	// 0644: 文件权限（所有者读写，其他只读）
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		// 文件创建失败，可能是竞态条件导致其他进程先创建了锁文件
		// 保守起见，假设有另一个实例正在运行
		return nil, ""
	}

	// 将当前进程的PID写入锁文件
	// 这样其他实例可以通过读取PID来判断锁的持有者是否仍然存活
	_, _ = fmt.Fprintf(f, "%d", os.Getpid())

	return f, lockPath
}

// AcquireLock 获取应用程序锁（返回 FileLock 结构体）
//
// 这是 AcquireApplicationLock 的封装版本，返回结构化的 FileLock 对象。
// 更便于在面向对象风格的代码中使用。
//
// 参数:
//   - appName: 应用程序名称
//
// 返回:
//   - *FileLock: 文件锁对象，获取失败时为 nil
//
// 示例:
//
//	lock := util.AcquireLock("my-app")
//	if lock == nil {
//	    log.Fatal("另一个实例正在运行")
//	}
//	defer lock.Release()
func AcquireLock(appName string) *FileLock {
	file, path := AcquireApplicationLock(appName)
	if file == nil {
		return nil
	}
	return &FileLock{
		File: file,
		Path: path,
	}
}

// Release 释放文件锁
//
// 关闭锁文件句柄并删除锁文件。
// 这是 FileLock 结构体的方法版本。
func (l *FileLock) Release() {
	if l == nil {
		return
	}
	ReleaseFileLock(l.File, l.Path)
}

// ReleaseFileLock 释放文件锁
//
// 关闭锁文件句柄并删除锁文件。
// 应该在应用程序退出时调用（通常通过 defer）。
//
// 参数:
//   - lockFile: 锁文件句柄
//   - lockPath: 锁文件路径
//
// 注意:
//   - 即使参数为 nil/空，函数也会安全处理
//   - 删除文件失败不会返回错误，因为这通常发生在非正常退出时
func ReleaseFileLock(lockFile *os.File, lockPath string) {
	// 关闭文件句柄
	if lockFile != nil {
		_ = lockFile.Close()
	}
	// 删除锁文件
	if lockPath != "" {
		_ = os.Remove(lockPath)
	}
}

// isProcessRunning 检查指定PID的进程是否正在运行
//
// 实现原理：
// - Unix系统：使用 syscall.Kill(pid, 0) 发送信号0
//   信号0不会实际发送给进程，但会检查进程是否存在和是否有权限发送信号
//   如果进程存在且有权限，返回nil；否则返回错误
// - Windows系统：由于缺乏类似机制，采用保守策略返回true
//   这意味着在Windows上，如果锁文件存在，会假设进程仍在运行
//
// 参数:
//   - pid: 要检查的进程ID
//
// 返回:
//   - bool: 进程是否正在运行
func isProcessRunning(pid int) bool {
	// PID必须为正数
	if pid <= 0 {
		return false
	}

	// Unix-like 系统（Linux、macOS、FreeBSD等）
	if runtime.GOOS != "windows" {
		// 发送信号0来检测进程是否存在
		// 这是一种标准的Unix进程存活检测方法
		// - 如果进程存在且调用者有权限，返回nil
		// - 如果进程不存在，返回ESRCH错误
		// - 如果没有权限，返回EPERM错误（但进程存在）
		if err := syscall.Kill(pid, 0); err == nil {
			return true
		}
		// 注意：这里简化处理，即使是EPERM也返回false
		// 在实际应用中，EPERM意味着进程存在但没有权限
		// 对于同一用户运行的应用，通常不会遇到权限问题
		return false
	}

	// Windows 系统
	// 由于Windows没有类似的信号机制，这里采用保守策略
	// 假设进程仍在运行，以避免误删其他进程的锁
	// 在实际应用中，可以通过OpenProcess+GetExitCodeProcess来实现更精确的检测
	return true
}

// TryAcquireLock 尝试获取锁，如果失败则等待重试
//
// 这是一个带重试机制的锁获取函数，适用于需要等待其他实例退出的场景。
//
// 参数:
//   - appName: 应用程序名称
//   - maxRetries: 最大重试次数，0表示不重试
//   - retryInterval: 重试间隔时间
//
// 返回:
//   - *FileLock: 文件锁对象，获取失败时为 nil
//
// 示例:
//
//	lock := util.TryAcquireLock("my-app", 3, time.Second)
//	if lock == nil {
//	    log.Fatal("无法获取锁")
//	}
//	defer lock.Release()
// func TryAcquireLock(appName string, maxRetries int, retryInterval time.Duration) *FileLock {
// 	for i := 0; i <= maxRetries; i++ {
// 		lock := AcquireLock(appName)
// 		if lock != nil {
// 			return lock
// 		}
// 		if i < maxRetries {
// 			time.Sleep(retryInterval)
// 		}
// 	}
// 	return nil
// }

// GetLockFilePath 获取锁文件的路径（不创建文件）
//
// 用于调试或检查锁文件位置。
//
// 参数:
//   - appName: 应用程序名称
//
// 返回:
//   - string: 锁文件的完整路径
func GetLockFilePath(appName string) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("%s.lock", appName))
}

// IsLocked 检查应用是否已被锁定（另一个实例正在运行）
//
// 这是一个只读检查函数，不会修改任何锁状态。
//
// 参数:
//   - appName: 应用程序名称
//
// 返回:
//   - bool: 是否已被锁定
//   - int: 持有锁的进程PID，未锁定时为0
func IsLocked(appName string) (bool, int) {
	lockPath := GetLockFilePath(appName)

	// 检查锁文件是否存在
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		return false, 0
	}

	// 读取PID
	pidBytes, err := os.ReadFile(lockPath)
	if err != nil {
		// 文件存在但无法读取，保守假设已锁定
		return true, 0
	}

	pid, err := strconv.Atoi(string(pidBytes))
	if err != nil {
		// PID格式错误，锁文件可能损坏
		return false, 0
	}

	// 检查进程是否运行
	if isProcessRunning(pid) {
		return true, pid
	}

	// 进程已终止，锁文件是陈旧的
	return false, 0
}

