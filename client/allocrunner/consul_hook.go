// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"context"
	"fmt"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/consul"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/client/widmgr"
	"github.com/hashicorp/nomad/nomad/structs"
)

type consulHook struct {
	alloc         *structs.Allocation
	widmgr        widmgr.IdentityManager
	client        consul.Client
	env           *taskenv.TaskEnv
	hookResources *cstructs.AllocHookResources
	authMethod    string

	logger log.Logger
}

func newConsulHook(logger log.Logger,
	alloc *structs.Allocation,
	widmgr widmgr.IdentityManager,
	client consul.Client,
	env *taskenv.TaskEnv,
	hookResources *cstructs.AllocHookResources,
	authMethod string,
) *consulHook {
	h := &consulHook{
		alloc:         alloc,
		widmgr:        widmgr,
		client:        client,
		env:           env,
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
		}
	}

	// store the tokens
	h.hookResources.SetConsulTokens(tokens)

	return mErr.ErrorOrNil()
}

func (h *consulHook) authWithConsul(req map[string]consul.JWTLoginRequest, task *structs.Task) (map[string]string, error) {

	return h.client.DeriveSITokenWithJWT(req)
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
