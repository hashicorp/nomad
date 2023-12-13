// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tasklifecycle

const (
	gateClosed = false
	gateOpened = true
)

// Gate is used by the Coordinator to block or allow tasks from running.
//
// It provides a channel that taskRunners listens on to determine when they are
// allowed to run. The Gate has an infinite loop that is either feeding this
// channel (therefore allowing listeners to proceed) or not doing anything
// (causing listeners to block an wait).
//
// The Coordinator uses the Gate Open() and Close() methods to control this
// producer loop.
type Gate struct {
	sendCh     chan struct{}
	updateCh   chan bool
	shutdownCh <-chan struct{}
}

// NewGate returns a new Gate that is initially closed. The Gate should not be
// used after the shutdownCh is closed.
func NewGate(shutdownCh <-chan struct{}) *Gate {
	g := &Gate{
		sendCh:     make(chan struct{}),
		updateCh:   make(chan bool),
		shutdownCh: shutdownCh,
	}
	go g.run(gateClosed)

	return g
}

// WaitCh returns a channel that the listener must block on before starting its
// task.
//
// Callers must also check the state of the shutdownCh used to create the Gate
// to avoid blocking indefinitely.
func (g *Gate) WaitCh() <-chan struct{} {
	return g.sendCh
}

// Open is used to allow listeners to proceed.
// If the gate shutdownCh channel is closed, this method is a no-op so callers
// should check its state.
func (g *Gate) Open() {
	select {
	case <-g.shutdownCh:
	case g.updateCh <- gateOpened:
	}
}

// Close is used to block listeners from proceeding.
// if the gate shutdownch channel is closed, this method is a no-op so callers
// should check its state.
func (g *Gate) Close() {
	select {
	case <-g.shutdownCh:
	case g.updateCh <- gateClosed:
	}
}

// run starts the infinite loop that feeds the channel if the Gate is opened.
func (g *Gate) run(initState bool) {
	isOpen := initState
	for {
		if isOpen {
			select {
			// Feed channel if the gate is open.
			case g.sendCh <- struct{}{}:
			case <-g.shutdownCh:
				return
			case isOpen = <-g.updateCh:
				continue
			}
		} else {
			select {
			case <-g.shutdownCh:
				return
			case isOpen = <-g.updateCh:
				continue
			}
		}
	}
}
