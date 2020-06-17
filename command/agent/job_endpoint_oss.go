// +build !ent

package agent

import (
	"github.com/hashicorp/nomad/api"
)

func regionForJob(job *api.Job, requestRegion *string) *string {
	return requestRegion
}
