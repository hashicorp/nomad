// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package commonplugins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

// SecretsPluginOption is a functional option for configuring an externalSecretsPlugin
type SecretsPluginOption func(*externalSecretsPlugin)

// WithTimeout sets a custom timeout for plugin execution.
// If not specified or set to 0, defaults to SecretsCmdTimeout (10 seconds).
func WithTimeout(timeout time.Duration) SecretsPluginOption {
	return func(p *externalSecretsPlugin) {
		if timeout > 0 {
			p.timeout = timeout
		}
	}
}

type SecretsPlugin interface {
	CommonPlugin
	Fetch(ctx context.Context, path string, env map[string]string) (*SecretResponse, error)
}

type SecretResponse struct {
	Result map[string]string `json:"result"`
	Error  *string           `json:"error"`
}

type externalSecretsPlugin struct {
	logger log.Logger

	// pluginPath is the path on the host to the plugin executable
	pluginPath string

	// timeout is the duration after which the plugin command is sent SIGTERM
	timeout time.Duration
}

// NewExternalSecretsPlugin creates an instance of a secrets plugin by validating the plugin
// binary exists and is executable, and parsing any string key/value pairs out of the config
// which will be used as environment variables for Fetch.
func NewExternalSecretsPlugin(commonPluginDir string, name string, opts ...SecretsPluginOption) (*externalSecretsPlugin, error) {
	// validate plugin
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
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

	// Create plugin with default timeout
	plugin := &externalSecretsPlugin{
		pluginPath: executable,
		timeout:    SecretsCmdTimeout,
	}

	// Apply options
	for _, opt := range opts {
		opt(plugin)
	}

	return plugin, nil
}

func (e *externalSecretsPlugin) Fingerprint(ctx context.Context) (*PluginFingerprint, error) {
	plugCtx, cancel := context.WithTimeout(ctx, e.timeout)
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

func (e *externalSecretsPlugin) Fetch(ctx context.Context, path string, env map[string]string) (*SecretResponse, error) {
	plugCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	cmd := exec.CommandContext(plugCtx, e.pluginPath, "fetch", path)
	cmd.Env = []string{
		"CPI_OPERATION=fetch",
	}

	for envKey, val := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", envKey, val))
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
