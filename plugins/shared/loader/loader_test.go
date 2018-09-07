package loader

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/stretchr/testify/require"
)

// harness is used to build a temp directory and copy our own test executable
// into it, allowing the plugin loader to scan for plugins.
type harness struct {
	t      *testing.T
	tmpDir string
}

// newHarness returns a harness and copies our test binary to the temp directory
// with teh passed plugin names.
func newHarness(t *testing.T, plugins []string) *harness {
	t.Helper()

	h := &harness{
		t: t,
	}

	// Build a temp directory
	path, err := ioutil.TempDir("", t.Name())
	if err != nil {
		t.Fatalf("failed to build tmp directory")
	}
	h.tmpDir = path

	// Get our own executable path
	selfExe, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to get self executable path: %v", err)
	}

	for _, p := range plugins {
		dest := filepath.Join(h.tmpDir, p)
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

// cleanup removes the temp directory
func (h *harness) cleanup() {
	if err := os.RemoveAll(h.tmpDir); err != nil {
		h.t.Fatalf("failed to remove tmp directory %q: %v", h.tmpDir, err)
	}
}

func TestPluginLoader_External(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Create two plugins
	plugins := []string{"mock-device", "mock-device-2"}
	pluginVersions := []string{"v0.0.1", "v0.0.2"}
	h := newHarness(t, plugins)
	defer h.cleanup()

	logger := log.Default()
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:    logger, // XXX Use testlog package
		PluginDir: h.pluginDir(),
		Configs: []*config.PluginConfig{
			{
				Name: plugins[0],
				Args: []string{"-plugin", "-name", plugins[0],
					"-type", base.PluginTypeDevice, "-version", pluginVersions[0]},
			},
			{
				Name: plugins[1],
				Args: []string{"-plugin", "-name", plugins[1],
					"-type", base.PluginTypeDevice, "-version", pluginVersions[1]},
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
			Name:             plugins[0],
			Type:             base.PluginTypeDevice,
			PluginVersion:    pluginVersions[0],
			PluginApiVersion: "v0.1.0",
		},
		{
			Name:             plugins[1],
			Type:             base.PluginTypeDevice,
			PluginVersion:    pluginVersions[1],
			PluginApiVersion: "v0.1.0",
		},
	}
	require.EqualValues(expected, detected)
}

func TestPluginLoader_External_Config(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Create two plugins
	plugins := []string{"mock-device", "mock-device-2"}
	pluginVersions := []string{"v0.0.1", "v0.0.2"}
	h := newHarness(t, plugins)
	defer h.cleanup()

	logger := log.Default()
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:    logger, // XXX Use testlog package
		PluginDir: h.pluginDir(),
		Configs: []*config.PluginConfig{
			{
				Name: plugins[0],
				Args: []string{"-plugin", "-name", plugins[0],
					"-type", base.PluginTypeDevice, "-version", pluginVersions[0]},
				Config: map[string]interface{}{
					"foo": "1",
					"bar": "2",
				},
			},
			{
				Name: plugins[1],
				Args: []string{"-plugin", "-name", plugins[1],
					"-type", base.PluginTypeDevice, "-version", pluginVersions[1]},
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
			Name:             plugins[0],
			Type:             base.PluginTypeDevice,
			PluginVersion:    pluginVersions[0],
			PluginApiVersion: "v0.1.0",
		},
		{
			Name:             plugins[1],
			Type:             base.PluginTypeDevice,
			PluginVersion:    pluginVersions[1],
			PluginApiVersion: "v0.1.0",
		},
	}
	require.EqualValues(expected, detected)
}

// Pass a config but make sure it is fatal
func TestPluginLoader_External_Config_Bad(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Create two plugins
	plugins := []string{"mock-device"}
	pluginVersions := []string{"v0.0.1"}
	h := newHarness(t, plugins)
	defer h.cleanup()

	logger := log.Default()
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:    logger, // XXX Use testlog package
		PluginDir: h.pluginDir(),
		Configs: []*config.PluginConfig{
			{
				Name: plugins[0],
				Args: []string{"-plugin", "-name", plugins[0],
					"-type", base.PluginTypeDevice, "-version", pluginVersions[0]},
				Config: map[string]interface{}{
					"foo":          "1",
					"bar":          "2",
					"non-existant": "3",
				},
			},
		},
	}

	_, err := NewPluginLoader(lconfig)
	require.Error(err)
	require.Contains(err.Error(), "No argument or block type is named \"non-existant\"")
}

