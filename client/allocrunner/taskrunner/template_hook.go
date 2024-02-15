// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	ti "github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/template"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	templateHookName = "template"
)

type templateHookConfig struct {
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
}

var (
	callsPrestart    int32
	callsPoststart   int32
	callsUpdate      int32
	callsStop        int32
	callsNewManager  int32
	callsStopManager int32
)

type templateHook struct {
	config *templateHookConfig

	// logger is used to log
	logger log.Logger

	// templateManager is used to manage any consul-templates this task may have
	templateManager *template.TaskTemplateManager
	managerLock     sync.Mutex

	// driverHandle is the task driver executor used by the template manager to
	// run scripts when the template change mode is set to script.
	//
	// Must obtain a managerLock before changing. It may be nil.
	driverHandle ti.ScriptExecutor

	// consulNamespace is the current Consul namespace
	consulNamespace string

	// vaultToken is the current Vault token
	vaultToken string

	// vaultNamespace is the current Vault namespace
	vaultNamespace string

	// nomadToken is the current Nomad token
	nomadToken string

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

	prestartVal := atomic.AddInt32(&callsPrestart, 1)
	fmt.Println("templateHook.Prestart()", "taskID", h.taskID, "prestart_calls", prestartVal)

	// If we have already run prerun before exit early.
	if h.templateManager != nil {
		if !h.config.renderOnTaskRestart {
			fmt.Println("templateHook.Prestart()", "no render on task restart")
			return nil
		}
		h.logger.Info("re-rendering templates on task restart")

		stopVal := atomic.AddInt32(&callsStopManager, 1)
		fmt.Println("templateHook.Prestart()", "do rerender on task restart, stopping old template manager", "calls_stopmanager", stopVal)
		h.templateManager.Stop()
		h.templateManager = nil
	}

	// Store the current Vault token and the task directory
	h.taskDir = req.TaskDir.Dir
	h.vaultToken = req.VaultToken
	h.nomadToken = req.NomadToken
	h.taskID = req.Alloc.ID + "-" + req.Task.Name

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
		fmt.Println("templateHook.Prestart()", "template render cancelled")
	case <-unblockCh:
		fmt.Println("templateHook.Prestart()", "template render done")
	}

	return nil
}

func (h *templateHook) Poststart(ctx context.Context, req *interfaces.TaskPoststartRequest, resp *interfaces.TaskPoststartResponse) error {
	h.managerLock.Lock()
	defer h.managerLock.Unlock()

	poststartVal := atomic.AddInt32(&callsPoststart, 1)
	fmt.Println("templateHook.Poststart", "taskID", h.taskID, "calls_poststart", poststartVal)

	if h.templateManager == nil {
		return nil
	}

	if req.DriverExec != nil {
		h.driverHandle = req.DriverExec
		h.templateManager.SetDriverHandle(h.driverHandle)
	} else {
		for _, tmpl := range h.config.templates {
			if tmpl.ChangeMode == structs.TemplateChangeModeScript {
				return fmt.Errorf("template has change mode set to 'script' but the driver it uses does not provide exec capability")
			}
		}
	}
	return nil
}

func (h *templateHook) newManager() (unblock chan struct{}, err error) {
	newManagerVal := atomic.AddInt32(&callsNewManager, 1)
	fmt.Println("templateHook.newManager()", "taskID", h.taskID, "calls_newmanager", newManagerVal)

	unblock = make(chan struct{})
	m, err := template.NewTaskTemplateManager(&template.TaskTemplateManagerConfig{
		UnblockCh:            unblock,
		Lifecycle:            h.config.lifecycle,
		Events:               h.config.events,
		Templates:            h.config.templates,
		ClientConfig:         h.config.clientConfig,
		ConsulNamespace:      h.config.consulNamespace,
		VaultToken:           h.vaultToken,
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
	if h.driverHandle != nil {
		h.templateManager.SetDriverHandle(h.driverHandle)
	}
	return unblock, nil
}

func (h *templateHook) Stop(ctx context.Context, req *interfaces.TaskStopRequest, resp *interfaces.TaskStopResponse) error {

	h.managerLock.Lock()
	defer h.managerLock.Unlock()

	stopVal := atomic.AddInt32(&callsStop, 1)
	fmt.Println("templateHook.Stop()", "enter", "taskID", h.taskID, "calls_stop", stopVal)

	// Shutdown any created template
	if h.templateManager != nil {
		stopManVal := atomic.AddInt32(&callsStopManager, 1)
		fmt.Println("templateHook.Stop()", "stopping template manager", "calls_stopmanager", stopManVal)
		h.templateManager.Stop()
	} else {
		fmt.Println("templateHook.Stop()", "there is a nil templateManager")
	}

	fmt.Println("templateHook.Stop()", "exit", "taskID", h.taskID)
	return nil
}

// Update is used to handle updates to vault and/or nomad tokens.
func (h *templateHook) Update(ctx context.Context, req *interfaces.TaskUpdateRequest, resp *interfaces.TaskUpdateResponse) error {

	h.managerLock.Lock()
	defer h.managerLock.Unlock()

	updateVal := atomic.AddInt32(&callsUpdate, 1)
	fmt.Println("templateHook.Update()", "enter", "taskID", h.taskID, "calls_update", updateVal)

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
	stopManVal := atomic.AddInt32(&callsStopManager, 1)
	fmt.Println("templateHook.Update()", "stopping template manager", "calls_stopmanager", stopManVal)
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
