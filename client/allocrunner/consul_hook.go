// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"fmt"

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
	// consulServicesAuthMethodName is the JWT auth method name that has to be
	// configured in Consul in order to authenticate Nomad services.
	consulServicesAuthMethodName = "nomad-workloads"

	// consulTasksAuthMethodName the JWT auth method name that has to be
	// configured in Consul in order to authenticate Nomad tasks (used by
	// templates).
	consulTasksAuthMethodName = "nomad-tasks"
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
		// this is always a programming error
		err := fmt.Errorf("alloc %v does not have a job", h.alloc.Name)
		h.logger.Error(err.Error())
		return err
	}

	mErr := multierror.Error{}

	// tokens are a map of Consul cluster to service identity name to Consul
	// ACL token
	tokens := map[string]map[string]string{}

	for _, tg := range job.TaskGroups {
		if err := h.prepareConsulTokensForServices(tg.Services, tokens); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
		for _, task := range tg.Tasks {
			if err := h.prepareConsulTokensForServices(task.Services, tokens); err != nil {
				mErr.Errors = append(mErr.Errors, err)
			}
			if err := h.prepareConsulTokensForTask(job, task, tg.Name, tokens); err != nil {
				mErr.Errors = append(mErr.Errors, err)
			}
		}
	}

	// write the tokens to hookResources
	h.hookResources.SetConsulTokens(tokens)

	return mErr.ErrorOrNil()
}

func (h *consulHook) prepareConsulTokensForTask(job *structs.Job, task *structs.Task, tgName string, tokens map[string]map[string]string) error {
	// if UseIdentity is unset of set to false, quit
	// FIXME Fetch from Task.Consul.Cluster once #18557 is in
	consulConfig := h.consulConfigs[structs.ConsulDefaultCluster]
	if consulConfig.UseIdentity == nil || !*consulConfig.UseIdentity {
		return nil
	}

	expectedIdentity := task.MakeUniqueIdentityName(tgName)

	// get tokens for alt identities for Consul
	mErr := multierror.Error{}
	for _, i := range task.Identities {
		if i.Name != expectedIdentity {
			continue
		}
		ti := widmgr.TaskIdentity{
			TaskName:     task.Name,
			IdentityName: i.Name,
		}

		req, err := h.prepareConsulClientReq(ti, consulTasksAuthMethodName)
		if err != nil {
			mErr.Errors = append(mErr.Errors, err)
			continue
		}

		jwt, err := h.widmgr.Get(ti)
		if err != nil {
			h.logger.Error("error getting signed identity", "error", err)
			mErr.Errors = append(mErr.Errors, err)
			continue
		}

		req[task.Identity.Name] = consul.JWTLoginRequest{
			JWT:            jwt.JWT,
			AuthMethodName: consulTasksAuthMethodName,
		}

		// FIXME Fetch from Task.Consul.Cluster once #18557 is in
		if err := h.getConsulTokens(structs.ConsulDefaultCluster, ti.IdentityName, tokens, req); err != nil {
			return err
		}
	}

	return mErr.ErrorOrNil()
}

func (h *consulHook) prepareConsulTokensForServices(services []*structs.Service, tokens map[string]map[string]string) error {
	if len(services) == 0 {
		return nil
	}

	mErr := multierror.Error{}
	for _, service := range services {
		// see if maybe we can quit early
		if service == nil || !service.IsConsul() {
			continue
		}
		if service.Identity == nil {
			continue
		}

		ti := widmgr.TaskIdentity{
			TaskName:     service.TaskName,
			IdentityName: service.Identity.Name,
		}

		req, err := h.prepareConsulClientReq(ti, consulServicesAuthMethodName)
		if err != nil {
			mErr.Errors = append(mErr.Errors, err)
			continue
		}

		// in case no service needs a consul token
		if len(req) == 0 {
			continue
		}

		if err := h.getConsulTokens(service.Cluster, service.Identity.Name, tokens, req); err != nil {
			mErr.Errors = append(mErr.Errors, err)
			continue
		}
	}

	return mErr.ErrorOrNil()
}

func (h *consulHook) getConsulTokens(cluster, identityName string, tokens map[string]map[string]string, req map[string]consul.JWTLoginRequest) error {
	// Consul auth
	consulConf, ok := h.consulConfigs[cluster]
	if !ok {
		return fmt.Errorf("unable to find configuration for consul cluster %v", cluster)
	}

	client, err := consul.NewConsulClient(consulConf, h.logger)
	if err != nil {
		return err
	}

	// get consul acl tokens
	t, err := client.DeriveSITokenWithJWT(req)
	if err != nil {
		return err
	}
	if tokens[cluster] == nil {
		tokens[cluster] = map[string]string{}
	}
	tokens[cluster][identityName] = t[identityName]

	return nil
}

func (h *consulHook) prepareConsulClientReq(identity widmgr.TaskIdentity, authMethodName string) (map[string]consul.JWTLoginRequest, error) {
	req := map[string]consul.JWTLoginRequest{}

	jwt, err := h.widmgr.Get(identity)
	if err != nil {
		h.logger.Error("error getting signed identity", "error", err)
		return req, err
	}

	req[identity.IdentityName] = consul.JWTLoginRequest{
		JWT:            jwt.JWT,
		AuthMethodName: authMethodName,
	}

	return req, nil
}
