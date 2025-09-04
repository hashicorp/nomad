// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	consulapi "github.com/hashicorp/consul/api"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/consul"
	cstate "github.com/hashicorp/nomad/client/state"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/client/widmgr"
	"github.com/hashicorp/nomad/nomad/structs"
	structsc "github.com/hashicorp/nomad/nomad/structs/config"
)

type consulHook struct {
	alloc                   *structs.Allocation
	allocdir                allocdir.Interface
	widmgr                  widmgr.IdentityManager
	consulConfigs           map[string]*structsc.ConsulConfig
	consulClientConstructor consul.ConsulClientFunc
	resourcesBackend        *resourcesBackend

	logger           log.Logger
	shutdownCtx      context.Context
	shutdownCancelFn context.CancelFunc
}

type consulHookConfig struct {
	alloc    *structs.Allocation
	allocdir allocdir.Interface
	widmgr   widmgr.IdentityManager
	db       cstate.StateDB

	// consulConfigs is a map of cluster names to Consul configs
	consulConfigs map[string]*structsc.ConsulConfig
	// consulClientConstructor injects the function that will return a consul
	// client (eases testing)
	consulClientConstructor consul.ConsulClientFunc

	// hookResources is used for storing and retrieving Consul tokens
	hookResources *cstructs.AllocHookResources

	logger log.Logger
}

func newConsulHook(cfg consulHookConfig) *consulHook {
	shutdownCtx, shutdownCancelFn := context.WithCancel(context.Background())
	h := &consulHook{
		alloc:                   cfg.alloc,
		allocdir:                cfg.allocdir,
		widmgr:                  cfg.widmgr,
		consulConfigs:           cfg.consulConfigs,
		consulClientConstructor: cfg.consulClientConstructor,
		resourcesBackend:        newResourcesBackend(cfg.alloc.ID, cfg.hookResources, cfg.db),
		shutdownCtx:             shutdownCtx,
		shutdownCancelFn:        shutdownCancelFn,
	}
	h.logger = cfg.logger.Named(h.Name())
	return h
}

// statically assert the hook implements the expected interfaces
var (
	_ interfaces.RunnerPrerunHook  = (*consulHook)(nil)
	_ interfaces.RunnerPostrunHook = (*consulHook)(nil)
	_ interfaces.RunnerDestroyHook = (*consulHook)(nil)
	_ interfaces.ShutdownHook      = (*consulHook)(nil)
)

func (*consulHook) Name() string {
	return "consul"
}

