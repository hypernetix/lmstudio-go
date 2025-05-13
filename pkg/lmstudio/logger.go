package lmstudio

import "log"

// LogLevel defines the level of logging
type LogLevel int

const (
	// LogLevelError only shows error messages
	LogLevelError LogLevel = iota
	// LogLevelWarn shows warning and error messages
	LogLevelWarn
	// LogLevelInfo shows info and error messages
	LogLevelInfo
	// LogLevelDebug shows all messages including debug
	LogLevelDebug
	// LogLevelTrace shows all messages including trace
	LogLevelTrace
)

// loggerStruct is a simple logging facility with support for different log levels
type loggerStruct struct {
	level LogLevel
}

// NewLogger creates a new logger with the specified log level
func NewLogger(level LogLevel) *loggerStruct {
	return &loggerStruct{
		level: level,
	}
}

func (l *loggerStruct) SetLevel(level LogLevel) {
	l.level = level
}

func (l *loggerStruct) Error(format string, v ...interface{}) {
	// Error messages are always shown
	log.Printf("[ERROR] "+format, v...)
}

// Warn logs a warning message if the log level is Warn or higher
func (l *loggerStruct) Warn(format string, v ...interface{}) {
	if l.level >= LogLevelWarn {
		log.Printf("[WARN] "+format, v...)
	}
}

func (l *loggerStruct) Info(format string, v ...interface{}) {
	if l.level >= LogLevelInfo {
		log.Printf("[INFO] "+format, v...)
	}
}

func (l *loggerStruct) Debug(format string, v ...interface{}) {
	if l.level >= LogLevelDebug {
		log.Printf("[DEBUG] "+format, v...)
	}
}

func (l *loggerStruct) Trace(format string, v ...interface{}) {
	if l.level >= LogLevelTrace {
		log.Printf("[TRACE] "+format, v...)
	}
}

// Logger is the interface for logging, it can be overridden by the client code
type Logger interface {
	SetLevel(level LogLevel)
	Error(format string, v ...interface{})
	Warn(format string, v ...interface{})
	Info(format string, v ...interface{})
	Debug(format string, v ...interface{})
	Trace(format string, v ...interface{})
}
