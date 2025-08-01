// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package reconciler

import "github.com/hashicorp/nomad/nomad/structs"

// cancelUnneededDeployments cancels any deployment that is not needed.
// A deployment update will be staged for jobs that should stop or have the
// wrong version. Unneeded deployments include:
// 1. Jobs that are marked for stop, but there is a non-terminal deployment.
// 2. Deployments that are active, but referencing a different job version.
// 3. Deployments that are already successful.
//
// returns: old deployment, current deployment and a slice of deployment status
// updates.
func cancelUnneededDeployments(j *structs.Job, d *structs.Deployment) (*structs.Deployment, *structs.Deployment, []*structs.DeploymentStatusUpdate) {
	var updates []*structs.DeploymentStatusUpdate

	// If the job is stopped and there is a non-terminal deployment, cancel it
	if j.Stopped() {
		if d != nil && d.Active() {
			updates = append(updates, &structs.DeploymentStatusUpdate{
				DeploymentID:      d.ID,
				Status:            structs.DeploymentStatusCancelled,
				StatusDescription: structs.DeploymentStatusDescriptionStoppedJob,
			})
		}

		// Nothing else to do
		return d, nil, updates
	}

	if d == nil {
		return nil, nil, nil
	}

	// Check if the deployment is active and referencing an older job and cancel it
	if d.JobCreateIndex != j.CreateIndex || d.JobVersion != j.Version {
		if d.Active() {
			updates = append(updates, &structs.DeploymentStatusUpdate{
				DeploymentID:      d.ID,
				Status:            structs.DeploymentStatusCancelled,
				StatusDescription: structs.DeploymentStatusDescriptionNewerJob,
			})
		}

		return d, nil, updates
	}

	// Clear it as the current deployment if it is successful
	if d.Status == structs.DeploymentStatusSuccessful {
		return d, nil, updates
	}

	return nil, d, updates
}
