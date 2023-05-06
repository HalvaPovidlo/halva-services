package log

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger(debug bool) *zap.Logger {
	config := zap.NewDevelopmentConfig()
	if debug {
		config.Level.SetLevel(zapcore.DebugLevel)
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config.Level.SetLevel(zapcore.InfoLevel)
		config.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	}
	zapLogger, _ := config.Build()
	return zapLogger
}
