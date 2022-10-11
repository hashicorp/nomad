package loggingmanager

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-plugin"

	"github.com/hashicorp/nomad/client/pluginmanager"
	"github.com/hashicorp/nomad/client/pluginmanager/loggingmanager/state"
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/plugins/base"
	loglib "github.com/hashicorp/nomad/plugins/logging/lib"
	pstructs "github.com/hashicorp/nomad/plugins/shared/structs"
)

// Manager is the interface implemented by manager, below
type Manager interface {
	pluginmanager.PluginManager

	Start(name string, config *loglib.LogConfig) error
	Stop(name string, config *loglib.LogConfig) error
}

// manager orchestrates launching logging plugins.
type manager struct {
	// logger is used by the logging plugin manager (i.e. for our own logs, not
	// for the plugins or the logs the plugins are reading)
	logger hclog.Logger

	// global context and cancel function for this manager
	ctx    context.Context
	cancel context.CancelFunc

	// state is used to manage the plugin manager's state
	state StateStorage

	// loader is the plugin loader
	loader loader.PluginCatalog

	// instances is the set of managed logging plugins
	instances   map[loader.PluginID]*instanceManager
	instancesMu sync.RWMutex

	// fingerprintResCh is triggered when there are new plugins
	fingerprintResCh chan struct{}

	// pluginConfig is the config passed to the launched plugins
	pluginConfig *base.AgentConfig

	// reattachConfigs stores the plugin reattach configs
	reattachConfigs    map[loader.PluginID]*pstructs.ReattachConfig
	reattachConfigLock sync.Mutex

	// readyCh is ticked once at the end of Run()
	readyCh chan struct{}
}

// Config is used to configure a plugin manager
type Config struct {
	// Logger is for the plugin manager's own logs
	Logger hclog.Logger

	// Loader is the plugin loader
	Loader loader.PluginCatalog

	// PluginConfig is the config passed to the launched plugins
	PluginConfig *base.AgentConfig

	// State is used to manage the plugin managers manager's state
	State StateStorage

	// FingerprintOutCh is used to emit new fingerprinted plugins
	FingerprintOutCh chan<- struct{}
}

// New returns a new logging plugin manager
func New(c *Config) *manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &manager{
		logger:          c.Logger.Named("logging_mgr"),
		state:           c.State,
		ctx:             ctx,
		cancel:          cancel,
		loader:          c.Loader,
		pluginConfig:    c.PluginConfig,
		instances:       make(map[loader.PluginID]*instanceManager),
		reattachConfigs: make(map[loader.PluginID]*pstructs.ReattachConfig),
		readyCh:         make(chan struct{}),
	}
}

// PluginType is the type of plugin which the manager manages
func (*manager) PluginType() string { return base.PluginTypeLogging }

// Run loads the logging plugins from the catalog and starts them up. It
// maintains a handle to each one so that the logging_hook can ask the plugin to
// set up log shipping.
func (m *manager) Run() {
	// Check if there are any plugins that didn't get cleanly shutdown before
	// and if there are shut them down.
	m.cleanupStalePlugins()

	plugins := m.loader.Catalog()[base.PluginTypeLogging]
	if len(plugins) == 0 {
		m.logger.Debug("exiting since there are no logging plugins")
		m.cancel()
		return
	}

	for _, d := range plugins {
		id := loader.PluginInfoID(d)
		storeFn := func(c *plugin.ReattachConfig) error {
			id := id
			return m.storePluginReattachConfig(id, c)
		}
		im := newInstanceManager(&instanceManagerConfig{
			Logger:        m.logger,
			Ctx:           m.ctx,
			Loader:        m.loader,
			StoreReattach: storeFn,
			PluginConfig:  m.pluginConfig,
			Id:            &id,
		})
		go im.run()

		m.instancesMu.Lock()
		m.instances[id] = im
		m.instancesMu.Unlock()
	}
	// signal ready
	close(m.readyCh)
}

// Shutdown stops any in-flight requests and shuts down the plugins. Plugins are
// expected to keep their log shippers running.
func (m *manager) Shutdown() {
	m.cancel()
	m.instancesMu.RLock()
	defer m.instancesMu.RUnlock()
	for _, i := range m.instances {
		i.cleanup()
	}
}

