// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/widmgr"
)

type identityHook struct {
	widmgr widmgr.IdentityManager
	logger log.Logger
}

func newIdentityHook(logger log.Logger, widmgr widmgr.IdentityManager) *identityHook {
	h := &identityHook{
		widmgr: widmgr,
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

// PreKill implements interfaces.PreKill and is called on allocation stop
func (h *identityHook) PreKill() {
	h.widmgr.Shutdown()
}

// Destroy implements interfaces.Destroy and is called on allocation GC
func (h *identityHook) Destroy() error {
	h.widmgr.Shutdown()
	return nil
}

// Shutdown implements interfaces.ShutdownHook and is called when the client
// gracefully shuts down
func (h *identityHook) Shutdown() {
	h.widmgr.Shutdown()
}
