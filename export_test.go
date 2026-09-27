/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp

// Hooks into the C marshalling layer for the external tests in osdp_test.

// RoundTripCommand marshals cmd into its C struct and decodes it back.
func RoundTripCommand(cmd *Command) (*Command, error) {
	c, err := commandToC(cmd)
	if err != nil {
		return nil, err
	}
	defer freeCmd(c)
	return decodeCommand(c), nil
}

// RoundTripEvent marshals ev into its C struct and decodes it back.
func RoundTripEvent(ev *Event) (*Event, error) {
	c, err := eventToC(ev)
	if err != nil {
		return nil, err
	}
	defer freeEvent(c)
	return decodeEvent(c), nil
}
