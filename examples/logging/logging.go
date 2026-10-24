/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/osdp-dev/libosdp-go"
)

// Example 1: Using standard Go log package
func useStandardLogger() {
	fmt.Println("=== Example 1: Standard Go Logger ===")

	// Create a custom logger with prefix
	logger := log.New(os.Stdout, "[OSDP] ", log.LstdFlags|log.Lshortfile)

	// Configure LibOSDP to use this logger
	osdp.SetStandardLogger(logger, "LibOSDP")

	// Now any LibOSDP operations will log through this logger
	fmt.Printf("LibOSDP Version: %s\n", osdp.GetVersion())
}

// Example 2: Using structured logging with slog
func useStructuredLogger() {
	fmt.Println("\n=== Example 2: Structured Logging (slog) ===")

	// Create a structured logger with JSON format
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(jsonHandler)

	// Configure LibOSDP to use structured logging
	osdp.SetSlogLogger(logger, "libosdp")

	fmt.Printf("LibOSDP Version: %s\n", osdp.GetVersion())
}

// Example 3: Custom log handler
func useCustomLogger() {
	fmt.Println("\n=== Example 3: Custom Log Handler ===")

	// Define a custom log handler
	customHandler := func(pd int, level osdp.LogLevel, file string, line int, message string) {
		timestamp := time.Now().Format("2006-01-02 15:04:05.000")
		fmt.Printf("[%s] PD%d %s %s:%d - %s\n",
			timestamp,
			pd,
			level.String(),
			file,
			line,
			message)
	}

	// Set the custom handler
	osdp.SetLogHandler(customHandler)

	fmt.Printf("LibOSDP Version: %s\n", osdp.GetVersion())
}

// Example 4: Level-filtered logging
func useLevelFilteredLogger() {
	fmt.Println("\n=== Example 4: Level-Filtered Logging ===")

	// Create a handler that only logs WARNING and above
	baseHandler := func(pd int, level osdp.LogLevel, file string, line int, message string) {
		fmt.Printf("[%s] PD%d %s - %s\n", level.String(), pd, file, message)
	}

	// Apply level filtering
	osdp.SetLevelFilteredLogger(osdp.LogWarning, baseHandler)

	fmt.Println("Only WARNING, ERROR, CRITICAL, ALERT, and EMERGENCY messages will be shown")
	fmt.Printf("LibOSDP Version: %s\n", osdp.GetVersion())
}

// Example 5: Integration with a Control Panel
func useWithControlPanel() {
	fmt.Println("\n=== Example 5: Logging with Control Panel ===")

	// Set up logging first
	logger := log.New(os.Stdout, "[CP-DEMO] ", log.LstdFlags)
	osdp.SetStandardLogger(logger, "OSDP")

	// Create FIFO channels for testing
	channel, err := osdp.NewFIFOChannel("/tmp/demo_rx.fifo", "/tmp/demo_tx.fifo")
	if err != nil {
		log.Printf("Note: Could not create FIFO channels (this is normal in demo): %v", err)
		return
	}
	defer channel.Close()

	// Create PD info
	pdInfo := &osdp.PdInfo{
		Address:  1,
		BaudRate: 9600,
		Flags:    osdp.FlagEnforceSecure,
		SCBK:     []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F},
		ID: osdp.PdId{
			Version:       1,
			Model:         1,
			VendorCode:    0x153,
			SerialNumber:  0x12345,
			FirmwareMajor: 1,
			FirmwareMinor: 0,
			FirmwareBuild: 0,
		},
	}

	// Event handler that also logs
	eventHandler := func(pdNumber int, event *osdp.Event) int {
		logger.Printf("Received event from PD %d: Type=%d", pdNumber, event.Event.GetType())
		return 0
	}

	// Create Control Panel - this will generate LibOSDP logs
	cp, err := osdp.NewControlPanelBuilder(channel).AddPD(pdInfo).LogLevel(osdp.LogDebug).Build()
	if err != nil {
		logger.Printf("Failed to create Control Panel: %v", err)
		return
	}
	defer cp.Stop()

	if err := cp.SetEventHandler(eventHandler); err != nil {
		logger.Printf("Failed to set event handler: %v", err)
	}

	logger.Println("Control Panel created - LibOSDP logs will appear above")
	time.Sleep(1 * time.Second)
}

// Example 6: Log level configuration from string
func useLogLevelFromString() {
	fmt.Println("\n=== Example 6: Configure Log Level from String ===")

	// Parse log level from string (could come from config file, environment variable, etc.)
	levelStr := "DEBUG"
	level := osdp.LogLevelFromString(levelStr)

	fmt.Printf("Parsed log level '%s' as: %s (%d)\n", levelStr, level.String(), int(level))
	fmt.Printf("Supported levels: %v\n", osdp.GetSupportedLogLevels())

	// Use the parsed level
	osdp.InitializeLogging("demo", level)
}

// Example 7: Disable logging completely
func useDiscardLogger() {
	fmt.Println("\n=== Example 7: Disable Logging ===")

	// Disable all LibOSDP logging
	osdp.SetDiscardLogger()

	fmt.Println("All LibOSDP logs are now discarded")
	fmt.Printf("LibOSDP Version: %s\n", osdp.GetVersion())
}

func main() {
	fmt.Println("LibOSDP Go Logging Examples")
	fmt.Println("============================")

	// Run all examples
	useStandardLogger()
	useStructuredLogger()
	useCustomLogger()
	useLevelFilteredLogger()
	useLogLevelFromString()
	useDiscardLogger()

	// This one might fail if FIFOs don't exist, but will show the concept
	useWithControlPanel()

	fmt.Println("\nDone! Check the different logging formats above.")
}
