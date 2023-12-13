// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package loader

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	log "github.com/hashicorp/go-hclog"
	version "github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/stretchr/testify/require"
)

var (
	// supportedApiVersions is the set of api versions that the "client" can
	// support
	supportedApiVersions = map[string][]string{
		base.PluginTypeDevice: {device.ApiVersion010},
	}
)

// harness is used to build a temp directory and copy our own test executable
// into it, allowing the plugin loader to scan for plugins.
type harness struct {
	t      *testing.T
	tmpDir string
}

// newHarness returns a harness and copies our test binary to the temp directory
// with the passed plugin names.
func newHarness(t *testing.T, plugins []string) *harness {
	t.Helper()

	h := &harness{
		t: t,
	}

	// Build a temp directory
	h.tmpDir = t.TempDir()

	// Get our own executable path
	selfExe, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to get self executable path: %v", err)
	}

	exeSuffix := ""
	if runtime.GOOS == "windows" {
		exeSuffix = ".exe"
	}
	for _, p := range plugins {
		dest := filepath.Join(h.tmpDir, p) + exeSuffix
		if err := copyFile(selfExe, dest); err != nil {
			t.Fatalf("failed to copy file: %v", err)
		}
	}

	return h
}

// copyFile copies the src file to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}

	return os.Chmod(dst, 0777)
}

// pluginDir returns the plugin directory.
func (h *harness) pluginDir() string {
	return h.tmpDir
}

func TestPluginLoader_External(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Create two plugins
	plugins := []string{"mock-device", "mock-device-2"}
	pluginVersions := []string{"v0.0.1", "v0.0.2"}
	h := newHarness(t, plugins)

	logger := testlog.HCLogger(t)
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:            logger,
		PluginDir:         h.pluginDir(),
		SupportedVersions: supportedApiVersions,
		Configs: []*config.PluginConfig{
			{
				Name: plugins[0],
				Args: []string{"-plugin", "-name", plugins[0],
					"-type", base.PluginTypeDevice, "-version", pluginVersions[0],
					"-api-version", device.ApiVersion010},
			},
			{
				Name: plugins[1],
				Args: []string{"-plugin", "-name", plugins[1],
					"-type", base.PluginTypeDevice, "-version", pluginVersions[1],
					"-api-version", device.ApiVersion010, "-api-version", "v0.2.0"},
			},
		},
	}

	l, err := NewPluginLoader(lconfig)
	require.NoError(err)

	// Get the catalog and assert we have the two plugins
	c := l.Catalog()
	require.Len(c, 1)
	require.Contains(c, base.PluginTypeDevice)
	detected := c[base.PluginTypeDevice]
	require.Len(detected, 2)
	sort.Slice(detected, func(i, j int) bool { return detected[i].Name < detected[j].Name })

	expected := []*base.PluginInfoResponse{
		{
			Name:              plugins[0],
			Type:              base.PluginTypeDevice,
			PluginVersion:     pluginVersions[0],
			PluginApiVersions: []string{"v0.1.0"},
		},
		{
			Name:              plugins[1],
			Type:              base.PluginTypeDevice,
			PluginVersion:     pluginVersions[1],
			PluginApiVersions: []string{"v0.1.0", "v0.2.0"},
		},
	}
	require.EqualValues(expected, detected)
}

