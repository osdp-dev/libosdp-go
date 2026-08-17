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
	"fmt"
	"unsafe"
)

// checkLen returns an error when a variable-length payload exceeds the fixed C
// buffer it will be copied into, so an over-length submission is rejected rather
// than overflowing the destination array.
func checkLen(name string, n, max int) error {
	if n > max {
		return fmt.Errorf("osdp: %s length %d exceeds max %d", name, n, max)
	}
	return nil
}

// freeCmd releases a C osdp_cmd allocated by commandToC.
func freeCmd(c *C.struct_osdp_cmd) { C.free(unsafe.Pointer(c)) }

// Command represents an OSDP command. Flags may carry command flags such as
// CmdFlagBroadcast.
type Command struct {
	Command CommandData
	Flags   uint32
}

// CommandData represents the specific command data
type CommandData interface {
	GetType() OsdpCommandType
}

// LedParams represents LED control parameters for temporary/permanent modes
type LedParams struct {
	ControlCode uint8
	OnCount     uint8
	OffCount    uint8
	OnColor     OsdpLedColor
	OffColor    OsdpLedColor
	TimerCount  uint16
}

// CommandLed represents LED command
type CommandLed struct {
	Reader    uint8
	LedNumber uint8
	Temporary LedParams
	Permanent LedParams
}

func (c CommandLed) GetType() OsdpCommandType { return CmdLED }

// CommandBuzzer represents buzzer command
type CommandBuzzer struct {
	Reader      uint8
	ControlCode uint8
	OnCount     uint8
	OffCount    uint8
	RepCount    uint8
}

func (c CommandBuzzer) GetType() OsdpCommandType { return CmdBuzzer }

// CommandOutput represents output command
type CommandOutput struct {
	OutputNo    uint8
	ControlCode uint8
	TimerCount  uint16
}

func (c CommandOutput) GetType() OsdpCommandType { return CmdOutput }

// CommandText represents text command
type CommandText struct {
	Reader      uint8
	ControlCode uint8
	TempTime    uint8
	OffsetRow   uint8
	OffsetCol   uint8
	Data        []byte
}

func (c CommandText) GetType() OsdpCommandType { return CmdText }

// CommandKeyset represents keyset command
type CommandKeyset struct {
	KeyType uint8
	Data    []byte
}

func (c CommandKeyset) GetType() OsdpCommandType { return CmdKeyset }

// CommandComset represents communication set command
type CommandComset struct {
	Address  uint8
	BaudRate uint32
}

func (c CommandComset) GetType() OsdpCommandType { return CmdComset }

// CommandFileTx represents file transfer command
type CommandFileTx struct {
	ID    int32
	Flags uint32
}

func (c CommandFileTx) GetType() OsdpCommandType { return CmdFileTx }

// CommandMfg represents manufacturer specific command
type CommandMfg struct {
	VendorCode [3]uint8
	Data       []byte
}

func (c CommandMfg) GetType() OsdpCommandType { return CmdMfg }

// CommandStatus represents a status report request (CP->PD). Type selects which
// report the PD should return (local, input, output, remote).
type CommandStatus struct {
	Type OsdpStatusReportType
}

func (c CommandStatus) GetType() OsdpCommandType { return CmdStatus }

