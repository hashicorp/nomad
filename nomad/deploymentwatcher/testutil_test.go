// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package deploymentwatcher

import (
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	mocker "github.com/stretchr/testify/mock"
)

type mockBackend struct {
	mocker.Mock
	index uint64
	state *state.StateStore
	l     sync.Mutex
}

func newMockBackend(t *testing.T) *mockBackend {
	m := &mockBackend{
		index: 10000,
		state: state.TestStateStore(t),
	}
	m.Test(t)
	return m
}

func (m *mockBackend) nextIndex() uint64 {
	m.l.Lock()
	defer m.l.Unlock()
	i := m.index
	m.index++
	return i
}

func (m *mockBackend) UpdateAllocDesiredTransition(u *structs.AllocUpdateDesiredTransitionRequest) (uint64, error) {
	m.Called(u)
	i := m.nextIndex()
	return i, m.state.UpdateAllocsDesiredTransitions(structs.MsgTypeTestSetup, i, u.Allocs, u.Evals)
}

// matchUpdateAllocDesiredTransitions is used to match an upsert request
func matchUpdateAllocDesiredTransitions(deploymentIDs []string) func(update *structs.AllocUpdateDesiredTransitionRequest) bool {
	return func(update *structs.AllocUpdateDesiredTransitionRequest) bool {
		if len(update.Evals) != len(deploymentIDs) {
			return false
		}

		dmap := make(map[string]struct{}, len(deploymentIDs))
		for _, d := range deploymentIDs {
			dmap[d] = struct{}{}
		}

		for _, e := range update.Evals {
			if _, ok := dmap[e.DeploymentID]; !ok {
				return false
			}

			delete(dmap, e.DeploymentID)
		}

		return true
	}
}

// matchUpdateAllocDesiredTransitionReschedule is used to match allocs that have their DesiredTransition set to Reschedule
func matchUpdateAllocDesiredTransitionReschedule(allocIDs []string) func(update *structs.AllocUpdateDesiredTransitionRequest) bool {
	return func(update *structs.AllocUpdateDesiredTransitionRequest) bool {
		amap := make(map[string]struct{}, len(allocIDs))
		for _, d := range allocIDs {
			amap[d] = struct{}{}
		}

		for allocID, dt := range update.Allocs {
			if _, ok := amap[allocID]; !ok {
				return false
			}
			if !*dt.Reschedule {
				return false
			}
		}

		return true
	}
}

func (m *mockBackend) UpsertJob(job *structs.Job) (uint64, error) {
	m.Called(job)
	i := m.nextIndex()
	return i, m.state.UpsertJob(structs.MsgTypeTestSetup, i, nil, job)
}

func (m *mockBackend) UpdateDeploymentStatus(u *structs.DeploymentStatusUpdateRequest) (uint64, error) {
	m.Called(u)
	i := m.nextIndex()
	return i, m.state.UpdateDeploymentStatus(structs.MsgTypeTestSetup, i, u)
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
			return false
		}

		if args.DeploymentUpdate.Status != c.Status && args.DeploymentUpdate.StatusDescription != c.StatusDescription {
			return false
		}

		if c.Eval && args.Eval == nil || !c.Eval && args.Eval != nil {
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
	return i, m.state.UpdateDeploymentPromotion(structs.MsgTypeTestSetup, i, req)
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
	return i, m.state.UpdateDeploymentAllocHealth(structs.MsgTypeTestSetup, i, req)
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

		// Require a timestamp
		if args.Timestamp.IsZero() {
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
