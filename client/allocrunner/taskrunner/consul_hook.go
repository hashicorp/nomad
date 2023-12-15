// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// consulTokenFilePrefix is the begging of the name of the file holding the
	// Consul SI token inside the task's secret directory. Full name of the file is
	// always consulTokenFilePrefix_identityName
	consulTokenFilePrefix = "nomad_consul"

	// consulTokenFilePerms is the level of file permissions granted on the file in
	// the secrets directory for the task
	consulTokenFilePerms = 0440
)

type consulHook struct {
	task          *structs.Task
	tokenDir      string
	hookResources *cstructs.AllocHookResources
	logger        log.Logger
}

func newConsulHook(logger log.Logger, tr *TaskRunner) *consulHook {
	h := &consulHook{
		task:          tr.Task(),
		tokenDir:      tr.taskDir.SecretsDir,
		hookResources: tr.allocHookResources,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*consulHook) Name() string {
	return "consul_task"
}

func (h *consulHook) Prestart(context.Context, *interfaces.TaskPrestartRequest, *interfaces.TaskPrestartResponse) error {
	mErr := multierror.Error{}

	tokens := h.hookResources.GetConsulTokens()

	// Write tokens to tasks' secret dirs
	for cluster, t := range tokens {
		for identity, token := range t {
			// do not write tokens that do not belong to any of this task's
			// identities
			if !slices.ContainsFunc(
				h.task.Identities,
				func(id *structs.WorkloadIdentity) bool { return id.Name == identity }) &&
				identity != h.task.Identity.Name {
				continue
			}

			filename := fmt.Sprintf("%s_%s_%s", consulTokenFilePrefix, cluster, identity)
			tokenPath := filepath.Join(h.tokenDir, filename)
			if err := os.WriteFile(tokenPath, []byte(token.SecretID), consulTokenFilePerms); err != nil {
				mErr.Errors = append(mErr.Errors, fmt.Errorf("failed to write Consul SI token: %w", err))
			}
		}
	}

	return mErr.ErrorOrNil()
}
