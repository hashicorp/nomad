// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	ti "github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/secrets"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/template"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/nomad/structs"
)

// SecretProvider currently only supports Vault and Nomad which use CT. Future
// work can modify this interface to include custom providers using a plugin
// interface.
type SecretProvider interface {
	// BuildTemplate should construct a template appropriate for that provider
	// and append it to the templateManager's templates.
	BuildTemplate() *structs.Template

	// Parse allows each provider implementation to parse its "response" object.
	Parse() (map[string]string, error)
}

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

func newSecretsHook(conf *secretsHookConfig, secrets []*structs.Secret) *secretsHook {
	return &secretsHook{
		logger:         conf.logger,
		lifecycle:      conf.lifecycle,
		events:         conf.events,
		clientConfig:   conf.clientConfig,
		envBuilder:     conf.envBuilder,
		nomadNamespace: conf.nomadNamespace,
		secrets:        secrets,
		// Future work will inject taskSecrets from the taskRunner, so that the taskrunner
		// can make these secrets available to other hooks.
		taskSecrets: make(map[string]string),
	}
}

func (h *secretsHook) Name() string {
	return "secrets"
}

func (h *secretsHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, _ *interfaces.TaskPrestartResponse) error {
	templates := []*structs.Template{}

	providers, err := h.buildSecretProviders(req.TaskDir.SecretsDir)
	if err != nil {
		return err
	}

	for _, p := range providers {
		templates = append(templates, p.BuildTemplate())
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
	defer tm.Stop()

	select {
	case <-ctx.Done():
		return nil
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

func (h *secretsHook) buildSecretProviders(secretDir string) ([]SecretProvider, error) {
	// Any configuration errors will be found when calling the secret providers constructor,
	// so use a multierror to collect all errors and return them to the user at the same time.
	providers, mErr := []SecretProvider{}, new(multierror.Error)

	for idx, s := range h.secrets {
		tmplPath := filepath.Join(secretDir, fmt.Sprintf("temp-%d", idx))
		switch s.Provider {
		case "nomad":
			if p, err := secrets.NewNomadProvider(s, tmplPath, h.nomadNamespace); err != nil {
				multierror.Append(mErr, err)
			} else {
				providers = append(providers, p)
			}
		case "vault":
			if p, err := secrets.NewVaultProvider(s, tmplPath); err != nil {
				multierror.Append(mErr, err)
			} else {
				providers = append(providers, p)
			}
		default:
			multierror.Append(mErr, fmt.Errorf("unknown secret provider type: %s", s.Provider))
		}
	}

	return providers, mErr.ErrorOrNil()
}
