// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"fmt"

	consulapi "github.com/hashicorp/consul/api"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/consul"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/widmgr"
	"github.com/hashicorp/nomad/nomad/structs"
	structsc "github.com/hashicorp/nomad/nomad/structs/config"
)

type consulHook struct {
	alloc                   *structs.Allocation
	allocdir                *allocdir.AllocDir
	widmgr                  widmgr.IdentityManager
	consulConfigs           map[string]*structsc.ConsulConfig
	consulClientConstructor func(*structsc.ConsulConfig, log.Logger) (consul.Client, error)
	hookResources           *cstructs.AllocHookResources

	logger log.Logger
}

type consulHookConfig struct {
	alloc    *structs.Allocation
	allocdir *allocdir.AllocDir
	widmgr   widmgr.IdentityManager

	// consulConfigs is a map of cluster names to Consul configs
	consulConfigs map[string]*structsc.ConsulConfig
	// consulClientConstructor injects the function that will return a consul
	// client (eases testing)
	consulClientConstructor func(*structsc.ConsulConfig, log.Logger) (consul.Client, error)

	// hookResources is used for storing and retrieving Consul tokens
	hookResources *cstructs.AllocHookResources

	logger log.Logger
}

func newConsulHook(cfg consulHookConfig) *consulHook {
	h := &consulHook{
		alloc:                   cfg.alloc,
		allocdir:                cfg.allocdir,
		widmgr:                  cfg.widmgr,
		consulConfigs:           cfg.consulConfigs,
		consulClientConstructor: cfg.consulClientConstructor,
		hookResources:           cfg.hookResources,
	}
	h.logger = cfg.logger.Named(h.Name())
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
	tokens := map[string]map[string]*consulapi.ACLToken{}

	tg := job.LookupTaskGroup(h.alloc.TaskGroup)
	if tg == nil { // this is always a programming error
		return fmt.Errorf("alloc %v does not have a valid task group", h.alloc.Name)
	}

	if err := h.prepareConsulTokensForServices(tg.Services, tg, tokens); err != nil {
		mErr.Errors = append(mErr.Errors, err)
	}
	for _, task := range tg.Tasks {
		if err := h.prepareConsulTokensForServices(task.Services, tg, tokens); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
		if err := h.prepareConsulTokensForTask(task, tg, tokens); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
	}

	err := mErr.ErrorOrNil()
	if err != nil {
		multierror.Flatten(err)
		h.revokeTokens(tokens)
		return err
	}

	// write the tokens to hookResources
	h.hookResources.SetConsulTokens(tokens)

	return nil
}

func (h *consulHook) prepareConsulTokensForTask(task *structs.Task, tg *structs.TaskGroup, tokens map[string]map[string]*consulapi.ACLToken) error {
	if task == nil {
		// programming error
		return fmt.Errorf("cannot prepare consul tokens, no task specified")
	}

	clusterName := task.GetConsulClusterName(tg)
	consulConfig, ok := h.consulConfigs[clusterName]
	if !ok {
		return fmt.Errorf("no such consul cluster: %s", clusterName)
	}

	// get tokens for alt identities for Consul
	mErr := multierror.Error{}
	for _, i := range task.Identities {
		if i.Name != fmt.Sprintf("%s_%s", structs.ConsulTaskIdentityNamePrefix, consulConfig.Name) {
			continue
		}

		ti := *task.IdentityHandle(i)
		jwt, err := h.widmgr.Get(ti)
		if err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf(
				"error getting signed identity for task %s: %v",
				task.Name, err,
			))
			continue
		}

		req := map[string]consul.JWTLoginRequest{}
		req[ti.IdentityName] = consul.JWTLoginRequest{
			JWT:            jwt.JWT,
			AuthMethodName: consulConfig.TaskIdentityAuthMethod,
		}

		if err := h.getConsulTokens(consulConfig.Name, ti.IdentityName, tokens, req); err != nil {
			return err
		}
	}

	return mErr.ErrorOrNil()
}

func (h *consulHook) prepareConsulTokensForServices(services []*structs.Service, tg *structs.TaskGroup, tokens map[string]map[string]*consulapi.ACLToken) error {
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

		clusterName := service.GetConsulClusterName(tg)
		consulConfig, ok := h.consulConfigs[clusterName]
		if !ok {
			return fmt.Errorf("no such consul cluster: %s", clusterName)
		}

		req := map[string]consul.JWTLoginRequest{}
		identity := *service.IdentityHandle()
		jwt, err := h.widmgr.Get(identity)
		if err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf(
				"error getting signed identity for service %s: %v",
				service.Name, err,
			))
			continue
		}

		req[identity.IdentityName] = consul.JWTLoginRequest{
			JWT:            jwt.JWT,
			AuthMethodName: consulConfig.ServiceIdentityAuthMethod,
		}
		if err != nil {
			mErr.Errors = append(mErr.Errors, err)
			continue
		}

		// in case no service needs a consul token
		if len(req) == 0 {
			continue
		}

		if err := h.getConsulTokens(clusterName, service.Identity.Name, tokens, req); err != nil {
			mErr.Errors = append(mErr.Errors, err)
			continue
		}
	}

	return mErr.ErrorOrNil()
}

func (h *consulHook) getConsulTokens(cluster, identityName string, tokens map[string]map[string]*consulapi.ACLToken, req map[string]consul.JWTLoginRequest) error {
	client, err := h.clientForCluster(cluster)
	if err != nil {
		return err
	}

	// get consul acl tokens
	t, err := client.DeriveTokenWithJWT(req)
	if err != nil {
		return err
	}
	if tokens[cluster] == nil {
		tokens[cluster] = map[string]*consulapi.ACLToken{}
	}
	tokens[cluster][identityName] = t[identityName]

	return nil
}

func (h *consulHook) clientForCluster(cluster string) (consul.Client, error) {
	consulConf, ok := h.consulConfigs[cluster]
	if !ok {
		return nil, fmt.Errorf("unable to find configuration for consul cluster %v", cluster)
	}

	return h.consulClientConstructor(consulConf, h.logger)
}

// Postrun cleans up the Consul tokens after the tasks have exited.
func (h *consulHook) Postrun() error {
	tokens := h.hookResources.GetConsulTokens()
	err := h.revokeTokens(tokens)
	if err != nil {
		return err
	}
	h.hookResources.SetConsulTokens(tokens)
	return nil
}

// Destroy cleans up any remaining Consul tokens if the alloc is GC'd or fails
// to restore after a client restart.
func (h *consulHook) Destroy() error {
	tokens := h.hookResources.GetConsulTokens()
	err := h.revokeTokens(tokens)
	if err != nil {
		return err
	}
	h.hookResources.SetConsulTokens(tokens)
	return nil
}

func (h *consulHook) revokeTokens(tokens map[string]map[string]*consulapi.ACLToken) error {
	mErr := multierror.Error{}

	for cluster, tokensForCluster := range tokens {
		if tokensForCluster == nil {
			// if called by Destroy, may have been removed by Postrun
			continue
		}
		client, err := h.clientForCluster(cluster)
		if err != nil {
			mErr.Errors = append(mErr.Errors, err)
			continue
		}
		toRevoke := []*consulapi.ACLToken{}
		for _, token := range tokensForCluster {
			toRevoke = append(toRevoke, token)
		}
		err = client.RevokeTokens(toRevoke)
		if err != nil {
			mErr.Errors = append(mErr.Errors, err)
			continue
		}
		tokens[cluster] = nil
	}

	return mErr.ErrorOrNil()
}
