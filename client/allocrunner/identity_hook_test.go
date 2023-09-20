// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"context"
	"testing"
	"time"

	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/widmgr"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

// statically assert network hook implements the expected interfaces
var _ interfaces.RunnerPrerunHook = (*identityHook)(nil)
var _ interfaces.ShutdownHook = (*identityHook)(nil)
var _ interfaces.TaskStopHook = (*identityHook)(nil)

func TestIdentityHook_Prerun(t *testing.T) {
	ci.Parallel(t)

	ttl := 30 * time.Second

	wid := &structs.WorkloadIdentity{
		Name:     "testing",
		Audience: []string{"consul.io"},
		Env:      true,
		File:     true,
		TTL:      ttl,
	}

	alloc := mock.Alloc()
	task := alloc.LookupTask("web")
	task.Identity = wid
	task.Identities = []*structs.WorkloadIdentity{wid}

	allocrunner, stopAR := TestAllocRunnerFromAlloc(t, alloc)
	defer stopAR()

	logger := testlog.HCLogger(t)

	// setup mock signer and WIDMgr
	mockSigner := widmgr.NewMockWIDSigner(task.Identities)
	mockWIDMgr := widmgr.NewWIDMgr(mockSigner, alloc, logger)
	allocrunner.widmgr = mockWIDMgr
	allocrunner.widsigner = mockSigner

	// do the initial signing
	_, err := mockSigner.SignIdentities(1, []*structs.WorkloadIdentityRequest{
		{
			AllocID:      alloc.ID,
			TaskName:     task.Name,
			IdentityName: task.Identities[0].Name,
		},
	})
	must.NoError(t, err)

	start := time.Now()
	hook := newIdentityHook(logger, mockWIDMgr)
	must.Eq(t, hook.Name(), "identity")
	must.NoError(t, hook.Prerun())

	time.Sleep(time.Second) // give goroutines a moment to run
	sid, err := hook.widmgr.Get(cstructs.TaskIdentity{
		TaskName:     task.Name,
		IdentityName: task.Identities[0].Name},
	)
	must.Nil(t, err)
	must.Eq(t, sid.IdentityName, task.Identity.Name)
	must.NotEq(t, sid.JWT, "")

	// pad expiry time with a second to be safe
	must.Between(t,
		start.Add(ttl).Add(-1*time.Second).Unix(),
		sid.Expiration.Unix(),
		start.Add(ttl).Add(1*time.Second).Unix(),
	)

	must.NoError(t, hook.Stop(context.Background(), nil, nil))
}
