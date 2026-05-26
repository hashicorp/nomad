// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package structs

// An SITokenAccessor is a reference to a created Consul Service Identity token on
// behalf of an allocation's task.
//
// DEPRECATED (1.10.0): this object exists only to allow decoding any accessors
// still left in state so they can be discarded during FSM restore
type SITokenAccessor struct {
	ConsulNamespace string
	NodeID          string
	AllocID         string
	AccessorID      string
	TaskName        string

	// Raft index
	CreateIndex uint64
}
