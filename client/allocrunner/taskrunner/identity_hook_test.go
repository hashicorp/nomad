// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	trtesting "github.com/hashicorp/nomad/client/allocrunner/taskrunner/testing"
	cstate "github.com/hashicorp/nomad/client/state"
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
			Name:       "consul",
			Audience:   []string{"consul"},
			Env:        true,
			TTL:        ttl,
			ChangeMode: "restart",
		},
		{
			Name:         "vault",
			Audience:     []string{"vault"},
			File:         true,
			TTL:          ttl,
			ChangeMode:   "signal",
			ChangeSignal: "SIGHUP",
		},
		{
			Name:     "foo",
			Audience: []string{"foo"},
			File:     true,
			Filepath: "foo.jwt",
			TTL:      ttl,
		},
	}

	mockTaskDir := &allocdir.TaskDir{
		SecretsDir: t.TempDir(),
		Dir:        t.TempDir(),
	}

	mockTR := &MockTokenSetter{}

	stopCtx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)

	// setup mock signer and WIDMgr
	logger := testlog.HCLogger(t)
	db := cstate.NewMemDB(logger)
	mockSigner := widmgr.NewMockWIDSigner(task.Identities)
	envBuilder := taskenv.NewBuilder(mock.Node(), alloc, nil, "global")

	mockWIDMgr := widmgr.NewWIDMgr(mockSigner, alloc, db, logger, envBuilder)
	mockWIDMgr.SetMinWait(time.Second) // fast renewals, because the default is 10s
	mockLifecycle := trtesting.NewMockTaskHooks()

	h := &identityHook{
		alloc:      alloc,
		task:       task,
		taskDir:    mockTaskDir,
		envBuilder: taskenv.NewBuilder(node, alloc, task, alloc.Job.Region),
		ts:         mockTR,
		lifecycle:  mockLifecycle,
		widmgr:     mockWIDMgr,
		logger:     testlog.HCLogger(t),
		stopCtx:    stopCtx,
		stop:       stop,
	}

	// do the initial renewal and start the loop
	must.NoError(t, h.widmgr.Run())

	start := time.Now()
	must.NoError(t, h.Prestart(context.Background(), nil, nil))
	env := h.envBuilder.Build().EnvMap

	// Assert initial tokens were set in Prestart
	must.Eq(t, alloc.SignedIdentities["web"], mockTR.defaultToken)
	must.FileNotExists(t, filepath.Join(mockTaskDir.SecretsDir, wiTokenFile))
	must.FileNotExists(t, filepath.Join(mockTaskDir.SecretsDir, "nomad_consul.jwt"))
	must.MapContainsKey(t, env, "NOMAD_TOKEN_consul")
	must.FileExists(t, filepath.Join(mockTaskDir.SecretsDir, "nomad_vault.jwt"))
	// Assert foo token was written to correct directory
	must.FileNotExists(t, filepath.Join(mockTaskDir.SecretsDir, "foo.jwt"))
	must.FileExists(t, filepath.Join(mockTaskDir.Dir, "foo.jwt"))

	origConsul := env["NOMAD_TOKEN_consul"]
	origVault := testutil.MustReadFile(t, mockTaskDir.SecretsDir, "nomad_vault.jwt")

	origFoo := testutil.MustReadFile(t, mockTaskDir.Dir, "foo.jwt")

	// Tokens should be rotated by their expiration
	wait := time.Until(start.Add(ttl))
	h.logger.Trace("sleeping until expiration", "wait", wait)
	time.Sleep(wait)

	// Stop renewal before checking to ensure stopping works
	must.NoError(t, h.Stop(context.Background(), nil, nil))

	// Ensure change_mode operations occurred
	select {
	case <-mockLifecycle.RestartCh:
		h.logger.Trace("restart happened")
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for restart")
	}

	select {
	case <-mockLifecycle.SignalCh:
		h.logger.Trace("signal happened")
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for restart")
	}

	newConsul := h.envBuilder.Build().EnvMap["NOMAD_TOKEN_consul"]
	must.StrContains(t, newConsul, ".") // ensure new token is JWTish
	must.NotEq(t, newConsul, origConsul)

	newVault := testutil.MustReadFile(t, mockTaskDir.SecretsDir, "nomad_vault.jwt")
	must.StrContains(t, string(newVault), ".") // ensure new token is JWTish
	must.NotEq(t, newVault, origVault)

	newFoo := testutil.MustReadFile(t, mockTaskDir.Dir, "foo.jwt")
	must.StrContains(t, string(newFoo), ".")
	must.NotEq(t, newFoo, origFoo)

	// Assert Stop work. Tokens should not have changed.
	time.Sleep(wait)
	must.Eq(t, newConsul, h.envBuilder.Build().EnvMap["NOMAD_TOKEN_consul"])
	must.Eq(t, newVault, testutil.MustReadFile(t, mockTaskDir.SecretsDir, "nomad_vault.jwt"))
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

	mockTaskDir := &allocdir.TaskDir{
		SecretsDir: t.TempDir(),
	}

	mockTR := &MockTokenSetter{}

	stopCtx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)

	// setup mock signer and WIDMgr
	logger := testlog.HCLogger(t)
	db := cstate.NewMemDB(logger)
	mockSigner := widmgr.NewMockWIDSigner(task.Identities)
	envBuilder := taskenv.NewBuilder(mock.Node(), alloc, nil, "global")
	mockWIDMgr := widmgr.NewWIDMgr(mockSigner, alloc, db, logger, envBuilder)
	mockWIDMgr.SetMinWait(time.Second) // fast renewals, because the default is 10s

	h := &identityHook{
		alloc:      alloc,
		task:       task,
		taskDir:    mockTaskDir,
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
	must.FileNotExists(t, filepath.Join(mockTaskDir.SecretsDir, wiTokenFile))
	must.FileNotExists(t, filepath.Join(mockTaskDir.SecretsDir, "nomad_consul.jwt"))
	must.MapContainsKey(t, env, "NOMAD_TOKEN_consul")
	must.FileExists(t, filepath.Join(mockTaskDir.SecretsDir, "nomad_vault.jwt"))

	origConsul := env["NOMAD_TOKEN_consul"]
	origVault := testutil.MustReadFile(t, mockTaskDir.SecretsDir, "nomad_vault.jwt")

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

	newVault := testutil.MustReadFile(t, mockTaskDir.SecretsDir, "nomad_vault.jwt")
	must.StrContains(t, string(newVault), ".") // ensure new token is JWTish
	must.NotEq(t, newVault, origVault)

	// Assert Stop work. Tokens should not have changed.
	time.Sleep(wait)
	must.Eq(t, newConsul, h.envBuilder.Build().EnvMap["NOMAD_TOKEN_consul"])
	must.Eq(t, newVault, testutil.MustReadFile(t, mockTaskDir.SecretsDir, "nomad_vault.jwt"))
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

	mockTaskDir := &allocdir.TaskDir{
		SecretsDir: "/this-should-not-exist",
	}

	h := &identityHook{
		alloc:      alloc,
		task:       task,
		taskDir:    mockTaskDir,
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
