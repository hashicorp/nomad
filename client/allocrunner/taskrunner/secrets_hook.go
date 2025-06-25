package taskrunner

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	ti "github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/secrets"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/template"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/nomad/structs"
)

type secretsHookConfig struct {
	// logger is used to log
	logger log.Logger

	// lifecycle is used to interact with the task's lifecycle
	lifecycle ti.TaskLifecycle

	// events is used to emit events
	events ti.EventEmitter

	// clientConfig is the Nomad Client configuration
	clientConfig *config.Config

	// envBuilder is the environment variable builder for the task.
	envBuilder *taskenv.Builder

	// nomadNamespace is the job's Nomad namespace
	nomadNamespace string
}

type secretsHook struct {
	// logger is used to log
	logger log.Logger

	// lifecycle is used to interact with the task's lifecycle
	lifecycle ti.TaskLifecycle

	// events is used to emit events
	events ti.EventEmitter

	// clientConfig is the Nomad Client configuration
	clientConfig *config.Config

	// envBuilder is the environment variable builder for the task
	envBuilder *taskenv.Builder

	// nomadNamespace is the job's Nomad namespace
	nomadNamespace string

	// secrets to be fetched and populated for interpolation
	secrets []*structs.Secret

	// taskrunner secrets map
	taskSecrets map[string]string
}

// Currently only support Vault and Nomad which use CT. Future work
// can modify this interface to include custom providers using
// a plugin interface.
type SecretProvider interface {
	// BuildTemplates should construct a template appropriate for
	// that provider and append it to the templateManager's templates.
	BuildTemplate() (*structs.Template, error)

	// Each provider implementation much be able to parse it's "response" object.
	Parse() (map[string]string, error)
}

func newSecretsHook(conf *secretsHookConfig, secrets []*structs.Secret) *secretsHook {
	return &secretsHook{
		logger:         conf.logger,
		lifecycle:      conf.lifecycle,
		events:         conf.events,
		clientConfig:   conf.clientConfig,
		envBuilder:     conf.envBuilder,
		nomadNamespace: conf.nomadNamespace,
		secrets:        secrets,
		taskSecrets:    make(map[string]string),
	}
}

func (h *secretsHook) Name() string {
	return "secrets"
}

func (h *secretsHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	providers, templates := []SecretProvider{}, []*structs.Template{}
	for idx, s := range h.secrets {
		tmplPath := filepath.Join(req.TaskDir.SecretsDir, fmt.Sprintf("%d", idx))
		switch s.Provider {
		case "nomad":
			providers = append(providers, secrets.NewNomadProvider(s, tmplPath))
		case "vault":
			// Unimplemented
		default:
			return fmt.Errorf("Unknown secret provider type: %s", s.Provider)
		}
	}

	for _, p := range providers {
		if t, err := p.BuildTemplate(); err != nil {
			return err
		} else {
			templates = append(templates, t)
		}
	}

	vaultCluster := req.Task.GetVaultClusterName()
	vaultConfig := h.clientConfig.GetVaultConfigs(h.logger)[vaultCluster]

	unblock := make(chan struct{})
	tm, err := template.NewTaskTemplateManager(&template.TaskTemplateManagerConfig{
		UnblockCh:            unblock,
		Lifecycle:            h.lifecycle,
		Events:               h.events,
		Templates:            templates,
		ClientConfig:         h.clientConfig,
		VaultToken:           req.VaultToken,
		VaultConfig:          vaultConfig,
		VaultNamespace:       req.Alloc.Job.VaultNamespace,
		TaskDir:              req.TaskDir.Dir,
		EnvBuilder:           h.envBuilder,
		MaxTemplateEventRate: template.DefaultMaxTemplateEventRate,
		NomadNamespace:       h.nomadNamespace,
		NomadToken:           req.NomadToken,
		TaskID:               req.Alloc.ID + "-" + req.Task.Name,
		Logger:               h.logger,
	})
	if err != nil {
		return err
	}

	// Run the template manager to render templates.
	go tm.Run()

	// Safeguard against the template manager continuing to run.
	// This should not happen because templates should all be in
	// once mode, resulting in the tm exiting after the first render.
	defer tm.Stop()

	select {
	case <-ctx.Done():
		tm.Stop()
	case <-unblock:
	}

	// parse and copy variables to taskSecrets
	for _, p := range providers {
		vars, err := p.Parse()
		if err != nil {
			return err
		}
		maps.Copy(h.taskSecrets, vars)
	}

	return nil
}
