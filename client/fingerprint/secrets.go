// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/commonplugins"
	"github.com/hashicorp/nomad/helper"
)

type SecretsPluginFingerprint struct {
	logger hclog.Logger
}

func NewPluginsSecretsFingerprint(logger hclog.Logger) Fingerprint {
	return &SecretsPluginFingerprint{
		logger: logger.Named("secrets_plugins"),
	}
}

func (s *SecretsPluginFingerprint) Fingerprint(request *FingerprintRequest, response *FingerprintResponse) error {
	// Add builtin secrets providers
	defer func() {
		response.AddAttribute("plugins.secrets.nomad.version", "1.0.0")
		response.AddAttribute("plugins.secrets.vault.version", "1.0.0")
	}()
	response.Detected = true

	pluginDir := request.Config.CommonPluginsDir
	if pluginDir == "" {
		return nil
	}

	secretsDir := filepath.Join(pluginDir, commonplugins.SecretsPluginDir)

	files, err := helper.FindExecutableFiles(secretsDir)
	if err != nil {
		if os.IsNotExist(err) {
			s.logger.Trace("secrets plugin dir does not exist", "dir", secretsDir)
		} else {
			s.logger.Warn("error finding secrets plugins", "dir", secretsDir, "error", err)
		}
		return nil // don't halt agent start
	}

	// map of plugin names to fingerprinted versions
	plugins := map[string]string{}
	for name := range files {
		plug, err := commonplugins.NewExternalSecretsPlugin(request.Config.CommonPluginsDir, name)
		if err != nil {
			return err
		}

		fprint, err := plug.Fingerprint(context.Background())
		if err != nil {
			s.logger.Error("secrets plugin failed fingerprint", "plugin", name, "error", err)
			continue
		}

		if fprint.Version == nil || fprint.Type == nil {
			continue
		}

		plugins[name] = fprint.Version.Original()
	}

	// if this was a reload, wipe what was there before
	for k := range request.Node.Attributes {
		if strings.HasPrefix(k, "plugins.secrets.") {
			response.RemoveAttribute(k)
		}
	}

	// set the attribute(s)
	for plugin, version := range plugins {
		s.logger.Debug("detected plugin", "plugin_id", plugin, "version", version)
		response.AddAttribute("plugins.secrets."+plugin+".version", version)
	}

	return nil
}

func (s *SecretsPluginFingerprint) Periodic() (bool, time.Duration) {
	return false, 0
}

func (s *SecretsPluginFingerprint) Reload() {
	// secrets plugins are re-detected on agent reload
}
