package logger

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"fuzoj/pkg/utils/contextkey"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var globalLogger *Logger

// Logger wraps zap logger with context support
type Logger struct {
	zap   *zap.Logger
	level zapcore.Level
}

// Config holds logger configuration
type Config struct {
	Level      string // debug, info, warn, error
	Format     string // json, console
	OutputPath string // file path or "stdout"
	ErrorPath  string // error log file path or "stderr"
	Service    string // service name
	Env        string // environment name
	Cluster    string // cluster name
}

// Init initializes the global logger
func Init(cfg Config) error {
	logger, err := NewLogger(cfg)
	if err != nil {
		return err
	}
	globalLogger = logger
	return nil
}

// NewLogger creates a new logger instance
func NewLogger(cfg Config) (*Logger, error) {
	// Parse log level
	level := zapcore.InfoLevel
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}

	// Encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    "func",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     customTimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Choose encoder
	var encoder zapcore.Encoder
	if cfg.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// Output paths
	outputPath := cfg.OutputPath
	if outputPath == "" {
		outputPath = "stdout"
	}

	errorPath := cfg.ErrorPath
	if errorPath == "" {
		errorPath = "stderr"
	}

	// Create writer syncer
	var writeSyncer zapcore.WriteSyncer
	if outputPath == "stdout" {
		writeSyncer = zapcore.AddSync(os.Stdout)
	} else {
		file, err := os.OpenFile(outputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		writeSyncer = zapcore.AddSync(file)
	}

	core := zapcore.NewCore(encoder, writeSyncer, level)

	// Create logger with caller info
	options := []zap.Option{zap.AddCaller(), zap.AddCallerSkip(1), zap.AddStacktrace(zapcore.ErrorLevel)}
	staticFields := buildStaticFields(cfg)
	if len(staticFields) > 0 {
		options = append(options, zap.Fields(staticFields...))
	}
	zapLogger := zap.New(core, options...)

	return &Logger{zap: zapLogger, level: level}, nil
}

func buildStaticFields(cfg Config) []zap.Field {
	fields := make([]zap.Field, 0, 3)
	if cfg.Service != "" {
		fields = append(fields, zap.String("service", cfg.Service))
	}
	if cfg.Env != "" {
		fields = append(fields, zap.String("env", cfg.Env))
	}
	if cfg.Cluster != "" {
		fields = append(fields, zap.String("cluster", cfg.Cluster))
	}
	return fields
}

// customTimeEncoder formats time in RFC3339 format
func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format(time.RFC3339))
}

// Sync flushes any buffered log entries
func (l *Logger) Sync() error {
	return l.zap.Sync()
}

// WithContext extracts fields from context (like trace_id) and returns logger with those fields
func (l *Logger) WithContext(ctx context.Context) *zap.Logger {
	fields := extractFieldsFromContext(ctx)
	return l.zap.With(fields...)
}

// extractFieldsFromContext extracts structured fields from context
func extractFieldsFromContext(ctx context.Context) []zap.Field {
	var fields []zap.Field

	// Extract trace ID if exists
	if traceID := ctx.Value(contextkey.TraceID); traceID != nil {
		fields = append(fields, zap.String("trace_id", fmt.Sprint(traceID)))
	}

	// Extract user ID if exists
	if userID := ctx.Value(contextkey.UserID); userID != nil {
		fields = append(fields, zap.Any("user_id", userID))
	}

	// Extract request ID if exists
	if requestID := ctx.Value(contextkey.RequestID); requestID != nil {
		fields = append(fields, zap.String("request_id", fmt.Sprint(requestID)))
	}

	return fields
}

// Global logger convenience functions

// Debug logs a debug message
func Debug(ctx context.Context, msg string, fields ...zap.Field) {
	if globalLogger == nil {
		return
	}
	globalLogger.WithContext(ctx).Debug(msg, fields...)
}

// Info logs an info message
func Info(ctx context.Context, msg string, fields ...zap.Field) {
	if globalLogger == nil {
		return
	}
	globalLogger.WithContext(ctx).Info(msg, fields...)
}

// Warn logs a warning message
func Warn(ctx context.Context, msg string, fields ...zap.Field) {
	if globalLogger == nil {
		return
	}
	globalLogger.WithContext(ctx).Warn(msg, fields...)
}

// Error logs an error message
func Error(ctx context.Context, msg string, fields ...zap.Field) {
	if globalLogger == nil {
		return
	}
	globalLogger.WithContext(ctx).Error(msg, fields...)
}

// Fatal logs a fatal message and exits
func Fatal(ctx context.Context, msg string, fields ...zap.Field) {
	if globalLogger == nil {
		return
	}
	globalLogger.WithContext(ctx).Fatal(msg, fields...)
}

// Debugf logs a debug message with format
func Debugf(ctx context.Context, format string, args ...interface{}) {
	if globalLogger == nil {
		return
	}
	globalLogger.WithContext(ctx).Debug(fmt.Sprintf(format, args...))
}

// Infof logs an info message with format
func Infof(ctx context.Context, format string, args ...interface{}) {
	if globalLogger == nil {
		return
	}
	globalLogger.WithContext(ctx).Info(fmt.Sprintf(format, args...))
}

// Warnf logs a warning message with format
func Warnf(ctx context.Context, format string, args ...interface{}) {
	if globalLogger == nil {
		return
	}
	globalLogger.WithContext(ctx).Warn(fmt.Sprintf(format, args...))
}

// Errorf logs an error message with format
func Errorf(ctx context.Context, format string, args ...interface{}) {
	if globalLogger == nil {
		return
	}
	globalLogger.WithContext(ctx).Error(fmt.Sprintf(format, args...))
}

// WithFields returns a logger with pre-set fields
func WithFields(ctx context.Context, fields ...zap.Field) *zap.Logger {
	if globalLogger == nil {
		return nil
	}
	return globalLogger.WithContext(ctx).With(fields...)
}

// Sync flushes the global logger
func Sync() error {
	if globalLogger == nil {
		return nil
	}
	return globalLogger.Sync()
}

// GetLogger returns the global logger instance
func GetLogger() *Logger {
	return globalLogger
}

// IsDebug reports whether the global logger is in debug level.
func IsDebug() bool {
	return globalLogger != nil && globalLogger.level == zapcore.DebugLevel
}

// CallerField returns a log field with file location and function name.
func CallerField(skip int) zap.Field {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return zap.String("location", "unknown:0")
	}
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return zap.String("location", fmt.Sprintf("%s:%d", file, line))
	}
	return zap.String("location", fmt.Sprintf("%s:%d %s", file, line, fn.Name()))
}
