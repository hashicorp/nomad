// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	cstate "github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/client/widmgr"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

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
	db := cstate.NewMemDB(logger)
	env := taskenv.NewBuilder(mock.Node(), alloc, nil, "global").Build()

	// setup mock signer and WIDMgr
	mockSigner := widmgr.NewMockWIDSigner(task.Identities)
	mockWIDMgr := widmgr.NewWIDMgr(mockSigner, alloc, db, logger, env)
	allocrunner.widmgr = mockWIDMgr
	allocrunner.widsigner = mockSigner

	// do the initial signing
	_, err := mockSigner.SignIdentities(1, []*structs.WorkloadIdentityRequest{
		{
			AllocID: alloc.ID,
			WIHandle: structs.WIHandle{
				WorkloadIdentifier: task.Name,
				IdentityName:       task.Identities[0].Name,
			},
		},
	})
	must.NoError(t, err)

	start := time.Now()
	hook := newIdentityHook(logger, mockWIDMgr)
	must.Eq(t, hook.Name(), "identity")
	must.NoError(t, hook.Prerun(env))

	time.Sleep(time.Second) // give goroutines a moment to run
	sid, err := hook.widmgr.Get(structs.WIHandle{
		WorkloadIdentifier: task.Name,
		IdentityName:       task.Identities[0].Name},
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

	// shutting down twice must not panic
	hook.PreKill()
	hook.PreKill()
}
