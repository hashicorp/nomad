// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import "github.com/hashicorp/nomad/helper/pointer"

const (
	// LimitsNonStreamingConnsPerClient is the number of connections per
	// peer to reserve for non-streaming RPC connections. Since streaming
	// RPCs require their own TCP connection, they have their own limit
	// this amount lower than the overall limit. This reserves a number of
	// connections for Raft and other RPCs.
	//
	// TODO Remove limit once MultiplexV2 is used.
	LimitsNonStreamingConnsPerClient = 20
)

// Limits configures timeout limits similar to Consul's limits configuration
// parameters. Limits is the internal version with the fields parsed.
type Limits struct {
	// HTTPSHandshakeTimeout is the deadline by which HTTPS TLS handshakes
	// must complete.
	//
	// 0 means no timeout.
	HTTPSHandshakeTimeout string `hcl:"https_handshake_timeout"`

	// HTTPMaxConnsPerClient is the maximum number of concurrent HTTP
	// connections from a single IP address. nil/0 means no limit.
	HTTPMaxConnsPerClient *int `hcl:"http_max_conns_per_client"`

	// RPCHandshakeTimeout is the deadline by which RPC handshakes must
	// complete. The RPC handshake includes the first byte read as well as
	// the TLS handshake and subsequent byte read if TLS is enabled.
	//
	// The deadline is reset after the first byte is read so when TLS is
	// enabled RPC connections may take (timeout * 2) to complete.
	//
	// The RPC handshake timeout only applies to servers. 0 means no
	// timeout.
	RPCHandshakeTimeout string `hcl:"rpc_handshake_timeout"`

	// RPCMaxConnsPerClient is the maximum number of concurrent RPC
	// connections from a single IP address. nil/0 means no limit.
	RPCMaxConnsPerClient *int `hcl:"rpc_max_conns_per_client"`
}

// DefaultLimits returns the default limits values. User settings should be
// merged into these defaults.
func DefaultLimits() Limits {
	return Limits{
		HTTPSHandshakeTimeout: "5s",
		HTTPMaxConnsPerClient: pointer.Of(100),
		RPCHandshakeTimeout:   "5s",
		RPCMaxConnsPerClient:  pointer.Of(100),
	}
}

// Merge returns a new Limits where non-empty/nil fields in the argument have
// precedence.
func (l *Limits) Merge(o Limits) Limits {
	m := *l

	if o.HTTPSHandshakeTimeout != "" {
		m.HTTPSHandshakeTimeout = o.HTTPSHandshakeTimeout
	}
	if o.HTTPMaxConnsPerClient != nil {
		m.HTTPMaxConnsPerClient = pointer.Of(*o.HTTPMaxConnsPerClient)
	}
	if o.RPCHandshakeTimeout != "" {
		m.RPCHandshakeTimeout = o.RPCHandshakeTimeout
	}
	if o.RPCMaxConnsPerClient != nil {
		m.RPCMaxConnsPerClient = pointer.Of(*o.RPCMaxConnsPerClient)
	}

	return m
}

// Copy returns a new deep copy of a Limits struct.
func (l *Limits) Copy() Limits {
	c := *l
	if l.HTTPMaxConnsPerClient != nil {
		c.HTTPMaxConnsPerClient = pointer.Of(*l.HTTPMaxConnsPerClient)
	}
	if l.RPCMaxConnsPerClient != nil {
		c.RPCMaxConnsPerClient = pointer.Of(*l.RPCMaxConnsPerClient)
	}
	return c
}
