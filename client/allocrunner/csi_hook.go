package allocrunner

import (
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
)

// csiHook will wait for remote csi volumes to be attached to the host before
// continuing.
//
// It is a noop for allocs that do not depend on CSI Volumes.
type csiHook struct {
	alloc  *structs.Allocation
	logger hclog.Logger
}

func (c *csiHook) Name() string {
	return "csi_hook"
}

func (c *csiHook) Prerun() error {
	if !c.shouldRun() {
		return nil
	}

	// TODO: Volume attachment flow
	return nil
}

func newCSIHook(logger hclog.Logger, alloc *structs.Allocation) *csiHook {
	return &csiHook{
		alloc:  alloc,
		logger: logger.Named("csi_hook"),
	}
}

func (h *csiHook) shouldRun() bool {
	tg := h.alloc.Job.LookupTaskGroup(h.alloc.TaskGroup)
	for _, vol := range tg.Volumes {
		if vol.Type == structs.VolumeTypeCSI {
			return true
		}
	}

	return false
}
