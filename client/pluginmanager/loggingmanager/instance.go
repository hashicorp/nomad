package loggingmanager

import (
	"context"
	"fmt"
	"sync"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/helper/pluginutils/singleton"
	"github.com/hashicorp/nomad/plugins/base"
	bstructs "github.com/hashicorp/nomad/plugins/base/structs"
	logplugins "github.com/hashicorp/nomad/plugins/logging"
	loglib "github.com/hashicorp/nomad/plugins/logging/lib"
)

// instanceManagerConfig configures a logging instance manager
type instanceManagerConfig struct {
	// Logger is the logger used by the logging instance manager
	Logger log.Logger

	// Ctx is used to shutdown the logging instance manager
	Ctx context.Context

	// Loader is the plugin loader
	Loader loader.PluginCatalog

	// StoreReattach is used to store a plugins reattach config
	StoreReattach StorePluginReattachFn

	// PluginConfig is the config passed to the launched plugins
	PluginConfig *base.AgentConfig

	// FingerprintOutCh is used to emit new fingerprinted devices
	FingerprintOutCh chan<- struct{}

	// Id is the ID of the plugin being managed
	Id *loader.PluginID
}

// instanceManager is used to manage a single logging plugin
type instanceManager struct {
	// logger is the logger used by the logging instance manager
	logger log.Logger

	// ctx is used to shutdown the logging manager
	ctx context.Context

	// cancel is used to shutdown management of this logging plugin
	cancel context.CancelFunc

	// loader is the plugin loader
	loader loader.PluginCatalog

	// storeReattach is used to store a plugins reattach config
	storeReattach StorePluginReattachFn

	// pluginConfig is the config passed to the launched plugins
	pluginConfig *base.AgentConfig

	// id is the ID of the plugin being managed
	id *loader.PluginID

	// plugin is the plugin instance being managed
	plugin loader.PluginInstance

	// logging is the logging plugin being managed
	logging logplugins.LoggingPlugin

	// pluginLock locks access to the logging and plugin
	pluginLock sync.Mutex

	// shutdownLock is used to serialize attempts to shutdown
	shutdownLock sync.Mutex

	// fingerprintOutCh is used to emit new fingerprinted plugins
	fingerprintOutCh chan<- struct{}

	// ??
	loggingLock sync.RWMutex

	// firstFingerprintCh is used to trigger that we have successfully
	// fingerprinted once. It is used to gate launching the stats collection.
	firstFingerprintCh chan struct{}
	hasFingerprinted   bool
}

// newInstanceManager returns a new logging instance manager. It is expected that
// the context passed in the configuration is cancelled in order to shutdown
// launched goroutines.
func newInstanceManager(c *instanceManagerConfig) *instanceManager {

	ctx, cancel := context.WithCancel(c.Ctx)
	i := &instanceManager{
		logger:             c.Logger.With("plugin", c.Id.Name),
		ctx:                ctx,
		cancel:             cancel,
		loader:             c.Loader,
		storeReattach:      c.StoreReattach,
		pluginConfig:       c.PluginConfig,
		id:                 c.Id,
		firstFingerprintCh: make(chan struct{}),
	}

	return i
}

func (i *instanceManager) Start(config *loglib.LogConfig) error {
	loggingPlugin, err := i.dispense()
	if err != nil {
		i.logger.Error("dispensing plugin failed", "error", err)
		return err
	}
	return loggingPlugin.Start(config)
}

func (i *instanceManager) Stop(config *loglib.LogConfig) error {
	loggingPlugin, err := i.dispense()
	if err != nil {
		i.logger.Debug("dispensing plugin failed", "error", err)
		return nil
	}
	return loggingPlugin.Stop(config)
}

// WaitForFirstFingerprint waits until either the plugin fingerprints, the
// passed context is done, or the plugin instance manager is shutdown.
func (i *instanceManager) WaitForFirstFingerprint(ctx context.Context) {
	select {
	case <-i.ctx.Done():
	case <-ctx.Done():
	case <-i.firstFingerprintCh:
	}
}

// run is a long lived goroutine that starts the fingerprinting collection
// goroutine and then shutsdown the plugin on agent exit.
func (i *instanceManager) run() {
	// Dispense once to ensure we are given a valid plugin
	if _, err := i.dispense(); err != nil {
		i.logger.Error("dispensing initial plugin failed", "error", err)
		return
	}

	// This waitgroup will block on shutdown of the fingerprinting goroutine,
	// which will exit once the instanceManager's context is done
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		i.fingerprint()
		wg.Done()
	}()

	// Wait for a valid fingerprint and then block on shutdown
	select {
	case <-i.ctx.Done():
	case <-i.firstFingerprintCh:
	}

	wg.Wait()
	i.cleanup()
}

