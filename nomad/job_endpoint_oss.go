// +build !ent

package nomad

import "github.com/hashicorp/nomad/nomad/structs"

// enforceSubmitJob is used to check any Sentinel policies for the submit-job scope
func (j *Job) enforceSubmitJob(override bool, job *structs.Job) (error, error) {
	return nil, nil
}
