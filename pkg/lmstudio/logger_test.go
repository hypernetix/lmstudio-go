package lmstudio

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
)

// TestNewLogger tests the creation of a new logger
func TestNewLogger(t *testing.T) {
	logger := NewLogger(LogLevelInfo)
	if logger == nil {
		t.Fatal("Expected non-nil logger")
	}

	if logger.level != LogLevelInfo {
		t.Errorf("Expected log level %d, got %d", LogLevelInfo, logger.level)
	}
}

// TestSetLevel tests setting the log level
func TestSetLevel(t *testing.T) {
	logger := NewLogger(LogLevelError)
	if logger.level != LogLevelError {
		t.Errorf("Initial log level should be %d, got %d", LogLevelError, logger.level)
	}

	logger.SetLevel(LogLevelDebug)
	if logger.level != LogLevelDebug {
		t.Errorf("Log level should be %d after SetLevel, got %d", LogLevelDebug, logger.level)
	}
}

// captureOutput captures log output for testing
func captureOutput(f func()) string {
	// Redirect log output to a buffer
	var buf bytes.Buffer
	log.SetOutput(&buf)

	// Execute the function that produces log output
	f()

	// Reset log output to stderr
	log.SetOutput(os.Stderr)

	return buf.String()
}

// TestErrorLog tests that error logs are always shown regardless of level
func TestErrorLog(t *testing.T) {
	testCases := []struct {
		level   LogLevel
		message string
	}{
		{LogLevelError, "Error at error level"},
		{LogLevelWarn, "Error at warn level"},
		{LogLevelInfo, "Error at info level"},
		{LogLevelDebug, "Error at debug level"},
		{LogLevelTrace, "Error at trace level"},
	}

	for _, tc := range testCases {
		t.Run("ErrorLevel"+logLevelToString(tc.level), func(t *testing.T) {
			logger := NewLogger(tc.level)
			output := captureOutput(func() {
				logger.Error(tc.message)
			})

			if !strings.Contains(output, "[ERROR]") || !strings.Contains(output, tc.message) {
				t.Errorf("Expected error log with message '%s', got: '%s'", tc.message, output)
			}
		})
	}
}

// TestWarnLog tests warn log behavior at different levels
func TestWarnLog(t *testing.T) {
	testCases := []struct {
		level    LogLevel
		message  string
		expected bool
	}{
		{LogLevelError, "Warn message", false},
		{LogLevelWarn, "Warn message", true},
		{LogLevelInfo, "Warn message", true},
		{LogLevelDebug, "Warn message", true},
		{LogLevelTrace, "Warn message", true},
	}

	for _, tc := range testCases {
		t.Run("WarnLevel"+logLevelToString(tc.level), func(t *testing.T) {
			logger := NewLogger(tc.level)
			output := captureOutput(func() {
				logger.Warn(tc.message)
			})

			containsMessage := strings.Contains(output, "[WARN]") && strings.Contains(output, tc.message)
			if tc.expected && !containsMessage {
				t.Errorf("Expected warn log with message '%s', got: '%s'", tc.message, output)
			} else if !tc.expected && containsMessage {
				t.Errorf("Did not expect warn log with message '%s', but got: '%s'", tc.message, output)
			}
		})
	}
}

// TestInfoLog tests info log behavior at different levels
func TestInfoLog(t *testing.T) {
	testCases := []struct {
		level    LogLevel
		message  string
		expected bool
	}{
		{LogLevelError, "Info message", false},
		{LogLevelWarn, "Info message", false},
		{LogLevelInfo, "Info message", true},
		{LogLevelDebug, "Info message", true},
		{LogLevelTrace, "Info message", true},
	}

	for _, tc := range testCases {
		t.Run("InfoLevel"+logLevelToString(tc.level), func(t *testing.T) {
			logger := NewLogger(tc.level)
			output := captureOutput(func() {
				logger.Info(tc.message)
			})

			containsMessage := strings.Contains(output, "[INFO]") && strings.Contains(output, tc.message)
			if tc.expected && !containsMessage {
				t.Errorf("Expected info log with message '%s', got: '%s'", tc.message, output)
			} else if !tc.expected && containsMessage {
				t.Errorf("Did not expect info log with message '%s', but got: '%s'", tc.message, output)
			}
		})
	}
}

