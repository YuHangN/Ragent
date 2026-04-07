package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var L *zap.Logger

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

// 全局快捷函数

func Info(msg string, fields ...zap.Field)  { L.Info(msg, fields...) }
func Error(msg string, fields ...zap.Field) { L.Error(msg, fields...) }
func Fatal(msg string, fields ...zap.Field) { L.Fatal(msg, fields...) }
