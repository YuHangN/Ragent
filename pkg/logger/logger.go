package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// L 是应用全局 zap 日志器。
var L *zap.Logger

// Init 初始化全局 JSON 日志器。
func Init() {
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "time"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.AddSync(os.Stdout),
		zapcore.InfoLevel,
	)
	L = zap.New(core, zap.AddCaller())
}

// Info 使用全局日志器记录 info 级别日志。
func Info(msg string, fields ...zap.Field) { L.Info(msg, fields...) }

// Error 使用全局日志器记录 error 级别日志。
func Error(msg string, fields ...zap.Field) { L.Error(msg, fields...) }

// Fatal 使用全局日志器记录 fatal 级别日志并退出进程。
func Fatal(msg string, fields ...zap.Field) { L.Fatal(msg, fields...) }
