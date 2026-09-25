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
	"time"
	"unsafe"
)

// Transparent Reader Support (TRS) lets a CP talk to a smart card through the
// reader: it opens a card session with TRSCmdStart, exchanges APDUs, and closes
// it with TRSCmdStop, all submitted in order as CommandTRS. Inside a session
// only TRS commands are accepted, and card commands are refused outside one.
// The card's answers arrive as EventTRS; the session's life cycle as
// NotificationTRSStatus (with FlagEnableNotification). A PD acting as a
// transparent reader advertises CapSmartCardSupport and answers each card
// command it receives by returning an EventTRS from its CommandHandler.

// TRSCommand selects the variant of a CommandTRS.
type TRSCommand int

const (
	TRSCmdStart    TRSCommand = C.OSDP_TRS_CMD_START     // open a card session
	TRSCmdStop     TRSCommand = C.OSDP_TRS_CMD_STOP      // close the session opened by TRSCmdStart
	TRSCmdSendAPDU TRSCommand = C.OSDP_TRS_CMD_SEND_APDU // send CommandTRS.APDU to the card
	TRSCmdEnterPIN TRSCommand = C.OSDP_TRS_CMD_ENTER_PIN // secure PIN entry (CommandTRS.PINEntry)
	TRSCmdCardScan TRSCommand = C.OSDP_TRS_CMD_CARD_SCAN // ask whether a card is in the field
)

// Buffer limits of the TRS structures. What fits on the wire is usually less
// than TRSAPDUMaxLen; see ControlPanel.TRSMaxAPDULen.
const (
	TRSAPDUMaxLen         = C.OSDP_TRS_APDU_MAX_LEN
	TRSCSNMaxLen          = C.OSDP_TRS_CSN_MAX_LEN
	TRSProtocolDataMaxLen = C.OSDP_TRS_PROTOCOL_DATA_MAX_LEN
)

// TRSPINFormat is how PIN digits are encoded into the PIN block.
type TRSPINFormat int

const (
	TRSPINFormatBinary TRSPINFormat = C.OSDP_TRS_PIN_FORMAT_BINARY
	TRSPINFormatBCD    TRSPINFormat = C.OSDP_TRS_PIN_FORMAT_BCD
	TRSPINFormatASCII  TRSPINFormat = C.OSDP_TRS_PIN_FORMAT_ASCII
)

// TRSPINCompleteOn is a bitmask of the conditions that end PIN entry.
type TRSPINCompleteOn uint32

const (
	TRSPINCompleteOnMaxDigits TRSPINCompleteOn = C.OSDP_TRS_PIN_COMPLETE_ON_MAX_DIGITS
	TRSPINCompleteOnKey       TRSPINCompleteOn = C.OSDP_TRS_PIN_COMPLETE_ON_KEY
	TRSPINCompleteOnTimeout   TRSPINCompleteOn = C.OSDP_TRS_PIN_COMPLETE_ON_TIMEOUT
)

// TRSPINEntry parameterises TRSCmdEnterPIN: the reader collects a PIN on its
// keypad, places it into APDU as described, and sends that to the card.
type TRSPINEntry struct {
	TimeoutInitial uint8 // first-digit timeout in seconds (0 = reader default)
	TimeoutDigit   uint8 // per-digit timeout in seconds

	Format          TRSPINFormat
	RightJustify    bool
	BlockOffsetBits uint16 // PIN block position in the APDU payload
	BlockSizeBytes  uint8

	LengthFieldSizeBits   uint8 // 0 if the APDU has no PIN-length field
	LengthFieldOffsetBits uint16

	MinDigits   uint8
	MaxDigits   uint8
	CompleteOn  TRSPINCompleteOn
	NumMessages uint8
	LanguageID  uint16
	MsgIndex    uint8
	TEOPrologue [3]uint8 // T=1 prologue
	APDU        []byte   // C-APDU the PIN is placed into
}

// CommandTRS is a transparent-mode (osdp_XWR) command. Command selects which of
// APDU or PINEntry is used.
type CommandTRS struct {
	Command  TRSCommand
	APDU     []byte
	PINEntry TRSPINEntry
}

func (c CommandTRS) GetType() OsdpCommandType { return CmdXWR }

// TRSReply identifies the variant of an EventTRS.
type TRSReply int

const (
	TRSReplyCardInfo    TRSReply = C.OSDP_TRS_REPLY_CARD_INFO
	TRSReplyCardPresent TRSReply = C.OSDP_TRS_REPLY_CARD_PRESENT
	TRSReplyCardData    TRSReply = C.OSDP_TRS_REPLY_CARD_DATA
	TRSReplyPINComplete TRSReply = C.OSDP_TRS_REPLY_PIN_COMPLETE
	TRSReplyError       TRSReply = C.OSDP_TRS_REPLY_ERROR
)

