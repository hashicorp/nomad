// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

// An SITokenAccessor is a reference to a created Consul Service Identity token on
// behalf of an allocation's task.
type SITokenAccessor struct {
	ConsulNamespace string
	NodeID          string
	AllocID         string
	AccessorID      string
	TaskName        string

	// Raft index
	CreateIndex uint64
}