// commandToC converts a Go Command to a newly malloc'd C osdp_cmd structure.
// The caller owns the returned memory. It returns an error (and frees the
// allocation) when a variable-length payload exceeds its C buffer.
func commandToC(cmd *Command) (*C.struct_osdp_cmd, error) {
	cCmd := (*C.struct_osdp_cmd)(C.malloc(C.size_t(unsafe.Sizeof(C.struct_osdp_cmd{}))))
	if cCmd == nil {
		return nil, fmt.Errorf("osdp: failed to allocate command")
	}

	// Initialize the structure to zero
	C.memset(unsafe.Pointer(cCmd), 0, C.size_t(unsafe.Sizeof(C.struct_osdp_cmd{})))

	cCmd.id = C.enum_osdp_cmd_e(cmd.Command.GetType())
	cCmd.flags = C.uint32_t(cmd.Flags)

	switch cmdData := cmd.Command.(type) {
	case CommandOutput:
		C.set_cmd_output(cCmd, C.uint8_t(cmdData.OutputNo), C.uint8_t(cmdData.ControlCode), C.uint16_t(cmdData.TimerCount))
	case CommandLed:
		C.set_cmd_led(cCmd, C.uint8_t(cmdData.Reader), C.uint8_t(cmdData.LedNumber),
			C.uint8_t(cmdData.Temporary.ControlCode), C.uint8_t(cmdData.Temporary.OnCount), C.uint8_t(cmdData.Temporary.OffCount),
			C.uint8_t(cmdData.Temporary.OnColor), C.uint8_t(cmdData.Temporary.OffColor), C.uint16_t(cmdData.Temporary.TimerCount),
			C.uint8_t(cmdData.Permanent.ControlCode), C.uint8_t(cmdData.Permanent.OnCount), C.uint8_t(cmdData.Permanent.OffCount),
			C.uint8_t(cmdData.Permanent.OnColor), C.uint8_t(cmdData.Permanent.OffColor), C.uint16_t(cmdData.Permanent.TimerCount))
	case CommandBuzzer:
		C.set_cmd_buzzer(cCmd, C.uint8_t(cmdData.Reader), C.uint8_t(cmdData.ControlCode), C.uint8_t(cmdData.OnCount), C.uint8_t(cmdData.OffCount), C.uint8_t(cmdData.RepCount))
	case CommandText:
		if err := checkLen("text", len(cmdData.Data), C.OSDP_CMD_TEXT_MAX_LEN); err != nil {
			C.free(unsafe.Pointer(cCmd))
			return nil, err
		}
		textData := (*C.char)(C.CBytes(cmdData.Data))
		defer C.free(unsafe.Pointer(textData))
		C.set_cmd_text(cCmd, C.uint8_t(cmdData.Reader), C.uint8_t(cmdData.ControlCode), C.uint8_t(cmdData.TempTime), C.uint8_t(cmdData.OffsetRow), C.uint8_t(cmdData.OffsetCol), textData, C.uint8_t(len(cmdData.Data)))
	case CommandStatus:
		C.set_cmd_status(cCmd, C.uint8_t(cmdData.Type))
	case CommandMfg:
		if err := checkLen("mfg", len(cmdData.Data), C.OSDP_CMD_MFG_MAX_DATALEN); err != nil {
			C.free(unsafe.Pointer(cCmd))
			return nil, err
		}
		vendorCode := uint32(cmdData.VendorCode[0]) | uint32(cmdData.VendorCode[1])<<8 | uint32(cmdData.VendorCode[2])<<16
		var dataPtr *C.uint8_t
		if len(cmdData.Data) > 0 {
			dataPtr = (*C.uint8_t)(unsafe.Pointer(&cmdData.Data[0]))
		}
		C.set_cmd_mfg(cCmd, C.uint32_t(vendorCode), dataPtr, C.uint8_t(len(cmdData.Data)))
	case CommandKeyset:
		if err := checkLen("keyset", len(cmdData.Data), C.OSDP_CMD_KEYSET_KEY_MAX_LEN); err != nil {
			C.free(unsafe.Pointer(cCmd))
			return nil, err
		}
		keysetData := make([]C.uint8_t, len(cmdData.Data))
		for i, b := range cmdData.Data {
			keysetData[i] = C.uint8_t(b)
		}
		var ksPtr *C.uint8_t
		if len(keysetData) > 0 {
			ksPtr = &keysetData[0]
		}
		C.set_cmd_keyset(cCmd, C.uint8_t(cmdData.KeyType), C.uint8_t(len(cmdData.Data)), ksPtr)
	case CommandComset:
		C.set_cmd_comset(cCmd, C.uint8_t(cmdData.Address), C.uint32_t(cmdData.BaudRate))
	case CommandFileTx:
		C.set_cmd_file_tx(cCmd, C.int32_t(cmdData.ID), C.uint32_t(cmdData.Flags))
	}

	return cCmd, nil
}