func TestPluginLoader_External_ApiVersions(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Create two plugins
	plugins := []string{"mock-device", "mock-device-2", "mock-device-3"}
	pluginVersions := []string{"v0.0.1", "v0.0.2"}
	h := newHarness(t, plugins)

	logger := testlog.HCLogger(t)
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:    logger,
		PluginDir: h.pluginDir(),
		SupportedVersions: map[string][]string{
			base.PluginTypeDevice: {"0.2.0", "0.2.1", "0.3.0"},
		},
		Configs: []*config.PluginConfig{
			{
				// No supporting version
				Name: plugins[0],
				Args: []string{"-plugin", "-name", plugins[0],
					"-type", base.PluginTypeDevice, "-version", pluginVersions[0],
					"-api-version", "v0.1.0"},
			},
			{
				// Pick highest matching
				Name: plugins[1],
				Args: []string{"-plugin", "-name", plugins[1],
					"-type", base.PluginTypeDevice, "-version", pluginVersions[1],
					"-api-version", "v0.1.0",
					"-api-version", "v0.2.0",
					"-api-version", "v0.2.1",
					"-api-version", "v0.2.2",
				},
			},
			{
				// Pick highest matching
				Name: plugins[2],
				Args: []string{"-plugin", "-name", plugins[2],
					"-type", base.PluginTypeDevice, "-version", pluginVersions[1],
					"-api-version", "v0.1.0",
					"-api-version", "v0.2.0",
					"-api-version", "v0.2.1",
					"-api-version", "v0.3.0",
				},
			},
		},
	}

	l, err := NewPluginLoader(lconfig)
	require.NoError(err)

	// Get the catalog and assert we have the two plugins
	c := l.Catalog()
	require.Len(c, 1)
	require.Contains(c, base.PluginTypeDevice)
	detected := c[base.PluginTypeDevice]
	require.Len(detected, 2)
	sort.Slice(detected, func(i, j int) bool { return detected[i].Name < detected[j].Name })

	expected := []*base.PluginInfoResponse{
		{
			Name:              plugins[1],
			Type:              base.PluginTypeDevice,
			PluginVersion:     pluginVersions[1],
			PluginApiVersions: []string{"v0.1.0", "v0.2.0", "v0.2.1", "v0.2.2"},
		},
		{
			Name:              plugins[2],
			Type:              base.PluginTypeDevice,
			PluginVersion:     pluginVersions[1],
			PluginApiVersions: []string{"v0.1.0", "v0.2.0", "v0.2.1", "v0.3.0"},
		},
	}
	require.EqualValues(expected, detected)

	// Test we chose the correct versions by dispensing and checking and then
	// reattaching and checking
	p1, err := l.Dispense(plugins[1], base.PluginTypeDevice, nil, logger)
	require.NoError(err)
	defer p1.Kill()
	require.Equal("v0.2.1", p1.ApiVersion())

	p2, err := l.Dispense(plugins[2], base.PluginTypeDevice, nil, logger)
	require.NoError(err)
	defer p2.Kill()
	require.Equal("v0.3.0", p2.ApiVersion())

	// Test reattach api versions
	rc1, ok := p1.ReattachConfig()
	require.True(ok)
	r1, err := l.Reattach(plugins[1], base.PluginTypeDriver, rc1)
	require.NoError(err)
	require.Equal("v0.2.1", r1.ApiVersion())

	rc2, ok := p2.ReattachConfig()
	require.True(ok)
	r2, err := l.Reattach(plugins[2], base.PluginTypeDriver, rc2)
	require.NoError(err)
	require.Equal("v0.3.0", r2.ApiVersion())
}

func TestPluginLoader_External_NoApiVersion(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Create two plugins
	plugins := []string{"mock-device"}
	pluginVersions := []string{"v0.0.1", "v0.0.2"}
	h := newHarness(t, plugins)

	logger := testlog.HCLogger(t)
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:            logger,
		PluginDir:         h.pluginDir(),
		SupportedVersions: supportedApiVersions,
		Configs: []*config.PluginConfig{
			{
				Name: plugins[0],
				Args: []string{"-plugin", "-name", plugins[0],
					"-type", base.PluginTypeDevice, "-version", pluginVersions[0]},
			},
		},
	}

	_, err := NewPluginLoader(lconfig)
	require.Error(err)
	require.Contains(err.Error(), "no compatible API versions")
}

func TestPluginLoader_External_Config(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Create two plugins
	plugins := []string{"mock-device", "mock-device-2"}
	pluginVersions := []string{"v0.0.1", "v0.0.2"}
	h := newHarness(t, plugins)

	logger := testlog.HCLogger(t)
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:            logger,
		PluginDir:         h.pluginDir(),
		SupportedVersions: supportedApiVersions,
		Configs: []*config.PluginConfig{
			{
				Name: plugins[0],
				Args: []string{"-plugin", "-name", plugins[0],
					"-type", base.PluginTypeDevice, "-version", pluginVersions[0], "-api-version", device.ApiVersion010},
				Config: map[string]interface{}{
					"foo": "1",
					"bar": "2",
				},
			},
			{
				Name: plugins[1],
				Args: []string{"-plugin", "-name", plugins[1],
					"-type", base.PluginTypeDevice, "-version", pluginVersions[1], "-api-version", device.ApiVersion010},
				Config: map[string]interface{}{
					"foo": "3",
					"bar": "4",
				},
			},
		},
	}

	l, err := NewPluginLoader(lconfig)
	require.NoError(err)

	// Get the catalog and assert we have the two plugins
	c := l.Catalog()
	require.Len(c, 1)
	require.Contains(c, base.PluginTypeDevice)
	detected := c[base.PluginTypeDevice]
	require.Len(detected, 2)
	sort.Slice(detected, func(i, j int) bool { return detected[i].Name < detected[j].Name })

	expected := []*base.PluginInfoResponse{
		{
			Name:              plugins[0],
			Type:              base.PluginTypeDevice,
			PluginVersion:     pluginVersions[0],
			PluginApiVersions: []string{device.ApiVersion010},
		},
		{
			Name:              plugins[1],
			Type:              base.PluginTypeDevice,
			PluginVersion:     pluginVersions[1],
			PluginApiVersions: []string{device.ApiVersion010},
		},
	}
	require.EqualValues(expected, detected)
}

