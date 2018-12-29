package deploymentwatcher

import (
	"log"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	mocker "github.com/stretchr/testify/mock"
)

func testLogger() *log.Logger {
	return log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)
}

type mockBackend struct {
	mocker.Mock
	index uint64
	state *state.StateStore
	l     sync.Mutex
}

func newMockBackend(t *testing.T) *mockBackend {
	return &mockBackend{
		index: 10000,
		state: state.TestStateStore(t),
	}
}

func (m *mockBackend) nextIndex() uint64 {
	m.l.Lock()
	defer m.l.Unlock()
	i := m.index
	m.index++
	return i
}

func (m *mockBackend) UpsertEvals(evals []*structs.Evaluation) (uint64, error) {
	m.Called(evals)
	i := m.nextIndex()
	return i, m.state.UpsertEvals(i, evals)
}

// matchUpsertEvals is used to match an upsert request
func matchUpsertEvals(deploymentIDs []string) func(evals []*structs.Evaluation) bool {
	return func(evals []*structs.Evaluation) bool {
		if len(evals) != len(deploymentIDs) {
			return false
		}

		dmap := make(map[string]struct{}, len(deploymentIDs))
		for _, d := range deploymentIDs {
			dmap[d] = struct{}{}
		}

		for _, e := range evals {
			if _, ok := dmap[e.DeploymentID]; !ok {
				return false
			}

			delete(dmap, e.DeploymentID)
		}

		return true
	}
}

func (m *mockBackend) UpsertJob(job *structs.Job) (uint64, error) {
	m.Called(job)
	i := m.nextIndex()
	return i, m.state.UpsertJob(i, job)
}

func (m *mockBackend) UpdateDeploymentStatus(u *structs.DeploymentStatusUpdateRequest) (uint64, error) {
	m.Called(u)
	i := m.nextIndex()
	return i, m.state.UpdateDeploymentStatus(i, u)
}

// matchDeploymentStatusUpdateConfig is used to configure the matching
// function
type matchDeploymentStatusUpdateConfig struct {
	// DeploymentID is the expected ID
	DeploymentID string

	// Status is the desired status
	Status string

	// StatusDescription is the desired status description
	StatusDescription string

	// JobVersion marks whether we expect a roll back job at the given version
	JobVersion *uint64

	// Eval marks whether we expect an evaluation.
	Eval bool
}

// matchDeploymentStatusUpdateRequest is used to match an update request
func matchDeploymentStatusUpdateRequest(c *matchDeploymentStatusUpdateConfig) func(args *structs.DeploymentStatusUpdateRequest) bool {
	return func(args *structs.DeploymentStatusUpdateRequest) bool {
		if args.DeploymentUpdate.DeploymentID != c.DeploymentID {
			testLogger().Printf("deployment ids dont match")
			return false
		}

		if args.DeploymentUpdate.Status != c.Status && args.DeploymentUpdate.StatusDescription != c.StatusDescription {
			testLogger().Printf("status's dont match")
			return false
		}

		if c.Eval && args.Eval == nil || !c.Eval && args.Eval != nil {
			testLogger().Printf("evals dont match")
			return false
		}

		if c.JobVersion != nil {
			if args.Job == nil {
				return false
			} else if args.Job.Version != *c.JobVersion {
				return false
			}
		} else if c.JobVersion == nil && args.Job != nil {
			return false
		}

		return true
	}
}

func (m *mockBackend) UpdateDeploymentPromotion(req *structs.ApplyDeploymentPromoteRequest) (uint64, error) {
	m.Called(req)
	i := m.nextIndex()
	return i, m.state.UpdateDeploymentPromotion(i, req)
}

// matchDeploymentPromoteRequestConfig is used to configure the matching
// function
type matchDeploymentPromoteRequestConfig struct {
	// Promotion holds the expected promote request
	Promotion *structs.DeploymentPromoteRequest

	// Eval marks whether we expect an evaluation.
	Eval bool
}

// matchDeploymentPromoteRequest is used to match a promote request
func matchDeploymentPromoteRequest(c *matchDeploymentPromoteRequestConfig) func(args *structs.ApplyDeploymentPromoteRequest) bool {
	return func(args *structs.ApplyDeploymentPromoteRequest) bool {
		if !reflect.DeepEqual(*c.Promotion, args.DeploymentPromoteRequest) {
			return false
		}

		if c.Eval && args.Eval == nil || !c.Eval && args.Eval != nil {
			return false
		}

		return true
	}
}
func (m *mockBackend) UpdateDeploymentAllocHealth(req *structs.ApplyDeploymentAllocHealthRequest) (uint64, error) {
	m.Called(req)
	i := m.nextIndex()
	return i, m.state.UpdateDeploymentAllocHealth(i, req)
}

// matchDeploymentAllocHealthRequestConfig is used to configure the matching
// function
type matchDeploymentAllocHealthRequestConfig struct {
	// DeploymentID is the expected ID
	DeploymentID string

	// Healthy and Unhealthy contain the expected allocation IDs that are having
	// their health set
	Healthy, Unhealthy []string

	// DeploymentUpdate holds the expected values of status and description. We
	// don't check for exact match but string contains
	DeploymentUpdate *structs.DeploymentStatusUpdate

	// JobVersion marks whether we expect a roll back job at the given version
	JobVersion *uint64

	// Eval marks whether we expect an evaluation.
	Eval bool
}

// matchDeploymentAllocHealthRequest is used to match an update request
func matchDeploymentAllocHealthRequest(c *matchDeploymentAllocHealthRequestConfig) func(args *structs.ApplyDeploymentAllocHealthRequest) bool {
	return func(args *structs.ApplyDeploymentAllocHealthRequest) bool {
		if args.DeploymentID != c.DeploymentID {
			return false
		}

		if len(c.Healthy) != len(args.HealthyAllocationIDs) {
			return false
		}
		if len(c.Unhealthy) != len(args.UnhealthyAllocationIDs) {
			return false
		}

		hmap, umap := make(map[string]struct{}, len(c.Healthy)), make(map[string]struct{}, len(c.Unhealthy))
		for _, h := range c.Healthy {
			hmap[h] = struct{}{}
		}
		for _, u := range c.Unhealthy {
			umap[u] = struct{}{}
		}

		for _, h := range args.HealthyAllocationIDs {
			if _, ok := hmap[h]; !ok {
				return false
			}
		}
		for _, u := range args.UnhealthyAllocationIDs {
			if _, ok := umap[u]; !ok {
				return false
			}
		}

		if c.DeploymentUpdate != nil {
			if args.DeploymentUpdate == nil {
				return false
			}

			if !strings.Contains(args.DeploymentUpdate.Status, c.DeploymentUpdate.Status) {
				return false
			}
			if !strings.Contains(args.DeploymentUpdate.StatusDescription, c.DeploymentUpdate.StatusDescription) {
				return false
			}
		} else if args.DeploymentUpdate != nil {
			return false
		}

		if c.Eval && args.Eval == nil || !c.Eval && args.Eval != nil {
			return false
		}

		if (c.JobVersion != nil && (args.Job == nil || args.Job.Version != *c.JobVersion)) || c.JobVersion == nil && args.Job != nil {
			return false
		}

		return true
	}
}
