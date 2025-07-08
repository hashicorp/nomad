// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

// NodeIdentityHandler is an interface that allows setting a node identity
// token. The client uses this to inform its subsystems about a new node
// identity that it should use for RPC calls.
type NodeIdentityHandler interface {
	SetNodeIdentityToken(token string)
}

// assertAndSetNodeIdentityToken safely asserts that the provided implementation
// satisfies the NodeIdentityHandler interface and calls SetNodeIdentityToken if
// it does.
func assertAndSetNodeIdentityToken(impl any, token string) {
	if handler, ok := impl.(NodeIdentityHandler); ok {
		handler.SetNodeIdentityToken(token)
	}
}
