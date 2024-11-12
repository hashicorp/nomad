// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
)

// allocDirHook creates and destroys the root directory and shared directories
// for an allocation.
type allocDirHook struct {
	builder  allocdir.Builder
	allocDir *allocdir.AllocDir
	logger   hclog.Logger
}

func newAllocDirHook(logger hclog.Logger, builder allocdir.Builder, allocDir *allocdir.AllocDir) *allocDirHook {
	ad := &allocDirHook{
		allocDir: allocDir,
		builder:  builder,
	}
	ad.logger = logger.Named(ad.Name())
	return ad
}

func (h *allocDirHook) Name() string {
	return "alloc_dir"
}

func (h *allocDirHook) Prerun() error {
	return h.builder.Build(h.allocDir)
}

func (h *allocDirHook) Destroy() error {
	return h.builder.Destroy(h.allocDir)
}
