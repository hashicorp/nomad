//go:build !release
// +build !release

package allocrunner

import (
	"fmt"
	"sync"
	"testing"

	"github.com/hashicorp/nomad/client/allocwatcher"
	clientconfig "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/client/devicemanager"
	"github.com/hashicorp/nomad/client/lib/cgutil"
	"github.com/hashicorp/nomad/client/pluginmanager/drivermanager"
	"github.com/hashicorp/nomad/client/serviceregistration/checks/checkstore"
	"github.com/hashicorp/nomad/client/serviceregistration/mock"
	"github.com/hashicorp/nomad/client/serviceregistration/wrapper"
	"github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

// MockStateUpdater implements the AllocStateHandler interface and records
// alloc updates.
type MockStateUpdater struct {
	Updates []*structs.Allocation
	mu      sync.Mutex
}

// AllocStateUpdated implements the AllocStateHandler interface and records an
// alloc update.
func (m *MockStateUpdater) AllocStateUpdated(alloc *structs.Allocation) {
	m.mu.Lock()
	m.Updates = append(m.Updates, alloc)
	m.mu.Unlock()
}

// PutAllocation satisfies the AllocStateHandler interface.
func (m *MockStateUpdater) PutAllocation(alloc *structs.Allocation) (err error) {
	return
}

// Last returns a copy of the last alloc (or nil) update. Safe for concurrent
// access with updates.
func (m *MockStateUpdater) Last() *structs.Allocation {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := len(m.Updates)
	if n == 0 {
		return nil
	}
	return m.Updates[n-1].Copy()
}

// Reset resets the recorded alloc updates.
func (m *MockStateUpdater) Reset() {
	m.mu.Lock()
	m.Updates = nil
	m.mu.Unlock()
}

func testAllocRunnerConfig(t *testing.T, alloc *structs.Allocation) (*Config, func()) {
	clientConf, cleanup := clientconfig.TestClientConfig(t)

	consulRegMock := mock.NewServiceRegistrationHandler(clientConf.Logger)
	nomadRegMock := mock.NewServiceRegistrationHandler(clientConf.Logger)

	stateDB := new(state.NoopDB)

	conf := &Config{
		// Copy the alloc in case the caller edits and reuses it
		Alloc:              alloc.Copy(),
		Logger:             clientConf.Logger,
		ClientConfig:       clientConf,
		StateDB:            stateDB,
		Consul:             consulRegMock,
		ConsulSI:           consul.NewMockServiceIdentitiesClient(),
		Vault:              vaultclient.NewMockVaultClient(),
		StateUpdater:       &MockStateUpdater{},
		PrevAllocWatcher:   allocwatcher.NoopPrevAlloc{},
		PrevAllocMigrator:  allocwatcher.NoopPrevAlloc{},
		DeviceManager:      devicemanager.NoopMockManager(),
		DriverManager:      drivermanager.TestDriverManager(t),
		CpusetManager:      new(cgutil.NoopCpusetManager),
		ServersContactedCh: make(chan struct{}),
		ServiceRegWrapper:  wrapper.NewHandlerWrapper(clientConf.Logger, consulRegMock, nomadRegMock),
		CheckStore:         checkstore.NewStore(clientConf.Logger, stateDB),
		// Getter:             getter.TestDefaultGetter(t),
	}

	return conf, cleanup
}

func TestAllocRunnerFromAlloc(t *testing.T, alloc *structs.Allocation) (*allocRunner, func()) {
	t.Helper()
	cfg, cleanup := testAllocRunnerConfig(t, alloc)
	ar, err := NewAllocRunner(cfg)
	if err != nil {
		require.NoError(t, err, "Failed to setup AllocRunner")
	}

	return ar, cleanup
}

func WaitForClientState(t *testing.T, ar *allocRunner, state string) {
	testutil.WaitForResult(func() (bool, error) {
		got := ar.AllocState().ClientStatus
		return got == state,
			fmt.Errorf("expected alloc runner to be in state %s, got %s", state, got)
	}, func(err error) {
		require.NoError(t, err)
	})
}