// Pass a config but make sure it is fatal
func TestPluginLoader_External_Config_Bad(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Create a plugin
	plugins := []string{"mock-device"}
	pluginVersions := []string{"v0.0.1"}
	h := newHarness(t, plugins)

	logger := testlog.HCLogger(t)
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:            logger,
		PluginDir:         h.pluginDir(),
		SupportedVersions: supportedApiVersions,
		Configs: []*config.PluginConfig{
			{
				Name: plugins[0],
				Args: []string{"-plugin", "-name", plugins[0],
					"-type", base.PluginTypeDevice, "-version", pluginVersions[0], "-api-version", device.ApiVersion010},
				Config: map[string]interface{}{
					"foo":          "1",
					"bar":          "2",
					"non-existent": "3",
				},
			},
		},
	}

	_, err := NewPluginLoader(lconfig)
	require.Error(err)
	require.Contains(err.Error(), "No argument or block type is named \"non-existent\"")
}

func TestPluginLoader_External_VersionOverlap(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Create two plugins
	plugins := []string{"mock-device", "mock-device-2"}
	pluginVersions := []string{"v0.0.1", "v0.0.2"}
	h := newHarness(t, plugins)

	logger := testlog.HCLogger(t)
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:            logger,
		PluginDir:         h.pluginDir(),
		SupportedVersions: supportedApiVersions,
		Configs: []*config.PluginConfig{
			{
				Name: plugins[0],
				Args: []string{"-plugin", "-name", plugins[0],
					"-type", base.PluginTypeDevice, "-version", pluginVersions[0], "-api-version", device.ApiVersion010},
			},
			{
				Name: plugins[1],
				Args: []string{"-plugin", "-name", plugins[0],
					"-type", base.PluginTypeDevice, "-version", pluginVersions[1], "-api-version", device.ApiVersion010},
			},
		},
	}

	l, err := NewPluginLoader(lconfig)
	require.NoError(err)

	// Get the catalog and assert we have the two plugins
	c := l.Catalog()
	require.Len(c, 1)
	require.Contains(c, base.PluginTypeDevice)
	detected := c[base.PluginTypeDevice]
	require.Len(detected, 1)
	sort.Slice(detected, func(i, j int) bool { return detected[i].Name < detected[j].Name })

	expected := []*base.PluginInfoResponse{
		{
			Name:              plugins[0],
			Type:              base.PluginTypeDevice,
			PluginVersion:     pluginVersions[1],
			PluginApiVersions: []string{device.ApiVersion010},
		},
	}
	require.EqualValues(expected, detected)
}

func TestPluginLoader_Internal(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Create the harness
	h := newHarness(t, nil)

	plugins := []string{"mock-device", "mock-device-2"}
	pluginVersions := []string{"v0.0.1", "v0.0.2"}
	pluginApiVersions := []string{device.ApiVersion010}

	logger := testlog.HCLogger(t)
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:            logger,
		PluginDir:         h.pluginDir(),
		SupportedVersions: supportedApiVersions,
		InternalPlugins: map[PluginID]*InternalPluginConfig{
			{
				Name:       plugins[0],
				PluginType: base.PluginTypeDevice,
			}: {
				Factory: mockFactory(plugins[0], base.PluginTypeDevice, pluginVersions[0], pluginApiVersions, true),
			},
			{
				Name:       plugins[1],
				PluginType: base.PluginTypeDevice,
			}: {
				Factory: mockFactory(plugins[1], base.PluginTypeDevice, pluginVersions[1], pluginApiVersions, true),
			},
		},
	}

	l, err := NewPluginLoader(lconfig)
	require.NoError(err)

	// Get the catalog and assert we have the two plugins
	c := l.Catalog()
	require.Len(c, 1)
	require.Contains(c, base.PluginTypeDevice)
	detected := c[base.PluginTypeDevice]
	require.Len(detected, 2)
	sort.Slice(detected, func(i, j int) bool { return detected[i].Name < detected[j].Name })

	expected := []*base.PluginInfoResponse{
		{
			Name:              plugins[0],
			Type:              base.PluginTypeDevice,
			PluginVersion:     pluginVersions[0],
			PluginApiVersions: []string{device.ApiVersion010},
		},
		{
			Name:              plugins[1],
			Type:              base.PluginTypeDevice,
			PluginVersion:     pluginVersions[1],
			PluginApiVersions: []string{device.ApiVersion010},
		},
	}
	require.EqualValues(expected, detected)
}

