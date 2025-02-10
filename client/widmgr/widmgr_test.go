// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package widmgr

import (
	"testing"
	"time"

	cstate "github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestWIDMgr_Restore(t *testing.T) {

	logger := testlog.HCLogger(t)

	db := cstate.NewMemDB(logger)

	alloc := mock.Alloc()
	service := alloc.Job.TaskGroups[0].Tasks[0].Services[0]
	widSpecs := []*structs.WorkloadIdentity{
		{ServiceName: service.MakeUniqueIdentityName()},
		{Name: "default"},
		{Name: "extra", TTL: time.Hour},
	}
	alloc.Job.TaskGroups[0].Tasks[0].Services[0].Identity = widSpecs[0]
	alloc.Job.TaskGroups[0].Tasks[0].Identities = widSpecs[1:]
	envBuilder := taskenv.NewBuilder(mock.Node(), alloc, nil, "global")

	signer := NewMockWIDSigner(widSpecs)
	mgr := NewWIDMgr(signer, alloc, db, logger, envBuilder)

	// restore, but we haven't previously saved to the db
	hasExpired, err := mgr.restoreStoredIdentities()
	must.NoError(t, err)
	must.True(t, hasExpired)

	// populate the lastToken and save to the db
	must.NoError(t, mgr.getInitialIdentities())

	// restore, and no identities are expired
	hasExpired, err = mgr.restoreStoredIdentities()
	must.NoError(t, err)
	must.False(t, hasExpired)

	// set the signer's clock back and set a low TTL to make the "extra" WI
	// expired when we force a re-sign
	signer.mockNow = time.Now().Add(-1 * time.Minute)
	widSpecs[2].TTL = time.Second
	signer.setWIDs(widSpecs)

	wiHandle := service.IdentityHandle(envBuilder.Build().ReplaceEnv)
	mgr.widSpecs[*wiHandle].TTL = time.Second

	// force a re-sign to re-populate the lastToken and save to the db
	must.NoError(t, mgr.getInitialIdentities())

	// restore, and at least one identity is expired
	hasExpired, err = mgr.restoreStoredIdentities()
	must.NoError(t, err)
	must.True(t, hasExpired)
}
