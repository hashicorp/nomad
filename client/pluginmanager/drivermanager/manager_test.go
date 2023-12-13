// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package drivermanager

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	log "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/pluginmanager"
	"github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	dtu "github.com/hashicorp/nomad/plugins/drivers/testutils"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ Manager = (*manager)(nil)
var _ pluginmanager.PluginManager = (*manager)(nil)

func testSetup(t *testing.T) (chan *drivers.Fingerprint, chan *drivers.TaskEvent, *manager) {
	fpChan := make(chan *drivers.Fingerprint)
	evChan := make(chan *drivers.TaskEvent)
	drv := mockDriver(fpChan, evChan)
	cat := mockCatalog(map[string]drivers.DriverPlugin{"mock": drv})
	cfg := &Config{
		Logger:              testlog.HCLogger(t),
		Loader:              cat,
		PluginConfig:        &base.AgentConfig{},
		Updater:             noopUpdater,
		EventHandlerFactory: noopEventHandlerFactory,
		State:               state.NoopDB{},
		AllowedDrivers:      make(map[string]struct{}),
		BlockedDrivers:      make(map[string]struct{}),
	}

	mgr := New(cfg)
	return fpChan, evChan, mgr
}

func mockDriver(fpChan chan *drivers.Fingerprint, evChan chan *drivers.TaskEvent) drivers.DriverPlugin {
	return &dtu.MockDriver{
		FingerprintF: func(ctx context.Context) (<-chan *drivers.Fingerprint, error) {
			return fpChan, nil
		},
		TaskEventsF: func(ctx context.Context) (<-chan *drivers.TaskEvent, error) {
			return evChan, nil
		},
	}
}

func mockCatalog(drivers map[string]drivers.DriverPlugin) *loader.MockCatalog {
	cat := map[string][]*base.PluginInfoResponse{
		base.PluginTypeDriver: {},
	}
	for d := range drivers {
		cat[base.PluginTypeDriver] = append(cat[base.PluginTypeDriver], &base.PluginInfoResponse{
			Name: d,
			Type: base.PluginTypeDriver,
		})
	}

	return &loader.MockCatalog{
		DispenseF: func(name, pluginType string, cfg *base.AgentConfig, logger log.Logger) (loader.PluginInstance, error) {
			d, ok := drivers[name]
			if !ok {
				return nil, fmt.Errorf("driver not found")
			}
			return loader.MockBasicExternalPlugin(d, "0.1.0"), nil
		},
		ReattachF: func(name, pluginType string, config *plugin.ReattachConfig) (loader.PluginInstance, error) {
			d, ok := drivers[name]
			if !ok {
				return nil, fmt.Errorf("driver not found")
			}
			return loader.MockBasicExternalPlugin(d, "0.1.0"), nil
		},
		CatalogF: func() map[string][]*base.PluginInfoResponse {
			return cat
		},
	}
}

func mockTaskEvent(taskID string) *drivers.TaskEvent {
	return &drivers.TaskEvent{
		TaskID:      taskID,
		Timestamp:   time.Now(),
		Annotations: map[string]string{},
		Message:     "event from " + taskID,
	}
}

func noopUpdater(string, *structs.DriverInfo)             {}
func noopEventHandlerFactory(string, string) EventHandler { return nil }

func TestManager_Fingerprint(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	fpChan, _, mgr := testSetup(t)
	var infos []*structs.DriverInfo
	mgr.updater = func(d string, i *structs.DriverInfo) {
		infos = append(infos, i)
	}
	go mgr.Run()
	defer mgr.Shutdown()
	fpChan <- &drivers.Fingerprint{Health: drivers.HealthStateHealthy}
	testutil.WaitForResult(func() (bool, error) {
		mgr.instancesMu.Lock()
		defer mgr.instancesMu.Unlock()
		return len(mgr.instances) == 1, fmt.Errorf("manager should have registered 1 instance")
	}, func(err error) {
		require.NoError(err)
	})

	testutil.WaitForResult(func() (bool, error) {
		mgr.instancesMu.Lock()
		defer mgr.instancesMu.Unlock()
		if mgr.instances["mock"].getLastHealth() != drivers.HealthStateHealthy {
			return false, fmt.Errorf("mock instance should be healthy")
		}
		return true, nil
	}, func(err error) {
		require.NoError(err)
	})

	fpChan <- &drivers.Fingerprint{
		Health: drivers.HealthStateUnhealthy,
	}
	testutil.WaitForResult(func() (bool, error) {
		mgr.instancesMu.Lock()
		defer mgr.instancesMu.Unlock()
		if mgr.instances["mock"].getLastHealth() == drivers.HealthStateHealthy {
			return false, fmt.Errorf("mock instance should be unhealthy")
		}
		return true, nil
	}, func(err error) {
		require.NoError(err)
	})

	fpChan <- &drivers.Fingerprint{
		Health: drivers.HealthStateUndetected,
	}
	testutil.WaitForResult(func() (bool, error) {
		mgr.instancesMu.Lock()
		defer mgr.instancesMu.Unlock()
		if mgr.instances["mock"].getLastHealth() != drivers.HealthStateUndetected {
			return false, fmt.Errorf("mock instance should be undetected")
		}
		return true, nil
	}, func(err error) {
		require.NoError(err)
	})

	require.Len(infos, 3)
	require.True(infos[0].Healthy)
	require.True(infos[0].Detected)
	require.False(infos[1].Healthy)
	require.True(infos[1].Detected)
	require.False(infos[2].Healthy)
	require.False(infos[2].Detected)
}