// EventTRS is a transparent-mode reply. Payload is the concrete TRSReplyData
// for Reply. When a PD submits one, Reply is derived from Payload.
type EventTRS struct {
	Reply   TRSReply
	Payload TRSReplyData
}

func (e EventTRS) GetType() OsdpEventType { return EvtTRS }

// TRSReplyData is the payload of an EventTRS.
type TRSReplyData interface{ trsReply() TRSReply }

// TRSCardProtocol is how a card talks to the reader.
type TRSCardProtocol int

const (
	TRSCardProtocolContact     TRSCardProtocol = C.OSDP_TRS_CARD_PROTOCOL_CONTACT     // ISO 7816
	TRSCardProtocolContactless TRSCardProtocol = C.OSDP_TRS_CARD_PROTOCOL_CONTACTLESS // ISO 14443
)

// TRSCardInfo reports a card that entered the field.
type TRSCardInfo struct {
	Reader       uint8
	Protocol     TRSCardProtocol
	CSN          []byte // card serial number, at most TRSCSNMaxLen
	ProtocolData []byte // at most TRSProtocolDataMaxLen
}

func (TRSCardInfo) trsReply() TRSReply { return TRSReplyCardInfo }

// TRSCardStatus is whether, and on which interface, a card is present.
type TRSCardStatus int

const (
	TRSCardStatusUnknown     TRSCardStatus = C.OSDP_TRS_CARD_STATUS_UNKNOWN
	TRSCardStatusNotPresent  TRSCardStatus = C.OSDP_TRS_CARD_NOT_PRESENT
	TRSCardStatusPresent     TRSCardStatus = C.OSDP_TRS_CARD_PRESENT // interface not specified
	TRSCardStatusContactless TRSCardStatus = C.OSDP_TRS_CARD_PRESENT_CONTACTLESS
	TRSCardStatusContact     TRSCardStatus = C.OSDP_TRS_CARD_PRESENT_CONTACT
)

// TRSCardPresent reports card presence on a reader.
type TRSCardPresent struct {
	Reader uint8
	Status TRSCardStatus
}

func (TRSCardPresent) trsReply() TRSReply { return TRSReplyCardPresent }

// TRSCardData carries the card's R-APDU. Status is the exchange result as the
// reader reports it.
type TRSCardData struct {
	Reader uint8
	Status uint8
	APDU   []byte
}

func (TRSCardData) trsReply() TRSReply { return TRSReplyCardData }

// TRSPINResult reports the end of a TRSCmdEnterPIN sequence.
type TRSPINResult struct {
	Reader uint8
	Status uint8
	Tries  uint8
}

func (TRSPINResult) trsReply() TRSReply { return TRSReplyPINComplete }

// TRSError is a transparent-mode error or NAK from the reader or card.
type TRSError struct{ Code uint8 }

func (TRSError) trsReply() TRSReply { return TRSReplyError }

// TRSSessionStatus is a card session (or presence scan) life-cycle event.
type TRSSessionStatus int

const (
	TRSSessionOpened TRSSessionStatus = C.OSDP_TRS_SESSION_OPENED // reader in transparent mode
	TRSSessionClosed TRSSessionStatus = C.OSDP_TRS_SESSION_CLOSED // STOP honoured, reader restored
	TRSSessionFailed TRSSessionStatus = C.OSDP_TRS_SESSION_FAILED // refused, or aborted by a link error
	TRSScanSuspended TRSSessionStatus = C.OSDP_TRS_SCAN_SUSPENDED // a scan probe was refused; backing off
	// TRSScanNoDeparture: the reader cannot report card removal, so the
	// presence scan will not notice a card leaving.
	TRSScanNoDeparture TRSSessionStatus = C.OSDP_TRS_SCAN_NO_DEPARTURE
)

// NotificationTRSStatus is the payload of a NotificationTypeTRSStatus
// notification.
type NotificationTRSStatus struct {
	Status TRSSessionStatus
}

func (NotificationTRSStatus) isNotification() {}

// TRSScanParams is the cadence of the background card presence scan. Zero
// fields select libosdp's defaults.
type TRSScanParams struct {
	Mode0Dwell time.Duration // time in the reader's default mode per cycle
	Mode1Dwell time.Duration // time in transparent mode watching for a card
	Hold       time.Duration // how long a sighting holds transparent mode
}

func durationMs16(d time.Duration) C.uint16_t {
	ms := d.Milliseconds()
	if ms > 0xFFFF {
		ms = 0xFFFF
	}
	return C.uint16_t(ms)
}

