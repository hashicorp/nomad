package pluginmanager

import (
	"fmt"
	"sync"

	log "github.com/hashicorp/go-hclog"
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
	if m.shutdown {
		return fmt.Errorf("plugin group already shutdown")
	}
	m.managers = append(m.managers, manager)
	m.mLock.Unlock()

	go func() {
		m.logger.Info("starting plugin manager", "plugin-type", manager.PluginType())
		manager.Run()
		m.logger.Info("plugin manager finished", "plugin-type", manager.PluginType())
	}()
	return nil
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
