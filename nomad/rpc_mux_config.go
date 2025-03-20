// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"github.com/hashicorp/yamux"
	"time"
)

// RPCMuxConfig allows for tunable yamux multiplex configuration
type RPCMuxConfig struct {
	// AcceptBacklog is used to limit how many streams may be
	// waiting an accept.
	AcceptBacklog int

	// EnableKeepalive is used to do a period keep alive
	// messages using a ping.
	EnableKeepAlive bool

	// KeepAliveInterval is how often to perform the keep alive
	KeepAliveInterval time.Duration

	// ConnectionWriteTimeout is meant to be a "safety valve" timeout after
	// we which will suspect a problem with the underlying connection and
	// close it. This is only applied to writes, where's there's generally
	// an expectation that things will move along quickly.
	ConnectionWriteTimeout time.Duration

	// StreamOpenTimeout is the maximum amount of time that a stream will
	// be allowed to remain in pending state while waiting for an ack from the peer.
	// Once the timeout is reached the session will be gracefully closed.
	// A zero value disables the StreamOpenTimeout allowing unbounded
	// blocking on OpenStream calls.
	StreamOpenTimeout time.Duration

	// StreamCloseTimeout is the maximum time that a stream will allowed to
	// be in a half-closed state when `Close` is called before forcibly
	// closing the connection. Forcibly closed connections will empty the
	// receive buffer, drop any future packets received for that stream,
	// and send a RST to the remote side.
	StreamCloseTimeout time.Duration
}

func (c *RPCMuxConfig) Copy() *RPCMuxConfig {
	if c == nil {
		return nil
	}

	nc := *c
	return &nc
}

func (c *RPCMuxConfig) GetYamuxConfig() *yamux.Config {
	cfg := yamux.DefaultConfig()
	if c != nil {
		cfg.EnableKeepAlive = c.EnableKeepAlive
		if c.AcceptBacklog > 0 {
			cfg.AcceptBacklog = c.AcceptBacklog
		}
		if c.KeepAliveInterval > 0 {
			cfg.KeepAliveInterval = c.KeepAliveInterval
		}
		if c.ConnectionWriteTimeout > 0 {
			cfg.ConnectionWriteTimeout = c.ConnectionWriteTimeout
		}
		if c.StreamCloseTimeout > 0 {
			cfg.StreamCloseTimeout = c.StreamCloseTimeout
		}
		if c.StreamOpenTimeout > 0 {
			cfg.StreamOpenTimeout = c.StreamOpenTimeout
		}
	}

	return cfg
}
