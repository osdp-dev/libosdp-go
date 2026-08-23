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

// OsdpFlag represents OSDP library flags
type OsdpFlag int

const (
	FlagEnforceSecure                OsdpFlag = C.OSDP_FLAG_ENFORCE_SECURE
	FlagInstallMode                  OsdpFlag = C.OSDP_FLAG_INSTALL_MODE
	FlagIgnoreUnsolicited            OsdpFlag = C.OSDP_FLAG_IGN_UNSOLICITED
	FlagEnableNotification           OsdpFlag = C.OSDP_FLAG_ENABLE_NOTIFICATION
	FlagCapturePackets               OsdpFlag = C.OSDP_FLAG_CAPTURE_PACKETS
	FlagAllowEmptyEncryptedDataBlock OsdpFlag = C.OSDP_FLAG_ALLOW_EMPTY_ENCRYPTED_DATA_BLOCK
	FlagBioReadRMultipart            OsdpFlag = C.OSDP_FLAG_BIOREADR_MULTIPART
)

// LogLevel represents OSDP log levels
type LogLevel int

const (
	LogEmergency LogLevel = C.OSDP_LOG_EMERG
	LogAlert     LogLevel = C.OSDP_LOG_ALERT
	LogCritical  LogLevel = C.OSDP_LOG_CRIT
	LogError     LogLevel = C.OSDP_LOG_ERROR
	LogWarning   LogLevel = C.OSDP_LOG_WARNING
	LogNotice    LogLevel = C.OSDP_LOG_NOTICE
	LogInfo      LogLevel = C.OSDP_LOG_INFO
	LogDebug     LogLevel = C.OSDP_LOG_DEBUG
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case LogEmergency:
		return "EMERG"
	case LogAlert:
		return "ALERT"
	case LogCritical:
		return "CRIT"
	case LogError:
		return "ERROR"
	case LogWarning:
		return "WARN"
	case LogNotice:
		return "NOTICE"
	case LogInfo:
		return "INFO"
	case LogDebug:
		return "DEBUG"
	default:
		return "UNKNOWN"
	}
}

// OsdpCommandType represents OSDP command types
type OsdpCommandType int

const (
	CmdOutput     OsdpCommandType = C.OSDP_CMD_OUTPUT
	CmdLED        OsdpCommandType = C.OSDP_CMD_LED
	CmdBuzzer     OsdpCommandType = C.OSDP_CMD_BUZZER
	CmdText       OsdpCommandType = C.OSDP_CMD_TEXT
	CmdKeyset     OsdpCommandType = C.OSDP_CMD_KEYSET
	CmdComset     OsdpCommandType = C.OSDP_CMD_COMSET
	CmdFileTx     OsdpCommandType = C.OSDP_CMD_FILE_TX
	CmdMfg        OsdpCommandType = C.OSDP_CMD_MFG
	CmdStatus     OsdpCommandType = C.OSDP_CMD_STATUS
	CmdComsetDone OsdpCommandType = C.OSDP_CMD_COMSET_DONE
	CmdTDSet      OsdpCommandType = C.OSDP_CMD_TDSET
	CmdBioRead    OsdpCommandType = C.OSDP_CMD_BIOREAD
	CmdBioMatch   OsdpCommandType = C.OSDP_CMD_BIOMATCH
	CmdPIVData    OsdpCommandType = C.OSDP_CMD_PIVDATA
	CmdGenAuth    OsdpCommandType = C.OSDP_CMD_GENAUTH
	CmdCRAuth     OsdpCommandType = C.OSDP_CMD_CRAUTH
)

// OsdpEventType represents OSDP event types
type OsdpEventType int

const (
	EvtCardRead     OsdpEventType = C.OSDP_EVENT_CARDREAD
	EvtKeypress     OsdpEventType = C.OSDP_EVENT_KEYPRESS
	EvtMfgReply     OsdpEventType = C.OSDP_EVENT_MFGREP
	EvtStatus       OsdpEventType = C.OSDP_EVENT_STATUS
	EvtNotification OsdpEventType = C.OSDP_EVENT_NOTIFICATION
	EvtMfgStatus    OsdpEventType = C.OSDP_EVENT_MFGSTATR
	EvtMfgError     OsdpEventType = C.OSDP_EVENT_MFGERRR
	EvtBioReadR     OsdpEventType = C.OSDP_EVENT_BIOREADR
	EvtBioMatchR    OsdpEventType = C.OSDP_EVENT_BIOMATCHR
	EvtPIVDataR     OsdpEventType = C.OSDP_EVENT_PIVDATAR
	EvtGenAuthR     OsdpEventType = C.OSDP_EVENT_GENAUTHR
	EvtCRAuthR      OsdpEventType = C.OSDP_EVENT_CRAUTHR
)