func TestPluginLoader_Internal_ApiVersions(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Create two plugins
	plugins := []string{"mock-device", "mock-device-2", "mock-device-3"}
	pluginVersions := []string{"v0.0.1", "v0.0.2"}
	h := newHarness(t, nil)

	logger := testlog.HCLogger(t)
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:    logger,
		PluginDir: h.pluginDir(),
		SupportedVersions: map[string][]string{
			base.PluginTypeDevice: {"0.2.0", "0.2.1", "0.3.0"},
		},
		InternalPlugins: map[PluginID]*InternalPluginConfig{
			{
				Name:       plugins[0],
				PluginType: base.PluginTypeDevice,
			}: {
				Factory: mockFactory(plugins[0], base.PluginTypeDevice, pluginVersions[0], []string{"v0.1.0"}, true),
			},
			{
				Name:       plugins[1],
				PluginType: base.PluginTypeDevice,
			}: {
				Factory: mockFactory(plugins[1], base.PluginTypeDevice, pluginVersions[1],
					[]string{"v0.1.0", "v0.2.0", "v0.2.1", "v0.2.2"}, true),
			},
			{
				Name:       plugins[2],
				PluginType: base.PluginTypeDevice,
			}: {
				Factory: mockFactory(plugins[2], base.PluginTypeDevice, pluginVersions[1],
					[]string{"v0.1.0", "v0.2.0", "v0.2.1", "v0.3.0"}, true),
			},
		},
	}

	l, err := NewPluginLoader(lconfig)
	require.NoError(err)

	// Get the catalog and assert we have the two plugins
	c := l.Catalog()
	require.Len(c, 1)
	require.Contains(c, base.PluginTypeDevice)
	detected := c[base.PluginTypeDevice]
	require.Len(detected, 2)
	sort.Slice(detected, func(i, j int) bool { return detected[i].Name < detected[j].Name })

	expected := []*base.PluginInfoResponse{
		{
			Name:              plugins[1],
			Type:              base.PluginTypeDevice,
			PluginVersion:     pluginVersions[1],
			PluginApiVersions: []string{"v0.1.0", "v0.2.0", "v0.2.1", "v0.2.2"},
		},
		{
			Name:              plugins[2],
			Type:              base.PluginTypeDevice,
			PluginVersion:     pluginVersions[1],
			PluginApiVersions: []string{"v0.1.0", "v0.2.0", "v0.2.1", "v0.3.0"},
		},
	}
	require.EqualValues(expected, detected)

	// Test we chose the correct versions by dispensing and checking and then
	// reattaching and checking
	p1, err := l.Dispense(plugins[1], base.PluginTypeDevice, nil, logger)
	require.NoError(err)
	defer p1.Kill()
	require.Equal("v0.2.1", p1.ApiVersion())

	p2, err := l.Dispense(plugins[2], base.PluginTypeDevice, nil, logger)
	require.NoError(err)
	defer p2.Kill()
	require.Equal("v0.3.0", p2.ApiVersion())
}

func TestPluginLoader_Internal_NoApiVersion(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Create two plugins
	plugins := []string{"mock-device"}
	pluginVersions := []string{"v0.0.1", "v0.0.2"}
	h := newHarness(t, nil)

	logger := testlog.HCLogger(t)
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:            logger,
		PluginDir:         h.pluginDir(),
		SupportedVersions: supportedApiVersions,
		InternalPlugins: map[PluginID]*InternalPluginConfig{
			{
				Name:       plugins[0],
				PluginType: base.PluginTypeDevice,
			}: {
				Factory: mockFactory(plugins[0], base.PluginTypeDevice, pluginVersions[0], nil, true),
			},
		},
	}

	_, err := NewPluginLoader(lconfig)
	require.Error(err)
	require.Contains(err.Error(), "no compatible API versions")
}

func TestPluginLoader_Internal_Config(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Create the harness
	h := newHarness(t, nil)

	plugins := []string{"mock-device", "mock-device-2"}
	pluginVersions := []string{"v0.0.1", "v0.0.2"}
	pluginApiVersions := []string{device.ApiVersion010}

	logger := testlog.HCLogger(t)
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:            logger,
		PluginDir:         h.pluginDir(),
		SupportedVersions: supportedApiVersions,
		InternalPlugins: map[PluginID]*InternalPluginConfig{
			{
				Name:       plugins[0],
				PluginType: base.PluginTypeDevice,
			}: {
				Factory: mockFactory(plugins[0], base.PluginTypeDevice, pluginVersions[0], pluginApiVersions, true),
				Config: map[string]interface{}{
					"foo": "1",
					"bar": "2",
				},
			},
			{
				Name:       plugins[1],
				PluginType: base.PluginTypeDevice,
			}: {
				Factory: mockFactory(plugins[1], base.PluginTypeDevice, pluginVersions[1], pluginApiVersions, true),
				Config: map[string]interface{}{
					"foo": "3",
					"bar": "4",
				},
			},
		},
	}

	l, err := NewPluginLoader(lconfig)
	require.NoError(err)

	// Get the catalog and assert we have the two plugins
	c := l.Catalog()
	require.Len(c, 1)
	require.Contains(c, base.PluginTypeDevice)
	detected := c[base.PluginTypeDevice]
	require.Len(detected, 2)
	sort.Slice(detected, func(i, j int) bool { return detected[i].Name < detected[j].Name })

	expected := []*base.PluginInfoResponse{
		{
			Name:              plugins[0],
			Type:              base.PluginTypeDevice,
			PluginVersion:     pluginVersions[0],
			PluginApiVersions: []string{device.ApiVersion010},
		},
		{
			Name:              plugins[1],
			Type:              base.PluginTypeDevice,
			PluginVersion:     pluginVersions[1],
			PluginApiVersions: []string{device.ApiVersion010},
		},
	}
	require.EqualValues(expected, detected)
}

