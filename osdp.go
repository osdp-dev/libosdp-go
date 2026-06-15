/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp

/*
#include <osdp.h>
*/
import "C"

// GetVersion returns the LibOSDP version string
func GetVersion() string {
	return C.GoString(C.osdp_get_version())
}

// GetSourceInfo returns source information about LibOSDP
func GetSourceInfo() string {
	return C.GoString(C.osdp_get_source_info())
}
