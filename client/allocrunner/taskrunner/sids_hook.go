// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	ti "github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// the name of this hook, used in logs
	sidsHookName = "consul_si_token"

	// sidsTokenFile is the name of the file holding the Consul SI token inside
	// the task's secret directory
	sidsTokenFile = "si_token"

	// sidsTokenFilePerms is the level of file permissions granted on the file
	// in the secrets directory for the task
	sidsTokenFilePerms = 0440
)

type sidsHookConfig struct {
	alloc              *structs.Allocation
	task               *structs.Task
	lifecycle          ti.TaskLifecycle
	logger             hclog.Logger
	allocHookResources *cstructs.AllocHookResources
}

// Service Identities hook for managing SI tokens of connect enabled tasks.
type sidsHook struct {
	// alloc is the allocation
	alloc *structs.Allocation

	// taskName is the name of the task
	task *structs.Task

	// lifecycle is used to signal, restart, and kill a task
	lifecycle ti.TaskLifecycle

	// logger is used to log
	logger hclog.Logger

	// lock variables that can be manipulated after hook creation
	lock sync.Mutex
	// firstRun keeps track of whether the hook is being called for the first
	// time (for this task) during the lifespan of the Nomad Client process.
	firstRun bool

	// allocHookResources gives us access to Consul tokens that may have been
	// set by the consul_hook
	allocHookResources *cstructs.AllocHookResources
}

func newSIDSHook(c sidsHookConfig) *sidsHook {
	return &sidsHook{
		alloc:              c.alloc,
		task:               c.task,
		lifecycle:          c.lifecycle,
		logger:             c.logger.Named(sidsHookName),
		firstRun:           true,
		allocHookResources: c.allocHookResources,
	}
}

func (h *sidsHook) Name() string {
	return sidsHookName
}

func (h *sidsHook) Prestart(
	ctx context.Context,
	req *interfaces.TaskPrestartRequest,
	resp *interfaces.TaskPrestartResponse) error {

	h.lock.Lock()
	defer h.lock.Unlock()

	// if we're using Workload Identities then this Connect task should already
	// have a token stored under the cluster + service ID.
	tokens := h.allocHookResources.GetConsulTokens()

	// Find the group-level service that this task belongs to
	tg := h.alloc.Job.LookupTaskGroup(h.alloc.TaskGroup)
	serviceName := h.task.Kind.Value()
	var serviceIdentityName string
	var cluster string
	for _, service := range tg.Services {
		if service.Name == serviceName {
			serviceIdentityName = service.MakeUniqueIdentityName()
			cluster = service.GetConsulClusterName(tg)
			break
		}
	}
	if cluster != "" && serviceIdentityName != "" {
		if token, ok := tokens[cluster][serviceIdentityName]; ok {
			if err := h.writeToken(req.TaskDir.SecretsDir, token.SecretID); err != nil {
				return err
			}
			return nil
		}
	}

	return nil
}

// writeToken writes token into the secrets directory for the task.
func (h *sidsHook) writeToken(dir string, token string) error {
	tokenPath := filepath.Join(dir, sidsTokenFile)
	if err := os.WriteFile(tokenPath, []byte(token), sidsTokenFilePerms); err != nil {
		return fmt.Errorf("failed to write SI token: %w", err)
	}
	return nil
}
