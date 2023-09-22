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
)

const (
	// consulTokenFilePrefix is the begging of the name of the file holding the
	// Consul SI token inside the task's secret directory. Full name of the file is
	// always consulTokenFilePrefix_identityName
	consulTokenFilePrefix = "consul_token_"

	// consulTokenFilePerms is the level of file permissions granted on the file in
	// the secrets directory for the task
	consulTokenFilePerms = 0440
)

type consulHook struct {
	alloc         *structs.Allocation
	allocdir      *allocdir.AllocDir
	widmgr        widmgr.IdentityManager
	client        consul.Client
	hookResources *cstructs.AllocHookResources
	authMethod    string

	logger log.Logger
}

func newConsulHook(logger log.Logger, alloc *structs.Allocation,
	allocdir *allocdir.AllocDir,
	widmgr widmgr.IdentityManager,
	client consul.Client,
	hookResources *cstructs.AllocHookResources,
	authMethod string,
) *consulHook {
	h := &consulHook{
		alloc:         alloc,
		allocdir:      allocdir,
		widmgr:        widmgr,
		client:        client,
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

	for _, tg := range job.TaskGroups {
		for _, task := range tg.Tasks {
			req := map[string]consul.JWTLoginRequest{}

			// handle default identity
			if task.Identity != nil {
				ti := widmgr.TaskIdentity{
					TaskName:     task.Name,
					IdentityName: task.Identity.Name,
				}

				jwt, err := h.widmgr.Get(ti)
				if err != nil {
					mErr.Errors = append(mErr.Errors, err)
					h.logger.Error("error getting signed identity", "error", err)
					continue
				}

				req[task.Identity.Name] = consul.JWTLoginRequest{
					JWT:            jwt.JWT,
					AuthMethodName: h.authMethod,
				}
			}

			// handle alt identities
			for _, t := range task.Identities {
				ti := widmgr.TaskIdentity{
					TaskName:     task.Name,
					IdentityName: t.Name,
				}

				jwt, err := h.widmgr.Get(ti)
				if err != nil {
					mErr.Errors = append(mErr.Errors, err)
					h.logger.Error("error getting signed identity", "error", err)
					continue
				}

				req[t.Name] = consul.JWTLoginRequest{
					JWT:            jwt.JWT,
					AuthMethodName: h.authMethod,
				}
			}

			// Consul auth
			var err error
			tokens, err = h.client.DeriveSITokenWithJWT(req)
			if err != nil {
				h.logger.Error("error authenticating with Consul", "error", err)
				return err
			}

			// Write tokens to tasks' secret dirs
			secretsDir := h.allocdir.TaskDirs[task.Name].SecretsDir
			for identity, token := range tokens {
				tokenPath := filepath.Join(secretsDir, consulTokenFilePrefix+identity)
				if err := os.WriteFile(tokenPath, []byte(token), consulTokenFilePerms); err != nil {
					mErr.Errors = append(mErr.Errors, fmt.Errorf("failed to write Consul SI token: %w", err))
				}
			}
		}
	}

	// store the tokens in hookResources
	h.hookResources.SetConsulTokens(tokens)

	return mErr.ErrorOrNil()
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