func TestPluginLoader_External_VersionOverlap(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Create two plugins
	plugins := []string{"mock-device", "mock-device"}
	pluginVersions := []string{"v0.0.1", "v0.0.2"}
	h := newHarness(t, plugins)
	defer h.cleanup()

	logger := log.Default()
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:    logger, // XXX Use testlog package
		PluginDir: h.pluginDir(),
		Configs: []*config.PluginConfig{
			{
				Name: plugins[0],
				Args: []string{"-plugin", "-name", plugins[0],
					"-type", base.PluginTypeDevice, "-version", pluginVersions[0]},
			},
			{
				Name: plugins[1],
				Args: []string{"-plugin", "-name", plugins[1],
					"-type", base.PluginTypeDevice, "-version", pluginVersions[1]},
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
			Name:             plugins[1],
			Type:             base.PluginTypeDevice,
			PluginVersion:    pluginVersions[1],
			PluginApiVersion: "v0.1.0",
		},
	}
	require.EqualValues(expected, detected)
}

func TestPluginLoader_Internal(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Create the harness
	h := newHarness(t, nil)
	defer h.cleanup()

	plugins := []string{"mock-device", "mock-device-2"}
	pluginVersions := []string{"v0.0.1", "v0.0.2"}

	logger := log.Default()
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:    logger, // XXX Use testlog package
		PluginDir: h.pluginDir(),
		InternalPlugins: map[PluginID]*InternalPluginConfig{
			{
				Name:       plugins[0],
				PluginType: base.PluginTypeDevice,
			}: {
				Factory: mockFactory(plugins[0], base.PluginTypeDevice, pluginVersions[0], true),
			},
			{
				Name:       plugins[1],
				PluginType: base.PluginTypeDevice,
			}: {
				Factory: mockFactory(plugins[1], base.PluginTypeDevice, pluginVersions[1], true),
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
			Name:             plugins[0],
			Type:             base.PluginTypeDevice,
			PluginVersion:    pluginVersions[0],
			PluginApiVersion: "v0.1.0",
		},
		{
			Name:             plugins[1],
			Type:             base.PluginTypeDevice,
			PluginVersion:    pluginVersions[1],
			PluginApiVersion: "v0.1.0",
		},
	}
	require.EqualValues(expected, detected)
}

func TestPluginLoader_Internal_Config(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Create the harness
	h := newHarness(t, nil)
	defer h.cleanup()

	plugins := []string{"mock-device", "mock-device-2"}
	pluginVersions := []string{"v0.0.1", "v0.0.2"}

	logger := log.Default()
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:    logger, // XXX Use testlog package
		PluginDir: h.pluginDir(),
		InternalPlugins: map[PluginID]*InternalPluginConfig{
			{
				Name:       plugins[0],
				PluginType: base.PluginTypeDevice,
			}: {
				Factory: mockFactory(plugins[0], base.PluginTypeDevice, pluginVersions[0], true),
				Config: map[string]interface{}{
					"foo": "1",
					"bar": "2",
				},
			},
			{
				Name:       plugins[1],
				PluginType: base.PluginTypeDevice,
			}: {
				Factory: mockFactory(plugins[1], base.PluginTypeDevice, pluginVersions[1], true),
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
			Name:             plugins[0],
			Type:             base.PluginTypeDevice,
			PluginVersion:    pluginVersions[0],
			PluginApiVersion: "v0.1.0",
		},
		{
			Name:             plugins[1],
			Type:             base.PluginTypeDevice,
			PluginVersion:    pluginVersions[1],
			PluginApiVersion: "v0.1.0",
		},
	}
	require.EqualValues(expected, detected)
}

