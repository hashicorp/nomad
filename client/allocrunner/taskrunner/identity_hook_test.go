// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/client/widmgr"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

var _ interfaces.TaskPrestartHook = (*identityHook)(nil)
var _ interfaces.TaskStopHook = (*identityHook)(nil)
var _ interfaces.ShutdownHook = (*identityHook)(nil)

// See task_runner_test.go:TestTaskRunner_IdentityHook

// MockTokenSetter is a mock implementation of tokenSetter which is satisfied
// by TaskRunner at runtime.
type MockTokenSetter struct {
	defaultToken string
}

func (m *MockTokenSetter) setNomadToken(token string) {
	m.defaultToken = token
}

// TestIdentityHook_RenewAll asserts token renewal happens when expected.
func TestIdentityHook_RenewAll(t *testing.T) {
	ci.Parallel(t)

	// TTL is used for expiration and the test will sleep this long before
	// checking that tokens were rotated. Therefore the time must be long enough
	// to generate new tokens. Since no Raft or IO (outside of potentially
	// writing 1 token file) is performed, this should be relatively fast.
	ttl := 3 * time.Second

	node := mock.Node()
	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	task := alloc.LookupTask("web")
	task.Identities = []*structs.WorkloadIdentity{
		{
			Name:     "consul",
			Audience: []string{"consul"},
			Env:      true,
			TTL:      ttl,
		},
		{
			Name:     "vault",
			Audience: []string{"vault"},
			File:     true,
			TTL:      ttl,
		},
	}

	secretsDir := t.TempDir()

	mockTR := &MockTokenSetter{}

	stopCtx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)

	// setup mock signer and WIDMgr
	mockSigner := widmgr.NewMockWIDSigner(task.Identities)
	mockWIDMgr := widmgr.NewWIDMgr(mockSigner, alloc, testlog.HCLogger(t))
	mockWIDMgr.SetMinWait(time.Second) // fast renewals, because the default is 10s

	// do the initial signing
	for _, i := range task.Identities {
		_, err := mockSigner.SignIdentities(1, []*structs.WorkloadIdentityRequest{
			{
				AllocID:      alloc.ID,
				TaskName:     task.Name,
				IdentityName: i.Name,
			},
		})
		must.NoError(t, err)
	}

	h := &identityHook{
		alloc:      alloc,
		task:       task,
		tokenDir:   secretsDir,
		envBuilder: taskenv.NewBuilder(node, alloc, task, alloc.Job.Region),
		ts:         mockTR,
		widmgr:     mockWIDMgr,
		logger:     testlog.HCLogger(t),
		stopCtx:    stopCtx,
		stop:       stop,
	}

	// do the initial renewal and start the loop
	must.NoError(t, h.widmgr.Run())

	start := time.Now()
	must.NoError(t, h.Prestart(context.Background(), nil, nil))
	time.Sleep(time.Second) // goroutines in the Prestart hook must run first before we Build the EnvMap
	env := h.envBuilder.Build().EnvMap

	// Assert initial tokens were set in Prestart
	must.Eq(t, alloc.SignedIdentities["web"], mockTR.defaultToken)
	must.FileNotExists(t, filepath.Join(secretsDir, wiTokenFile))
	must.FileNotExists(t, filepath.Join(secretsDir, "nomad_consul.jwt"))
	must.MapContainsKey(t, env, "NOMAD_TOKEN_consul")
	must.FileExists(t, filepath.Join(secretsDir, "nomad_vault.jwt"))

	origConsul := env["NOMAD_TOKEN_consul"]
	origVault := testutil.MustReadFile(t, secretsDir, "nomad_vault.jwt")

	// Tokens should be rotated by their expiration
	wait := time.Until(start.Add(ttl))
	h.logger.Trace("sleeping until expiration", "wait", wait)
	time.Sleep(wait)

	// Stop renewal before checking to ensure stopping works
	must.NoError(t, h.Stop(context.Background(), nil, nil))
	time.Sleep(time.Second) // Stop is async so give renewal time to exit

	newConsul := h.envBuilder.Build().EnvMap["NOMAD_TOKEN_consul"]
	must.StrContains(t, newConsul, ".") // ensure new token is JWTish
	must.NotEq(t, newConsul, origConsul)

	newVault := testutil.MustReadFile(t, secretsDir, "nomad_vault.jwt")
	must.StrContains(t, string(newVault), ".") // ensure new token is JWTish
	must.NotEq(t, newVault, origVault)

	// Assert Stop work. Tokens should not have changed.
	time.Sleep(wait)
	must.Eq(t, newConsul, h.envBuilder.Build().EnvMap["NOMAD_TOKEN_consul"])
	must.Eq(t, newVault, testutil.MustReadFile(t, secretsDir, "nomad_vault.jwt"))
}