// TRSScanEnable starts a background card presence scan on a PD whose reader
// does not announce smart cards in its default mode. Sightings arrive as
// EventTRS; respond by submitting TRSCmdStart.
func (cp *ControlPanel) TRSScanEnable(pdNumber int, params TRSScanParams) error {
	return cp.trsCall(pdNumber, "enable TRS scan", func() C.int {
		p := C.struct_osdp_trs_scan_params{
			mode0_dwell_ms: durationMs16(params.Mode0Dwell),
			mode1_dwell_ms: durationMs16(params.Mode1Dwell),
			hold_ms:        durationMs16(params.Hold),
		}
		return C.osdp_cp_trs_scan_enable(unsafe.Pointer(cp.ctx), C.int(pdNumber), &p)
	})
}

// TRSScanDisable stops the presence scan started by TRSScanEnable.
func (cp *ControlPanel) TRSScanDisable(pdNumber int) error {
	return cp.trsCall(pdNumber, "disable TRS scan", func() C.int {
		return C.osdp_cp_trs_scan_disable(unsafe.Pointer(cp.ctx), C.int(pdNumber))
	})
}

// TRSMaxAPDULen returns the largest C-APDU that fits in one TRSCmdSendAPDU to
// this PD. It is final only once the PD has been online.
func (cp *ControlPanel) TRSMaxAPDULen(pdNumber int) (int, error) {
	var n C.int
	err := cp.trsCall(pdNumber, "get TRS APDU limit", func() C.int {
		n = C.osdp_cp_trs_get_max_apdu_len(unsafe.Pointer(cp.ctx), C.int(pdNumber))
		if n < 0 {
			return n
		}
		return 0
	})
	return int(n), err
}

func (cp *ControlPanel) trsCall(pdNumber int, what string, call func() C.int) error {
	if pdNumber < 0 || pdNumber >= len(cp.pdInfoList) {
		return fmt.Errorf("invalid PD number: %d", pdNumber)
	}
	cp.mu.Lock()
	defer cp.mu.Unlock()
	if cp.ctx == nil {
		return fmt.Errorf("control panel context is not initialized")
	}
	if call() != 0 {
		return fmt.Errorf("failed to %s on PD %d", what, pdNumber)
	}
	return nil
}

func setTRSAPDU(a *C.struct_osdp_trs_apdu, b []byte) error {
	if err := checkLen("trs apdu", len(b), C.OSDP_TRS_APDU_MAX_LEN); err != nil {
		return err
	}
	a.length = C.uint16_t(len(b))
	for i, v := range b {
		a.data[i] = C.uint8_t(v)
	}
	return nil
}

func trsAPDUBytes(a *C.struct_osdp_trs_apdu) []byte {
	return C.GoBytes(unsafe.Pointer(&a.data[0]), C.int(a.length))
}

func encodeTRSCommand(t *C.struct_osdp_trs_cmd, c CommandTRS) error {
	t.command = C.enum_osdp_trs_cmd_e(c.Command)
	switch c.Command {
	case TRSCmdSendAPDU:
		return setTRSAPDU(C.trs_cmd_apdu(t), c.APDU)
	case TRSCmdEnterPIN:
		p, e := C.trs_cmd_pin(t), c.PINEntry
		p.timeout_initial = C.uint8_t(e.TimeoutInitial)
		p.timeout_digit = C.uint8_t(e.TimeoutDigit)
		p.pin_block.format = C.enum_osdp_trs_pin_format_e(e.Format)
		p.pin_block.right_justify = C.bool(e.RightJustify)
		p.pin_block.offset_bits = C.uint16_t(e.BlockOffsetBits)
		p.pin_block.size_bytes = C.uint8_t(e.BlockSizeBytes)
		p.pin_length_field.size_bits = C.uint8_t(e.LengthFieldSizeBits)
		p.pin_length_field.offset_bits = C.uint16_t(e.LengthFieldOffsetBits)
		p.min_digits = C.uint8_t(e.MinDigits)
		p.max_digits = C.uint8_t(e.MaxDigits)
		p.complete_on = C.uint32_t(e.CompleteOn)
		p.num_messages = C.uint8_t(e.NumMessages)
		p.language_id = C.uint16_t(e.LanguageID)
		p.msg_index = C.uint8_t(e.MsgIndex)
		for i, v := range e.TEOPrologue {
			p.teo_prologue[i] = C.uint8_t(v)
		}
		return setTRSAPDU(&p.apdu, e.APDU)
	}
	return nil
}

