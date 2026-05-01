// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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

	mounts := map[string]string{} // name -> mount path

	for _, sandbox := range c.alloc.AllocatedResources.Shared.Sandboxes {
		path := filepath.Join(c.sandboxDir, sandbox.ID)
		_, err := root.Stat(sandbox.ID)
		if os.IsNotExist(err) {
			c.logger.Debug("creating new sandbox", "path", path)
			f, err := root.Create(sandbox.ID)
			if err != nil {
				return fmt.Errorf("could not create sandbox %q file: %w", sandbox.ID, err)
			}
			// creates a sparse file. equivalent to:
			// dd if=/dev/zero of=$path bs=1 count=0 seek=$size
			err = f.Truncate(sandbox.CapacityBytes)
			if err != nil {
				f.Close()
				return fmt.Errorf("could not make sandbox %q sparse file: %w", sandbox.ID, err)
			}
			// TODO: obviously we'll want this to be a plugin like DHV
			f.Close()
			c.logger.Debug("file created, making file system", "path", path)

			cmd := exec.CommandContext(c.shutdownCtx, "mkfs.ext4", path)
			err = cmd.Run()
			if err != nil {
				rerr := root.Remove(sandbox.ID)
				if rerr != nil {
					err = fmt.Errorf("%w (failed to cleanup: %w)", err, rerr)
				}
				return fmt.Errorf("could not create sandbox %q filesystem: %w", sandbox.ID, err)
			}
			c.logger.Debug("filesystem created", "path", path)

		}
		mounts[sandbox.Name] = path

	}
	if len(mounts) > 0 {
		c.resources.SetSandboxMounts(mounts)
	}

	return nil
}

func (c *sandboxHook) shouldRun() bool {
	return len(c.alloc.AllocatedResources.Shared.Sandboxes) > 0
}
