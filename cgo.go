/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp

//go:generate go run ./internal/vendorgen

// This file holds the single, package-wide cgo build configuration. The libosdp
// C sources are vendored into internal/clib (which compiles them) from the
// libosdp/ submodule by "go generate .", so the package builds with a plain
// "go get" — no prebuilt library, CMake, submodule, or external link
// dependency (the bundled TinyAES backend is self-contained).

/*
#cgo CFLAGS: -I${SRCDIR} -I${SRCDIR}/internal/clib/include -I${SRCDIR}/internal/clib/src -I${SRCDIR}/internal/clib/src/crypto -I${SRCDIR}/internal/clib/utils/include
#cgo CFLAGS: -DOPT_OSDP_PACKET_TRACE
*/
import "C"

// Link the vendored LibOSDP C object code (compiled by the internal/clib cgo
// package) into any binary that uses this package. internal/clib exposes no Go
// API; the blank import exists purely to pull in its compiled libosdp symbols.
import _ "github.com/osdp-dev/libosdp-go/internal/clib"
