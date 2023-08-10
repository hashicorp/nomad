// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package pluginmanager

import (
	"context"
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

	// mLock gaurds managers and shutdown
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

	m.logger.Info("starting plugin manager", "plugin-type", manager.PluginType())
	manager.Run()
	return nil
}

// WaitForFirstFingerprint returns a channel which will be closed once all
// plugin managers are ready. A timeout for waiting on each manager is given
func (m *PluginGroup) WaitForFirstFingerprint(ctx context.Context) (<-chan struct{}, error) {
	m.mLock.Lock()
	defer m.mLock.Unlock()
	if m.shutdown {
		return nil, fmt.Errorf("plugin group already shutdown")
	}

	var wg sync.WaitGroup
	for i := range m.managers {
		manager, ok := m.managers[i].(FingerprintingPluginManager)
		if !ok {
			continue
		}
		logger := m.logger.With("plugin-type", manager.PluginType())
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.Debug("waiting on plugin manager initial fingerprint")

			select {
			case <-manager.WaitForFirstFingerprint(ctx):
				logger.Debug("finished plugin manager initial fingerprint")
			case <-ctx.Done():
				logger.Warn("timeout waiting for plugin manager to be ready")
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
		m.logger.Info("plugin manager finished", "plugin-type", m.managers[i].PluginType())
	}
	m.shutdown = true
}
