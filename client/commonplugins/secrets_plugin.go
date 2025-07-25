// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package commonplugins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper"
)

const (
	SecretsPluginDir = "secrets"

	// The timeout for the plugin command before it is send SIGTERM
	SecretsCmdTimeout = 10 * time.Second

	// The timeout before the command is sent SIGKILL after being SIGTERM'd
	SecretsKillTimeout = 2 * time.Second
)

type SecretsPlugin interface {
	CommonPlugin
	Fetch(ctx context.Context, path string) (*SecretResponse, error)
}

type SecretResponse struct {
	Result map[string]string `json:"result"`
	Error  *string           `json:"error"`
}

type externalSecretsPlugin struct {
	logger log.Logger

	// pluginPath is the path on the host to the plugin executable
	pluginPath string
}

func NewExternalSecretsPlugin(commonPluginDir string, name string) (*externalSecretsPlugin, error) {
	executable := filepath.Join(commonPluginDir, SecretsPluginDir, name)
	f, err := os.Stat(executable)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %q", ErrPluginNotExists, name)
		}
		return nil, err
	}
	if !helper.IsExecutable(f) {
		return nil, fmt.Errorf("%w: %q", ErrPluginNotExecutable, name)
	}

	return &externalSecretsPlugin{
		pluginPath: executable,
	}, nil
}

func (e *externalSecretsPlugin) Fingerprint(ctx context.Context) (*PluginFingerprint, error) {
	plugCtx, cancel := context.WithTimeout(ctx, SecretsCmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(plugCtx, e.pluginPath, "fingerprint")
	cmd.Env = []string{
		"CPI_OPERATION=fingerprint",
	}

	stdout, stderr, err := runPlugin(cmd, SecretsKillTimeout)
	if err != nil {
		return nil, err
	}

	if len(stderr) > 0 {
		e.logger.Info("fingerprint command stderr output", "msg", string(stderr))
	}

	res := &PluginFingerprint{}
	if err := json.Unmarshal(stdout, &res); err != nil {
		return nil, err
	}

	return res, nil
}

func (e *externalSecretsPlugin) Fetch(ctx context.Context, path string) (*SecretResponse, error) {
	plugCtx, cancel := context.WithTimeout(ctx, SecretsCmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(plugCtx, e.pluginPath, "fetch", path)
	cmd.Env = []string{
		"CPI_OPERATION=fetch",
	}

	stdout, stderr, err := runPlugin(cmd, SecretsKillTimeout)
	if err != nil {
		return nil, err
	}

	if len(stderr) > 0 {
		e.logger.Info("fetch command stderr output", "msg", string(stderr))
	}

	res := &SecretResponse{}
	if err := json.Unmarshal(stdout, &res); err != nil {
		return nil, err
	}

	return res, nil
}
