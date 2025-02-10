// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"sync"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	ti "github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/template"
	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	templateHookName = "template"
)

type templateHookConfig struct {
	// the allocation
	alloc *structs.Allocation

	// logger is used to log
	logger log.Logger

	// lifecycle is used to interact with the task's lifecycle
	lifecycle ti.TaskLifecycle

	// events is used to emit events
	events ti.EventEmitter

	// templates is the set of templates we are managing
	templates []*structs.Template

	// clientConfig is the Nomad Client configuration
	clientConfig *config.Config

	// envBuilder is the environment variable builder for the task.
	envBuilder *taskenv.Builder

	// consulNamespace is the current Consul namespace
	consulNamespace string

	// nomadNamespace is the job's Nomad namespace
	nomadNamespace string

	// renderOnTaskRestart is flag to explicitly render templates on task restart
	renderOnTaskRestart bool

	// hookResources are used to fetch Consul tokens
	hookResources *cstructs.AllocHookResources
}

type templateHook struct {
	config *templateHookConfig

	// logger is used to log
	logger log.Logger

	// templateManager is used to manage any consul-templates this task may have
	templateManager *template.TaskTemplateManager
	managerLock     sync.Mutex

	// consulNamespace is the current Consul namespace
	consulNamespace string

	// vaultToken is the current Vault token
	vaultToken string

	// vaultNamespace is the current Vault namespace
	vaultNamespace string

	// nomadToken is the current Nomad token
	nomadToken string

	// consulToken is the Consul ACL token obtained from consul_hook via
	// workload identity
	consulToken string

	// task is the task that defines these templates
	task *structs.Task

	// taskDir is the task directory
	taskDir string

	// taskID is a unique identifier for this templateHook, for use in
	// downstream platform-specific template runner consumers
	taskID string
}

func newTemplateHook(config *templateHookConfig) *templateHook {
	return &templateHook{
		config:          config,
		consulNamespace: config.consulNamespace,
		logger:          config.logger.Named(templateHookName),
	}
}

func (*templateHook) Name() string {
	return templateHookName
}

func (h *templateHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	h.managerLock.Lock()
	defer h.managerLock.Unlock()

	// If we have already run prerun before exit early.
	if h.templateManager != nil {
		if !h.config.renderOnTaskRestart {
			return nil
		}
		h.logger.Info("re-rendering templates on task restart")
		h.templateManager.Stop()
		h.templateManager = nil
	}

	// Store request information so they can be used in other hooks.
	h.task = req.Task
	h.taskDir = req.TaskDir.Dir
	h.vaultToken = req.VaultToken
	h.nomadToken = req.NomadToken
	h.taskID = req.Alloc.ID + "-" + req.Task.Name

	// Set the consul token if the task uses WI.
	tg := h.config.alloc.Job.LookupTaskGroup(h.config.alloc.TaskGroup)
	consulBlock := tg.Consul
	if req.Task.Consul != nil {
		consulBlock = req.Task.Consul
	}
	consulWIDName := consulBlock.IdentityName()

	// Check if task has an identity for Consul and assume WI flow if it does.
	// COMPAT simplify this logic and assume WI flow in 1.9+
	hasConsulIdentity := false
	for _, wid := range req.Task.Identities {
		if wid.Name == consulWIDName {
			hasConsulIdentity = true
			break
		}
	}
	if hasConsulIdentity {
		consulCluster := req.Task.GetConsulClusterName(tg)
		consulTokens := h.config.hookResources.GetConsulTokens()
		clusterTokens := consulTokens[consulCluster]

		if clusterTokens == nil {
			return fmt.Errorf(
				"consul tokens for cluster %s requested by task %s not found",
				consulCluster, req.Task.Name,
			)
		}

		consulToken := clusterTokens[consulWIDName+"/"+req.Task.Name]
		if consulToken == nil {
			return fmt.Errorf(
				"consul tokens for cluster %s and identity %s requested by task %s not found",
				consulCluster, consulWIDName, req.Task.Name,
			)
		}

		h.consulToken = consulToken.SecretID
	}

	// Set vault namespace if specified
	if req.Task.Vault != nil {
		h.vaultNamespace = req.Task.Vault.Namespace
	}

	unblockCh, err := h.newManager()
	if err != nil {
		return err
	}

	// Wait for the template to render
	select {
	case <-ctx.Done():
	case <-unblockCh:
	}

	return nil
}

func (h *templateHook) newManager() (unblock chan struct{}, err error) {
	unblock = make(chan struct{})

	vaultCluster := h.task.GetVaultClusterName()
	vaultConfig := h.config.clientConfig.GetVaultConfigs(h.logger)[vaultCluster]

	// Fail if task has a vault block but not client config was found.
	if h.task.Vault != nil && vaultConfig == nil {
		return nil, fmt.Errorf("Vault cluster %q is disabled or not configured", vaultCluster)
	}

	tg := h.config.alloc.Job.LookupTaskGroup(h.config.alloc.TaskGroup)
	consulCluster := h.task.GetConsulClusterName(tg)
	consulConfig := h.config.clientConfig.GetConsulConfigs(h.logger)[consulCluster]

	m, err := template.NewTaskTemplateManager(&template.TaskTemplateManagerConfig{
		UnblockCh:            unblock,
		Lifecycle:            h.config.lifecycle,
		Events:               h.config.events,
		Templates:            h.config.templates,
		ClientConfig:         h.config.clientConfig,
		ConsulNamespace:      h.config.consulNamespace,
		ConsulToken:          h.consulToken,
		ConsulConfig:         consulConfig,
		VaultToken:           h.vaultToken,
		VaultConfig:          vaultConfig,
		VaultNamespace:       h.vaultNamespace,
		TaskDir:              h.taskDir,
		EnvBuilder:           h.config.envBuilder,
		MaxTemplateEventRate: template.DefaultMaxTemplateEventRate,
		NomadNamespace:       h.config.nomadNamespace,
		NomadToken:           h.nomadToken,
		TaskID:               h.taskID,
		Logger:               h.logger,
	})
	if err != nil {
		h.logger.Error("failed to create template manager", "error", err)
		return nil, err
	}

	h.templateManager = m
	return unblock, nil
}

func (h *templateHook) Stop(_ context.Context, req *interfaces.TaskStopRequest, resp *interfaces.TaskStopResponse) error {
	h.managerLock.Lock()
	defer h.managerLock.Unlock()

	// Shutdown any created template
	if h.templateManager != nil {
		h.templateManager.Stop()
	}

	return nil
}

// Update is used to handle updates to vault and/or nomad tokens.
func (h *templateHook) Update(_ context.Context, req *interfaces.TaskUpdateRequest, resp *interfaces.TaskUpdateResponse) error {
	h.managerLock.Lock()
	defer h.managerLock.Unlock()

	// no template manager to manage
	if h.templateManager == nil {
		return nil
	}

	// neither vault or nomad token has been updated, nothing to do
	if req.VaultToken == h.vaultToken && req.NomadToken == h.nomadToken {
		return nil
	} else {
		h.vaultToken = req.VaultToken
		h.nomadToken = req.NomadToken
	}

	// shutdown the old template
	h.templateManager.Stop()
	h.templateManager = nil

	// create the new template
	if _, err := h.newManager(); err != nil {
		err = fmt.Errorf("failed to build template manager: %v", err)
		h.logger.Error("failed to build template manager", "error", err)
		_ = h.config.lifecycle.Kill(context.Background(),
			structs.NewTaskEvent(structs.TaskKilling).
				SetFailsTask().
				SetDisplayMessage(fmt.Sprintf("Template update %v", err)))
	}

	return nil
}