func (h *consulHook) Prerun(allocEnv *taskenv.TaskEnv) error {
	job := h.alloc.Job

	if job == nil {
		// this is always a programming error
		err := fmt.Errorf("alloc %v does not have a job", h.alloc.Name)
		h.logger.Error(err.Error())
		return err
	}

	// tokens are a map of Consul cluster to identity name to Consul ACL token.
	tokens, err := h.resourcesBackend.loadAllocTokens()
	if err != nil {
		h.logger.Error("error reading stored ACL tokens", "error", err)
	}

	tg := job.LookupTaskGroup(h.alloc.TaskGroup)
	if tg == nil { // this is always a programming error
		return fmt.Errorf("alloc %v does not have a valid task group", h.alloc.Name)
	}

	var mErr *multierror.Error
	if err := h.prepareConsulTokensForServices(tg.Services, tg, tokens, allocEnv); err != nil {
		mErr = multierror.Append(mErr, err)
	}
	for _, task := range tg.Tasks {
		taskEnv := allocEnv.WithTask(h.alloc, task)
		if err := h.prepareConsulTokensForServices(task.Services, tg, tokens, taskEnv); err != nil {
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
	if err := h.resourcesBackend.setConsulTokens(tokens); err != nil {
		h.logger.Error("unable to update tokens in state", "error", err)
	}

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

	tokenName := widName + "/" + task.Name
	token := tokens[clusterName][tokenName]

	// If no token was previously stored, create one.
	if token == nil {
		// Find signed workload identity.
		ti := *task.IdentityHandle(wid)
		swi, err := h.widmgr.Get(ti)
		if err != nil {
			return fmt.Errorf("error getting signed identity for task %s: %v", task.Name, err)
		}

		h.logger.Debug("logging into consul", "name", ti.IdentityName, "type", ti.WorkloadType)
		req := consul.JWTLoginRequest{
			JWT:            swi.JWT,
			AuthMethodName: consulConfig.TaskIdentityAuthMethod,
			Meta: map[string]string{
				"requested_by": fmt.Sprintf("nomad_task_%s", task.Name),
			},
		}

		token, err = h.getConsulToken(consulConfig.Name, req)
		if err != nil {
			return fmt.Errorf("failed to derive Consul token for task %s: %v", task.Name, err)
		}
	}

	// Store token in results.
	if _, ok = tokens[clusterName]; !ok {
		tokens[clusterName] = make(map[string]*consulapi.ACLToken)
	}

	tokens[clusterName][tokenName] = token

	return nil
}

func (h *consulHook) prepareConsulTokensForServices(services []*structs.Service, tg *structs.TaskGroup, tokens map[string]map[string]*consulapi.ACLToken, env *taskenv.TaskEnv) error {
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
		ti := *service.IdentityHandle(env.ReplaceEnv)
		tokenName := service.Identity.Name
		token := tokens[clusterName][tokenName]

		// If no token was previously stored, create one.
		if token == nil {
			swi, err := h.widmgr.Get(ti)
			if err != nil {
				mErr = multierror.Append(mErr, fmt.Errorf(
					"error getting signed identity for service %s: %v",
					service.Name, err,
				))
				continue
			}

			h.logger.Debug("logging into consul", "name", ti.IdentityName, "type", ti.WorkloadType)
			req := consul.JWTLoginRequest{
				JWT:            swi.JWT,
				AuthMethodName: consulConfig.ServiceIdentityAuthMethod,
				Meta: map[string]string{
					"requested_by": fmt.Sprintf("nomad_service_%s", ti.InterpolatedWorkloadIdentifier),
				},
			}

			token, err = h.getConsulToken(clusterName, req)
			if err != nil {
				mErr = multierror.Append(mErr, fmt.Errorf(
					"failed to derive Consul token for service %s: %v",
					service.Name, err,
				))
				continue
			}

		}

		// Store token in results.
		if _, ok = tokens[clusterName]; !ok {
			tokens[clusterName] = make(map[string]*consulapi.ACLToken)
		}

		tokens[clusterName][tokenName] = token
	}

	return mErr.ErrorOrNil()
}

func (h *consulHook) getConsulToken(cluster string, req consul.JWTLoginRequest) (*consulapi.ACLToken, error) {
	client, err := h.clientForCluster(cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve Consul client for cluster %s: %v", cluster, err)
	}

	t, err := client.DeriveTokenWithJWT(req)
	if err == nil {
		err = client.TokenPreflightCheck(h.shutdownCtx, t)
	}

	return t, err
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
	return h.Destroy()
}

// Shutdown will get called when the client is gracefully stopping.
func (h *consulHook) Shutdown() {
	h.shutdownCancelFn()
}

// Destroy cleans up any remaining Consul tokens if the alloc is GC'd or fails
// to restore after a client restart.
func (h *consulHook) Destroy() error {
	tokens := h.resourcesBackend.getConsulTokens()
	err := h.revokeTokens(tokens)
	if err != nil {
		return err
	}

	h.resourcesBackend.setConsulTokens(tokens)
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

type resourcesBackend struct {
	allocID       string
	hookResources *cstructs.AllocHookResources
	db            cstate.StateDB
}

func newResourcesBackend(allocID string, hr *cstructs.AllocHookResources, db cstate.StateDB) *resourcesBackend {
	return &resourcesBackend{
		allocID:       allocID,
		hookResources: hr,
		db:            db,
	}
}

func decodeACLToken(b64ACLToken string, token *consulapi.ACLToken) error {
	decodedBytes, err := base64.StdEncoding.DecodeString(b64ACLToken)
	if err != nil {
		return fmt.Errorf("unable to process ACLToken: %w", err)
	}

	if len(decodedBytes) != 0 {
		if err := json.Unmarshal(decodedBytes, token); err != nil {
			return fmt.Errorf("unable to unmarshal ACLToken: %w", err)
		}
	}

	return nil
}

func encodeACLToken(token *consulapi.ACLToken) (string, error) {
	jsonBytes, err := json.Marshal(token)
	if err != nil {
		return "", fmt.Errorf("unable to marshal ACL token: %w", err)
	}

	return base64.StdEncoding.EncodeToString(jsonBytes), nil
}

// This function will never return nil, even in case of error
func (rs *resourcesBackend) loadAllocTokens() (map[string]map[string]*consulapi.ACLToken, error) {
	allocTokens := map[string]map[string]*consulapi.ACLToken{}

	ts, err := rs.db.GetAllocConsulACLTokens(rs.allocID)
	if err != nil {
		return allocTokens, err
	}

	var mErr *multierror.Error
	for _, st := range ts {

		token := &consulapi.ACLToken{}
		err := decodeACLToken(st.ACLToken, token)
		if err != nil {
			mErr = multierror.Append(mErr, err)
			continue
		}

		if allocTokens[st.Cluster] == nil {
			allocTokens[st.Cluster] = map[string]*consulapi.ACLToken{}
		}

		allocTokens[st.Cluster][st.TokenID] = token
	}

	return allocTokens, mErr.ErrorOrNil()
}

func (rs *resourcesBackend) setConsulTokens(m map[string]map[string]*consulapi.ACLToken) error {
	rs.hookResources.SetConsulTokens(m)

	var mErr *multierror.Error
	ts := []*cstructs.ConsulACLToken{}
	for cCluster, tokens := range m {
		for tokenID, aclToken := range tokens {

			stringToken, err := encodeACLToken(aclToken)
			if err != nil {
				mErr = multierror.Append(mErr, err)
				continue
			}

			ts = append(ts, &cstructs.ConsulACLToken{
				Cluster:  cCluster,
				TokenID:  tokenID,
				ACLToken: stringToken,
			})
		}
	}

	return rs.db.PutAllocConsulACLTokens(rs.allocID, ts)
}

func (rs *resourcesBackend) getConsulTokens() map[string]map[string]*consulapi.ACLToken {
	return rs.hookResources.GetConsulTokens()
}