// Pass a config but make sure it is fatal
func TestPluginLoader_Internal_Config_Bad(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Create the harness
	h := newHarness(t, nil)
	defer h.cleanup()

	plugins := []string{"mock-device"}
	pluginVersions := []string{"v0.0.1"}

	logger := log.Default()
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:    logger, // XXX Use testlog package
		PluginDir: h.pluginDir(),
		InternalPlugins: map[PluginID]*InternalPluginConfig{
			{
				Name:       plugins[0],
				PluginType: base.PluginTypeDevice,
			}: {
				Factory: mockFactory(plugins[0], base.PluginTypeDevice, pluginVersions[0], true),
				Config: map[string]interface{}{
					"foo":          "1",
					"bar":          "2",
					"non-existant": "3",
				},
			},
		},
	}

	_, err := NewPluginLoader(lconfig)
	require.Error(err)
	require.Contains(err.Error(), "No argument or block type is named \"non-existant\"")
}

func TestPluginLoader_InternalOverrideExternal(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Create two plugins
	plugins := []string{"mock-device"}
	pluginVersions := []string{"v0.0.1", "v0.0.2"}
	h := newHarness(t, plugins)
	defer h.cleanup()

	logger := log.Default()
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:    logger, // XXX Use testlog package
		PluginDir: h.pluginDir(),
		Configs: []*config.PluginConfig{
			{
				Name: plugins[0],
				Args: []string{"-plugin", "-name", plugins[0],
					"-type", base.PluginTypeDevice, "-version", pluginVersions[0]},
			},
		},
		InternalPlugins: map[PluginID]*InternalPluginConfig{
			{
				Name:       plugins[0],
				PluginType: base.PluginTypeDevice,
			}: {
				Factory: mockFactory(plugins[0], base.PluginTypeDevice, pluginVersions[1], true),
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
			Name:             plugins[0],
			Type:             base.PluginTypeDevice,
			PluginVersion:    pluginVersions[1],
			PluginApiVersion: "v0.1.0",
		},
	}
	require.EqualValues(expected, detected)
}

func TestPluginLoader_ExternalOverrideInternal(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Create two plugins
	plugins := []string{"mock-device"}
	pluginVersions := []string{"v0.0.1", "v0.0.2"}
	h := newHarness(t, plugins)
	defer h.cleanup()

	logger := log.Default()
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:    logger, // XXX Use testlog package
		PluginDir: h.pluginDir(),
		Configs: []*config.PluginConfig{
			{
				Name: plugins[0],
				Args: []string{"-plugin", "-name", plugins[0],
					"-type", base.PluginTypeDevice, "-version", pluginVersions[1]},
			},
		},
		InternalPlugins: map[PluginID]*InternalPluginConfig{
			{
				Name:       plugins[0],
				PluginType: base.PluginTypeDevice,
			}: {
				Factory: mockFactory(plugins[0], base.PluginTypeDevice, pluginVersions[0], true),
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
			Name:             plugins[0],
			Type:             base.PluginTypeDevice,
			PluginVersion:    pluginVersions[1],
			PluginApiVersion: "v0.1.0",
		},
	}
	require.EqualValues(expected, detected)
}

func TestPluginLoader_Dispense_External(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Create two plugins
	plugin := "mock-device"
	pluginVersion := "v0.0.1"
	h := newHarness(t, []string{plugin})
	defer h.cleanup()

	expKey := "set_config_worked"

	logger := log.Default()
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:    logger, // XXX Use testlog package
		PluginDir: h.pluginDir(),
		Configs: []*config.PluginConfig{
			{
				Name: plugin,
				Args: []string{"-plugin", "-name", plugin,
					"-type", base.PluginTypeDevice, "-version", pluginVersion},
				Config: map[string]interface{}{
					"res_key": expKey,
				},
			},
		},
	}

	l, err := NewPluginLoader(lconfig)
	require.NoError(err)

	// Dispense a device plugin
	p, err := l.Dispense(plugin, base.PluginTypeDevice, logger)
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
	t.Parallel()
	require := require.New(t)

	// Create two plugins
	plugin := "mock-device"
	pluginVersion := "v0.0.1"
	h := newHarness(t, nil)
	defer h.cleanup()

	expKey := "set_config_worked"

	logger := log.Default()
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:    logger, // XXX Use testlog package
		PluginDir: h.pluginDir(),
		InternalPlugins: map[PluginID]*InternalPluginConfig{
			{
				Name:       plugin,
				PluginType: base.PluginTypeDevice,
			}: {
				Factory: mockFactory(plugin, base.PluginTypeDevice, pluginVersion, true),
				Config: map[string]interface{}{
					"res_key": expKey,
				},
			},
		},
	}

	l, err := NewPluginLoader(lconfig)
	require.NoError(err)

	// Dispense a device plugin
	p, err := l.Dispense(plugin, base.PluginTypeDevice, logger)
	require.NoError(err)
	defer p.Kill()

	instance, ok := p.Plugin().(device.DevicePlugin)
	require.True(ok)

	res, err := instance.Reserve([]string{"fake"})
	require.NoError(err)
	require.NotNil(res)
	require.Contains(res.Envs, expKey)
}

