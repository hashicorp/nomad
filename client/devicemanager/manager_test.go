package devicemanager

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	log "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/hashicorp/nomad/plugins/shared/loader"
	psstructs "github.com/hashicorp/nomad/plugins/shared/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/require"
)

var (
	nvidiaDevice0ID   = uuid.Generate()
	nvidiaDevice1ID   = uuid.Generate()
	nvidiaDeviceGroup = &device.DeviceGroup{
		Vendor: "nvidia",
		Type:   "gpu",
		Name:   "1080ti",
		Devices: []*device.Device{
			{
				ID:      nvidiaDevice0ID,
				Healthy: true,
			},
			{
				ID:      nvidiaDevice1ID,
				Healthy: true,
			},
		},
		Attributes: map[string]*psstructs.Attribute{
			"memory": {
				Int:  helper.Int64ToPtr(4),
				Unit: "GB",
			},
		},
	}

	intelDeviceID    = uuid.Generate()
	intelDeviceGroup = &device.DeviceGroup{
		Vendor: "intel",
		Type:   "gpu",
		Name:   "640GT",
		Devices: []*device.Device{
			{
				ID:      intelDeviceID,
				Healthy: true,
			},
		},
		Attributes: map[string]*psstructs.Attribute{
			"memory": {
				Int:  helper.Int64ToPtr(2),
				Unit: "GB",
			},
		},
	}

	nvidiaDeviceGroupStats = &device.DeviceGroupStats{
		Vendor: "nvidia",
		Type:   "gpu",
		Name:   "1080ti",
		InstanceStats: map[string]*device.DeviceStats{
			nvidiaDevice0ID: &device.DeviceStats{
				Summary: &device.StatValue{
					IntNumeratorVal: 212,
					Unit:            "F",
					Desc:            "Temperature",
				},
			},
			nvidiaDevice1ID: &device.DeviceStats{
				Summary: &device.StatValue{
					IntNumeratorVal: 218,
					Unit:            "F",
					Desc:            "Temperature",
				},
			},
		},
	}

	intelDeviceGroupStats = &device.DeviceGroupStats{
		Vendor: "intel",
		Type:   "gpu",
		Name:   "640GT",
		InstanceStats: map[string]*device.DeviceStats{
			intelDeviceID: &device.DeviceStats{
				Summary: &device.StatValue{
					IntNumeratorVal: 220,
					Unit:            "F",
					Desc:            "Temperature",
				},
			},
		},
	}
)

func baseTestConfig(t *testing.T) (
	config *Config,
	deviceUpdateCh chan []*structs.NodeDeviceResource,
	catalog *loader.MockCatalog) {

	// Create an update handler
	deviceUpdates := make(chan []*structs.NodeDeviceResource, 1)
	updateFn := func(devices []*structs.NodeDeviceResource) {
		deviceUpdates <- devices
	}

	// Create a mock plugin catalog
	mc := &loader.MockCatalog{}

	// Create the config
	config = &Config{
		Logger:        testlog.HCLogger(t),
		PluginConfig:  &base.ClientAgentConfig{},
		StatsInterval: 100 * time.Millisecond,
		State:         state.NewMemDB(),
		Updater:       updateFn,
		Loader:        mc,
	}

	return config, deviceUpdates, mc
}

func configureCatalogWith(catalog *loader.MockCatalog, plugins map[*base.PluginInfoResponse]loader.PluginInstance) {

	catalog.DispenseF = func(name, _ string, _ *base.ClientAgentConfig, _ log.Logger) (loader.PluginInstance, error) {
		for info, v := range plugins {
			if info.Name == name {
				return v, nil
			}
		}

		return nil, fmt.Errorf("no matching plugin")
	}

	catalog.ReattachF = func(name, _ string, _ *plugin.ReattachConfig) (loader.PluginInstance, error) {
		for info, v := range plugins {
			if info.Name == name {
				return v, nil
			}
		}

		return nil, fmt.Errorf("no matching plugin")
	}

	catalog.CatalogF = func() map[string][]*base.PluginInfoResponse {
		devices := make([]*base.PluginInfoResponse, 0, len(plugins))
		for k := range plugins {
			devices = append(devices, k)
		}
		out := map[string][]*base.PluginInfoResponse{
			base.PluginTypeDevice: devices,
		}
		return out
	}
}

func pluginInfoResponse(name string) *base.PluginInfoResponse {
	return &base.PluginInfoResponse{
		Type:             base.PluginTypeDevice,
		PluginApiVersion: "v0.0.1",
		PluginVersion:    "v0.0.1",
		Name:             name,
	}
}