// TestDebugLog tests debug log behavior at different levels
func TestDebugLog(t *testing.T) {
	testCases := []struct {
		level    LogLevel
		message  string
		expected bool
	}{
		{LogLevelError, "Debug message", false},
		{LogLevelWarn, "Debug message", false},
		{LogLevelInfo, "Debug message", false},
		{LogLevelDebug, "Debug message", true},
		{LogLevelTrace, "Debug message", true},
	}

	for _, tc := range testCases {
		t.Run("DebugLevel"+logLevelToString(tc.level), func(t *testing.T) {
			logger := NewLogger(tc.level)
			output := captureOutput(func() {
				logger.Debug(tc.message)
			})

			containsMessage := strings.Contains(output, "[DEBUG]") && strings.Contains(output, tc.message)
			if tc.expected && !containsMessage {
				t.Errorf("Expected debug log with message '%s', got: '%s'", tc.message, output)
			} else if !tc.expected && containsMessage {
				t.Errorf("Did not expect debug log with message '%s', but got: '%s'", tc.message, output)
			}
		})
	}
}

// TestTraceLog tests trace log behavior at different levels
func TestTraceLog(t *testing.T) {
	testCases := []struct {
		level    LogLevel
		message  string
		expected bool
	}{
		{LogLevelError, "Trace message", false},
		{LogLevelWarn, "Trace message", false},
		{LogLevelInfo, "Trace message", false},
		{LogLevelDebug, "Trace message", false},
		{LogLevelTrace, "Trace message", true},
	}

	for _, tc := range testCases {
		t.Run("TraceLevel"+logLevelToString(tc.level), func(t *testing.T) {
			logger := NewLogger(tc.level)
			output := captureOutput(func() {
				logger.Trace(tc.message)
			})

			containsMessage := strings.Contains(output, "[TRACE]") && strings.Contains(output, tc.message)
			if tc.expected && !containsMessage {
				t.Errorf("Expected trace log with message '%s', got: '%s'", tc.message, output)
			} else if !tc.expected && containsMessage {
				t.Errorf("Did not expect trace log with message '%s', but got: '%s'", tc.message, output)
			}
		})
	}
}

// Helper function to convert LogLevel to string for testing purposes
func logLevelToString(level LogLevel) string {
	switch level {
	case LogLevelError:
		return "Error"
	case LogLevelWarn:
		return "Warn"
	case LogLevelInfo:
		return "Info"
	case LogLevelDebug:
		return "Debug"
	case LogLevelTrace:
		return "Trace"
	default:
		return "Unknown"
	}
}

// TestLogLevelString tests string representation of log levels
func TestLogLevelString(t *testing.T) {
	testCases := []struct {
		level LogLevel
		str   string
	}{
		{LogLevelError, "Error"},
		{LogLevelWarn, "Warn"},
		{LogLevelInfo, "Info"},
		{LogLevelDebug, "Debug"},
		{LogLevelTrace, "Trace"},
		{LogLevel(99), "Unknown"},
	}

	for _, tc := range testCases {
		t.Run(tc.str, func(t *testing.T) {
			if logLevelToString(tc.level) != tc.str {
				t.Errorf("Expected string '%s' for level %d, got '%s'", tc.str, tc.level, logLevelToString(tc.level))
			}
		})
	}
}

// TestFormatting tests log message formatting
func TestFormatting(t *testing.T) {
	logger := NewLogger(LogLevelDebug)
	message := "Test %s %d"
	arg1 := "string"
	arg2 := 42

	output := captureOutput(func() {
		logger.Debug(message, arg1, arg2)
	})

	expected := "Test string 42"
	if !strings.Contains(output, expected) {
		t.Errorf("Expected formatted message '%s', got: '%s'", expected, output)
	}
}
