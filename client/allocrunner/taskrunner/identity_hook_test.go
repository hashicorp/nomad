// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
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

	key   ed25519.PrivateKey
	keyID string
}

func NewMockWIDMgr(wids []*structs.WorkloadIdentity) *MockWIDMgr {
	_, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		panic(err)
	}
	m := &MockWIDMgr{
		key:   privKey,
		keyID: uuid.Generate(),
	}

	if wids != nil {
		m.setWIDs(wids)
	}

	return m
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
		// Set test values for default claims
		claims := &structs.IdentityClaims{
			Namespace:    "default",
			JobID:        "test",
			AllocationID: idReq.AllocID,
			TaskName:     idReq.TaskName,
		}
		claims.ID = uuid.Generate()

		// If test has set workload identities. Lookup claims or reject unknown
		// identity.
		if m.wids != nil {
			wid, ok := m.wids[idReq.IdentityName]
			if !ok {
				return nil, fmt.Errorf("unknown identity: %q", idReq.IdentityName)
			}

			claims.Audience = slices.Clone(wid.Audience)

			if wid.TTL > 0 {
				claims.Expiry = jwt.NewNumericDate(time.Now().Add(wid.TTL))
			}
		}

		opts := (&jose.SignerOptions{}).WithHeader("kid", m.keyID).WithType("JWT")
		sig, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.EdDSA, Key: m.key}, opts)
		if err != nil {
			return nil, fmt.Errorf("error creating signer: %w", err)
		}
		token, err := jwt.Signed(sig).Claims(claims).CompactSerialize()
		if err != nil {
			return nil, fmt.Errorf("error signing: %w", err)
		}

		swid := &structs.SignedWorkloadIdentity{
			WorkloadIdentityRequest: *idReq,
			JWT:                     token,
			Expiration:              claims.Expiry.Time(),
		}

		swids = append(swids, swid)
	}
	return swids, nil
}

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
	ttl := 8 * time.Second

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

	widmgr := NewMockWIDMgr(task.Identities)

	mockTR := &MockTokenSetter{}

	stopCtx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)

	h := &identityHook{
		alloc:      alloc,
		task:       task,
		tokenDir:   secretsDir,
		envBuilder: taskenv.NewBuilder(node, alloc, task, alloc.Job.Region),
		ts:         mockTR,
		widmgr:     widmgr,
		minWait:    time.Second,
		logger:     testlog.HCLogger(t),
		stopCtx:    stopCtx,
		stop:       stop,
	}

	start := time.Now()
	must.NoError(t, h.Prestart(context.Background(), nil, nil))
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

	ttl := 8 * time.Second

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

	widmgr := NewMockWIDMgr(task.Identities)

	mockTR := &MockTokenSetter{}

	stopCtx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)

	h := &identityHook{
		alloc:      alloc,
		task:       task,
		tokenDir:   secretsDir,
		envBuilder: taskenv.NewBuilder(node, alloc, task, alloc.Job.Region),
		ts:         mockTR,
		widmgr:     widmgr,
		minWait:    time.Second,
		logger:     testlog.HCLogger(t),
		stopCtx:    stopCtx,
		stop:       stop,
	}

	start := time.Now()
	must.NoError(t, h.Prestart(context.Background(), nil, nil))
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
		widmgr:     NewMockWIDMgr(nil),
		minWait:    time.Second,
		logger:     testlog.HCLogger(t),
		stopCtx:    stopCtx,
		stop:       stop,
	}

	// Prestart should fail when trying to write the default identity file
	err := h.Prestart(context.Background(), nil, nil)
	must.ErrorContains(t, err, "failed to write nomad token")
}

// TestIdentityHook_GetIdentitiesMismatch asserts that if SignIdentities() does
// not return enough identities then Prestart fails.
func TestIdentityHook_GetIdentitiesMismatch(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	task := alloc.LookupTask("web")
	task.Identities = []*structs.WorkloadIdentity{
		{
			Name:     "consul",
			Audience: []string{"consul"},
			TTL:      time.Minute,
		},
	}
	node := mock.Node()
	stopCtx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)

	wids := []*structs.WorkloadIdentity{
		{
			Name: "not-consul",
		},
	}
	h := &identityHook{
		alloc:      alloc,
		task:       task,
		tokenDir:   t.TempDir(),
		envBuilder: taskenv.NewBuilder(node, alloc, task, alloc.Job.Region),
		ts:         &MockTokenSetter{},
		widmgr:     NewMockWIDMgr(wids),
		minWait:    time.Second,
		logger:     testlog.HCLogger(t),
		stopCtx:    stopCtx,
		stop:       stop,
	}

	// Prestart should fail when trying to write the default identity file
	err := h.Prestart(context.Background(), nil, nil)
	must.ErrorContains(t, err, "error fetching alternate identities")
}
