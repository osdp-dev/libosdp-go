/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp

/*
#include "cgo_helpers.h"
*/
import "C"
import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"
)

// LogHandler represents a function that handles log messages from LibOSDP.
// pd is the PD index that produced the log (-1 for global/non-PD-scoped messages).
type LogHandler func(pd int, level LogLevel, file string, line int, message string)

var (
	// libosdp's log callback is process-global, so the handler is too. It is
	// read from the C callback (any thread) and written by SetLogHandler, so it
	// must be synchronized.
	logMu            sync.RWMutex
	globalLogHandler LogHandler
)

// SetLogHandler sets a custom log handler for LibOSDP messages
func SetLogHandler(handler LogHandler) {
	logMu.Lock()
	globalLogHandler = handler
	logMu.Unlock()
	C.set_log_callback_wrapper()
}

// SetStandardLogger configures LibOSDP to use Go's standard log package
func SetStandardLogger(logger *log.Logger, prefix string) {
	if logger == nil {
		logger = log.Default()
	}

	SetLogHandler(func(pd int, level LogLevel, file string, line int, message string) {
		// Format: [LEVEL] file:line message
		logLine := fmt.Sprintf("[%s] %s:%d %s",
			level.String(),
			filepath.Base(file),
			line,
			strings.TrimSpace(message))

		if prefix != "" {
			logLine = prefix + " " + logLine
		}

		switch level {
		case LogEmergency, LogAlert, LogCritical, LogError:
			logger.Printf("ERROR: %s", logLine)
		case LogWarning:
			logger.Printf("WARN: %s", logLine)
		case LogNotice, LogInfo:
			logger.Printf("INFO: %s", logLine)
		case LogDebug:
			logger.Printf("DEBUG: %s", logLine)
		default:
			logger.Printf("%s", logLine)
		}
	})
}

// SetSlogLogger configures LibOSDP to use Go's structured logging (slog)
func SetSlogLogger(logger *slog.Logger, component string) {
	if logger == nil {
		logger = slog.Default()
	}

	SetLogHandler(func(pd int, level LogLevel, file string, line int, message string) {
		// Convert LibOSDP log level to slog level
		var slogLevel slog.Level
		switch level {
		case LogEmergency, LogAlert, LogCritical:
			slogLevel = slog.LevelError + 4 // Higher than normal error
		case LogError:
			slogLevel = slog.LevelError
		case LogWarning:
			slogLevel = slog.LevelWarn
		case LogNotice, LogInfo:
			slogLevel = slog.LevelInfo
		case LogDebug:
			slogLevel = slog.LevelDebug
		default:
			slogLevel = slog.LevelInfo
		}

		// Create structured log entry
		args := []any{
			"component", component,
			"file", filepath.Base(file),
			"line", line,
			"osdp_level", level.String(),
		}

		logger.Log(context.Background(), slogLevel, strings.TrimSpace(message), args...)
	})
}

// SetLevelFilteredLogger wraps another LogHandler with level filtering
func SetLevelFilteredLogger(minLevel LogLevel, handler LogHandler) {
	SetLogHandler(func(pd int, level LogLevel, file string, line int, message string) {
		if level <= minLevel {
			handler(pd, level, file, line, message)
		}
	})
}

// SetDiscardLogger disables LibOSDP logging completely
func SetDiscardLogger() {
	SetLogHandler(func(pd int, level LogLevel, file string, line int, message string) {
		// Discard all log messages
	})
}

// InitializeLogging initializes LibOSDP logging with sensible defaults
func InitializeLogging(name string, level LogLevel) {
	// Set up standard Go logger as default
	SetStandardLogger(log.Default(), fmt.Sprintf("[%s]", name))

	// Initialize LibOSDP logger with the specified level
	nameC := C.CString(name)
	defer C.free(unsafe.Pointer(nameC))

	C.osdp_logger_init(nameC, C.int(level), nil)
}

// logCallbackWrapper is the C callback wrapper that bridges LibOSDP logs to Go
//
//export logCallbackWrapper
func logCallbackWrapper(pd C.int, level C.int, msg *C.char, file *C.char, line C.ulong) {
	logMu.RLock()
	handler := globalLogHandler
	logMu.RUnlock()
	if handler == nil {
		return
	}

	handler(int(pd), LogLevel(level), C.GoString(file), int(line), C.GoString(msg))
}

// LogLevelFromString converts a string to a LogLevel
func LogLevelFromString(levelStr string) LogLevel {
	switch strings.ToUpper(strings.TrimSpace(levelStr)) {
	case "EMERG", "EMERGENCY":
		return LogEmergency
	case "ALERT":
		return LogAlert
	case "CRIT", "CRITICAL":
		return LogCritical
	case "ERROR", "ERR":
		return LogError
	case "WARN", "WARNING":
		return LogWarning
	case "NOTICE":
		return LogNotice
	case "INFO", "INFORMATION":
		return LogInfo
	case "DEBUG":
		return LogDebug
	default:
		return LogInfo // Default to INFO level
	}
}

// GetSupportedLogLevels returns all supported log level strings
func GetSupportedLogLevels() []string {
	return []string{
		"EMERGENCY", "ALERT", "CRITICAL", "ERROR",
		"WARNING", "NOTICE", "INFO", "DEBUG",
	}
}

// Example custom loggers for popular Go logging libraries

// SetZerologLogger configures LibOSDP to use github.com/rs/zerolog
// Note: This requires the zerolog package to be imported
func SetZerologLogger(logger interface{}, component string) {
	// Type assertion would be done here for zerolog.Logger
	// This is a template - actual implementation would depend on zerolog being imported
	SetLogHandler(func(pd int, level LogLevel, file string, line int, message string) {
		// Convert to zerolog format
		// logger.Info().Str("component", component).Str("file", file).Int("line", line).Msg(message)
		// This would be implemented when zerolog is available
		log.Printf("[%s] %s:%d %s", level.String(), filepath.Base(file), line, message)
	})
}

// SetLogrusLogger configures LibOSDP to use github.com/sirupsen/logrus
// Note: This requires the logrus package to be imported
func SetLogrusLogger(logger interface{}, component string) {
	// Type assertion would be done here for logrus.Logger
	// This is a template - actual implementation would depend on logrus being imported
	SetLogHandler(func(pd int, level LogLevel, file string, line int, message string) {
		// Convert to logrus format
		// logger.WithFields(logrus.Fields{
		//     "component": component,
		//     "file": file,
		//     "line": line,
		// }).Info(message)
		// This would be implemented when logrus is available
		log.Printf("[%s] %s:%d %s", level.String(), filepath.Base(file), line, message)
	})
}
