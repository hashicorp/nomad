package pluginmanager

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"
)

var (
	// DefaultManagerReadyTimeout is the default amount of time we will wait
	// for a plugin mananger to be ready before logging it and moving on.
	DefaultManagerReadyTimeout = time.Second * 5
)

// PluginGroup is a utility struct to manage a collectively orchestrate a
// set of PluginManagers
type PluginGroup struct {
	// managers is the set of managers managed, access is synced by mLock
	managers []PluginManager

	// shutdown indicates if shutdown has been called, access is synced by mLock
	shutdown bool

	// mLock gaurds manangers and shutdown
	mLock sync.Mutex

	logger log.Logger
}

// New returns an initialized PluginGroup
func New(logger log.Logger) *PluginGroup {
	return &PluginGroup{
		managers: []PluginManager{},
		logger:   logger.Named("plugin"),
	}
}

// RegisterAndRun registers the manager and starts it in a separate goroutine
func (m *PluginGroup) RegisterAndRun(manager PluginManager) error {
	m.mLock.Lock()
	defer m.mLock.Unlock()
	if m.shutdown {
		return fmt.Errorf("plugin group already shutdown")
	}
	m.managers = append(m.managers, manager)

	go func() {
		m.logger.Info("starting plugin manager", "plugin-type", manager.PluginType())
		manager.Run()
		m.logger.Info("plugin manager finished", "plugin-type", manager.PluginType())
	}()
	return nil
}

// Ready returns a channel which will be closed once all plugin manangers are ready.
// A timeout for waiting on each manager is given
func (m *PluginGroup) Ready(ctx context.Context) (<-chan struct{}, error) {
	m.mLock.Lock()
	defer m.mLock.Unlock()
	if m.shutdown {
		return nil, fmt.Errorf("plugin group already shutdown")
	}

	var wg sync.WaitGroup
	for i := range m.managers {
		manager := m.managers[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case <-manager.Ready():
			case <-ctx.Done():
				m.logger.Warn("timeout waiting for plugin manager to be ready",
					"plugin-type", manager.PluginType())
			}
		}()
	}

	ret := make(chan struct{})
	go func() {
		wg.Wait()
		close(ret)
	}()
	return ret, nil
}

// Shutdown shutsdown all registered PluginManagers in reverse order of how
// they were started.
func (m *PluginGroup) Shutdown() {
	m.mLock.Lock()
	defer m.mLock.Unlock()
	for i := len(m.managers) - 1; i >= 0; i-- {
		m.logger.Info("shutting down plugin manager", "plugin-type", m.managers[i].PluginType())
		m.managers[i].Shutdown()
	}
	m.shutdown = true
}
