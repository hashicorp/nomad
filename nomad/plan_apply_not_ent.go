// +build !ent

package nomad

import (
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// evaluatePlanQuota returns whether the plan would be over quota
func evaluatePlanQuota(snap *state.StateSnapshot, plan *structs.Plan) (bool, error) {
	return false, nil
}
