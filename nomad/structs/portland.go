// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

type PortlandRequest struct {
	WriteRequest

	UpsertAllocs []*Allocation
	UpsertJobs   []*Job

	DeleteAllocs     []string
	DeleteJobs       []NamespacedID
	DeleteNodes      []string
	DeleteNamespaces []string
}
