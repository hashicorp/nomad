package allocrunner

import (
	"io/ioutil"
	"os"
	"sync"
	"testing"

	"github.com/boltdb/bolt"
	"github.com/hashicorp/nomad/client/allocwatcher"
	"github.com/hashicorp/nomad/client/config"
	consulApi "github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

type MockAllocStateUpdater struct {
	Allocs []*structs.Allocation
	mu     sync.Mutex
}

// Update fulfills the TaskStateUpdater interface
func (m *MockAllocStateUpdater) Update(alloc *structs.Allocation) {
	m.mu.Lock()
	m.Allocs = append(m.Allocs, alloc)
	m.mu.Unlock()
}

// Last returns a copy of the last alloc (or nil) sync'd
func (m *MockAllocStateUpdater) Last() *structs.Allocation {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := len(m.Allocs)
	if n == 0 {
		return nil
	}
	return m.Allocs[n-1].Copy()
}

func TestAllocRunnerFromAlloc(t *testing.T, alloc *structs.Allocation, restarts bool) (*MockAllocStateUpdater, *AllocRunner) {
	conf := config.DefaultConfig()
	conf.Node = mock.Node()
	conf.StateDir = os.TempDir()
	conf.AllocDir = os.TempDir()
	tmp, _ := ioutil.TempFile("", "state-db")
	db, _ := bolt.Open(tmp.Name(), 0600, nil)
	upd := &MockAllocStateUpdater{}
	if !restarts {
		*alloc.Job.LookupTaskGroup(alloc.TaskGroup).RestartPolicy = structs.RestartPolicy{Attempts: 0}
		alloc.Job.Type = structs.JobTypeBatch
	}
	vclient := vaultclient.NewMockVaultClient()
	ar := NewAllocRunner(testlog.Logger(t), conf, db, upd.Update, alloc, vclient, consulApi.NewMockConsulServiceClient(t), allocwatcher.NoopPrevAlloc{})
	return upd, ar
}

func TestAllocRunner(t *testing.T, restarts bool) (*MockAllocStateUpdater, *AllocRunner) {
	// Use mock driver
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config["run_for"] = "500ms"
	return TestAllocRunnerFromAlloc(t, alloc, restarts)
}
