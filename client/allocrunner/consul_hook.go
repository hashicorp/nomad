// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"fmt"
	"os"
	"path/filepath"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/allocdir"
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

	// consulServicesAuthMethodName is the JWT auth method name that has to be
	// configured in Consul in order to authenticate Nomad services.
	consulServicesAuthMethodName = "nomad-workloads"

	// consulTemplatesAuthMethodName the JWT auth method name that has to be
	// configured in Consul in order to authenticate Nomad templates.
	consulTemplatesAuthMethodName = "nomad-templates"
)

type consulHook struct {
	alloc         *structs.Allocation
	allocdir      *allocdir.AllocDir
	widmgr        widmgr.IdentityManager
	consulConfigs map[string]*structsc.ConsulConfig
	hookResources *cstructs.AllocHookResources

	logger log.Logger
}

func newConsulHook(logger log.Logger, alloc *structs.Allocation,
	allocdir *allocdir.AllocDir,
	widmgr widmgr.IdentityManager,
	consulConfigs map[string]*structsc.ConsulConfig,
	hookResources *cstructs.AllocHookResources,
) *consulHook {
	h := &consulHook{
		alloc:         alloc,
		allocdir:      allocdir,
		widmgr:        widmgr,
		consulConfigs: consulConfigs,
		hookResources: hookResources,
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
		for _, service := range tg.Services {
			if err := h.prepareConsulTokens(service, clients); err != nil {
				mErr.Errors = append(mErr.Errors, err)
				continue
			}
		}
		for _, task := range tg.Tasks {
			for _, service := range task.Services {
				if err := h.prepareConsulTokens(service, clients); err != nil {
					mErr.Errors = append(mErr.Errors, err)
					continue
				}
			}
		}
	}

	// store the tokens in hookResources
	h.hookResources.SetConsulTokens(tokens)

	return mErr.ErrorOrNil()
}

func (h *consulHook) prepareConsulTokens(service *structs.Service, clients map[string]consul.Client) error {
	req, err := h.prepareConsulClientReq(service)
	if err != nil {
		return err
	}

	// in case no service needs a consul token
	if len(req) == 0 {
		return nil
	}

	mErr := multierror.Error{}

	// Consul auth
	for consulName, client := range clients {
		tokens, err := client.DeriveSITokenWithJWT(req)
		if err != nil {
			h.logger.Error("error authenticating with Consul", "error", err)
			return err
		}

		// Write tokens to tasks' secret dirs
		secretsDir := h.allocdir.TaskDirs[service.TaskName].SecretsDir
		for identity, token := range tokens {
			filename := fmt.Sprintf("%s_%s_%s", consulTokenFilePrefix, consulName, identity)
			tokenPath := filepath.Join(secretsDir, filename)
			if err := os.WriteFile(tokenPath, []byte(token), consulTokenFilePerms); err != nil {
				mErr.Errors = append(mErr.Errors, fmt.Errorf("failed to write Consul SI token: %w", err))
			}
		}
	}

	return mErr.ErrorOrNil()
}

func (h *consulHook) prepareConsulClientReq(service *structs.Service) (map[string]consul.JWTLoginRequest, error) {
	req := map[string]consul.JWTLoginRequest{}

	// see if maybe we can quit early
	if service == nil || !service.IsConsul() {
		return req, nil
	}

	if service.Identity == nil {
		return req, nil
	}

	ti := widmgr.TaskIdentity{
		TaskName:     service.TaskName,
		IdentityName: service.Identity.Name,
	}

	jwt, err := h.widmgr.Get(ti)
	if err != nil {
		h.logger.Error("error getting signed identity", "error", err)
		return req, err
	}

	req[service.Identity.Name] = consul.JWTLoginRequest{
		JWT:            jwt.JWT,
		AuthMethodName: consulServicesAuthMethodName,
	}

	return req, nil
}
