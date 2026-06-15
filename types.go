/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp

import "time"

// EventHandler is the callback invoked when a CP receives an event from a PD.
type EventHandler func(pdNumber int, event *Event) int

// CommandHandler is the callback invoked when a PD receives a command. It
// returns an ACK/NAK status and, optionally, a reply event.
type CommandHandler func(command *Command) (int, *Event)

// CommandCompletionHandler is invoked when a CP command submitted via
// SubmitCommand reaches a terminal state (delivered, failed, flushed, aborted).
type CommandCompletionHandler func(pdNumber int, command *Command, status CompletionStatus)

// EventCompletionHandler is invoked when a PD event submitted via SubmitEvent
// reaches a terminal state (delivered, failed, flushed, aborted).
type EventCompletionHandler func(event *Event, status CompletionStatus)

// RefreshContext holds the signalling channels for a CP/PD refresh goroutine.
type RefreshContext struct {
	stop     chan bool
	stopped  chan bool
	interval time.Duration
}