// decodeCommand converts a C osdp_cmd to a Go Command. It reads only cCmd, so it
// is shared by the PD command callback and the CP command completion path.
func decodeCommand(cCmd *C.struct_osdp_cmd) *Command {
	var commandData CommandData

	switch OsdpCommandType(cCmd.id) {
	case CmdOutput:
		var outputNo, controlCode C.uint8_t
		var timerCount C.uint16_t
		C.get_cmd_output(cCmd, &outputNo, &controlCode, &timerCount)
		commandData = CommandOutput{
			OutputNo:    uint8(outputNo),
			ControlCode: uint8(controlCode),
			TimerCount:  uint16(timerCount),
		}
	case CmdLED:
		var reader, ledNumber, tempControl, tempOnCount, tempOffCount, tempOnColor, tempOffColor C.uint8_t
		var permControl, permOnCount, permOffCount, permOnColor, permOffColor C.uint8_t
		var tempTimer, permTimer C.uint16_t
		C.get_cmd_led(cCmd, &reader, &ledNumber, &tempControl, &tempOnCount, &tempOffCount, &tempOnColor, &tempOffColor, &tempTimer,
			&permControl, &permOnCount, &permOffCount, &permOnColor, &permOffColor, &permTimer)
		commandData = CommandLed{
			Reader:    uint8(reader),
			LedNumber: uint8(ledNumber),
			Temporary: LedParams{
				ControlCode: uint8(tempControl),
				OnCount:     uint8(tempOnCount),
				OffCount:    uint8(tempOffCount),
				OnColor:     OsdpLedColor(tempOnColor),
				OffColor:    OsdpLedColor(tempOffColor),
				TimerCount:  uint16(tempTimer),
			},
			Permanent: LedParams{
				ControlCode: uint8(permControl),
				OnCount:     uint8(permOnCount),
				OffCount:    uint8(permOffCount),
				OnColor:     OsdpLedColor(permOnColor),
				OffColor:    OsdpLedColor(permOffColor),
				TimerCount:  uint16(permTimer),
			},
		}
	case CmdBuzzer:
		var reader, controlCode, onCount, offCount, repCount C.uint8_t
		C.get_cmd_buzzer(cCmd, &reader, &controlCode, &onCount, &offCount, &repCount)
		commandData = CommandBuzzer{
			Reader:      uint8(reader),
			ControlCode: uint8(controlCode),
			OnCount:     uint8(onCount),
			OffCount:    uint8(offCount),
			RepCount:    uint8(repCount),
		}
	case CmdText:
		var reader, controlCode, tempTime, offsetRow, offsetCol, length C.uint8_t
		textData := make([]C.char, C.OSDP_CMD_TEXT_MAX_LEN)
		C.get_cmd_text(cCmd, &reader, &controlCode, &tempTime, &offsetRow, &offsetCol, &length, &textData[0])
		commandData = CommandText{
			Reader:      uint8(reader),
			ControlCode: uint8(controlCode),
			TempTime:    uint8(tempTime),
			OffsetRow:   uint8(offsetRow),
			OffsetCol:   uint8(offsetCol),
			// Copy exactly length bytes so embedded NULs are preserved.
			Data: C.GoBytes(unsafe.Pointer(&textData[0]), C.int(length)),
		}
	case CmdStatus:
		// A STATUS command carries the requested report type; the PD responds
		// with a matching status report event.
		var statusType C.uint8_t
		C.get_cmd_status(cCmd, &statusType)
		commandData = CommandStatus{
			Type: OsdpStatusReportType(statusType),
		}
	case CmdMfg:
		var vendorCode C.uint32_t
		var length C.uint8_t
		mfgData := make([]C.uint8_t, C.OSDP_CMD_MFG_MAX_DATALEN)
		C.get_cmd_mfg(cCmd, &vendorCode, &length, &mfgData[0])

		// Convert vendor code to 3-byte array
		var vendorArray [3]uint8
		vendorArray[0] = uint8(vendorCode)
		vendorArray[1] = uint8(vendorCode >> 8)
		vendorArray[2] = uint8(vendorCode >> 16)

		commandData = CommandMfg{
			VendorCode: vendorArray,
			Data:       C.GoBytes(unsafe.Pointer(&mfgData[0]), C.int(length)),
		}
	case CmdKeyset:
		var keysetType, length C.uint8_t
		keysetData := make([]C.uint8_t, C.OSDP_CMD_KEYSET_KEY_MAX_LEN)
		C.get_cmd_keyset(cCmd, &keysetType, &length, &keysetData[0])
		commandData = CommandKeyset{
			KeyType: uint8(keysetType),
			Data:    C.GoBytes(unsafe.Pointer(&keysetData[0]), C.int(length)),
		}
	case CmdComset:
		var address C.uint8_t
		var baudRate C.uint32_t
		C.get_cmd_comset(cCmd, &address, &baudRate)
		commandData = CommandComset{
			Address:  uint8(address),
			BaudRate: uint32(baudRate),
		}
	case CmdFileTx:
		var id C.int
		var flags C.uint32_t
		C.get_cmd_file_tx(cCmd, &id, &flags)
		commandData = CommandFileTx{
			ID:    int32(id),
			Flags: uint32(flags),
		}
	default:
		// For unknown commands, return a basic status command
		commandData = CommandStatus{
			Type: StatusReportInput,
		}
	}

	return &Command{
		Command: commandData,
		Flags:   uint32(cCmd.flags),
	}
}
