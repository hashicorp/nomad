// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

var _ interfaces.TaskPrestartHook = (*identityHook)(nil)

// See task_runner_test.go:TestTaskRunner_IdentityHook

// MockWIDMgr allows TaskRunner unit tests to avoid having to setup a Server,
// Client, and Allocation.
type MockWIDMgr struct {
	// wids maps identity names to workload identities. If wids is non-nil then
	// SignIdentities will use it to find expirations or reject invalid identity
	// names
	wids map[string]*structs.WorkloadIdentity
}

// setWIDs is a test helper to use Task.Identities in the MockWIDMgr for
// sharing TTLs and validating names.
func (m *MockWIDMgr) setWIDs(wids []*structs.WorkloadIdentity) {
	m.wids = make(map[string]*structs.WorkloadIdentity, len(wids))
	for _, wid := range wids {
		m.wids[wid.Name] = wid
	}
}

func (m *MockWIDMgr) SignIdentities(minIndex uint64, req []*structs.WorkloadIdentityRequest) ([]*structs.SignedWorkloadIdentity, error) {
	swids := make([]*structs.SignedWorkloadIdentity, 0, len(req))
	for _, idReq := range req {
		swid := &structs.SignedWorkloadIdentity{
			WorkloadIdentityRequest: *idReq,
			// Just the sample jwt from jwt.io so it "looks" like a jwt
			JWT: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
		}

		if m.wids != nil {
			// Test has set workload identities. Lookup expiration or reject unknown
			// identity.
			wid, ok := m.wids[idReq.IdentityName]
			if !ok {
				return nil, fmt.Errorf("unknown identity: %q", idReq.IdentityName)
			}

			if wid.TTL > 0 {
				swid.Expiration = time.Now().Add(wid.TTL)
			}
		}

		swids = append(swids, swid)
	}
	return swids, nil
}

func TestIdentityHook_Renew(t *testing.T) {
	ci.Parallel(t)

	node := mock.Node()
	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	task := alloc.LookupTask("web")
	task.Identities = []*structs.WorkloadIdentity{
		{
			Name:     "consul",
			Audience: []string{"consul"},
			Env:      true,
			TTL:      10 * time.Second,
		},
		{
			Name:     "vault",
			Audience: []string{"vault"},
			File:     true,
			TTL:      10 * time.Second,
		},
	}

	widmgr := &MockWIDMgr{}
	widmgr.setWIDs(task.Identities)

	stopCtx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)

	h := &identityHook{
		alloc:      alloc,
		task:       task,
		tokenDir:   t.TempDir(),
		envBuilder: taskenv.NewBuilder(node, alloc, task, alloc.Job.Region),
		tr:         nil, //TODO FIX ME
		widmgr:     widmgr,
		logger:     testlog.HCLogger(t),
		stopCtx:    stopCtx,
		stop:       stop,
	}

	h.Prestart(context.TODO(), nil, nil)
}