// PdCapabilityFunction represents PD capability function codes
type PdCapabilityFunction int

const (
	CapContactStatusMonitoring PdCapabilityFunction = C.OSDP_PD_CAP_CONTACT_STATUS_MONITORING
	CapOutputControl           PdCapabilityFunction = C.OSDP_PD_CAP_OUTPUT_CONTROL
	CapCardDataFormat          PdCapabilityFunction = C.OSDP_PD_CAP_CARD_DATA_FORMAT
	CapReaderLEDControl        PdCapabilityFunction = C.OSDP_PD_CAP_READER_LED_CONTROL
	CapReaderAudibleOutput     PdCapabilityFunction = C.OSDP_PD_CAP_READER_AUDIBLE_OUTPUT
	CapReaderTextOutput        PdCapabilityFunction = C.OSDP_PD_CAP_READER_TEXT_OUTPUT
	CapTimeKeeping             PdCapabilityFunction = C.OSDP_PD_CAP_TIME_KEEPING
	CapCheckCharacterSupport   PdCapabilityFunction = C.OSDP_PD_CAP_CHECK_CHARACTER_SUPPORT
	CapCommunicationSecurity   PdCapabilityFunction = C.OSDP_PD_CAP_COMMUNICATION_SECURITY
	CapReceiveBufferSize       PdCapabilityFunction = C.OSDP_PD_CAP_RECEIVE_BUFFERSIZE
	CapLargestCombinedMessage  PdCapabilityFunction = C.OSDP_PD_CAP_LARGEST_COMBINED_MESSAGE_SIZE
	CapSmartCardSupport        PdCapabilityFunction = C.OSDP_PD_CAP_SMART_CARD_SUPPORT
	CapReaders                 PdCapabilityFunction = C.OSDP_PD_CAP_READERS
	CapBiometrics              PdCapabilityFunction = C.OSDP_PD_CAP_BIOMETRICS
)

// Status report semantics changed upstream: report bytes are now one byte per
// entity (not bit-packed). StatusReportLocal is always exactly 2 entries.
// osdp_pd_submit_event rejects a report whose entry count does not match the
// capability the PD advertised. StatusReportReader entries are 0=normal,
// 1=not connected, 2=tamper.
//
// OsdpStatusReportType represents different status report types
type OsdpStatusReportType int

const (
	StatusReportInput  OsdpStatusReportType = C.OSDP_STATUS_REPORT_INPUT
	StatusReportOutput OsdpStatusReportType = C.OSDP_STATUS_REPORT_OUTPUT
	StatusReportLocal  OsdpStatusReportType = C.OSDP_STATUS_REPORT_LOCAL
	StatusReportReader OsdpStatusReportType = C.OSDP_STATUS_REPORT_READER
)

// Deprecated: OSDP_STATUS_REPORT_REMOTE was renamed to
// OSDP_STATUS_REPORT_READER upstream (same ordinal, per-reader byte
// semantics). Use StatusReportReader.
const StatusReportRemote = StatusReportReader

// OsdpLedColor represents LED color options
type OsdpLedColor int

const (
	LEDColorNone    OsdpLedColor = C.OSDP_LED_COLOR_NONE
	LEDColorRed     OsdpLedColor = C.OSDP_LED_COLOR_RED
	LEDColorGreen   OsdpLedColor = C.OSDP_LED_COLOR_GREEN
	LEDColorAmber   OsdpLedColor = C.OSDP_LED_COLOR_AMBER
	LEDColorBlue    OsdpLedColor = C.OSDP_LED_COLOR_BLUE
	LEDColorMagenta OsdpLedColor = C.OSDP_LED_COLOR_MAGENTA
	LEDColorCyan    OsdpLedColor = C.OSDP_LED_COLOR_CYAN
	LEDColorWhite   OsdpLedColor = C.OSDP_LED_COLOR_WHITE
)

// NotificationType identifies the kind of a libosdp notification event.
type NotificationType int