func TestManager_TaskEvents(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	fpChan, evChan, mgr := testSetup(t)
	go mgr.Run()
	defer mgr.Shutdown()
	fpChan <- &drivers.Fingerprint{Health: drivers.HealthStateHealthy}
	testutil.WaitForResult(func() (bool, error) {
		mgr.instancesMu.Lock()
		defer mgr.instancesMu.Unlock()
		return len(mgr.instances) == 1, fmt.Errorf("manager should have registered 1 instance")
	}, func(err error) {
		require.NoError(err)
	})

	event1 := mockTaskEvent("abc1")
	var wg sync.WaitGroup
	wg.Add(1)
	mgr.instancesMu.Lock()
	mgr.instances["mock"].eventHandlerFactory = func(string, string) EventHandler {
		return func(ev *drivers.TaskEvent) {
			defer wg.Done()
			assert.Exactly(t, event1, ev)
		}
	}
	mgr.instancesMu.Unlock()

	evChan <- event1
	wg.Wait()
}

func TestManager_Run_AllowedDrivers(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	fpChan, _, mgr := testSetup(t)
	mgr.allowedDrivers = map[string]struct{}{"foo": {}}
	go mgr.Run()
	defer mgr.Shutdown()
	select {
	case fpChan <- &drivers.Fingerprint{Health: drivers.HealthStateHealthy}:
	default:
	}
	testutil.AssertUntil(200*time.Millisecond, func() (bool, error) {
		mgr.instancesMu.Lock()
		defer mgr.instancesMu.Unlock()
		return len(mgr.instances) == 0, fmt.Errorf("manager should have no registered instances")
	}, func(err error) {
		require.NoError(err)
	})
}

func TestManager_Run_BlockedDrivers(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	fpChan, _, mgr := testSetup(t)
	mgr.blockedDrivers = map[string]struct{}{"mock": {}}
	go mgr.Run()
	defer mgr.Shutdown()
	select {
	case fpChan <- &drivers.Fingerprint{Health: drivers.HealthStateHealthy}:
	default:
	}
	testutil.AssertUntil(200*time.Millisecond, func() (bool, error) {
		mgr.instancesMu.Lock()
		defer mgr.instancesMu.Unlock()
		return len(mgr.instances) == 0, fmt.Errorf("manager should have no registered instances")
	}, func(err error) {
		require.NoError(err)
	})
}

func TestManager_Run_AllowedBlockedDrivers_Combined(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	drvs := map[string]drivers.DriverPlugin{}
	fpChs := map[string]chan *drivers.Fingerprint{}
	names := []string{"mock1", "mock2", "mock3", "mock4", "mock5"}
	for _, d := range names {
		ch := make(chan *drivers.Fingerprint)
		drv := mockDriver(ch, nil)
		drvs[d] = drv
		fpChs[d] = ch
	}
	cat := mockCatalog(drvs)
	cfg := &Config{
		Logger:       testlog.HCLogger(t),
		Loader:       cat,
		PluginConfig: nil,
		Updater:      noopUpdater,
		State:        state.NoopDB{},
		AllowedDrivers: map[string]struct{}{
			"mock2": {},
			"mock3": {},
			"mock4": {},
			"foo":   {},
		},
		BlockedDrivers: map[string]struct{}{
			"mock2": {},
			"mock4": {},
			"bar":   {},
		},
	}
	mgr := New(cfg)

	go mgr.Run()
	defer mgr.Shutdown()
	for _, d := range names {
		go func(drv string) {
			select {
			case fpChs[drv] <- &drivers.Fingerprint{Health: drivers.HealthStateHealthy}:
			case <-time.After(200 * time.Millisecond):
			}
		}(d)
	}

	testutil.AssertUntil(250*time.Millisecond, func() (bool, error) {
		mgr.instancesMu.Lock()
		defer mgr.instancesMu.Unlock()
		return len(mgr.instances) < 2, fmt.Errorf("manager should have 1 registered instance, %v", len(mgr.instances))
	}, func(err error) {
		require.NoError(err)
	})
	mgr.instancesMu.Lock()
	require.Len(mgr.instances, 1)
	_, ok := mgr.instances["mock3"]
	mgr.instancesMu.Unlock()
	require.True(ok)
}
