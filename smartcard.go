/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

// Package-internal file: biometric, PIV, and smartcard authentication command
// and event bindings, kept together and separate from the core command/event
// files. The encode/decode switch arms for these types live in commands.go and
// events.go (Go requires the switch and the types in the same package, which
// they are).
package osdp

/*
#include "cgo_helpers.h"
*/
import "C"

// BiometricType identifies the body part or scan a biometric command targets.
type BiometricType uint8

const (
	BioTypeNotSpecified           BiometricType = C.OSDP_BIO_TYPE_NOT_SPECIFIED
	BioTypeRightThumbPrint        BiometricType = C.OSDP_BIO_TYPE_RIGHT_THUMB_PRINT
	BioTypeRightIndexFingerPrint  BiometricType = C.OSDP_BIO_TYPE_RIGHT_INDEX_FINGER_PRINT
	BioTypeRightMiddleFingerPrint BiometricType = C.OSDP_BIO_TYPE_RIGHT_MIDDLE_FINGER_PRINT
	BioTypeRightRingFingerPrint   BiometricType = C.OSDP_BIO_TYPE_RIGHT_RING_FINGER_PRINT
	BioTypeRightLittleFingerPrint BiometricType = C.OSDP_BIO_TYPE_RIGHT_LITTLE_FINGER_PRINT
	BioTypeLeftThumbPrint         BiometricType = C.OSDP_BIO_TYPE_LEFT_THUMB_PRINT
	BioTypeLeftIndexFingerPrint   BiometricType = C.OSDP_BIO_TYPE_LEFT_INDEX_FINGER_PRINT
	BioTypeLeftMiddleFingerPrint  BiometricType = C.OSDP_BIO_TYPE_LEFT_MIDDLE_FINGER_PRINT
	BioTypeLeftRingFingerPrint    BiometricType = C.OSDP_BIO_TYPE_LEFT_RING_FINGER_PRINT
	BioTypeLeftLittleFingerPrint  BiometricType = C.OSDP_BIO_TYPE_LEFT_LITTLE_FINGER_PRINT
	BioTypeRightIrisScan          BiometricType = C.OSDP_BIO_TYPE_RIGHT_IRIS_SCAN
	BioTypeRightRetinaScan        BiometricType = C.OSDP_BIO_TYPE_RIGHT_RETINA_SCAN
	BioTypeLeftIrisScan           BiometricType = C.OSDP_BIO_TYPE_LEFT_IRIS_SCAN
	BioTypeLeftRetinaScan         BiometricType = C.OSDP_BIO_TYPE_LEFT_RETINA_SCAN
	BioTypeFullFaceImage          BiometricType = C.OSDP_BIO_TYPE_FULL_FACE_IMAGE
	BioTypeRightHandGeometry      BiometricType = C.OSDP_BIO_TYPE_RIGHT_HAND_GEOMETRY
	BioTypeLeftHandGeometry       BiometricType = C.OSDP_BIO_TYPE_LEFT_HAND_GEOMETRY
)

// BiometricFormat identifies the encoding of biometric template data.
type BiometricFormat uint8

const (
	BioFmtNotSpecified  BiometricFormat = C.OSDP_BIO_FMT_NOT_SPECIFIED
	BioFmtRawPGM        BiometricFormat = C.OSDP_BIO_FMT_RAW_PGM
	BioFmtANSIINCITS378 BiometricFormat = C.OSDP_BIO_FMT_ANSI_INCITS_378
)

// BiometricStatus is the outcome reported in a biometric reply event. The enum
// is non-sequential (Success=0x00, Timeout=0x01, UnknownError=0xFF).
type BiometricStatus uint8

const (
	BioStatusSuccess      BiometricStatus = C.OSDP_BIO_STATUS_SUCCESS
	BioStatusTimeout      BiometricStatus = C.OSDP_BIO_STATUS_TIMEOUT
	BioStatusUnknownError BiometricStatus = C.OSDP_BIO_STATUS_UNKNOWN_ERROR
)

// CommandBioMatch asks the PD to scan and match against an attached template.
// Data holds the template (at most OSDP_CMD_BIOMATCH_MAX_TEMPLATE_LEN bytes).
type CommandBioMatch struct {
	Reader  uint8
	Type    BiometricType
	Format  BiometricFormat
	Quality uint8
	Data    []byte
}

func (c CommandBioMatch) GetType() OsdpCommandType { return CmdBioMatch }

// EventBioMatchR is the reply to a CommandBioMatch. Score is valid only when
// Status is BioStatusSuccess.
type EventBioMatchR struct {
	Reader uint8
	Status BiometricStatus
	Score  uint8
}

func (e EventBioMatchR) GetType() OsdpEventType { return EvtBioMatchR }

// CommandBioRead asks the PD to scan a biometric and return it in a
// EventBioReadR. Large templates require FlagBioReadRMultipart on both roles.
type CommandBioRead struct {
	Reader  uint8
	Type    BiometricType
	Format  BiometricFormat
	Quality uint8
}

func (c CommandBioRead) GetType() OsdpCommandType { return CmdBioRead }

// EventBioReadR is the reply to a CommandBioRead. The remaining fields are valid
// only when Status is BioStatusSuccess. Data holds the template/image (at most
// OSDP_EVENT_BIOREADR_MAX_TEMPLATE_LEN bytes).
type EventBioReadR struct {
	Reader  uint8
	Status  BiometricStatus
	Type    BiometricType
	Quality uint8
	Data    []byte
}

func (e EventBioReadR) GetType() OsdpEventType { return EvtBioReadR }

// CommandPIVData requests PIV object data by OID, element, and offset.
type CommandPIVData struct {
	OID     [3]byte
	Element uint8
	Offset  uint8
}

func (c CommandPIVData) GetType() OsdpCommandType { return CmdPIVData }

// EventPIVData is a smartcard/PIV reply (PIVDATAR), reassembled from the
// multipart transfer. Data is at most OSDP_PIV_DATA_MAX_LEN bytes.
type EventPIVData struct{ Data []byte }

func (e EventPIVData) GetType() OsdpEventType { return EvtPIVDataR }
