// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	hvm "github.com/hashicorp/nomad/client/hostvolumemanager"
	"github.com/hashicorp/nomad/helper"
)

func NewPluginsHostVolumeFingerprint(logger hclog.Logger) Fingerprint {
	return &DynamicHostVolumePluginFingerprint{
		logger: logger.Named("host_volume_plugins"),
	}
}

var _ ReloadableFingerprint = &DynamicHostVolumePluginFingerprint{}

type DynamicHostVolumePluginFingerprint struct {
	logger hclog.Logger
}

func (h *DynamicHostVolumePluginFingerprint) Reload() {
	// host volume plugins are re-detected on agent reload
}

func (h *DynamicHostVolumePluginFingerprint) Fingerprint(request *FingerprintRequest, response *FingerprintResponse) error {
	// always add "mkdir" plugin
	h.logger.Debug("detected plugin built-in",
		"plugin_id", hvm.HostVolumePluginMkdirID, "version", hvm.HostVolumePluginMkdirVersion)
	defer response.AddAttribute("plugins.host_volume.version."+hvm.HostVolumePluginMkdirID, hvm.HostVolumePluginMkdirVersion)
	response.Detected = true

	// this config value will be empty in -dev mode
	pluginDir := request.Config.HostVolumePluginDir
	if pluginDir == "" {
		return nil
	}

	plugins, err := GetHostVolumePluginVersions(h.logger, pluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			h.logger.Debug("plugin dir does not exist", "dir", pluginDir)
		} else {
			h.logger.Warn("error finding plugins", "dir", pluginDir, "error", err)
		}
		return nil // don't halt agent start
	}

	// if this was a reload, wipe what was there before
	for k := range request.Node.Attributes {
		if strings.HasPrefix(k, "plugins.host_volume.") {
			response.RemoveAttribute(k)
		}
	}

	// set the attribute(s)
	for plugin, version := range plugins {
		h.logger.Debug("detected plugin", "plugin_id", plugin, "version", version)
		response.AddAttribute("plugins.host_volume.version."+plugin, version)
	}

	return nil
}

func (h *DynamicHostVolumePluginFingerprint) Periodic() (bool, time.Duration) {
	return false, 0
}

// GetHostVolumePluginVersions finds all the executable files on disk
// that respond to a Version call (arg $1 = 'version' / env $OPERATION = 'version')
// The return map's keys are plugin IDs, and the values are version strings.
func GetHostVolumePluginVersions(log hclog.Logger, pluginDir string) (map[string]string, error) {
	files, err := helper.FindExecutableFiles(pluginDir)
	if err != nil {
		return nil, err
	}

	plugins := make(map[string]string)
	mut := sync.Mutex{}
	var wg sync.WaitGroup

	for file, fullPath := range files {
		wg.Add(1)
		go func(file, fullPath string) {
			defer wg.Done()
			// really should take way less than a second
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			log := log.With("plugin_id", file)

			p, err := hvm.NewHostVolumePluginExternal(log, file, fullPath, "")
			if err != nil {
				log.Warn("error getting plugin", "error", err)
				return
			}

			version, err := p.Version(ctx)
			if err != nil {
				log.Debug("failed to get version from plugin", "error", err)
				return
			}

			mut.Lock()
			plugins[file] = version.String()
			mut.Unlock()
		}(file, fullPath)
	}

	wg.Wait()
	return plugins, nil
}
