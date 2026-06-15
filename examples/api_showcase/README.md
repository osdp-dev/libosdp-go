# OSDP API Showcase

A comprehensive demonstration of all OSDP Go API features including commands, events, and management operations.

## Features Demonstrated

- All OSDP command types (OUTPUT, LED, BUZZER, TEXT, KEYSET, COMSET, MFG, STATUS, FILE_TX)
- All event types (CARDREAD, KEYPRESS, MFGREP, STATUS, NOTIFICATION)
- Advanced CP features (PD management, capability queries, flag modification)
- Secure channel setup
- Error handling and monitoring

## Running the Showcase

You need **TWO terminals** to run the showcase - one for the CP (api_showcase) and one for the PD (companion_pd).

### Option 1: Using the Companion PD (Recommended)

**Terminal 1 - Start the companion PD:**
```bash
cd examples/go/api_showcase
mkfifo /tmp/showcase_rx.fifo /tmp/showcase_tx.fifo
./companion_pd/companion_pd /tmp/showcase_rx.fifo /tmp/showcase_tx.fifo
```

**Terminal 2 - Start the API showcase:**
```bash
cd examples/go/api_showcase
./api_showcase /tmp/showcase_rx.fifo /tmp/showcase_tx.fifo
```

The companion PD automatically:
- Swaps TX/RX FIFOs to connect properly to the CP
- Responds to all commands from api_showcase
- Displays received commands in real-time
- Sends simulated card read events every 15 seconds

### Option 2: Using pd_sample

**Terminal 1 - Start pd_sample (note swapped FIFOs):**
```bash
mkfifo /tmp/showcase_rx.fifo /tmp/showcase_tx.fifo
./examples/go/pd_sample/pd_sample /tmp/showcase_tx.fifo /tmp/showcase_rx.fifo
```

**Terminal 2 - Start api_showcase:**
```bash
./examples/go/api_showcase/api_showcase /tmp/showcase_rx.fifo /tmp/showcase_tx.fifo
```

## Building

```bash
cd examples/go/api_showcase
go build .              # Builds api_showcase
go build ./companion_pd # Builds companion_pd
```

## What You'll See

The showcase runs through 15 demonstration steps:

1. **PD Status Check** - Online status, secure channel, PD ID
2. **LED Commands** - All 7 LED colors (Red, Green, Blue, Amber, Magenta, Cyan, White)
3. **Buzzer Commands** - Different beep patterns
4. **Text Display** - Send text to PD display
5. **Output Control** - Toggle outputs with timers
6. **Status Requests** - Query PD status
7. **Manufacturer Commands** - Custom vendor commands
8. **Key Set** - Update secure channel keys
9. **Communication Set** - Change baud rate/address
10. **File Transfer** - Initiate file transfers
11. **Advanced Management** - Flush commands, modify flags
12. **Capability Queries** - Check PD capabilities
13. **Flag Modification** - Enable/disable features
14. **PD Enable/Disable** - Temporarily disable PD
15. **Cleanup** - Final cleanup operations

## Signal Handling

Press **Ctrl+C** at any time to stop. The showcase will:
- Catch the signal immediately
- Stop all operations cleanly
- Exit within 2 seconds

## Notes

- The showcase runs for ~60 seconds (15 steps × 4 seconds each)
- Commands are sent sequentially with 4-second intervals
- FIFOs use non-blocking I/O for responsive signal handling
- Secure channel is established automatically
