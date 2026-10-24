# LibOSDP — Go Bindings

[![CI](https://github.com/osdp-dev/libosdp-go/actions/workflows/ci.yml/badge.svg)](https://github.com/osdp-dev/libosdp-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/osdp-dev/libosdp-go.svg)](https://pkg.go.dev/github.com/osdp-dev/libosdp-go)
[![Docs](https://img.shields.io/badge/docs-doc.osdp.dev-blue.svg)](https://doc.osdp.dev)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

Go bindings for [LibOSDP](https://github.com/osdp-dev/libosdp), an implementation
of the IEC 60839-11-5 [Open Supervised Device Protocol](https://osdp.dev) used in
physical access-control systems. This is a child project of
[osdp-dev/libosdp](https://github.com/osdp-dev/libosdp) (tracked here as a git
submodule), alongside the Rust bindings in
[osdp-dev/libosdp-rs](https://github.com/osdp-dev/libosdp-rs).

## Installation

```bash
go get github.com/osdp-dev/libosdp-go
```

That's it. The LibOSDP C sources are vendored into the module and compiled by cgo
as part of `go build`, so there is no prebuilt library, CMake step, submodule
checkout, or external link dependency — the bundled TinyAES crypto backend is
self-contained.

Requirements: Go 1.21+ and a C compiler (gcc/clang).

## Quick Start

### Control Panel

```go
package main

import (
	"log"
	"time"

	"github.com/osdp-dev/libosdp-go"
)

func main() {
	channel, err := osdp.NewSerialChannel("/dev/ttyUSB0", 9600)
	if err != nil {
		log.Fatal(err)
	}
	defer channel.Close()

	pdInfo := &osdp.PdInfo{
		Address:  1,
		BaudRate: 9600,
		Flags:    osdp.FlagEnforceSecure,
		SCBK:     []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
		ID:       osdp.PdId{Version: 1, Model: 1, VendorCode: 0x153, SerialNumber: 0x12345},
	}

	cp, err := osdp.NewControlPanelBuilder(channel).AddPD(pdInfo).Build()
	if err != nil {
		log.Fatal(err)
	}
	defer cp.Stop()

	cp.SetEventHandler(func(pdNumber int, event *osdp.Event) int {
		log.Printf("event from PD %d: type=%d", pdNumber, event.Event.GetType())
		return 0
	})

	if !cp.OnlineWait(0, 10*time.Second) {
		log.Fatal("PD did not come online")
	}

	led := &osdp.Command{Command: osdp.CommandLed{
		Reader: 0, LedNumber: 0,
		Permanent: osdp.LedParams{ControlCode: osdp.LEDPermanentCCSet, OnCount: 10, OffCount: 0, OnColor: osdp.LEDColorGreen},
	}}
	if err := cp.SubmitCommand(0, led); err != nil {
		log.Print(err)
	}
	time.Sleep(2 * time.Second)
}
```

### Peripheral Device

```go
pdInfo.Capabilities = []osdp.PdCapability{{Function: osdp.CapReaderLEDControl, Compliance: 1, NumItems: 2}}
pd, err := osdp.NewPeripheralDevice(channel, pdInfo, osdp.LogInfo)
if err != nil {
	log.Fatal(err)
}
defer pd.Stop()

pd.SetCommandHandler(func(cmd *osdp.Command) (int, *osdp.Event) {
	log.Printf("got command type=%d", cmd.Command.GetType())
	return 0, nil
})
```

Runnable end-to-end programs (CP, PD, file transfer, multi-drop CP, custom
logging) live under [`examples/`](examples/).

## Logging

```go
osdp.SetLogHandler(func(pd int, level osdp.LogLevel, file string, line int, msg string) {
	log.Printf("[osdp pd=%d %s] %s:%d %s", pd, level, file, line, msg)
})
```

Built-in helpers: `SetStandardLogger`, `SetSlogLogger`, `SetLevelFilteredLogger`,
`SetDiscardLogger`.

## Updating the vendored C sources (maintainers)

The C sources live in the `libosdp/` git submodule and are vendored into the
`internal/clib` package by `go generate` (cgo compiles them there, keeping the
binding package free of C). To track a new LibOSDP release:

```bash
git submodule update --init --recursive   # first time only
git -C libosdp checkout <new-libosdp-ref>  # bump to the desired release
go generate .                              # refresh internal/clib (*.c, headers, osdp_config.h)
go test -race ./...                        # verify
git add -A && git commit                   # commit the bump + regenerated sources
```

CI fails if the committed vendored copy drifts from the submodule.

## License

Apache 2.0 — see [LICENSE](LICENSE).

## Support

- Project home: <https://osdp.dev>
- Documentation: <https://doc.osdp.dev>
- Issues: <https://github.com/osdp-dev/libosdp-go/issues>
- Discussions: <https://github.com/osdp-dev/libosdp/discussions>
