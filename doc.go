/*
Package osdp provides Go bindings for the LibOSDP C library.

LibOSDP is a library implementation of IEC 60839-11-5 Open Supervised Device
Protocol (OSDP), a communication protocol for secure exchange between Control
Panels (CP) and Peripheral Devices (PD) in access control systems. See
https://osdp.dev for an overview of the protocol and the LibOSDP project.

The package wraps the C API with idiomatic Go: interfaces for channels,
goroutine-managed refresh loops (20ms per the OSDP standard), and Go errors
in place of integer status codes.

# Getting Started

Two main types: ControlPanel (one bus, one or more PDs) and PeripheralDevice
(one PD on a bus). Both take a Channel as their first constructor argument.

	channel, err := osdp.NewSerialChannel("/dev/ttyUSB0", 9600)
	if err != nil {
		return err
	}
	defer channel.Close()

	pdInfo := &osdp.PdInfo{
		Address:  1,
		BaudRate: 9600,
		Flags:    osdp.FlagEnforceSecure,
		SCBK:     scbk16Bytes,
	}

	cp, err := osdp.NewControlPanel(channel, []*osdp.PdInfo{pdInfo}, osdp.LogInfo)
	if err != nil {
		return err
	}
	defer cp.Stop()

For a runnable PD example, a complete CP↔PD walkthrough, and patterns for
multi-drop buses, see the examples/ tree in the repository.

# Channels

Implement the Channel interface to wrap any transport. NewSerialChannel and
NewFIFOChannel are provided out of the box (FIFO for tests).

# Logging

LibOSDP routes log messages through a single global callback:

	osdp.SetLogHandler(func(pd int, level osdp.LogLevel, file string, line int, msg string) {
		log.Printf("[osdp pd=%d %s] %s:%d %s", pd, level, file, line, msg)
	})

Convenience helpers are provided for the standard log package
(SetStandardLogger), slog (SetSlogLogger), level filtering
(SetLevelFilteredLogger), and silencing (SetDiscardLogger).

# Thread Safety

Both ControlPanel and PeripheralDevice serialize their own state with an
internal mutex. The bindings spawn one goroutine per instance to drive the
refresh loop. User-facing methods are safe to call concurrently.

# Memory Management

C resources are released by Stop() (explicit) and by a runtime.SetFinalizer
backstop. Always call Stop() in production code; finalizers are best-effort.

# Compatibility

Requires Go 1.21+ and a C compiler. The LibOSDP C sources are vendored into the
internal/clib package and compiled by cgo, so no prebuilt library or external
link dependency is needed. Maintainers refresh the vendored copy from the
libosdp/ submodule with "go generate .".

# See Also

  - OSDP protocol and the LibOSDP project: https://osdp.dev
  - Documentation: https://doc.osdp.dev
  - LibOSDP C library: https://github.com/osdp-dev/libosdp
*/
package osdp
