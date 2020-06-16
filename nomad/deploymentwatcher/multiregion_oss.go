// +build !ent

package deploymentwatcher

import "github.com/hashicorp/nomad/nomad/structs"

type multiregionTriggers interface{}

func (w *deploymentWatcher) nextRegion(status string) error {
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
