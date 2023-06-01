// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !ent
// +build !ent

package nomad

import "github.com/hashicorp/nomad/nomad/structs"

// enterpriseValidation implements any admission hooks for node pools for Nomad
// Enterprise.
func (j jobNodePoolValidatingHook) enterpriseValidation(_ *structs.Job, _ *structs.NodePool) ([]error, error) {
	return nil, nil
}

// jobNodePoolMutatingHook mutates the job on Nomad Enterprise only.
type jobNodePoolMutatingHook struct {
	srv *Server
}

func (c jobNodePoolMutatingHook) Name() string {
	return "node-pool-mutation"
}

func (c jobNodePoolMutatingHook) Mutate(job *structs.Job) (*structs.Job, []error, error) {
	return job, nil, nil
}