// Tests that an external config can override the config of an internal plugin
func TestPluginLoader_Internal_ExternalConfig(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Create the harness
	h := newHarness(t, nil)

	plugin := "mock-device"
	pluginVersion := "v0.0.1"
	pluginApiVersions := []string{device.ApiVersion010}

	id := PluginID{
		Name:       plugin,
		PluginType: base.PluginTypeDevice,
	}
	expectedConfig := map[string]interface{}{
		"foo": "2",
		"bar": "3",
	}

	logger := testlog.HCLogger(t)
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:            logger,
		PluginDir:         h.pluginDir(),
		SupportedVersions: supportedApiVersions,
		InternalPlugins: map[PluginID]*InternalPluginConfig{
			id: {
				Factory: mockFactory(plugin, base.PluginTypeDevice, pluginVersion, pluginApiVersions, true),
				Config: map[string]interface{}{
					"foo": "1",
					"bar": "2",
				},
			},
		},
		Configs: []*config.PluginConfig{
			{
				Name:   plugin,
				Config: expectedConfig,
			},
		},
	}

	l, err := NewPluginLoader(lconfig)
	require.NoError(err)

	// Get the catalog and assert we have the two plugins
	c := l.Catalog()
	require.Len(c, 1)
	require.Contains(c, base.PluginTypeDevice)
	detected := c[base.PluginTypeDevice]
	require.Len(detected, 1)

	expected := []*base.PluginInfoResponse{
		{
			Name:              plugin,
			Type:              base.PluginTypeDevice,
			PluginVersion:     pluginVersion,
			PluginApiVersions: []string{device.ApiVersion010},
		},
	}
	require.EqualValues(expected, detected)

	// Check the config
	loaded, ok := l.plugins[id]
	require.True(ok)
	require.EqualValues(expectedConfig, loaded.config)
}

// Pass a config but make sure it is fatal
func TestPluginLoader_Internal_Config_Bad(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Create the harness
	h := newHarness(t, nil)

	plugins := []string{"mock-device"}
	pluginVersions := []string{"v0.0.1"}
	pluginApiVersions := []string{device.ApiVersion010}

	logger := testlog.HCLogger(t)
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:            logger,
		PluginDir:         h.pluginDir(),
		SupportedVersions: supportedApiVersions,
		InternalPlugins: map[PluginID]*InternalPluginConfig{
			{
				Name:       plugins[0],
				PluginType: base.PluginTypeDevice,
			}: {
				Factory: mockFactory(plugins[0], base.PluginTypeDevice, pluginVersions[0], pluginApiVersions, true),
				Config: map[string]interface{}{
					"foo":          "1",
					"bar":          "2",
					"non-existent": "3",
				},
			},
		},
	}

	_, err := NewPluginLoader(lconfig)
	require.Error(err)
	require.Contains(err.Error(), "No argument or block type is named \"non-existent\"")
}

func TestPluginLoader_InternalOverrideExternal(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Create two plugins
	plugins := []string{"mock-device"}
	pluginVersions := []string{"v0.0.1", "v0.0.2"}
	pluginApiVersions := []string{device.ApiVersion010}

	h := newHarness(t, plugins)

	logger := testlog.HCLogger(t)
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:            logger,
		PluginDir:         h.pluginDir(),
		SupportedVersions: supportedApiVersions,
		Configs: []*config.PluginConfig{
			{
				Name: plugins[0],
				Args: []string{"-plugin", "-name", plugins[0],
					"-type", base.PluginTypeDevice, "-version", pluginVersions[0], "-api-version", pluginApiVersions[0]},
			},
		},
		InternalPlugins: map[PluginID]*InternalPluginConfig{
			{
				Name:       plugins[0],
				PluginType: base.PluginTypeDevice,
			}: {
				Factory: mockFactory(plugins[0], base.PluginTypeDevice, pluginVersions[1], pluginApiVersions, true),
			},
		},
	}

	l, err := NewPluginLoader(lconfig)
	require.NoError(err)

	// Get the catalog and assert we have the two plugins
	c := l.Catalog()
	require.Len(c, 1)
	require.Contains(c, base.PluginTypeDevice)
	detected := c[base.PluginTypeDevice]
	require.Len(detected, 1)
	sort.Slice(detected, func(i, j int) bool { return detected[i].Name < detected[j].Name })

	expected := []*base.PluginInfoResponse{
		{
			Name:              plugins[0],
			Type:              base.PluginTypeDevice,
			PluginVersion:     pluginVersions[1],
			PluginApiVersions: []string{device.ApiVersion010},
		},
	}
	require.EqualValues(expected, detected)
}

