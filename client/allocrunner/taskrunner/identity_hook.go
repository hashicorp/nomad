// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package taskrunner

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	log "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/helper/users"
)

// identityHook sets the task runner's Nomad workload identity token
// based on the signed identity stored on the Allocation

const (
	// wiTokenFile is the name of the file holding the Nomad token inside the
	// task's secret directory
	wiTokenFile = "nomad_token"
)

type identityHook struct {
	tr       *TaskRunner
	logger   log.Logger
	taskName string
	lock     sync.Mutex

	// tokenPath is the path in which to read and write the token
	tokenPath string
}

func newIdentityHook(tr *TaskRunner, logger log.Logger) *identityHook {
	h := &identityHook{
		tr:       tr,
		taskName: tr.taskName,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*identityHook) Name() string {
	return "identity"
}

func (h *identityHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.tokenPath = filepath.Join(req.TaskDir.SecretsDir, wiTokenFile)

	return h.setToken()
}

// setToken adds the Nomad token to the task's environment and writes it to a
// file if requested by the jobsepc.
func (h *identityHook) setToken() error {
	token := h.tr.alloc.SignedIdentities[h.taskName]
	if token == "" {
		return nil
	}

	h.tr.setNomadToken(token)

	if id := h.tr.task.Identity; id != nil && id.File {
		if err := h.writeToken(token); err != nil {
			return err
		}
	}

	return nil
}

// writeToken writes the given token to disk
func (h *identityHook) writeToken(token string) error {
	// Write token as owner readable only
	if err := users.WriteFileFor(h.tokenPath, []byte(token), h.tr.task.User); err != nil {
		return fmt.Errorf("failed to write nomad token: %w", err)
	}

	return nil
}
