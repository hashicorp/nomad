// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package drivermanager

import (
	"context"
	"errors"
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
)

func TestInstanceManager_dispense(t *testing.T) {
	// This subtest wraps the existing test here that covers the logic of
	// the dispense function's behavior.
	t.Run("logic", func(t *testing.T) {
		dispenseCount := 0
		reattachCount := 0
		resetCounts := func() {
			dispenseCount = 0
			reattachCount = 0
		}

		dispenseF := func(string, string, *base.AgentConfig, log.Logger) (loader.PluginInstance, error) {
			dispenseCount++
			return loader.MockBasicExternalPlugin(&dtu.MockDriver{}, "0.1.0"), nil
		}
		reattachF := func(string, string, *plugin.ReattachConfig) (loader.PluginInstance, error) {
			reattachCount++
			return loader.MockBasicExternalPlugin(&dtu.MockDriver{}, "0.1.0"), nil
		}
		cat := &loader.MockCatalog{
			DispenseF: dispenseF,
			ReattachF: reattachF,
			CatalogF: func() map[string][]*base.PluginInfoResponse {
				return map[string][]*base.PluginInfoResponse{
					base.PluginTypeDriver: {&base.PluginInfoResponse{Name: "mock", Type: base.PluginTypeDriver}},
				}
			},
		}
		resetCatalog := func() {
			cat.DispenseF = dispenseF
			cat.ReattachF = reattachF
		}

		ctx, cancel := context.WithCancel(t.Context())
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
		unexpectedDispense := must.Sprint("unexpected number of dispense calls")
		unexpectedReattach := must.Sprint("unexpected number of reattach calls")
		differentPlugins := must.Sprint("dispensed plugins are not the same")

		// First test the happy path, no reattach config is stored, plugin dispenses without error
		fetchRet = false
		plug, err := i.dispense()
		must.NoError(t, err)
		must.One(t, dispenseCount, unexpectedDispense)
		must.Zero(t, reattachCount, unexpectedReattach)

		// Dispensing a second time should not dispense a new plugin from the catalog, but reuse the existing
		plug2, err := i.dispense()
		must.NoError(t, err)
		must.One(t, dispenseCount, unexpectedDispense)
		must.Zero(t, reattachCount, unexpectedReattach)
		must.Eq(t, plug, plug2, differentPlugins)

		// If the plugin has exited test that the manager attempts to retry dispense
		resetCounts()
		i.plugin = nil
		cat.DispenseF = func(string, string, *base.AgentConfig, log.Logger) (loader.PluginInstance, error) {
			dispenseCount++
			return loader.MockBasicExternalPlugin(&dtu.MockDriver{}, "0.1.0"), singleton.SingletonPluginExited
		}
		_, err = i.dispense()
		must.Error(t, err)
		must.Eq(t, 2, dispenseCount, unexpectedDispense)
		must.Zero(t, reattachCount, unexpectedReattach)

		// Test that when a reattach config exists it attempts plugin reattachment
		// First case is when plugin reattachment is successful
		fetchRet = true
		resetCounts()
		resetCatalog()
		i.plugin = nil
		plug, err = i.dispense()
		must.NoError(t, err)
		must.Zero(t, dispenseCount, unexpectedDispense)
		must.One(t, reattachCount, unexpectedReattach)
		// Dispensing a second time should not dispense a new plugin from the catalog
		plug2, err = i.dispense()
		must.NoError(t, err)
		must.Zero(t, dispenseCount, unexpectedDispense)
		must.One(t, reattachCount, unexpectedReattach)
		must.Eq(t, plug, plug2, differentPlugins)

		// Finally test when reattachment fails. A new plugin should be dispensed
		resetCounts()
		resetCatalog()
		i.plugin = nil
		cat.ReattachF = func(string, string, *plugin.ReattachConfig) (loader.PluginInstance, error) {
			reattachCount++
			return nil, fmt.Errorf("failed to dispense")
		}
		plug, err = i.dispense()
		must.NoError(t, err)
		must.One(t, dispenseCount, unexpectedDispense)
		must.One(t, reattachCount, unexpectedReattach)
		// Dispensing a second time should not dispense a new plugin from the catalog
		plug2, err = i.dispense()
		must.NoError(t, err)
		must.One(t, dispenseCount, unexpectedDispense)
		must.One(t, reattachCount, unexpectedReattach)
		must.Eq(t, plug, plug2, differentPlugins)
	})

	// These tests cover the dispense behavior around the DriverIniter interface
	// for driver plugins that do and do not implement the optional interface.
	t.Run("DriverIniter", func(t *testing.T) {
		mkInstance := func(t *testing.T, impl drivers.DriverPlugin) (*instanceManager, *loader.MockInstance) {
			ctx, cancel := context.WithCancel(t.Context())
			d := dtu.NewTestGRPCDriver(t, impl)
			instance, err := d.Client.Dispense(base.PluginTypeDriver)
			must.NoError(t, err)
			instPlug := loader.MockBasicExternalPlugin(instance, "0.1.0")
			cat := &loader.MockCatalog{
				DispenseF: func(string, string, *base.AgentConfig, log.Logger) (loader.PluginInstance, error) {
					return instPlug, nil
				},
			}

			return &instanceManager{
				logger:        testlog.HCLogger(t),
				ctx:           ctx,
				cancel:        cancel,
				loader:        cat,
				storeReattach: func(*plugin.ReattachConfig) error { return nil },
				fetchReattach: func() (*plugin.ReattachConfig, bool) { return nil, false },
				pluginConfig:  &base.AgentConfig{},
				id:            &loader.PluginID{Name: "mock", PluginType: base.PluginTypeDriver},
			}, instPlug
		}

		t.Run("not implemented", func(t *testing.T) {
			instance, _ := mkInstance(t, &dtu.MockDriver{})
			result, err := instance.dispense()
			must.NoError(t, err)
			must.NotNil(t, result)
		})

		t.Run("ok", func(t *testing.T) {
			var called bool
			instance, _ := mkInstance(t, &dtu.MockDriverInit{
				InitF: func(context.Context) error {
					called = true
					return nil
				},
			})

			result, err := instance.dispense()
			must.NoError(t, err)
			must.NotNil(t, result)
			must.True(t, called, must.Sprint("Init was not called on plugin"))
		})

		t.Run("error", func(t *testing.T) {
			testErr := errors.New("test-error")
			instance, extPlug := mkInstance(t, &dtu.MockDriverInit{
				InitF: func(context.Context) error {
					return testErr
				},
			})

			result, err := instance.dispense()
			must.Nil(t, result)
			must.Error(t, err, must.Sprint("Init was not called on plugin"))
			must.ErrorContains(t, err, testErr.Error())
			must.True(t, extPlug.Exited(), must.Sprint("plugin was not killed"))
		})
	})
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
