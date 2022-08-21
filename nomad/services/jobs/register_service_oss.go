//go:build !ent
//go:build !ent
// +build !ent
// +build !ent

package jobs

import "github.com/hashicorp/nomad/nomad/structs"

// enforceSubmitJob is used to check any Sentinel policies for the submit-job scope
func (svc *RegisterService) enforceSubmitJob(override bool, job *structs.Job) (error, error) {
	return nil, nil
}

// multiregionRegister is used to send a job across multiple regions
func (svc *RegisterService) multiregionRegister(args *structs.JobRegisterRequest, reply *structs.JobRegisterResponse, newVersion uint64) (bool, error) {
	return false, nil
}

// multiregionStart is used to kick-off a deployment across multiple regions
func (svc *RegisterService) multiregionStart(args *structs.JobRegisterRequest, reply *structs.JobRegisterResponse) error {
	return nil
}

// multiregionDrop is used to deregister regions from a previous version of the
// job that are no longer in use
func (svc *RegisterService) multiregionDrop(args *structs.JobRegisterRequest, reply *structs.JobRegisterResponse) error {
	return nil
}

// multiregionStop is used to fan-out Job.Deregister RPCs to all regions if
// the global flag is passed to Job.Deregister
func (svc *RegisterService) multiregionStop(job *structs.Job, args *structs.JobDeregisterRequest, reply *structs.JobDeregisterResponse) error {
	return nil
}

// interpolateMultiregionFields interpolates a job for a specific region
func (svc *RegisterService) interpolateMultiregionFields(args *structs.JobPlanRequest) error {
	return nil
}
