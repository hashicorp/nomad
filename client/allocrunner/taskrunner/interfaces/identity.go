// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package interfaces

import "github.com/hashicorp/nomad/nomad/structs"

type WorkloadIdentityStore interface {
	StoreWorkloadIdentity(string, *structs.SignedWorkloadIdentity) error
	GetWorkloadIdentity(string) (*structs.SignedWorkloadIdentity, error)
}