// drainNodeDeviceUpdates drains all updates to the node device fingerprint channel
func drainNodeDeviceUpdates(ctx context.Context, in chan []*structs.NodeDeviceResource) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-in:
			}
		}
	}()
}

func deviceReserveFn(ids []string) (*device.ContainerReservation, error) {
	return &device.ContainerReservation{
		Envs: map[string]string{
			"DEVICES": strings.Join(ids, ","),
		},
	}, nil
}

// nvidiaAndIntelDefaultPlugins adds an nvidia and intel mock plugin to the
// catalog
func nvidiaAndIntelDefaultPlugins(catalog *loader.MockCatalog) {
	pluginInfoNvidia := pluginInfoResponse("nvidia")
	deviceNvidia := &device.MockDevicePlugin{
		MockPlugin: &base.MockPlugin{
			PluginInfoF:   base.StaticInfo(pluginInfoNvidia),
			ConfigSchemaF: base.TestConfigSchema(),
			SetConfigF:    base.NoopSetConfig(),
		},
		FingerprintF: device.StaticFingerprinter([]*device.DeviceGroup{nvidiaDeviceGroup}),
		ReserveF:     deviceReserveFn,
		StatsF:       device.StaticStats([]*device.DeviceGroupStats{nvidiaDeviceGroupStats}),
	}
	pluginNvidia := loader.MockBasicExternalPlugin(deviceNvidia)

	pluginInfoIntel := pluginInfoResponse("intel")
	deviceIntel := &device.MockDevicePlugin{
		MockPlugin: &base.MockPlugin{
			PluginInfoF:   base.StaticInfo(pluginInfoIntel),
			ConfigSchemaF: base.TestConfigSchema(),
			SetConfigF:    base.NoopSetConfig(),
		},
		FingerprintF: device.StaticFingerprinter([]*device.DeviceGroup{intelDeviceGroup}),
		ReserveF:     deviceReserveFn,
		StatsF:       device.StaticStats([]*device.DeviceGroupStats{intelDeviceGroupStats}),
	}
	pluginIntel := loader.MockBasicExternalPlugin(deviceIntel)

	// Configure the catalog with two plugins
	configureCatalogWith(catalog, map[*base.PluginInfoResponse]loader.PluginInstance{
		pluginInfoNvidia: pluginNvidia,
		pluginInfoIntel:  pluginIntel,
	})
}

// Test collecting statistics from all devices
func TestManager_AllStats(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	config, updateCh, catalog := baseTestConfig(t)
	nvidiaAndIntelDefaultPlugins(catalog)

	m := New(config)
	go m.Run()
	defer m.Shutdown()

	// Wait till we get a fingerprint result
	select {
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	case devices := <-updateCh:
		require.Len(devices, 2)
	}

	// Now collect all the stats
	var stats []*device.DeviceGroupStats
	testutil.WaitForResult(func() (bool, error) {
		stats = m.AllStats()
		l := len(stats)
		if l == 2 {
			return true, nil
		}

		return false, fmt.Errorf("expected count 2; got %d", l)
	}, func(err error) {
		t.Fatal(err)
	})

	// Check we got stats from both the devices
	var nstats, istats bool
	for _, stat := range stats {
		switch stat.Vendor {
		case "intel":
			istats = true
		case "nvidia":
			nstats = true
		default:
			t.Fatalf("unexpected vendor %q", stat.Vendor)
		}
	}
	require.True(nstats)
	require.True(istats)
}

// Test collecting statistics from a particular device
func TestManager_DeviceStats(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	config, updateCh, catalog := baseTestConfig(t)
	nvidiaAndIntelDefaultPlugins(catalog)

	m := New(config)
	go m.Run()
	defer m.Shutdown()

	// Wait till we get a fingerprint result
	select {
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	case devices := <-updateCh:
		require.Len(devices, 2)
	}

	testutil.WaitForResult(func() (bool, error) {
		stats := m.AllStats()
		l := len(stats)
		if l == 2 {
			t.Logf("% #v", pretty.Formatter(stats))
			return true, nil
		}

		return false, fmt.Errorf("expected count 2; got %d", l)
	}, func(err error) {
		t.Fatal(err)
	})

	// Now collect the stats for one nvidia device
	stat, err := m.DeviceStats(&structs.AllocatedDeviceResource{
		Vendor:    "nvidia",
		Type:      "gpu",
		Name:      "1080ti",
		DeviceIDs: []string{nvidiaDevice1ID},
	})
	require.NoError(err)
	require.NotNil(stat)

	require.Len(stat.InstanceStats, 1)
	require.Contains(stat.InstanceStats, nvidiaDevice1ID)

	istat := stat.InstanceStats[nvidiaDevice1ID]
	require.EqualValues(218, istat.Summary.IntNumeratorVal)
}

