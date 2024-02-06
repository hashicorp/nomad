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
	allocdir                allocdir.Interface
	widmgr                  widmgr.IdentityManager
	consulConfigs           map[string]*structsc.ConsulConfig
	consulClientConstructor func(*structsc.ConsulConfig, log.Logger) (consul.Client, error)
	hookResources           *cstructs.AllocHookResources

	logger log.Logger
}

type consulHookConfig struct {
	alloc    *structs.Allocation
	allocdir allocdir.Interface
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

	// tokens are a map of Consul cluster to identity name to Consul ACL token.
	tokens := map[string]map[string]*consulapi.ACLToken{}

	tg := job.LookupTaskGroup(h.alloc.TaskGroup)
	if tg == nil { // this is always a programming error
		return fmt.Errorf("alloc %v does not have a valid task group", h.alloc.Name)
	}

	var mErr *multierror.Error
	if err := h.prepareConsulTokensForServices(tg.Services, tg, tokens); err != nil {
		mErr = multierror.Append(mErr, err)
	}
	for _, task := range tg.Tasks {
		if err := h.prepareConsulTokensForServices(task.Services, tg, tokens); err != nil {
			mErr = multierror.Append(mErr, err)
		}
		if err := h.prepareConsulTokensForTask(task, tg, tokens); err != nil {
			mErr = multierror.Append(mErr, err)
		}
	}

	if err := mErr.ErrorOrNil(); err != nil {
		revokeErr := h.revokeTokens(tokens)
		mErr = multierror.Append(mErr, revokeErr)
		return mErr.ErrorOrNil()
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

	// Find task workload identity for Consul.
	widName := fmt.Sprintf("%s_%s", structs.ConsulTaskIdentityNamePrefix, consulConfig.Name)
	wid := task.GetIdentity(widName)
	if wid == nil {
		// Skip task if it doesn't have an identity for Consul since it doesn't
		// need a token.
		return nil
	}

	// Find signed workload identity.
	ti := *task.IdentityHandle(wid)
	jwt, err := h.widmgr.Get(ti)
	if err != nil {
		return fmt.Errorf("error getting signed identity for task %s: %v", task.Name, err)
	}

	// Derive token for task.
	req := consul.JWTLoginRequest{
		JWT:            jwt.JWT,
		AuthMethodName: consulConfig.TaskIdentityAuthMethod,
		Meta: map[string]string{
			"requested_by": fmt.Sprintf("nomad_task_%s", task.Name),
		},
	}
	token, err := h.getConsulToken(consulConfig.Name, req)
	if err != nil {
		return fmt.Errorf("failed to derive Consul token for task %s: %v", task.Name, err)
	}

	// Store token in results.
	if _, ok = tokens[clusterName]; !ok {
		tokens[clusterName] = make(map[string]*consulapi.ACLToken)
	}
	tokens[clusterName][widName] = token

	return nil
}

func (h *consulHook) prepareConsulTokensForServices(services []*structs.Service, tg *structs.TaskGroup, tokens map[string]map[string]*consulapi.ACLToken) error {
	if len(services) == 0 {
		return nil
	}

	var mErr *multierror.Error
	for _, service := range services {
		// Exit early if service doesn't need a Consul token.
		if service == nil || !service.IsConsul() || service.Identity == nil {
			continue
		}

		clusterName := service.GetConsulClusterName(tg)
		consulConfig, ok := h.consulConfigs[clusterName]
		if !ok {
			return fmt.Errorf("no such consul cluster: %s", clusterName)
		}

		// Find signed identity workload.
		identity := *service.IdentityHandle()
		jwt, err := h.widmgr.Get(identity)
		if err != nil {
			mErr = multierror.Append(mErr, fmt.Errorf(
				"error getting signed identity for service %s: %v",
				service.Name, err,
			))
			continue
		}

		// Derive token for service.
		req := consul.JWTLoginRequest{
			JWT:            jwt.JWT,
			AuthMethodName: consulConfig.ServiceIdentityAuthMethod,
			Meta: map[string]string{
				"requested_by": fmt.Sprintf("nomad_service_%s", identity.WorkloadIdentifier),
			},
		}
		token, err := h.getConsulToken(clusterName, req)
		if err != nil {
			mErr = multierror.Append(mErr, fmt.Errorf(
				"failed to derive Consul token for service %s: %v",
				service.Name, err,
			))
			continue
		}

		// Store token in results.
		if _, ok = tokens[clusterName]; !ok {
			tokens[clusterName] = make(map[string]*consulapi.ACLToken)
		}
		tokens[clusterName][service.Identity.Name] = token
	}

	return mErr.ErrorOrNil()
}

func (h *consulHook) getConsulToken(cluster string, req consul.JWTLoginRequest) (*consulapi.ACLToken, error) {
	client, err := h.clientForCluster(cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve Consul client for cluster %s: %v", cluster, err)
	}

	return client.DeriveTokenWithJWT(req)
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
