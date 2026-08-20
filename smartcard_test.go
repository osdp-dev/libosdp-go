/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp

import "testing"

func TestBiometricEnumValues(t *testing.T) {
	if BioTypeNotSpecified != 0x00 || BioTypeLeftHandGeometry != 0x11 {
		t.Errorf("bio type bounds: %d..%d", BioTypeNotSpecified, BioTypeLeftHandGeometry)
	}
	if BioFmtANSIINCITS378 != 0x02 {
		t.Errorf("BioFmtANSIINCITS378 = %d", BioFmtANSIINCITS378)
	}
	if BioStatusSuccess != 0x00 || BioStatusTimeout != 0x01 || BioStatusUnknownError != 0xFF {
		t.Errorf("bio status = %d/%d/%d", BioStatusSuccess, BioStatusTimeout, BioStatusUnknownError)
	}
}
