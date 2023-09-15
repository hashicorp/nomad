// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"context"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/widmgr"
)

type identityHook struct {
	ar     *allocRunner
	widmgr widmgr.IdentityManager
	logger log.Logger
}

func newIdentityHook(ar *allocRunner, logger log.Logger) *identityHook {
	h := &identityHook{
		ar:     ar,
		widmgr: ar.widmgr,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*identityHook) Name() string {
	return "identity"
}

func (h *identityHook) Prerun() error {
	// run the renewal
	if err := h.widmgr.Run(); err != nil {
		return err
	}

	return nil
}

// Stop implements interfaces.TaskStopHook
func (h *identityHook) Stop(context.Context, *interfaces.TaskStopRequest, *interfaces.TaskStopResponse) error {
	h.widmgr.Shutdown()
	return nil
}

// Shutdown implements interfaces.ShutdownHook
func (h *identityHook) Shutdown() {
	h.widmgr.Shutdown()
}
