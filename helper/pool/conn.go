// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package pool

type RPCType byte

const (
	RpcNomad     RPCType = 0x01
	RpcRaft              = 0x02
	RpcMultiplex         = 0x03
	RpcTLS               = 0x04
	RpcStreaming         = 0x05

	// RpcMultiplexV2 allows a multiplexed connection to switch modes between
	// RpcNomad and RpcStreaming per opened stream.
	RpcMultiplexV2 = 0x06
)
