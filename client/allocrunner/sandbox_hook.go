// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/hashicorp/go-hclog"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/nomad/structs"
)

type sandboxHook struct {
	alloc          *structs.Allocation
	logger         hclog.Logger
	sandboxDir     string
	resources      *cstructs.AllocHookResources
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
}

func newSandboxHook(alloc *structs.Allocation, logger hclog.Logger, sandboxDir string, resources *cstructs.AllocHookResources) *sandboxHook {
	shutdownCtx, shutdownCancelFn := context.WithCancel(context.Background())
	return &sandboxHook{
		alloc:          alloc,
		logger:         logger,
		sandboxDir:     sandboxDir,
		resources:      resources,
		shutdownCtx:    shutdownCtx,
		shutdownCancel: shutdownCancelFn,
	}
}

func (c *sandboxHook) Name() string {
	return "sandbox_hook"
}

func (c *sandboxHook) Prerun(_ *taskenv.TaskEnv) error {
	if !c.shouldRun() {
		return nil
	}
	root, err := os.OpenRoot(c.sandboxDir)
	if err != nil {
		return fmt.Errorf("could not open sandbox root dir: %w", err)
	}

	// TODO: need to make sure we're not going to let a type=host volume mount
	// one of these?

	for _, sandbox := range c.alloc.AllocatedResources.Shared.Sandboxes {
		_, err := root.Stat(sandbox.ID)
		if errors.Is(err, os.ErrExist) {
			f, err := root.Create(sandbox.ID)
			if err != nil {
				return fmt.Errorf("could not open sandbox file: %w", err)
			}
			// TODO: how do we want to size and sparsely populate this?
			f.Close()
		}
	}

	return nil
}

func (c *sandboxHook) shouldRun() bool {
	return len(c.alloc.AllocatedResources.Shared.Sandboxes) > 0
}
