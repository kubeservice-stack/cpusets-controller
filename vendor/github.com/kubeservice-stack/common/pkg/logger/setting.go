package logger

import (
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/kubeservice-stack/common/pkg/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

var (
	isTerminal         = IsTerminal(os.Stdout)
	maxModuleNameLen   uint32 //最大长度
	mediaLogger        atomic.Value
	accessLogger       atomic.Value
	crashLogger        atomic.Value
	defaultLogger      = newDefaultLogger()                      //默认logger
	RunningAtomicLevel = zap.NewAtomicLevelAt(zapcore.InfoLevel) //设置日志等级
)

const (
	LogFilename       = "application.log"
	accessLogFileName = "access.log"
	crashLogFileName  = "crash.log"
)

// GetLogger return logger with module name
func GetLogger(module, role string) *Logger {
	length := len(module)
	for {
		currentMaxModuleLen := atomic.LoadUint32(&maxModuleNameLen)
		if uint32(length) <= currentMaxModuleLen {
			break
		}
		if atomic.CompareAndSwapUint32(&maxModuleNameLen, currentMaxModuleLen, uint32(length)) {
			break
		}
	}
	return &Logger{
		module: module,
		role:   role,
	}
}

// newDefaultLogger creates a default logger for uninitialized usage
func newDefaultLogger() *zap.Logger {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = SimpleTimeEncoder
	encoderConfig.EncodeLevel = SimpleLevelEncoder
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		os.Stdout,
		RunningAtomicLevel)
	return zap.New(core)
}

// NewLogger initializes a zap logger from user config
func NewLogger(cfg config.Logging) error {
	if cfg.Filename != "" {
		if err := newLogger(cfg.Filename, cfg); err != nil {
			return err
		}
	} else {
		if err := newLogger(LogFilename, cfg); err != nil {
			return err
		}
	}
	if err := newLogger(accessLogFileName, cfg); err != nil {
		return err
	}
	if err := newLogger(crashLogFileName, cfg); err != nil {
		return err
	}
	return nil
}

// newLogger initializes a zap logger for different module
func newLogger(logFilename string, cfg config.Logging) error {
	w := zapcore.AddSync(&lumberjack.Logger{
		Filename:   filepath.Join(cfg.Dir, logFilename),
		MaxSize:    int(cfg.MaxSize),
		MaxBackups: int(cfg.MaxBackups),
		MaxAge:     int(cfg.MaxAge),
	})
	// check if it is terminal
	if isTerminal && cfg.IsTerminal {
		w = os.Stdout
	}
	// parse logging level
	if err := RunningAtomicLevel.UnmarshalText([]byte(cfg.Level)); err != nil {
		return err
	}
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = SimpleTimeEncoder
	switch {
	case logFilename == accessLogFileName:
		encoderConfig.EncodeLevel = SimpleAccessLevelEncoder
	default:
		encoderConfig.EncodeLevel = SimpleLevelEncoder
	}
	// check format
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		w,
		RunningAtomicLevel)
	switch {
	case logFilename == accessLogFileName:
		accessLogger.Store(zap.New(core))
	case logFilename == crashLogFileName:
		crashLogger.Store(zap.New(core))
	default:
		mediaLogger.Store(zap.New(core))
	}
	return nil
}
