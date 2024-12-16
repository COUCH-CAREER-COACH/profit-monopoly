package utils

import (
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	log  *zap.Logger
	once sync.Once
)

// InitLogger initializes the global logger instance
func InitLogger(debug bool) *zap.Logger {
	once.Do(func() {
		config := zap.NewProductionConfig()
		if debug {
			config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
		}

		config.OutputPaths = []string{"stdout", "mev-bot.log"}
		config.ErrorOutputPaths = []string{"stderr", "mev-bot-error.log"}

		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		config.EncoderConfig.StacktraceKey = "stacktrace"

		logger, err := config.Build(
			zap.AddCaller(),
			zap.AddStacktrace(zapcore.ErrorLevel),
		)
		if err != nil {
			panic(err)
		}

		log = logger
	})

	return log
}

// GetLogger returns the global logger instance
func GetLogger() *zap.Logger {
	if log == nil {
		return InitLogger(false)
	}
	return log
}

// CleanupLogger flushes any buffered log entries
func CleanupLogger() {
	if log != nil {
		_ = log.Sync()
	}
}
