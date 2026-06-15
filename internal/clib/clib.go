/*
 * Copyright (c) 2025 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

// Package clib compiles the vendored LibOSDP C sources. It exposes no Go API and
// is imported (blank) by the parent osdp package solely so the libosdp object
// code is linked into the final binary. The C sources here (flattened *.c files
// plus the include tree) are generated from the libosdp/ git submodule by
// "go generate ." in the parent package; do not edit them by hand.
package clib

/*
#cgo CFLAGS: -I${SRCDIR}/include -I${SRCDIR}/src -I${SRCDIR}/src/crypto -I${SRCDIR}/utils/include
#cgo CFLAGS: -DOPT_OSDP_PACKET_TRACE
*/
import "C"
