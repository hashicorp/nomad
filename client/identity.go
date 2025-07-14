// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

// NodeIdentityHandler is an interface that allows setting a node identity
// token. The client uses this to inform its subsystems about a new node
// identity that it should use for RPC calls.
type NodeIdentityHandler interface {
	SetNodeIdentityToken(token string)
}

// assertAndSetNodeIdentityToken expects the passed interface implements
// NodeIdentityHandler and calls SetNodeIdentityToken. It is a programming error
// if the interface does not implement NodeIdentityHandler and will panic. The
// test file performs test assertions.
func assertAndSetNodeIdentityToken(impl any, token string) {
	if impl != nil {
		impl.(NodeIdentityHandler).SetNodeIdentityToken(token)
	}
}
