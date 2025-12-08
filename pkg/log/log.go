/*
 * SubNodeSync - 分布式节点同步框架
 * pkg/log/log.go
 * 日志模块 - 基于zap的结构化日志封装
 *
 * Copyright (c) 2024. All Rights Reserved.
 * Licensed under the MIT License.
 */

package log

import (
	"os"
	gosync "sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Level 日志级别类型
type Level = zapcore.Level

// 日志级别常量
const (
	DebugLevel = zapcore.DebugLevel
	InfoLevel  = zapcore.InfoLevel
	WarnLevel  = zapcore.WarnLevel
	ErrorLevel = zapcore.ErrorLevel
	PanicLevel = zapcore.PanicLevel
	FatalLevel = zapcore.FatalLevel
)

// Logger 日志接口
type Logger interface {
	Debug(msg string, fields ...zap.Field)
	Info(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
	Fatal(msg string, fields ...zap.Field)
	With(fields ...zap.Field) Logger
	Sugar() *zap.SugaredLogger
	Sync() error
}

// Options 日志配置选项
type Options struct {
	Level       string   `json:"level" mapstructure:"level"`
	Format      string   `json:"format" mapstructure:"format"`
	OutputPaths []string `json:"output-paths" mapstructure:"output-paths"`
	Development bool     `json:"development" mapstructure:"development"`
	EnableColor bool     `json:"enable-color" mapstructure:"enable-color"`
}

// DefaultOptions 默认日志配置
func DefaultOptions() *Options {
	return &Options{
		Level:       "info",
		Format:      "console",
		OutputPaths: []string{"stdout"},
		Development: false,
		EnableColor: true,
	}
}

// logger 日志实现
type logger struct {
	zapLogger *zap.Logger
}

var (
	std  *logger
	once gosync.Once
)

// Init 初始化全局日志
func Init(opts *Options) {
	once.Do(func() {
		std = newLogger(opts)
	})
}

// newLogger 创建日志实例
func newLogger(opts *Options) *logger {
	if opts == nil {
		opts = DefaultOptions()
	}

	var level zapcore.Level
	if err := level.UnmarshalText([]byte(opts.Level)); err != nil {
		level = zapcore.InfoLevel
	}

	encoderConfig := zapcore.EncoderConfig{
		MessageKey:     "message",
		LevelKey:       "level",
		TimeKey:        "timestamp",
		NameKey:        "logger",
		CallerKey:      "caller",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	if opts.EnableColor && opts.Format == "console" {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	var encoder zapcore.Encoder
	if opts.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// 创建输出
	var writers []zapcore.WriteSyncer
	for _, path := range opts.OutputPaths {
		if path == "stdout" {
			writers = append(writers, zapcore.AddSync(os.Stdout))
		} else if path == "stderr" {
			writers = append(writers, zapcore.AddSync(os.Stderr))
		} else {
			file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err == nil {
				writers = append(writers, zapcore.AddSync(file))
			}
		}
	}

	if len(writers) == 0 {
		writers = append(writers, zapcore.AddSync(os.Stdout))
	}

	core := zapcore.NewCore(
		encoder,
		zapcore.NewMultiWriteSyncer(writers...),
		level,
	)

	zapOpts := []zap.Option{
		zap.AddCaller(),
		zap.AddCallerSkip(1),
	}
	if opts.Development {
		zapOpts = append(zapOpts, zap.Development())
	}

	return &logger{
		zapLogger: zap.New(core, zapOpts...),
	}
}

// getStd 获取全局日志实例
func getStd() *logger {
	if std == nil {
		Init(DefaultOptions())
	}
	return std
}

// Debug 输出Debug级别日志
func (l *logger) Debug(msg string, fields ...zap.Field) {
	l.zapLogger.Debug(msg, fields...)
}

// Info 输出Info级别日志
func (l *logger) Info(msg string, fields ...zap.Field) {
	l.zapLogger.Info(msg, fields...)
}

// Warn 输出Warn级别日志
func (l *logger) Warn(msg string, fields ...zap.Field) {
	l.zapLogger.Warn(msg, fields...)
}

// Error 输出Error级别日志
func (l *logger) Error(msg string, fields ...zap.Field) {
	l.zapLogger.Error(msg, fields...)
}

// Fatal 输出Fatal级别日志
func (l *logger) Fatal(msg string, fields ...zap.Field) {
	l.zapLogger.Fatal(msg, fields...)
}

// With 创建带有字段的子日志
func (l *logger) With(fields ...zap.Field) Logger {
	return &logger{
		zapLogger: l.zapLogger.With(fields...),
	}
}

// Sugar 获取SugaredLogger
func (l *logger) Sugar() *zap.SugaredLogger {
	return l.zapLogger.Sugar()
}

// Sync 同步日志
func (l *logger) Sync() error {
	return l.zapLogger.Sync()
}

// 全局函数

// Debug 输出Debug级别日志
func Debug(msg string, fields ...zap.Field) {
	getStd().Debug(msg, fields...)
}

// Debugf 格式化输出Debug级别日志
func Debugf(format string, args ...interface{}) {
	getStd().Sugar().Debugf(format, args...)
}

// Info 输出Info级别日志
func Info(msg string, fields ...zap.Field) {
	getStd().Info(msg, fields...)
}

// Infof 格式化输出Info级别日志
func Infof(format string, args ...interface{}) {
	getStd().Sugar().Infof(format, args...)
}

// Warn 输出Warn级别日志
func Warn(msg string, fields ...zap.Field) {
	getStd().Warn(msg, fields...)
}

// Warnf 格式化输出Warn级别日志
func Warnf(format string, args ...interface{}) {
	getStd().Sugar().Warnf(format, args...)
}

// Error 输出Error级别日志
func Error(msg string, fields ...zap.Field) {
	getStd().Error(msg, fields...)
}

// Errorf 格式化输出Error级别日志
func Errorf(format string, args ...interface{}) {
	getStd().Sugar().Errorf(format, args...)
}

// Fatal 输出Fatal级别日志
func Fatal(msg string, fields ...zap.Field) {
	getStd().Fatal(msg, fields...)
}

// Fatalf 格式化输出Fatal级别日志
func Fatalf(format string, args ...interface{}) {
	getStd().Sugar().Fatalf(format, args...)
}

// With 创建带有字段的子日志
func With(fields ...zap.Field) Logger {
	return getStd().With(fields...)
}

// Sync 同步日志
func Sync() error {
	return getStd().Sync()
}

// 字段构造函数

// String 创建字符串字段
func String(key, val string) zap.Field {
	return zap.String(key, val)
}

// Int 创建整数字段
func Int(key string, val int) zap.Field {
	return zap.Int(key, val)
}

// Int64 创建64位整数字段
func Int64(key string, val int64) zap.Field {
	return zap.Int64(key, val)
}

// Float64 创建浮点数字段
func Float64(key string, val float64) zap.Field {
	return zap.Float64(key, val)
}

// Bool 创建布尔字段
func Bool(key string, val bool) zap.Field {
	return zap.Bool(key, val)
}

// Any 创建任意类型字段
func Any(key string, val interface{}) zap.Field {
	return zap.Any(key, val)
}

// Err 创建错误字段
func Err(err error) zap.Field {
	return zap.Error(err)
}