// StateStorage is used to persist the logging plugin managers state across
// agent restarts.
type StateStorage interface {
	// GetLoggingPluginState is used to retrieve the logging plugin manager's plugin
	// state.
	GetLoggingPluginState() (*state.PluginState, error)

	// PutLoggingPluginState is used to store the logging plugin manager's plugin
	// state.
	PutLoggingPluginState(state *state.PluginState) error
}

// StorePluginReattachFn is used to store plugin reattachment configurations.
type StorePluginReattachFn func(*plugin.ReattachConfig) error

func (m *manager) WaitForFirstFingerprint(ctx context.Context) <-chan struct{} {
	ctx, cancel := context.WithCancel(ctx)
	go m.waitForFirstFingerprint(ctx, cancel)
	return ctx.Done()
}

func (m *manager) waitForFirstFingerprint(ctx context.Context, cancel context.CancelFunc) {
	defer cancel()
	// We don't want to start initial fingerprint wait until Run loop has
	// finished
	select {
	case <-m.readyCh:
	case <-ctx.Done():
		return // parent context canceled or timedout
	case <-m.ctx.Done():
		return // shutdown called
	}

	// wait for each plugin to finish initial fingerprint
	var wg sync.WaitGroup

	m.instancesMu.RLock()
	for _, instance := range m.instances {
		wg.Add(1)
		go func(instance *instanceManager) {
			defer wg.Done()
			instance.WaitForFirstFingerprint(ctx)
		}(instance)
	}
	m.instancesMu.RUnlock()
	wg.Wait()
}

func (m *manager) Start(name string, config *loglib.LogConfig) error {
	m.instancesMu.RLock()
	defer m.instancesMu.RUnlock()

	instance := m.instances[loader.PluginID{Name: name, PluginType: base.PluginTypeLogging}]
	if instance == nil {
		// TODO: should we be able to return a default here?
		return fmt.Errorf("unknown logging plugin %q", name)
	}
	return instance.Start(config)
}

func (m *manager) Stop(name string, config *loglib.LogConfig) error {
	m.instancesMu.RLock()
	defer m.instancesMu.RUnlock()

	instance := m.instances[loader.PluginID{Name: name, PluginType: base.PluginTypeLogging}]
	if instance == nil {
		return nil
	}
	return instance.Stop(config)
}

// cleanupStalePlugins reads the logging plugin managers state and shuts down
// any previously launched plugin.
func (m *manager) cleanupStalePlugins() error {

	// Read the old plugin state
	s, err := m.state.GetLoggingPluginState()
	if err != nil {
		return fmt.Errorf("failed to read plugin state: %v", err)
	}

	// No state was stored so there is nothing to do.
	if s == nil {
		return nil
	}

	// For each plugin go through and try to shut it down
	var mErr multierror.Error
	for name, c := range s.ReattachConfigs {
		rc, err := pstructs.ReattachConfigToGoPlugin(c)
		if err != nil {
			_ = multierror.Append(&mErr, fmt.Errorf("failed to convert reattach config: %v", err))
			continue
		}

		instance, err := m.loader.Reattach(name, base.PluginTypeLogging, rc)
		if err != nil {
			_ = multierror.Append(&mErr, fmt.Errorf("failed to reattach to plugin %q: %v", name, err))
			continue
		}

		// Kill the instance
		instance.Kill()
	}

	return mErr.ErrorOrNil()
}

// storePluginReattachConfig is used as a callback to the instance managers and
// persists thhe plugin reattach configurations.
func (m *manager) storePluginReattachConfig(id loader.PluginID, c *plugin.ReattachConfig) error {
	m.reattachConfigLock.Lock()
	defer m.reattachConfigLock.Unlock()

	// Store the new reattach config
	m.reattachConfigs[id] = pstructs.ReattachConfigFromGoPlugin(c)

	// Persist the state
	s := &state.PluginState{
		ReattachConfigs: make(map[string]*pstructs.ReattachConfig, len(m.reattachConfigs)),
	}

	for id, c := range m.reattachConfigs {
		s.ReattachConfigs[id.Name] = c
	}

	return m.state.PutLoggingPluginState(s)
}