func TestPluginLoader_Dispense_NoConfigSchema_External(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Create two plugins
	plugin := "mock-device"
	pluginVersion := "v0.0.1"
	h := newHarness(t, []string{plugin})
	defer h.cleanup()

	expKey := "set_config_worked"

	logger := log.Default()
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:    logger, // XXX Use testlog package
		PluginDir: h.pluginDir(),
		Configs: []*config.PluginConfig{
			{
				Name: plugin,
				Args: []string{"-plugin", "-config-schema=false", "-name", plugin,
					"-type", base.PluginTypeDevice, "-version", pluginVersion},
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
	p, err := l.Dispense(plugin, base.PluginTypeDevice, logger)
	require.NoError(err)
	defer p.Kill()

	_, ok := p.Plugin().(device.DevicePlugin)
	require.True(ok)
}

func TestPluginLoader_Dispense_NoConfigSchema_Internal(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Create two plugins
	plugin := "mock-device"
	pluginVersion := "v0.0.1"
	h := newHarness(t, nil)
	defer h.cleanup()

	expKey := "set_config_worked"

	logger := log.Default()
	logger.SetLevel(log.Trace)
	pid := PluginID{
		Name:       plugin,
		PluginType: base.PluginTypeDevice,
	}
	lconfig := &PluginLoaderConfig{
		Logger:    logger, // XXX Use testlog package
		PluginDir: h.pluginDir(),
		InternalPlugins: map[PluginID]*InternalPluginConfig{
			pid: {
				Factory: mockFactory(plugin, base.PluginTypeDevice, pluginVersion, false),
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
	lconfig.InternalPlugins[pid].Factory = mockFactory(plugin, base.PluginTypeDevice, pluginVersion, true)
	l, err := NewPluginLoader(lconfig)
	require.NoError(err)

	// Dispense a device plugin
	p, err := l.Dispense(plugin, base.PluginTypeDevice, logger)
	require.NoError(err)
	defer p.Kill()

	_, ok := p.Plugin().(device.DevicePlugin)
	require.True(ok)
}

func TestPluginLoader_Reattach_External(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Create a plugin
	plugin := "mock-device"
	pluginVersion := "v0.0.1"
	h := newHarness(t, []string{plugin})
	defer h.cleanup()

	expKey := "set_config_worked"

	logger := log.Default()
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:    logger, // XXX Use testlog package
		PluginDir: h.pluginDir(),
		Configs: []*config.PluginConfig{
			{
				Name: plugin,
				Args: []string{"-plugin", "-name", plugin,
					"-type", base.PluginTypeDevice, "-version", pluginVersion},
				Config: map[string]interface{}{
					"res_key": expKey,
				},
			},
		},
	}

	l, err := NewPluginLoader(lconfig)
	require.NoError(err)

	// Dispense a device plugin
	p, err := l.Dispense(plugin, base.PluginTypeDevice, logger)
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

	p2, err := l.Reattach(base.PluginTypeDevice, reattach)
	require.NoError(err)

	// Get the reattached plugin and ensure its teh same
	instance2, ok := p2.Plugin().(device.DevicePlugin)
	require.True(ok)

	res2, err := instance2.Reserve([]string{"fake"})
	require.NoError(err)
	require.NotNil(res2)
	require.Contains(res2.Envs, expKey)
}

// Test the loader trying to launch a non-plugin binary
func TestPluginLoader_Bad_Executable(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Create a plugin
	plugin := "mock-device"
	h := newHarness(t, []string{plugin})
	defer h.cleanup()

	logger := log.Default()
	logger.SetLevel(log.Trace)
	lconfig := &PluginLoaderConfig{
		Logger:    logger, // XXX Use testlog package
		PluginDir: h.pluginDir(),
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
