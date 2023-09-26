// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/consul"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/widmgr"
	"github.com/hashicorp/nomad/nomad/structs"
	structsc "github.com/hashicorp/nomad/nomad/structs/config"
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
	alloc         *structs.Allocation
	allocdir      *allocdir.AllocDir
	widmgr        widmgr.IdentityManager
	consulConfigs map[string]*structsc.ConsulConfig
	hookResources *cstructs.AllocHookResources
	authMethod    string

	logger log.Logger
}

func newConsulHook(logger log.Logger, alloc *structs.Allocation,
	allocdir *allocdir.AllocDir,
	widmgr widmgr.IdentityManager,
	consulConfigs map[string]*structsc.ConsulConfig,
	hookResources *cstructs.AllocHookResources,
	authMethod string,
) *consulHook {
	h := &consulHook{
		alloc:         alloc,
		allocdir:      allocdir,
		widmgr:        widmgr,
		consulConfigs: consulConfigs,
		hookResources: hookResources,
		authMethod:    authMethod,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*consulHook) Name() string {
	return "consul"
}

func (h *consulHook) Prerun() error {
	job := h.alloc.Job

	if job == nil {
		// something crazy happened
		err := fmt.Errorf("alloc %v does not have a job", h.alloc.Name)
		h.logger.Error(err.Error())
		return err
	}

	mErr := multierror.Error{}
	tokens := map[string]string{}

	// get consul clients
	clients, err := getConsulClients(h.consulConfigs, h.logger)
	if err != nil {
		return err
	}

	for _, tg := range job.TaskGroups {
		for _, task := range tg.Tasks {
			req, err := h.prepareConsulClientReq(task)
			if err != nil {
				mErr.Errors = append(mErr.Errors, err)
				continue
			}

			// in case no service needs a consul token in this task
			if len(req) == 0 {
				continue
			}

			// Consul auth
			for consulName, client := range clients {
				tokens, err := client.DeriveSITokenWithJWT(req)
				if err != nil {
					h.logger.Error("error authenticating with Consul", "error", err)
					return err
				}

				// Write tokens to tasks' secret dirs
				secretsDir := h.allocdir.TaskDirs[task.Name].SecretsDir
				for identity, token := range tokens {
					filename := fmt.Sprintf("%s_%s_%s", consulTokenFilePrefix, consulName, identity)
					tokenPath := filepath.Join(secretsDir, filename)
					if err := os.WriteFile(tokenPath, []byte(token), consulTokenFilePerms); err != nil {
						mErr.Errors = append(mErr.Errors, fmt.Errorf("failed to write Consul SI token: %w", err))
					}
				}
			}
		}
	}

	// store the tokens in hookResources
	h.hookResources.SetConsulTokens(tokens)

	return mErr.ErrorOrNil()
}

func (h *consulHook) prepareConsulClientReq(task *structs.Task) (map[string]consul.JWTLoginRequest, error) {
	req := map[string]consul.JWTLoginRequest{}

	// see if maybe we can quit early
	if task.Services == nil {
		return req, nil
	}
	for _, s := range task.Services {
		if !s.IsConsul() {
			return req, nil
		}

		if s.Identity == nil {
			return req, nil
		}
	}

	// handle default identity
	if task.Identity != nil {
		ti := widmgr.TaskIdentity{
			TaskName:     task.Name,
			IdentityName: task.Identity.Name,
		}

		jwt, err := h.widmgr.Get(ti)
		if err != nil {
			h.logger.Error("error getting signed identity", "error", err)
			return req, err
		}

		req[task.Identity.Name] = consul.JWTLoginRequest{
			JWT:            jwt.JWT,
			AuthMethodName: h.authMethod,
		}
	}

	return req, nil
}

// Stop implements interfaces.TaskStopHook
func (h *consulHook) Stop(context.Context, *interfaces.TaskStopRequest, *interfaces.TaskStopResponse) error {
	h.widmgr.Shutdown()
	return nil
}

// Shutdown implements interfaces.ShutdownHook
func (h *consulHook) Shutdown() {
	h.widmgr.Shutdown()
}
