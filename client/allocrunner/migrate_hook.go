// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"context"
	"fmt"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
)

// diskMigrationHook migrates ephemeral disk volumes. Depends on alloc dir
// being built but must be run before anything else manipulates the alloc dir.
type diskMigrationHook struct {
	allocDir     *allocdir.AllocDir
	allocWatcher config.PrevAllocMigrator
	logger       log.Logger
}

func newDiskMigrationHook(logger log.Logger, allocWatcher config.PrevAllocMigrator, allocDir *allocdir.AllocDir) *diskMigrationHook {
	h := &diskMigrationHook{
		allocDir:     allocDir,
		allocWatcher: allocWatcher,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (h *diskMigrationHook) Name() string {
	return "migrate_disk"
}

func (h *diskMigrationHook) Prerun() error {
	ctx := context.TODO()

	// Wait for a previous alloc - if any - to terminate
	if err := h.allocWatcher.Wait(ctx); err != nil {
		return err
	}

	// Wait for data to be migrated from a previous alloc if applicable
	if err := h.allocWatcher.Migrate(ctx, h.allocDir); err != nil {
		if err == context.Canceled {
			return err
		}

		// Soft-fail on migration errors
		h.logger.Warn("error migrating data from previous alloc", "error", err)

		// Recreate alloc dir to ensure a clean slate
		h.allocDir.Destroy()
		if err := h.allocDir.Build(); err != nil {
			return fmt.Errorf("failed to clean task directories after failed migration: %v", err)
		}
	}

	return nil
}
