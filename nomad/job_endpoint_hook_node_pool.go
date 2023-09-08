// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

// jobNodePoolValidatingHook is an admission hook that ensures the job has valid
// node pool configuration.
type jobNodePoolValidatingHook struct {
	srv *Server
}

func (j jobNodePoolValidatingHook) Name() string {
	return "node-pool-validation"
}

func (j jobNodePoolValidatingHook) Validate(job *structs.Job) ([]error, error) {
	poolName := job.NodePool

	pool, err := j.srv.State().NodePoolByName(nil, poolName)
	if err != nil {
		return nil, err
	}
	if pool == nil {
		return nil, fmt.Errorf("job %q is in nonexistent node pool %q", job.ID, poolName)
	}

	return j.enterpriseValidation(job, pool)
}
