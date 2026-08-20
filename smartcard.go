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
