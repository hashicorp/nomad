// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package drivermanager

import (
	"context"
	"fmt"
	"testing"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/helper/pluginutils/singleton"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	dtu "github.com/hashicorp/nomad/plugins/drivers/testutils"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockedCatalog struct {
	mock.Mock
}

func (m *mockedCatalog) Dispense(name, pluginType string, cfg *base.AgentConfig, logger log.Logger) (loader.PluginInstance, error) {
	args := m.Called(name, pluginType, cfg, logger)
	return loader.MockBasicExternalPlugin(&dtu.MockDriver{}, "0.1.0"), args.Error(0)
}

func (m *mockedCatalog) Reattach(name, pluginType string, config *plugin.ReattachConfig) (loader.PluginInstance, error) {
	args := m.Called(name, pluginType, config)
	return loader.MockBasicExternalPlugin(&dtu.MockDriver{}, "0.1.0"), args.Error(0)
}

func (m *mockedCatalog) Catalog() map[string][]*base.PluginInfoResponse {
	m.Called()
	return map[string][]*base.PluginInfoResponse{
		base.PluginTypeDriver: {&base.PluginInfoResponse{Name: "mock", Type: base.PluginTypeDriver}},
	}
}

func (m *mockedCatalog) resetMock() {
	m.ExpectedCalls = []*mock.Call{}
	m.Calls = []mock.Call{}
}

func TestInstanceManager_dispense(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cat := new(mockedCatalog)
	cat.Test(t)
	var fetchRet bool
	i := &instanceManager{
		logger:               testlog.HCLogger(t),
		ctx:                  ctx,
		cancel:               cancel,
		loader:               cat,
		storeReattach:        func(*plugin.ReattachConfig) error { return nil },
		fetchReattach:        func() (*plugin.ReattachConfig, bool) { return nil, fetchRet },
		pluginConfig:         &base.AgentConfig{},
		id:                   &loader.PluginID{Name: "mock", PluginType: base.PluginTypeDriver},
		updateNodeFromDriver: noopUpdater,
		eventHandlerFactory:  noopEventHandlerFactory,
		firstFingerprintCh:   make(chan struct{}),
	}
	require := require.New(t)

	// First test the happy path, no reattach config is stored, plugin dispenses without error
	fetchRet = false
	cat.On("Dispense", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	plug, err := i.dispense()
	require.NoError(err)
	cat.AssertNumberOfCalls(t, "Dispense", 1)
	cat.AssertNumberOfCalls(t, "Reattach", 0)

	// Dispensing a second time should not dispense a new plugin from the catalog, but reuse the existing
	plug2, err := i.dispense()
	require.NoError(err)
	cat.AssertNumberOfCalls(t, "Dispense", 1)
	cat.AssertNumberOfCalls(t, "Reattach", 0)
	require.Same(plug, plug2)

	// If the plugin has exited test that the manager attempts to retry dispense
	cat.resetMock()
	i.plugin = nil
	cat.On("Dispense", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(singleton.SingletonPluginExited)
	_, err = i.dispense()
	require.Error(err)
	cat.AssertNumberOfCalls(t, "Dispense", 2)
	cat.AssertNumberOfCalls(t, "Reattach", 0)

	// Test that when a reattach config exists it attempts plugin reattachment
	// First case is when plugin reattachment is successful
	fetchRet = true
	cat.resetMock()
	i.plugin = nil
	cat.On("Dispense", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	cat.On("Reattach", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	plug, err = i.dispense()
	require.NoError(err)
	cat.AssertNumberOfCalls(t, "Dispense", 0)
	cat.AssertNumberOfCalls(t, "Reattach", 1)
	// Dispensing a second time should not dispense a new plugin from the catalog
	plug2, err = i.dispense()
	require.NoError(err)
	cat.AssertNumberOfCalls(t, "Dispense", 0)
	cat.AssertNumberOfCalls(t, "Reattach", 1)
	require.Same(plug, plug2)

	// Finally test when reattachment fails. A new plugin should be dispensed
	cat.resetMock()
	i.plugin = nil
	cat.On("Dispense", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	cat.On("Reattach", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("failed to dispense"))
	plug, err = i.dispense()
	require.NoError(err)
	cat.AssertNumberOfCalls(t, "Dispense", 1)
	cat.AssertNumberOfCalls(t, "Reattach", 1)
	// Dispensing a second time should not dispense a new plugin from the catalog
	plug2, err = i.dispense()
	require.NoError(err)
	cat.AssertNumberOfCalls(t, "Dispense", 1)
	cat.AssertNumberOfCalls(t, "Reattach", 1)
	require.Same(plug, plug2)

}

func TestInstanceManager_cleanup(t *testing.T) {
	mkInstance := func(t *testing.T, impl drivers.DriverPlugin) *instanceManager {
		ctx, cancel := context.WithCancel(t.Context())
		d := dtu.NewTestGRPCDriver(t, impl)
		instance, err := d.Client.Dispense(base.PluginTypeDriver)
		must.NoError(t, err)
		pluginInst := loader.MockBasicExternalPlugin(instance, "")
		i := &instanceManager{
			logger:        testlog.HCLogger(t),
			ctx:           ctx,
			cancel:        cancel,
			storeReattach: func(*plugin.ReattachConfig) error { return nil },
			plugin:        pluginInst,
		}

		return i
	}

	t.Run("nil", func(t *testing.T) {
		instance := mkInstance(t, nil)
		must.NotPanic(t, instance.cleanup)
		must.NotNil(t, instance.ctx.Err(), must.Sprint("context should be canceled"))
	})

	t.Run("no shutdown", func(t *testing.T) {
		var reattachCalled bool
		m := &dtu.MockDriver{}
		instance := mkInstance(t, m)
		instance.storeReattach = func(*plugin.ReattachConfig) error {
			reattachCalled = true
			return nil
		}
		instance.cleanup()

		must.True(t, instance.plugin.Exited(), must.Sprint("plugin should have exited"))
		must.True(t, reattachCalled, must.Sprint("storeReattach was expected"))
		must.NotNil(t, instance.ctx.Err(), must.Sprint("context should be canceled"))
	})

	t.Run("with shutdown", func(t *testing.T) {
		var reattachCalled bool
		var shutdownCalled bool
		m := &dtu.MockDriverShutdown{
			ShutdownF: func(context.Context) error {
				shutdownCalled = true
				return nil
			},
		}
		instance := mkInstance(t, m)
		instance.storeReattach = func(*plugin.ReattachConfig) error {
			reattachCalled = true
			return nil
		}
		instance.cleanup()

		must.True(t, shutdownCalled, must.Sprint("shutdown was not called"))
		must.True(t, reattachCalled, must.Sprint("storeReattach was expected"))
		must.NotNil(t, instance.ctx.Err(), must.Sprint("context should be canceled"))
	})

	t.Run("already exited", func(t *testing.T) {
		var reattachCalled bool
		var shutdownCalled bool
		m := &dtu.MockDriverShutdown{
			ShutdownF: func(context.Context) error {
				shutdownCalled = true
				return nil
			},
		}
		instance := mkInstance(t, m)
		instance.storeReattach = func(*plugin.ReattachConfig) error {
			reattachCalled = true
			return nil
		}
		instance.plugin.Kill() // do this to mark plugin as exited

		instance.cleanup()

		must.False(t, shutdownCalled, must.Sprint("shutdown was called unexpectedly"))
		must.True(t, reattachCalled, must.Sprint("storeReattach was expected"))
		must.NotNil(t, instance.ctx.Err(), must.Sprint("context should be canceled"))
	})
}
