package storagemanager

import (
	"context"
	"fmt"
	"sync"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/pluginmanager"
	"github.com/hashicorp/nomad/client/storage"
	"github.com/hashicorp/nomad/nomad/structs"
)

// ErrPluginNotFound is returned during Dispense when the requested plugin
// is not found in the catalog. This will occur before a driver has first been
// fingerprinted or when the plugin does not exist.
// TODO: Name Plugins in Config to remove this ambiguity.
var ErrPluginNotFound = fmt.Errorf("plugin not found")

// Manager is the interface used to manage storage plugins
type Manager interface {
	pluginmanager.PluginManager

	// Dispense returns a storage.StoragePlugin for the given storage plugin name
	// Dispense(plugin string) (storage.StoragePlugin, error)
}

// UpdateNodeStorageInfoFn is the callback used to update the node from
// fingerprinting.
// The first argument is the name of the plugin as defined in the CSI spec and
// the second is the fingerprint result.
type UpdateNodeStorageInfoFn func(string, *structs.StoragePluginInfo)

func New(logger hclog.Logger, loader storage.PluginCatalog, updater UpdateNodeStorageInfoFn) Manager {
	return &manager{
		logger:  logger.Named("storage_mgr"),
		loader:  loader,
		updater: updater,
	}
}

// manager is a struct that manages a group of storage plugins
type manager struct {
	logger hclog.Logger

	// loader is the plugin loader
	loader storage.PluginCatalog

	// cancelFn ends the manager context and shuts down all plugin managers
	cancelFn context.CancelFunc

	// updater is the function call back passed to instance managers for returning
	// fingerprint info back to the client
	updater UpdateNodeStorageInfoFn

	// managers is a map of initialized instance managers
	managers     map[string]*instanceManager
	managersLock sync.RWMutex
}

func (m *manager) Run() {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFn = cancel

	m.logger.Info("Starting Storage manager", "configs", m.loader.Configs())

	m.managersLock.Lock()
	defer m.managersLock.Unlock()

	instanceManagers := make(map[string]*instanceManager, len(m.loader.Configs()))

	for name, cfg := range m.loader.Configs() {
		m.logger.Info("Starting manager", "name", name, "addr", cfg.Address)
		instanceManagers[name] = newInstanceManager(name, &instanceManagerConfig{
			Logger:               m.logger,
			Ctx:                  ctx,
			Cfg:                  cfg,
			UpdateNodeFromPlugin: m.updater,
		})
	}

	m.managers = instanceManagers
}

func (m *manager) Shutdown() {
	m.logger.Info("Shutting down storage manager")
	m.cancelFn()
}

func (m *manager) PluginType() string {
	return "storage"
}

// WaitForFirstFingerprint returns a channel that is closed once all plugin
// instances managed by the plugin manager have fingerprinted once. A
// context can be passed which when done will also close the channel
func (m *manager) WaitForFirstFingerprint(ctx context.Context) <-chan struct{} {
	resultCh := make(chan struct{}, 1)
	var wg sync.WaitGroup

	m.managersLock.RLock()
	defer m.managersLock.RUnlock()

	for _, im := range m.managers {
		wg.Add(1)
		go func(im *instanceManager) {
			im.WaitForFirstFingerprint(ctx)
			wg.Done()
		}(im)
	}

	go func() {
		wg.Wait()
		m.logger.Trace("finished initial fingerprinting")
		close(resultCh)
	}()

	return resultCh
}
