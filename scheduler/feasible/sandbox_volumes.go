package feasible

import (
	"fmt"
	"slices"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	sstructs "github.com/hashicorp/nomad/scheduler/structs"
)

func allocateSandboxVolumes(
	fastPath bool,
	store sstructs.State, ns string,
	volReqs map[string]*structs.VolumeRequest, option *RankedNode,
) (structs.AllocatedSandboxes, error) {

	// TODO: currently this doesn't correctly account for allocations on the
	// same node that want the same sandbox and need to count against the total
	// available claims, or client-terminal allocations
	sandboxes := []*structs.AllocatedSandbox{}
	if option.AllocResources != nil {
		sandboxes = slices.Clone(option.AllocResources.Sandboxes)
	}

NEXT_REQ:
	for _, req := range volReqs {
		if req.Type != structs.VolumeTypeSandbox {
			continue
		}
		iter, err := store.SandboxesByNode(ns, req.Source, option.Node.ID)
		if err != nil {
			return nil, fmt.Errorf("could not query sandboxes: %w", err)
		}
		for obj := iter.Next(); obj != nil; obj = iter.Next() {
			sandbox := obj.(*structs.SandboxVolume)
			if sandbox.IsFree(req.AccessMode) {
				allocated := &structs.AllocatedSandbox{
					ID:            sandbox.ID,
					Namespace:     ns,
					Name:          sandbox.Name,
					CapacityBytes: sandbox.CapacityBytes,
					TTL:           sandbox.TTL,
				}
				sandboxes = append(sandboxes, allocated)
				continue NEXT_REQ
			}
		}
		// in the fast path we're trying to only find nodes with unclaimed
		// sandboxes, so we won't allocate a new sandbox here and wait until
		// we're not in the fast path
		if fastPath {
			return nil, fmt.Errorf("could not find unclaimed sandbox")
		} else {
			allocated := &structs.AllocatedSandbox{
				ID:            uuid.Generate(),
				Namespace:     ns,
				Name:          req.Source,
				CapacityBytes: req.Sandbox.MinBytes, // TODO
				TTL:           req.Sandbox.TTL,
			}
			sandboxes = append(sandboxes, allocated)
			continue NEXT_REQ
		}
	}

	return structs.AllocatedSandboxes(sandboxes), nil
}