// Test reserving a particular device
func TestManager_Reserve(t *testing.T) {
	t.Parallel()
	r := require.New(t)

	config, updateCh, catalog := baseTestConfig(t)
	nvidiaAndIntelDefaultPlugins(catalog)

	m := New(config)
	go m.Run()
	defer m.Shutdown()

	// Wait till we get a fingerprint result
	select {
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	case devices := <-updateCh:
		r.Len(devices, 2)
	}

	cases := []struct {
		in       *structs.AllocatedDeviceResource
		expected string
		err      bool
	}{
		{
			in: &structs.AllocatedDeviceResource{
				Vendor:    "nvidia",
				Type:      "gpu",
				Name:      "1080ti",
				DeviceIDs: []string{nvidiaDevice1ID},
			},
			expected: nvidiaDevice1ID,
		},
		{
			in: &structs.AllocatedDeviceResource{
				Vendor:    "nvidia",
				Type:      "gpu",
				Name:      "1080ti",
				DeviceIDs: []string{nvidiaDevice0ID},
			},
			expected: nvidiaDevice0ID,
		},
		{
			in: &structs.AllocatedDeviceResource{
				Vendor:    "nvidia",
				Type:      "gpu",
				Name:      "1080ti",
				DeviceIDs: []string{nvidiaDevice0ID, nvidiaDevice1ID},
			},
			expected: fmt.Sprintf("%s,%s", nvidiaDevice0ID, nvidiaDevice1ID),
		},
		{
			in: &structs.AllocatedDeviceResource{
				Vendor:    "nvidia",
				Type:      "gpu",
				Name:      "1080ti",
				DeviceIDs: []string{nvidiaDevice0ID, nvidiaDevice1ID, "foo"},
			},
			err: true,
		},
		{
			in: &structs.AllocatedDeviceResource{
				Vendor:    "intel",
				Type:      "gpu",
				Name:      "640GT",
				DeviceIDs: []string{intelDeviceID},
			},
			expected: intelDeviceID,
		},
		{
			in: &structs.AllocatedDeviceResource{
				Vendor:    "intel",
				Type:      "gpu",
				Name:      "foo",
				DeviceIDs: []string{intelDeviceID},
			},
			err: true,
		},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			r = require.New(t)

			// Reserve a particular device
			res, err := m.Reserve(c.in)
			if !c.err {
				r.NoError(err)
				r.NotNil(res)

				r.Len(res.Envs, 1)
				r.Equal(res.Envs["DEVICES"], c.expected)
			} else {
				r.Error(err)
			}
		})
	}
}

// Test that shutdown shutsdown the plugins
func TestManager_Shutdown(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	config, updateCh, catalog := baseTestConfig(t)
	nvidiaAndIntelDefaultPlugins(catalog)

	m := New(config)
	go m.Run()
	defer m.Shutdown()

	// Wait till we get a fingerprint result
	select {
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	case devices := <-updateCh:
		require.Len(devices, 2)
	}

	// Call shutdown and assert that we killed the plugins
	m.Shutdown()

	for _, resp := range catalog.Catalog()[base.PluginTypeDevice] {
		pinst, _ := catalog.Dispense(resp.Name, resp.Type, &base.ClientAgentConfig{}, config.Logger)
		require.True(pinst.Exited())
	}
}

// Test that startup shutsdown previously launched plugins
func TestManager_Run_ShutdownOld(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	config, updateCh, catalog := baseTestConfig(t)
	nvidiaAndIntelDefaultPlugins(catalog)

	m := New(config)
	go m.Run()
	defer m.Shutdown()

	// Wait till we get a fingerprint result
	select {
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	case devices := <-updateCh:
		require.Len(devices, 2)
	}

	// Create a new manager with the same config so that it reads the old state
	m2 := New(config)
	go m2.Run()
	defer m2.Shutdown()

	testutil.WaitForResult(func() (bool, error) {
		for _, resp := range catalog.Catalog()[base.PluginTypeDevice] {
			pinst, _ := catalog.Dispense(resp.Name, resp.Type, &base.ClientAgentConfig{}, config.Logger)
			if !pinst.Exited() {
				return false, fmt.Errorf("plugin %q not shutdown", resp.Name)
			}
		}

		return true, nil
	}, func(err error) {
		t.Fatal(err)
	})
}
