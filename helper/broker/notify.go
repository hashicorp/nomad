// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package broker

import (
	"time"

	"github.com/hashicorp/nomad/helper"
)

// GenericNotifier allows a process to send updates to many subscribers in an
// easy manner.
type GenericNotifier struct {

	// publishCh is the channel used to receive the update which will be sent
	// to all subscribers.
	publishCh chan interface{}

	// subscribeCh and unsubscribeCh are the channels used to modify the
	// subscription membership mapping.
	subscribeCh   chan chan interface{}
	unsubscribeCh chan chan interface{}
}

// NewGenericNotifier returns a generic notifier which can be used by a process
// to notify many subscribers when a specific update is triggered.
func NewGenericNotifier() *GenericNotifier {
	return &GenericNotifier{
		publishCh:     make(chan interface{}, 1),
		subscribeCh:   make(chan chan interface{}, 1),
		unsubscribeCh: make(chan chan interface{}, 1),
	}
}

// Notify allows the implementer to notify all subscribers with a specific
// update. There is no guarantee the order in which subscribers receive the
// message which is sent linearly.
func (g *GenericNotifier) Notify(msg interface{}) {
	select {
	case g.publishCh <- msg:
	default:
	}
}

// Run is a long-lived process which handles updating subscribers as well as
// ensuring any update is sent to them. The passed stopCh is used to coordinate
// shutdown.
func (g *GenericNotifier) Run(stopCh <-chan struct{}) {

	// Store our subscribers inline with a map. This map can only be accessed
	// via a single channel update at a time, meaning we can manage without
	// using a lock.
	subscribers := map[chan interface{}]struct{}{}

	for {
		select {
		case <-stopCh:
			return
		case msgCh := <-g.subscribeCh:
			subscribers[msgCh] = struct{}{}
		case msgCh := <-g.unsubscribeCh:
			delete(subscribers, msgCh)
		case update := <-g.publishCh:
			for subscriberCh := range subscribers {

				// The subscribers channels are buffered, but ensure we don't
				// block the whole process on this.
				select {
				case subscriberCh <- update:
				default:
				}
			}
		}
	}
}

// WaitForChange allows a subscriber to wait until there is a notification
// change, or the timeout is reached. The function will block until one
// condition is met.
func (g *GenericNotifier) WaitForChange(timeout time.Duration) interface{} {

	// Create a channel and subscribe to any update. This channel is buffered
	// to ensure we do not block the main broker process.
	updateCh := make(chan interface{}, 1)
	g.subscribeCh <- updateCh

	// Create a timeout timer and use the helper to ensure this routine doesn't
	// panic and making the stop call clear.
	timeoutTimer, timeoutStop := helper.NewSafeTimer(timeout)

	// Defer a function which performs all the required cleanup of the
	// subscriber once it has been notified of a change, or reached its wait
	// timeout.
	defer func() {
		g.unsubscribeCh <- updateCh
		close(updateCh)
		timeoutStop()
	}()

	// Enter the main loop which listens for an update or timeout and returns
	// this information to the subscriber.
	select {
	case <-timeoutTimer.C:
		return "wait timed out after " + timeout.String()
	case update := <-updateCh:
		return update
	}
}