func decodeTRSCommand(t *C.struct_osdp_trs_cmd) CommandTRS {
	c := CommandTRS{Command: TRSCommand(t.command)}
	switch c.Command {
	case TRSCmdSendAPDU:
		c.APDU = trsAPDUBytes(C.trs_cmd_apdu(t))
	case TRSCmdEnterPIN:
		p := C.trs_cmd_pin(t)
		c.PINEntry = TRSPINEntry{
			TimeoutInitial:        uint8(p.timeout_initial),
			TimeoutDigit:          uint8(p.timeout_digit),
			Format:                TRSPINFormat(p.pin_block.format),
			RightJustify:          bool(p.pin_block.right_justify),
			BlockOffsetBits:       uint16(p.pin_block.offset_bits),
			BlockSizeBytes:        uint8(p.pin_block.size_bytes),
			LengthFieldSizeBits:   uint8(p.pin_length_field.size_bits),
			LengthFieldOffsetBits: uint16(p.pin_length_field.offset_bits),
			MinDigits:             uint8(p.min_digits),
			MaxDigits:             uint8(p.max_digits),
			CompleteOn:            TRSPINCompleteOn(p.complete_on),
			NumMessages:           uint8(p.num_messages),
			LanguageID:            uint16(p.language_id),
			MsgIndex:              uint8(p.msg_index),
			TEOPrologue:           [3]uint8{uint8(p.teo_prologue[0]), uint8(p.teo_prologue[1]), uint8(p.teo_prologue[2])},
			APDU:                  trsAPDUBytes(&p.apdu),
		}
	}
	return c
}

func encodeTRSReply(r *C.struct_osdp_trs_reply, e EventTRS) error {
	if e.Payload == nil {
		return fmt.Errorf("osdp: EventTRS has no payload")
	}
	r.reply = C.enum_osdp_trs_reply_e(e.Payload.trsReply())
	switch d := e.Payload.(type) {
	case TRSCardInfo:
		if err := checkLen("trs csn", len(d.CSN), C.OSDP_TRS_CSN_MAX_LEN); err != nil {
			return err
		}
		if err := checkLen("trs protocol data", len(d.ProtocolData), C.OSDP_TRS_PROTOCOL_DATA_MAX_LEN); err != nil {
			return err
		}
		ci := C.trs_card_info(r)
		ci.reader = C.uint8_t(d.Reader)
		ci.protocol = C.enum_osdp_trs_card_protocol_e(d.Protocol)
		ci.csn_len = C.uint8_t(len(d.CSN))
		for i, v := range d.CSN {
			ci.csn[i] = C.uint8_t(v)
		}
		ci.protocol_data_len = C.uint8_t(len(d.ProtocolData))
		for i, v := range d.ProtocolData {
			ci.protocol_data[i] = C.uint8_t(v)
		}
	case TRSCardPresent:
		cp := C.trs_card_present(r)
		cp.reader = C.uint8_t(d.Reader)
		cp.status = C.enum_osdp_trs_card_status_e(d.Status)
	case TRSCardData:
		cd := C.trs_card_data(r)
		cd.reader = C.uint8_t(d.Reader)
		cd.status = C.uint8_t(d.Status)
		return setTRSAPDU(&cd.apdu, d.APDU)
	case TRSPINResult:
		pc := C.trs_pin_complete(r)
		pc.reader = C.uint8_t(d.Reader)
		pc.status = C.uint8_t(d.Status)
		pc.tries = C.uint8_t(d.Tries)
	case TRSError:
		C.trs_error(r).code = C.uint8_t(d.Code)
	}
	return nil
}

func decodeTRSReply(r *C.struct_osdp_trs_reply) EventTRS {
	e := EventTRS{Reply: TRSReply(r.reply)}
	switch e.Reply {
	case TRSReplyCardInfo:
		ci := C.trs_card_info(r)
		e.Payload = TRSCardInfo{
			Reader:       uint8(ci.reader),
			Protocol:     TRSCardProtocol(ci.protocol),
			CSN:          C.GoBytes(unsafe.Pointer(&ci.csn[0]), C.int(ci.csn_len)),
			ProtocolData: C.GoBytes(unsafe.Pointer(&ci.protocol_data[0]), C.int(ci.protocol_data_len)),
		}
	case TRSReplyCardPresent:
		cp := C.trs_card_present(r)
		e.Payload = TRSCardPresent{Reader: uint8(cp.reader), Status: TRSCardStatus(cp.status)}
	case TRSReplyCardData:
		cd := C.trs_card_data(r)
		e.Payload = TRSCardData{Reader: uint8(cd.reader), Status: uint8(cd.status), APDU: trsAPDUBytes(&cd.apdu)}
	case TRSReplyPINComplete:
		pc := C.trs_pin_complete(r)
		e.Payload = TRSPINResult{Reader: uint8(pc.reader), Status: uint8(pc.status), Tries: uint8(pc.tries)}
	case TRSReplyError:
		e.Payload = TRSError{Code: uint8(C.trs_error(r).code)}
	}
	return e
}
