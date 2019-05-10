package log

import (
	_log "log"
	"os"
)

const (
	// EnvLogLevel ks the environment variable name to configure
	// the default logger's log level at program startup.
	EnvLogLevel = "CYBOZU_LOG_LEVEL"
)

var (
	defaultLogger *Logger
)

func init() {
	defaultLogger = NewLogger()
	_log.SetOutput(defaultLogger.Writer(LvInfo))
	// no date/time needed
	_log.SetFlags(0)

	level := os.Getenv(EnvLogLevel)
	if len(level) > 0 {
		defaultLogger.SetThresholdByName(level)
	}
}

// DefaultLogger returns the pointer to the default logger.
func DefaultLogger() *Logger {
	return defaultLogger
}

// Enabled does the same for Logger.Enabled() for the default logger.
func Enabled(level int) bool {
	return defaultLogger.Enabled(level)
}

// Critical outputs a critical log using the default logger.
// fields can be nil.
func Critical(msg string, fields map[string]interface{}) error {
	return defaultLogger.Log(LvCritical, msg, fields)
}

// Error outputs an error log using the default logger.
// fields can be nil.
func Error(msg string, fields map[string]interface{}) error {
	return defaultLogger.Log(LvError, msg, fields)
}

// Warn outputs a warning log using the default logger.
// fields can be nil.
func Warn(msg string, fields map[string]interface{}) error {
	return defaultLogger.Log(LvWarn, msg, fields)
}

// Info outputs an informational log using the default logger.
// fields can be nil.
func Info(msg string, fields map[string]interface{}) error {
	return defaultLogger.Log(LvInfo, msg, fields)
}

// Debug outputs a debug log using the default logger.
// fields can be nil.
func Debug(msg string, fields map[string]interface{}) error {
	return defaultLogger.Log(LvDebug, msg, fields)
}

// ErrorExit outputs an error log using the default logger, then exit.
func ErrorExit(err error) {
	Error(err.Error(), nil)
	os.Exit(1)
}
