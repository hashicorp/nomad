// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// BufConnWrapper implements consul-template's TransportDialer using a
// bufconn listener, to provide a way to Dial the in-memory listener
//
// Copied from github.com/hashicorp/vault/internalshared/listenerutil/bufconn.go

package bufconndialer

import (
	"context"
	"net"

	"google.golang.org/grpc/test/bufconn"
)

// BufConnWrapper implements consul-template's TransportDialer using a
// bufconn listener, to provide a way to Dial the in-memory listener
type BufConnWrapper struct {
	listener *bufconn.Listener
}

// New returns a new BufConnWrapper with a new bufconn.Listener. The wrapper
// provides a dialer for creating connections to the listener.
func New() (net.Listener, *BufConnWrapper) {
	ln := bufconn.Listen(1024 * 1024)
	return ln, &BufConnWrapper{listener: ln}
}

// NewBufConnWrapper returns a new BufConnWrapper using an
// existing bufconn.Listener
func NewBufConnWrapper(bcl *bufconn.Listener) *BufConnWrapper {
	return &BufConnWrapper{
		listener: bcl,
	}
}

// Dial connects to the listening end of the bufconn (satisfies
// consul-template's TransportDialer interface). This is essentially the client
// side of the bufconn connection.
func (bcl *BufConnWrapper) Dial(_, _ string) (net.Conn, error) {
	return bcl.listener.Dial()
}

// DialContext connects to the listening end of the bufconn (satisfies
// consul-template's TransportDialer interface). This is essentially the client
// side of the bufconn connection.
func (bcl *BufConnWrapper) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	return bcl.listener.DialContext(ctx)
}