// TestIdentityHook_RenewOne asserts token renewal only renews tokens with a TTL.
func TestIdentityHook_RenewOne(t *testing.T) {
	ci.Parallel(t)

	ttl := 3 * time.Second

	node := mock.Node()
	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	alloc.SignedIdentities = map[string]string{"web": "does.not.matter"}
	task := alloc.LookupTask("web")
	task.Identities = []*structs.WorkloadIdentity{
		{
			Name:     "consul",
			Audience: []string{"consul"},
			Env:      true,
		},
		{
			Name:     "vault",
			Audience: []string{"vault"},
			File:     true,
			TTL:      ttl,
		},
	}

	secretsDir := t.TempDir()

	mockTR := &MockTokenSetter{}

	stopCtx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)

	// setup mock signer and WIDMgr
	mockSigner := widmgr.NewMockWIDSigner(task.Identities)
	mockWIDMgr := widmgr.NewWIDMgr(mockSigner, alloc, testlog.HCLogger(t))
	mockWIDMgr.SetMinWait(time.Second) // fast renewals, because the default is 10s

	// do the initial signing
	for _, i := range task.Identities {
		_, err := mockSigner.SignIdentities(1, []*structs.WorkloadIdentityRequest{
			{
				AllocID:      alloc.ID,
				TaskName:     task.Name,
				IdentityName: i.Name,
			},
		})
		must.NoError(t, err)
	}

	h := &identityHook{
		alloc:      alloc,
		task:       task,
		tokenDir:   secretsDir,
		envBuilder: taskenv.NewBuilder(node, alloc, task, alloc.Job.Region),
		ts:         mockTR,
		widmgr:     mockWIDMgr,
		logger:     testlog.HCLogger(t),
		stopCtx:    stopCtx,
		stop:       stop,
	}

	// do the initial renewal and start the loop
	must.NoError(t, h.widmgr.Run())

	start := time.Now()
	must.NoError(t, h.Prestart(context.Background(), nil, nil))
	time.Sleep(time.Second) // goroutines in the Prestart hook must run first before we Build the EnvMap
	env := h.envBuilder.Build().EnvMap

	// Assert initial tokens were set in Prestart
	must.Eq(t, alloc.SignedIdentities["web"], mockTR.defaultToken)
	must.FileNotExists(t, filepath.Join(secretsDir, wiTokenFile))
	must.FileNotExists(t, filepath.Join(secretsDir, "nomad_consul.jwt"))
	must.MapContainsKey(t, env, "NOMAD_TOKEN_consul")
	must.FileExists(t, filepath.Join(secretsDir, "nomad_vault.jwt"))

	origConsul := env["NOMAD_TOKEN_consul"]
	origVault := testutil.MustReadFile(t, secretsDir, "nomad_vault.jwt")

	// One token should be rotated by their expiration
	wait := time.Until(start.Add(ttl))
	h.logger.Trace("sleeping until expiration", "wait", wait)
	time.Sleep(wait)

	// Stop renewal before checking to ensure stopping works
	must.NoError(t, h.Stop(context.Background(), nil, nil))
	time.Sleep(time.Second) // Stop is async so give renewal time to exit

	newConsul := h.envBuilder.Build().EnvMap["NOMAD_TOKEN_consul"]
	must.StrContains(t, newConsul, ".") // ensure new token is JWTish
	must.Eq(t, newConsul, origConsul)

	newVault := testutil.MustReadFile(t, secretsDir, "nomad_vault.jwt")
	must.StrContains(t, string(newVault), ".") // ensure new token is JWTish
	must.NotEq(t, newVault, origVault)

	// Assert Stop work. Tokens should not have changed.
	time.Sleep(wait)
	must.Eq(t, newConsul, h.envBuilder.Build().EnvMap["NOMAD_TOKEN_consul"])
	must.Eq(t, newVault, testutil.MustReadFile(t, secretsDir, "nomad_vault.jwt"))
}

// TestIdentityHook_ErrorWriting assert Prestart returns an error if the
// default token could not be written when requested.
func TestIdentityHook_ErrorWriting(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	alloc.SignedIdentities = map[string]string{"web": "does.not.need.to.be.valid"}
	task := alloc.LookupTask("web")
	task.Identity.File = true
	node := mock.Node()
	stopCtx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)

	h := &identityHook{
		alloc:      alloc,
		task:       task,
		tokenDir:   "/this-should-not-exist",
		envBuilder: taskenv.NewBuilder(node, alloc, task, alloc.Job.Region),
		ts:         &MockTokenSetter{},
		logger:     testlog.HCLogger(t),
		stopCtx:    stopCtx,
		stop:       stop,
	}

	// Prestart should fail when trying to write the default identity file
	err := h.Prestart(context.Background(), nil, nil)
	must.ErrorContains(t, err, "failed to write nomad token")
}