func TestPluginLoader_ExternalOverrideInternal(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Create two plugins
	plugins := []string{"mock-device"}
	pluginVersions := []string{"v0.0.1", "v0.0.2"}
	pluginApiVersions := []string{device.ApiVersion010}

	h := newHarness(t, plugins)

	logger := testlog.HCLogger(t)
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:            logger,
		PluginDir:         h.pluginDir(),
		SupportedVersions: supportedApiVersions,
		Configs: []*config.PluginConfig{
			{
				Name: plugins[0],
				Args: []string{"-plugin", "-name", plugins[0],
					"-type", base.PluginTypeDevice, "-version", pluginVersions[1], "-api-version", pluginApiVersions[0]},
			},
		},
		InternalPlugins: map[PluginID]*InternalPluginConfig{
			{
				Name:       plugins[0],
				PluginType: base.PluginTypeDevice,
			}: {
				Factory: mockFactory(plugins[0], base.PluginTypeDevice, pluginVersions[0], pluginApiVersions, true),
			},
		},
	}

	l, err := NewPluginLoader(lconfig)
	require.NoError(err)

	// Get the catalog and assert we have the two plugins
	c := l.Catalog()
	require.Len(c, 1)
	require.Contains(c, base.PluginTypeDevice)
	detected := c[base.PluginTypeDevice]
	require.Len(detected, 1)
	sort.Slice(detected, func(i, j int) bool { return detected[i].Name < detected[j].Name })

	expected := []*base.PluginInfoResponse{
		{
			Name:              plugins[0],
			Type:              base.PluginTypeDevice,
			PluginVersion:     pluginVersions[1],
			PluginApiVersions: []string{device.ApiVersion010},
		},
	}
	require.EqualValues(expected, detected)
}

func TestPluginLoader_Dispense_External(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Create two plugins
	plugin := "mock-device"
	pluginVersion := "v0.0.1"
	h := newHarness(t, []string{plugin})

	expKey := "set_config_worked"

	logger := testlog.HCLogger(t)
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:            logger,
		PluginDir:         h.pluginDir(),
		SupportedVersions: supportedApiVersions,
		Configs: []*config.PluginConfig{
			{
				Name: plugin,
				Args: []string{"-plugin", "-name", plugin,
					"-type", base.PluginTypeDevice, "-version", pluginVersion, "-api-version", device.ApiVersion010},
				Config: map[string]interface{}{
					"res_key": expKey,
				},
			},
		},
	}

	l, err := NewPluginLoader(lconfig)
	require.NoError(err)

	// Dispense a device plugin
	p, err := l.Dispense(plugin, base.PluginTypeDevice, nil, logger)
	require.NoError(err)
	defer p.Kill()

	instance, ok := p.Plugin().(device.DevicePlugin)
	require.True(ok)

	res, err := instance.Reserve([]string{"fake"})
	require.NoError(err)
	require.NotNil(res)
	require.Contains(res.Envs, expKey)
}

func TestPluginLoader_Dispense_Internal(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Create two plugins
	plugin := "mock-device"
	pluginVersion := "v0.0.1"
	pluginApiVersions := []string{device.ApiVersion010}
	h := newHarness(t, nil)

	expKey := "set_config_worked"
	expNomadConfig := &base.AgentConfig{
		Driver: &base.ClientDriverConfig{
			ClientMinPort: 100,
		},
	}

	logger := testlog.HCLogger(t)
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:            logger,
		PluginDir:         h.pluginDir(),
		SupportedVersions: supportedApiVersions,
		InternalPlugins: map[PluginID]*InternalPluginConfig{
			{
				Name:       plugin,
				PluginType: base.PluginTypeDevice,
			}: {
				Factory: mockFactory(plugin, base.PluginTypeDevice, pluginVersion, pluginApiVersions, true),
				Config: map[string]interface{}{
					"res_key": expKey,
				},
			},
		},
	}

	l, err := NewPluginLoader(lconfig)
	require.NoError(err)

	// Dispense a device plugin
	p, err := l.Dispense(plugin, base.PluginTypeDevice, expNomadConfig, logger)
	require.NoError(err)
	defer p.Kill()

	instance, ok := p.Plugin().(device.DevicePlugin)
	require.True(ok)

	res, err := instance.Reserve([]string{"fake"})
	require.NoError(err)
	require.NotNil(res)
	require.Contains(res.Envs, expKey)

	mock, ok := p.Plugin().(*mockPlugin)
	require.True(ok)
	require.Exactly(expNomadConfig, mock.nomadConfig)
	require.Equal(device.ApiVersion010, mock.negotiatedApiVersion)
}

