// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package deploymentwatcher

import "github.com/hashicorp/nomad/nomad/structs"

// DeploymentRPC and JobRPC hold methods for interacting with peer regions
// in enterprise edition.
type DeploymentRPC interface{}
type JobRPC interface{}

func (w *deploymentWatcher) nextRegion(status string) error {
	return nil
}

// RunDeployment is used to run a pending multiregion deployment.  In
// single-region deployments, the pending state is unused.
func (w *deploymentWatcher) RunDeployment(req *structs.DeploymentRunRequest, resp *structs.DeploymentUpdateResponse) error {
	return nil
}

// UnblockDeployment is used to unblock a multiregion deployment.  In
// single-region deployments, the blocked state is unused.
func (w *deploymentWatcher) UnblockDeployment(req *structs.DeploymentUnblockRequest, resp *structs.DeploymentUpdateResponse) error {
	return nil
}

// CancelDeployment is used to cancel a multiregion deployment.  In
// single-region deployments, the deploymentwatcher has sole responsibility to
// cancel deployments so this RPC is never used.
func (w *deploymentWatcher) CancelDeployment(req *structs.DeploymentCancelRequest, resp *structs.DeploymentUpdateResponse) error {
	return nil
}
