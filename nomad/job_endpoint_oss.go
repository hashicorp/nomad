// +build !ent

package nomad

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
	vapi "github.com/hashicorp/vault/api"
)

// enforceSubmitJob is used to check any Sentinel policies for the submit-job scope
func (j *Job) enforceSubmitJob(override bool, job *structs.Job) (error, error) {
	return nil, nil
}

// multiregionRegister is used to send a job across multiple regions
func (j *Job) multiregionRegister(args *structs.JobRegisterRequest, reply *structs.JobRegisterResponse, existingVersion uint64) (bool, error) {
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

// multiVaultNamespaceValidation provides a convience check to ensure
// multiple vault namespaces were not requested, this returns an early friendly
// error before job registry and further feature checks.
func (j *Job) multiVaultNamespaceValidation(
	policies map[string]map[string]*structs.Vault,
	s *vapi.Secret,
) error {
	requestedNamespaces := structs.VaultNamespaceSet(policies)
	if len(requestedNamespaces) > 0 {
		return fmt.Errorf("multiple vault namespaces requires Nomad Enterprise, Namespaces: %s", strings.Join(requestedNamespaces, ", "))
	}
	return nil
}
