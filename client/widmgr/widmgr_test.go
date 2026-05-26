// Copyright IBM Corp. 2015, 2026
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
	service0 := alloc.Job.TaskGroups[0].Tasks[0].Services[0] // WI identity
	alloc.Job.TaskGroups[0].Tasks[0].Services[1].Provider = "nomad"
	widSpecs := []*structs.WorkloadIdentity{
		{Name: service0.MakeUniqueIdentityName(), ServiceName: service0.Name},
		{Name: "default"},
		{Name: "extra", TTL: time.Hour},
	}
	alloc.Job.TaskGroups[0].Tasks[0].Services[0].Identity = widSpecs[0]
	alloc.Job.TaskGroups[0].Tasks[0].Identities = widSpecs[1:]
	env := taskenv.NewBuilder(mock.Node(), alloc, nil, "global").Build()

	signer := NewMockWIDSigner(widSpecs)
	mgr := NewWIDMgr(signer, alloc, db, logger, env, false)

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

	wiHandle := service0.IdentityHandle(env.ReplaceEnv)
	mgr.widSpecs[*wiHandle].TTL = time.Second

	// force a re-sign to re-populate the lastToken and save to the db
	must.NoError(t, mgr.getInitialIdentities())

	// restore, and at least one identity is expired
	hasExpired, err = mgr.restoreStoredIdentities()
	must.NoError(t, err)
	must.True(t, hasExpired)
}

// TestWIDMgr_RestoreLegacy tests the restore path on existing pre-WI
// allocations that don't have any injected implicit identities for Consul/Vault
func TestWIDMgr_RestoreLegacy(t *testing.T) {

	logger := testlog.HCLogger(t)

	db := cstate.NewMemDB(logger)

	alloc := mock.Alloc()
	tg0 := alloc.Job.TaskGroups[0]
	task0 := tg0.Tasks[0]

	// non-nil Vault block triggers fallback identity request for Vault WI for task
	task0.Vault = &structs.Vault{}

	// non-nil template block + ClientConfig.TemplateConfig.DeriveConsulToken
	// triggers fallback identity request for Consul WI for task
	deriveConsulWIForTemplates := true
	task0.Templates = []*structs.Template{{
		EmbeddedTmpl: "template-contents-foo",
	}}

	// Consul service block triggers fallback identity request for Consul WI for
	// service
	service0 := alloc.Job.TaskGroups[0].Tasks[0].Services[0]
	task0.Services[1].Provider = "nomad"

	widSpecs := []*structs.WorkloadIdentity{
		{Name: service0.MakeUniqueIdentityName(), ServiceName: service0.Name},
		{Name: "vault_default"},
		{Name: "consul_default"},
		{Name: "default"},
	}
	env := taskenv.NewBuilder(mock.Node(), alloc, nil, "global").Build()
	signer := NewMockWIDSigner(widSpecs)
	mgr := NewWIDMgr(signer, alloc, db, logger, env, deriveConsulWIForTemplates)

	// restore, but we haven't previously saved to the db
	hasExpired, err := mgr.restoreStoredIdentities()
	must.NoError(t, err)
	must.True(t, hasExpired)

	// populate the lastToken and save to the db
	must.NoError(t, mgr.getInitialIdentities())
}