// dispense is used to dispense a plugin.
func (i *instanceManager) dispense() (logplugins.LoggingPlugin, error) {
	i.pluginLock.Lock()
	defer i.pluginLock.Unlock()

	// See if we already have a running instance
	if i.plugin != nil && !i.plugin.Exited() {
		return i.logging, nil
	}

	// Get an instance of the plugin
	pluginInstance, err := i.loader.Dispense(i.id.Name, i.id.PluginType, i.pluginConfig, i.logger)
	if err != nil {
		// Retry as the error just indicates the singleton has exited
		if err == singleton.SingletonPluginExited {
			pluginInstance, err = i.loader.Dispense(i.id.Name, i.id.PluginType, i.pluginConfig, i.logger)
		}

		// If we still have an error there is a real problem
		if err != nil {
			return nil, fmt.Errorf("failed to start plugin: %v", err)
		}
	}

	// Convert to a logging plugin
	plugin, ok := pluginInstance.Plugin().(logplugins.LoggingPlugin)
	if !ok {
		pluginInstance.Kill()
		return nil, fmt.Errorf("plugin loaded does not implement the logging plugin interface")
	}

	// Store the plugin and logging
	i.plugin = pluginInstance
	i.logging = plugin

	// Store the reattach config
	if c, ok := pluginInstance.ReattachConfig(); ok {
		i.storeReattach(c)
	}

	return plugin, nil
}

// cleanup shutsdown the plugin
func (i *instanceManager) cleanup() {
	i.shutdownLock.Lock()
	i.pluginLock.Lock()
	defer i.pluginLock.Unlock()
	defer i.shutdownLock.Unlock()

	if i.plugin != nil && !i.plugin.Exited() {
		i.plugin.Kill()
		i.storeReattach(nil)
	}
}

// fingerprint is a long lived routine used to fingerprint the logging
func (i *instanceManager) fingerprint() {

START:
	// Get a logging plugin
	loggingPlugin, err := i.dispense()
	if err != nil {
		i.logger.Error("dispensing plugin failed", "error", err)
		i.cancel()
		return
	}

	// Start fingerprinting
	fingerprintCh, err := loggingPlugin.Fingerprint(i.ctx)
	if err == logplugins.ErrPluginDisabled {
		i.logger.Info("fingerprinting failed: plugin is not enabled")
		i.handleFingerprintError()
		return
	} else if err != nil {
		i.logger.Error("fingerprinting failed", "error", err)
		i.handleFingerprintError()
		return
	}

	var fresp *logplugins.FingerprintResponse
	var ok bool
	for {
		select {
		case <-i.ctx.Done():
			return
		case fresp, ok = <-fingerprintCh:
		}

		if !ok {
			i.logger.Trace("exiting since fingerprinting gracefully shutdown")
			i.handleFingerprintError()
			return
		}

		// Guard against error by the plugin
		if fresp == nil {
			continue
		}

		// Handle any errors
		if fresp.Error != nil {
			if fresp.Error == bstructs.ErrPluginShutdown {
				i.logger.Error("plugin exited unexpectedly")
				goto START
			}

			i.logger.Error("fingerprinting returned an error", "error", fresp.Error)
			i.handleFingerprintError()
			return
		}

		if err := i.handleFingerprint(fresp); err != nil {
			// Cancel the context so we cleanup all goroutines
			i.logger.Error("returned loggings failed fingerprinting", "error", err)
			i.handleFingerprintError()
		}
	}
}

// handleFingerprintError exits the manager and shutsdown the plugin.
func (i *instanceManager) handleFingerprintError() {
	// Clear out the loggings and trigger a node update
	i.loggingLock.Lock()
	defer i.loggingLock.Unlock()

	// If we have fingerprinted before clear it out
	if i.hasFingerprinted {
		i.logging = nil
	}

	// Cancel the context so we cleanup all goroutines
	i.cancel()
}

// handleFingerprint receives the fingerprint from the plugin, sets a flag that
// it's been received, and alerts the manager via the fingerprintOutCh
func (i *instanceManager) handleFingerprint(f *logplugins.FingerprintResponse) error {
	i.loggingLock.Lock()
	defer i.loggingLock.Unlock()

	// TODO: once we start having data in the response, we'll want to validate
	// it here

	if !i.hasFingerprinted {
		close(i.firstFingerprintCh)
		i.hasFingerprinted = true
	}

	// Let the manager know we have a new fingerprint
	select {
	case i.fingerprintOutCh <- struct{}{}:
	default:
	}

	return nil
}
