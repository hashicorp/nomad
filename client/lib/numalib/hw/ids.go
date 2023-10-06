// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package hw provides types for identifying hardware.
//
// This is a separate "leaf" package that is easy to import from many other
// packages without creating circular imports.
package hw

type (
	// A NodeID represents a NUMA node. There could be more than
	// one NUMA node per socket.
	//
	// Must be an alias because go-msgpack cannot handle the real type.
	NodeID = uint8

	// A SocketID represents a physicsl CPU socket.
	SocketID uint8

	// A CoreID represents one logical (vCPU) core.
	CoreID uint16
)
