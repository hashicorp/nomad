package allocrunner

import (
	"sync"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Ensure that csiHook implements all expected runner hooks at compile time.
var _ interfaces.RunnerHook = &csiHook{}
var _ interfaces.RunnerPrerunHook = &csiHook{}
var _ interfaces.RunnerPostrunHook = &csiHook{}
var _ interfaces.RunnerUpdateHook = &csiHook{}

// csiHook is the hook that is used to manage notifying the csiManager that a
// volume is required, and ensuring that it is available to tasks that require
// it.
//
// To do this, in a PreRun hook we:
// - Notify the csiManager that a volume was requested
// - Wait for the volume to become available
// - Create the mount configuration required for the task volume_hook
//
// In a PostRun hook we:
// - Notify the csiManager that the volume is no longer in active use
//
// In an Update hook we:
type csiHook struct {
	// hookLock is held by hook methods to prevent concurrent access by
	// Update and synchronous hooks.
	hookLock sync.Mutex

	// alloc set by new func or Update. Must hold hookLock to access.
	alloc *structs.Allocation

	logger log.Logger
}

func newCSIHook(logger log.Logger, alloc *structs.Allocation) interfaces.RunnerHook {
	return &csiHook{
		logger: logger,
		alloc:  alloc,
	}
}

func (c *csiHook) Name() string {
	return "csi_hook"
}

func (c *csiHook) Prerun() error {
	c.logger.Info("Skipping pre run hook")
	return nil
}

func (c *csiHook) Postrun() error {
	c.logger.Info("Skipping post run hook")
	return nil
}

// Unsure if this is needed rn
func (c *csiHook) Update(update *interfaces.RunnerUpdateRequest) error {
	c.logger.Info("Skipping update hook")
	return nil
}
