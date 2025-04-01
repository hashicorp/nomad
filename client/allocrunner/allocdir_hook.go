// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/taskenv"
)

// allocDirHook creates and destroys the root directory and shared directories
// for an allocation.
type allocDirHook struct {
	allocDir allocdir.Interface
	logger   hclog.Logger
}

func newAllocDirHook(logger hclog.Logger, allocDir allocdir.Interface) *allocDirHook {
	ad := &allocDirHook{
		allocDir: allocDir,
	}
	ad.logger = logger.Named(ad.Name())
	return ad
}

// statically assert that the hook meets the expected interfaces
var (
	_ interfaces.RunnerPrerunHook  = (*allocDirHook)(nil)
	_ interfaces.RunnerDestroyHook = (*allocDirHook)(nil)
)

func (h *allocDirHook) Name() string {
	return "alloc_dir"
}

func (h *allocDirHook) Prerun(_ *taskenv.TaskEnv) error {
	return h.allocDir.Build()
}

func (h *allocDirHook) Destroy() error {
	return h.allocDir.Destroy()
}