const (
	NotificationTypeCommand    NotificationType = C.OSDP_NOTIFICATION_COMMAND
	NotificationTypeSCStatus   NotificationType = C.OSDP_NOTIFICATION_SC_STATUS
	NotificationTypePDStatus   NotificationType = C.OSDP_NOTIFICATION_PD_STATUS
	NotificationTypeMPStart    NotificationType = C.OSDP_NOTIFICATION_MP_START
	NotificationTypeMPProgress NotificationType = C.OSDP_NOTIFICATION_MP_PROGRESS
	NotificationTypeMPDone     NotificationType = C.OSDP_NOTIFICATION_MP_DONE
)

// MPMsgType identifies the multipart message family carried by an MP_*
// notification.
type MPMsgType uint8

const (
	MPMsgFileTransfer MPMsgType = C.OSDP_MP_MSG_FILE_TRANSFER
	MPMsgPIV          MPMsgType = C.OSDP_MP_MSG_PIV
	MPMsgGenAuth      MPMsgType = C.OSDP_MP_MSG_GENAUTH
	MPMsgCRAuth       MPMsgType = C.OSDP_MP_MSG_CRAUTH
	MPMsgBioRead      MPMsgType = C.OSDP_MP_MSG_BIOREAD
)

// FileTxOutcome enumerates the possible outcomes of a multipart file transfer.
// It is surfaced as NotificationMP.Outcome on a NotificationTypeMPDone
// notification (meaningful only for that type).
type FileTxOutcome int

const (
	FileTxOutcomeOK           FileTxOutcome = C.OSDP_FILE_TX_OUTCOME_OK
	FileTxOutcomeOKRebooting  FileTxOutcome = C.OSDP_FILE_TX_OUTCOME_OK_REBOOTING
	FileTxOutcomeAborted      FileTxOutcome = C.OSDP_FILE_TX_OUTCOME_ABORTED
	FileTxOutcomeUnrecognized FileTxOutcome = C.OSDP_FILE_TX_OUTCOME_UNRECOGNIZED
	FileTxOutcomeInvalid      FileTxOutcome = C.OSDP_FILE_TX_OUTCOME_INVALID
)

// NakCode is the reason a PD rejected a command.
type NakCode int

const (
	NakNone       NakCode = C.OSDP_PD_NAK_NONE
	NakMsgChk     NakCode = C.OSDP_PD_NAK_MSG_CHK
	NakCmdLen     NakCode = C.OSDP_PD_NAK_CMD_LEN
	NakCmdUnknown NakCode = C.OSDP_PD_NAK_CMD_UNKNOWN
	NakSeqNum     NakCode = C.OSDP_PD_NAK_SEQ_NUM
	NakSCUnsup    NakCode = C.OSDP_PD_NAK_SC_UNSUP
	NakSCCond     NakCode = C.OSDP_PD_NAK_SC_COND
	NakBioType    NakCode = C.OSDP_PD_NAK_BIO_TYPE
	NakBioFmt     NakCode = C.OSDP_PD_NAK_BIO_FMT
	NakRecord     NakCode = C.OSDP_PD_NAK_RECORD
)

// CmdFileTxFlagCancel cancels an in-progress file transfer when set in
// CommandFileTx.Flags.
const CmdFileTxFlagCancel uint32 = C.OSDP_CMD_FILE_TX_FLAG_CANCEL

// CmdFlagBroadcast marks a command for broadcast to all PDs (Command.Flags).
const CmdFlagBroadcast uint32 = C.OSDP_CMD_FLAG_BROADCAST

// CompletionStatus represents the terminal state of a submitted command/event,
// delivered via the command/event completion callbacks.
type CompletionStatus int

const (
	CompletionOK      CompletionStatus = C.OSDP_COMPLETION_OK
	CompletionFailed  CompletionStatus = C.OSDP_COMPLETION_FAILED
	CompletionFlushed CompletionStatus = C.OSDP_COMPLETION_FLUSHED
	CompletionAborted CompletionStatus = C.OSDP_COMPLETION_ABORTED
)

// OsdpCardFormat represents card data format types
type OsdpCardFormat int

const (
	CardFormatUnspecified OsdpCardFormat = C.OSDP_CARD_FMT_RAW_UNSPECIFIED
	CardFormatWiegand     OsdpCardFormat = C.OSDP_CARD_FMT_RAW_WIEGAND
	CardFormatASCII       OsdpCardFormat = C.OSDP_CARD_FMT_ASCII
)
