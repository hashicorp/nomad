// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package pool

type RPCType byte

const (
	RpcNomad     RPCType = 0x01
	RpcRaft      RPCType = 0x02
	RpcMultiplex RPCType = 0x03
	RpcTLS       RPCType = 0x04
	RpcStreaming RPCType = 0x05

	// RpcMultiplexV2 allows a multiplexed connection to switch modes between
	// RpcNomad and RpcStreaming per opened stream.
	RpcMultiplexV2 RPCType = 0x06
)
