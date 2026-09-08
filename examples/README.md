# Go OSDP Examples

This directory contains Go examples demonstrating the LibOSDP API. Each example is structured as an independent Go module to avoid "main redeclared" issues in VS Code and other IDEs.

## Examples

Each subdirectory contains a complete, runnable example:

| Example | Description |
|---------|-------------|
| **api_showcase** | Comprehensive demonstration of all OSDP commands and events |
| **cp_sample** | Basic Control Panel example with LED and buzzer commands |
| **file_transfer** | File transfer functionality between CP and PD |
| **logging** | Different logging configurations and formats |
| **multi_pd_cp** | Control Panel managing multiple Peripheral Devices |
| **multi_pd_device** | Multiple PD simulation with different capabilities |
| **pd_sample** | Basic Peripheral Device example with event generation |
| **secure_channel** | Provision a PD's SCBK in install mode, then enforce the secure channel (self-contained, no hardware) |

## Building and Running

Each example can be built and run independently:

```bash
# Build a specific example
cd cp_sample
go build

# Run the example
./cp_sample /tmp/pd_tx.fifo /tmp/pd_rx.fifo
```

## VS Code Integration

Each example is its own `main` package (in its own directory) within the
repository module, so each builds independently without "main redeclared" errors.

## Prerequisites

Examples require:
1. FIFO files for communication (see individual example usage)
2. Go 1.21+ with cgo enabled and a C compiler (the LibOSDP C sources are
   vendored into the `go/osdp` package, so no prebuilt library is needed)

## API Structure

All examples use the modern typed interface:

```go
// Event handling
func eventHandler(pdNumber int, event *osdp.Event) int {
    switch eventData := event.Event.(type) {
    case osdp.EventCardRead:
        fmt.Printf("Card: %X", eventData.Data)
    }
    return 0
}

// Command creation
ledCmd := &osdp.Command{
    Command: osdp.CommandLed{
        LedNumber: 0,
        Permanent: osdp.LedParams{
            ControlCode: osdp.LEDPermanentCCSet,
            OnColor:     osdp.LEDColorGreen,
        },
    },
}
```

This replaces the old `map[string]interface{}` pattern with compile-time type safety.

## Running Examples

### Using FIFOs for testing

```bash
# Create FIFOs
mkfifo /tmp/pd_rx.fifo /tmp/pd_tx.fifo

# Run PD example in one terminal
cd pd_sample
go run . /tmp/pd_tx.fifo /tmp/pd_rx.fifo

# Run CP example in another terminal
cd cp_sample
go run . /tmp/pd_rx.fifo /tmp/pd_tx.fifo
```

### Usage with Real Hardware

Replace FIFO channel creation with:

```go
channel, err := osdp.NewSerialChannel("/dev/ttyUSB0", 9600)
```