func TestPluginLoader_Dispense_NoConfigSchema_External(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Create two plugins
	plugin := "mock-device"
	pluginVersion := "v0.0.1"
	h := newHarness(t, []string{plugin})

	expKey := "set_config_worked"

	logger := testlog.HCLogger(t)
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:            logger,
		PluginDir:         h.pluginDir(),
		SupportedVersions: supportedApiVersions,
		Configs: []*config.PluginConfig{
			{
				Name: plugin,
				Args: []string{"-plugin", "-config-schema=false", "-name", plugin,
					"-type", base.PluginTypeDevice, "-version", pluginVersion, "-api-version", device.ApiVersion010},
				Config: map[string]interface{}{
					"res_key": expKey,
				},
			},
		},
	}

	_, err := NewPluginLoader(lconfig)
	require.Error(err)
	require.Contains(err.Error(), "configuration not allowed")

	// Remove the config and try again
	lconfig.Configs[0].Config = nil
	l, err := NewPluginLoader(lconfig)
	require.NoError(err)

	// Dispense a device plugin
	p, err := l.Dispense(plugin, base.PluginTypeDevice, nil, logger)
	require.NoError(err)
	defer p.Kill()

	_, ok := p.Plugin().(device.DevicePlugin)
	require.True(ok)
}

func TestPluginLoader_Dispense_NoConfigSchema_Internal(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Create two plugins
	plugin := "mock-device"
	pluginVersion := "v0.0.1"
	pluginApiVersions := []string{device.ApiVersion010}
	h := newHarness(t, nil)

	expKey := "set_config_worked"

	logger := testlog.HCLogger(t)
	logger.SetLevel(log.Trace)
	pid := PluginID{
		Name:       plugin,
		PluginType: base.PluginTypeDevice,
	}
	lconfig := &PluginLoaderConfig{
		Logger:            logger,
		PluginDir:         h.pluginDir(),
		SupportedVersions: supportedApiVersions,
		InternalPlugins: map[PluginID]*InternalPluginConfig{
			pid: {
				Factory: mockFactory(plugin, base.PluginTypeDevice, pluginVersion, pluginApiVersions, false),
				Config: map[string]interface{}{
					"res_key": expKey,
				},
			},
		},
	}

	_, err := NewPluginLoader(lconfig)
	require.Error(err)
	require.Contains(err.Error(), "configuration not allowed")

	// Remove the config and try again
	lconfig.InternalPlugins[pid].Factory = mockFactory(plugin, base.PluginTypeDevice, pluginVersion, pluginApiVersions, true)
	l, err := NewPluginLoader(lconfig)
	require.NoError(err)

	// Dispense a device plugin
	p, err := l.Dispense(plugin, base.PluginTypeDevice, nil, logger)
	require.NoError(err)
	defer p.Kill()

	_, ok := p.Plugin().(device.DevicePlugin)
	require.True(ok)
}

func TestPluginLoader_Reattach_External(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Create a plugin
	plugin := "mock-device"
	pluginVersion := "v0.0.1"
	h := newHarness(t, []string{plugin})

	expKey := "set_config_worked"

	logger := testlog.HCLogger(t)
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:            logger,
		PluginDir:         h.pluginDir(),
		SupportedVersions: supportedApiVersions,
		Configs: []*config.PluginConfig{
			{
				Name: plugin,
				Args: []string{"-plugin", "-name", plugin,
					"-type", base.PluginTypeDevice, "-version", pluginVersion, "-api-version", device.ApiVersion010},
				Config: map[string]interface{}{
					"res_key": expKey,
				},
			},
		},
	}

	l, err := NewPluginLoader(lconfig)
	require.NoError(err)

	// Dispense a device plugin
	p, err := l.Dispense(plugin, base.PluginTypeDevice, nil, logger)
	require.NoError(err)
	defer p.Kill()

	instance, ok := p.Plugin().(device.DevicePlugin)
	require.True(ok)

	res, err := instance.Reserve([]string{"fake"})
	require.NoError(err)
	require.NotNil(res)
	require.Contains(res.Envs, expKey)

	// Reattach to the plugin
	reattach, ok := p.ReattachConfig()
	require.True(ok)

	p2, err := l.Reattach(plugin, base.PluginTypeDevice, reattach)
	require.NoError(err)

	// Get the reattached plugin and ensure its the same
	instance2, ok := p2.Plugin().(device.DevicePlugin)
	require.True(ok)

	res2, err := instance2.Reserve([]string{"fake"})
	require.NoError(err)
	require.NotNil(res2)
	require.Contains(res2.Envs, expKey)
}

