// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package nomad

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

// enforceSubmitJob is used to check any Sentinel policies for the submit-job scope
func (j *Job) enforceSubmitJob(override bool, job *structs.Job, nomadACLToken *structs.ACLToken, ns *structs.Namespace) (error, error) {
	return nil, nil
}

// multiregionCreateDeployment is used to create a deployment to register along
// with the job, if required.
func (j *Job) multiregionCreateDeployment(job *structs.Job, eval *structs.Evaluation) *structs.Deployment {
	return nil
}

// multiregionRegister is used to send a job across multiple regions
func (j *Job) multiregionRegister(args *structs.JobRegisterRequest, reply *structs.JobRegisterResponse, newVersion uint64) (bool, error) {
	return false, nil
}

// multiregionStart is used to kick-off a deployment across multiple regions
func (j *Job) multiregionStart(args *structs.JobRegisterRequest, reply *structs.JobRegisterResponse) error {
	return nil
}

// multiregionDrop is used to deregister regions from a previous version of the
// job that are no longer in use
func (j *Job) multiregionDrop(args *structs.JobRegisterRequest, reply *structs.JobRegisterResponse) error {
	return nil
}

// multiregionStop is used to fan-out Job.Deregister RPCs to all regions if
// the global flag is passed to Job.Deregister
func (j *Job) multiregionStop(job *structs.Job, args *structs.JobDeregisterRequest, reply *structs.JobDeregisterResponse) error {
	return nil
}

// interpolateMultiregionFields interpolates a job for a specific region
func (j *Job) interpolateMultiregionFields(args *structs.JobPlanRequest) error {
	return nil
}

// multiregionSpecChanged checks to see if the job spec has changed. If the job is multiregion,
// it checks all regions to determine if any deployed jobs instances have been stopped or
// otherwise differ from the incoming jobspec. Since multiregion jobs require coordinated
// deployments and synchronized job versions across all regions, a change in one requires
// redeployment of all.
func (j *Job) multiregionSpecChanged(existingJob *structs.Job, args *structs.JobRegisterRequest) (bool, error) {
	return existingJob.SpecChanged(args.Job), nil
}
