// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/consul-template/renderer"
	"github.com/hashicorp/go-envparse"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	ti "github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/secrets"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/template"
	"github.com/hashicorp/nomad/client/commonplugins"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/nomad/structs"
)

type TemplateProvider interface {
	BuildTemplate() *structs.Template
}

type PluginProvider interface {
	Fetch(context.Context) (map[string]string, error)
}

type TemplateProvider interface {
	SecretProvider
	BuildTemplate() *structs.Template
}

type PluginProvider interface {
	SecretProvider
	Fetch(context.Context) error
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
	}
}

func (h *secretsHook) Name() string {
	return "secrets"
}

func (h *secretsHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	tmplProvider, pluginProvider, err := h.buildSecretProviders(req.TaskDir.SecretsDir)
	if err != nil {
		return err
	}

	templates := []*structs.Template{}
	for _, p := range tmplProvider {
		templates = append(templates, p.BuildTemplate())
	}

	vaultCluster := req.Task.GetVaultClusterName()
	vaultConfig := h.clientConfig.GetVaultConfigs(h.logger)[vaultCluster]

	mu := &sync.Mutex{}
	contents := []byte{}
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

		// This RenderFunc is used to keep any secret data from being written to disk.
		RenderFunc: func(ri *renderer.RenderInput) (*renderer.RenderResult, error) {
			// This RenderFunc is called by a single goroutine synchronously, but we
			// lock the append in the event this behavior changes without us knowing.
			mu.Lock()
			defer mu.Unlock()
			contents = append(contents, ri.Contents...)
			return &renderer.RenderResult{
				DidRender:   true,
				WouldRender: true,
				Contents:    ri.Contents,
			}, nil
		},
	})
	if err != nil {
		return err
	}

	go tm.Run()

	// Safeguard against the template manager continuing to run.
	defer tm.Stop()

	select {
	case <-ctx.Done():
		return nil
	case <-unblock:
	}

	// Set secrets from templates
	m, err := envparse.Parse(bytes.NewBuffer(contents))
	if err != nil {
		return err
	}
	h.envBuilder.SetSecrets(m)

	// Set secrets from plugin providers
	for _, p := range pluginProvider {
		vars, err := p.Fetch(ctx)
		if err != nil {
			return err
		}
		h.envBuilder.SetSecrets(vars)
	}

	resp.Done = true
	return nil
}

func (h *secretsHook) buildSecretProviders(secretDir string) ([]TemplateProvider, []PluginProvider, error) {
	// Any configuration errors will be found when calling the secret providers constructor,
	// so use a multierror to collect all errors and return them to the user at the same time.
	tmplProvider, pluginProvider, mErr := []TemplateProvider{}, []PluginProvider{}, new(multierror.Error)

	for idx, s := range h.secrets {
		if s == nil {
			continue
		}

		tmplFile := fmt.Sprintf("temp-%d", idx)
		switch s.Provider {
		case "nomad":
			if p, err := secrets.NewNomadProvider(s, secretDir, tmplFile, h.nomadNamespace); err != nil {
				multierror.Append(mErr, err)
			} else {
				tmplProvider = append(tmplProvider, p)
			}
		case "vault":
			if p, err := secrets.NewVaultProvider(s, secretDir, tmplFile); err != nil {
				multierror.Append(mErr, err)
			} else {
				tmplProvider = append(tmplProvider, p)
			}
		default:
			plug, err := commonplugins.NewExternalSecretsPlugin(h.clientConfig.CommonPluginDir, s.Provider, s.Env)
			if err != nil {
				multierror.Append(mErr, err)
				continue
			}
			pluginProvider = append(pluginProvider, secrets.NewExternalPluginProvider(plug, s.Name, s.Path))
		}
	}

	return tmplProvider, pluginProvider, mErr.ErrorOrNil()
}