// Test the loader trying to launch a non-plugin binary
func TestPluginLoader_Bad_Executable(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Create a plugin
	plugin := "mock-device"
	h := newHarness(t, []string{plugin})

	logger := testlog.HCLogger(t)
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:            logger,
		PluginDir:         h.pluginDir(),
		SupportedVersions: supportedApiVersions,
		Configs: []*config.PluginConfig{
			{
				Name: plugin,
				Args: []string{"-bad-flag"},
			},
		},
	}

	_, err := NewPluginLoader(lconfig)
	require.Error(err)
	require.Contains(err.Error(), "failed to fingerprint plugin")
}

// Test that we skip directories, non-executables and follow symlinks
func TestPluginLoader_External_SkipBadFiles(t *testing.T) {
	ci.Parallel(t)
	if runtime.GOOS == "windows" {
		t.Skip("Windows currently does not skip non exe files")
	}
	require := require.New(t)

	// Create two plugins
	plugins := []string{"mock-device"}
	pluginVersions := []string{"v0.0.1"}
	h := newHarness(t, nil)

	// Create a folder inside our plugin dir
	require.NoError(os.Mkdir(filepath.Join(h.pluginDir(), "folder"), 0666))

	// Get our own executable path
	selfExe, err := os.Executable()
	require.NoError(err)

	// Create a symlink from our own binary to the directory
	require.NoError(os.Symlink(selfExe, filepath.Join(h.pluginDir(), plugins[0])))

	// Create a non-executable file
	require.NoError(os.WriteFile(filepath.Join(h.pluginDir(), "some.yaml"), []byte("hcl > yaml"), 0666))

	logger := testlog.HCLogger(t)
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:            logger,
		PluginDir:         h.pluginDir(),
		SupportedVersions: supportedApiVersions,
		Configs: []*config.PluginConfig{
			{
				Name: plugins[0],
				Args: []string{"-plugin", "-name", plugins[0],
					"-type", base.PluginTypeDevice, "-version", pluginVersions[0], "-api-version", device.ApiVersion010},
			},
		},
	}

	l, err := NewPluginLoader(lconfig)
	require.NoError(err)

	// Get the catalog and assert we have the two plugins
	c := l.Catalog()
	require.Len(c, 1)
	require.Contains(c, base.PluginTypeDevice)
	detected := c[base.PluginTypeDevice]
	require.Len(detected, 1)
	sort.Slice(detected, func(i, j int) bool { return detected[i].Name < detected[j].Name })

	expected := []*base.PluginInfoResponse{
		{
			Name:              plugins[0],
			Type:              base.PluginTypeDevice,
			PluginVersion:     pluginVersions[0],
			PluginApiVersions: []string{device.ApiVersion010},
		},
	}
	require.EqualValues(expected, detected)
}

func TestPluginLoader_ConvertVersions(t *testing.T) {
	ci.Parallel(t)

	v010 := version.Must(version.NewVersion("v0.1.0"))
	v020 := version.Must(version.NewVersion("v0.2.0"))
	v021 := version.Must(version.NewVersion("v0.2.1"))
	v030 := version.Must(version.NewVersion("v0.3.0"))

	cases := []struct {
		in  []string
		out []*version.Version
		err bool
	}{
		{
			in:  []string{"v0.1.0", "0.2.0", "v0.2.1"},
			out: []*version.Version{v021, v020, v010},
		},
		{
			in:  []string{"0.3.0", "v0.1.0", "0.2.0", "v0.2.1"},
			out: []*version.Version{v030, v021, v020, v010},
		},
		{
			in:  []string{"foo", "v0.1.0", "0.2.0", "v0.2.1"},
			err: true,
		},
	}

	for _, c := range cases {
		t.Run(strings.Join(c.in, ","), func(t *testing.T) {
			act, err := convertVersions(c.in)
			if err != nil {
				if c.err {
					return
				}
				t.Fatalf("unexpected err: %v", err)
			}
			require.Len(t, act, len(c.out))
			for i, v := range act {
				if !v.Equal(c.out[i]) {
					t.Fatalf("parsed version[%d] not equal: %v != %v", i, v, c.out[i])
				}
			}
		})
	}
}
