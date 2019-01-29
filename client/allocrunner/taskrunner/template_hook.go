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
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/nomad/structs"
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
}

type templateHook struct {
	config *templateHookConfig

	// logger is used to log
	logger log.Logger

	// templateManager is used to manage any consul-templates this task may have
	templateManager *template.TaskTemplateManager
	managerLock     sync.Mutex

	// vaultToken is the current Vault token
	vaultToken string

	// taskDir is the task directory
	taskDir string
}

func newTemplateHook(config *templateHookConfig) *templateHook {
	h := &templateHook{
		config: config,
	}
	h.logger = config.logger.Named(h.Name())
	return h
}

func (*templateHook) Name() string {
	return "template"
}

func (h *templateHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	h.managerLock.Lock()
	defer h.managerLock.Unlock()

	// If we have already run prerun before exit early.
	if h.templateManager != nil {
		return nil
	}

	// Store the current Vault token and the task directory
	h.taskDir = req.TaskDir.Dir
	h.vaultToken = req.VaultToken
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
	m, err := template.NewTaskTemplateManager(&template.TaskTemplateManagerConfig{
		UnblockCh:            unblock,
		Lifecycle:            h.config.lifecycle,
		Events:               h.config.events,
		Templates:            h.config.templates,
		ClientConfig:         h.config.clientConfig,
		VaultToken:           h.vaultToken,
		TaskDir:              h.taskDir,
		EnvBuilder:           h.config.envBuilder,
		MaxTemplateEventRate: template.DefaultMaxTemplateEventRate,
	})
	if err != nil {
		h.logger.Error("failed to create template manager", "error", err)
		return nil, err
	}

	h.templateManager = m
	return unblock, nil
}

func (h *templateHook) Stop(ctx context.Context, req *interfaces.TaskStopRequest, resp *interfaces.TaskStopResponse) error {
	h.managerLock.Lock()
	defer h.managerLock.Unlock()

	// Shutdown any created template
	if h.templateManager != nil {
		h.templateManager.Stop()
	}

	return nil
}

// Handle new Vault token
func (h *templateHook) Update(ctx context.Context, req *interfaces.TaskUpdateRequest, resp *interfaces.TaskUpdateResponse) error {
	h.managerLock.Lock()
	defer h.managerLock.Unlock()

	// Nothing to do
	if h.templateManager == nil {
		return nil
	}

	// Check if the Vault token has changed
	if req.VaultToken == h.vaultToken {
		return nil
	} else {
		h.vaultToken = req.VaultToken
	}

	// Shutdown the old template
	h.templateManager.Stop()
	h.templateManager = nil

	// Create the new template
	if _, err := h.newManager(); err != nil {
		err := fmt.Errorf("failed to build template manager: %v", err)
		h.logger.Error("failed to build template manager", "error", err)
		h.config.lifecycle.Kill(context.Background(),
			structs.NewTaskEvent(structs.TaskKilling).
				SetFailsTask().
				SetDisplayMessage(fmt.Sprintf("Template update %v", err)))
	}

	return nil
}
