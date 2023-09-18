// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/shoenig/test/must"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/widmgr"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

// statically assert network hook implements the expected interfaces
var _ interfaces.RunnerPrerunHook = (*identityHook)(nil)
var _ interfaces.ShutdownHook = (*identityHook)(nil)
var _ interfaces.TaskStopHook = (*identityHook)(nil)

// MockWIDMgr allows TaskRunner unit tests to avoid having to setup a Server,
// Client, and Allocation.
type MockWIDMgr struct {
	// wids maps identity names to workload identities. If wids is non-nil then
	// SignIdentities will use it to find expirations or reject invalid identity
	// names
	wids  map[string]*structs.WorkloadIdentity
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
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Identity = wid
	task.Identities = []*structs.WorkloadIdentity{wid}

	allocrunner, stopAR := TestAllocRunnerFromAlloc(t, alloc)
	defer stopAR()

	logger := testlog.HCLogger(t)

	// setup mock signer and WIDMgr
	mockSigner := NewMockWIDMgr([]*structs.WorkloadIdentity{task.Identity})
	mockWIDMgr := widmgr.NewWIDMgr(mockSigner, alloc, logger)
	allocrunner.widmgr = mockWIDMgr
	allocrunner.widsigner = mockSigner

	// do the initial signing
	_, err := mockSigner.SignIdentities(1, []*structs.WorkloadIdentityRequest{
		{
			AllocID:      alloc.ID,
			TaskName:     task.Name,
			IdentityName: task.Identity.Name,
		},
	})
	must.NoError(t, err)

	start := time.Now()
	hook := newIdentityHook(logger, allocrunner)
	must.Eq(t, hook.Name(), "identity")
	must.NoError(t, hook.Prerun())

	sid, err := hook.widmgr.Get(cstructs.TaskIdentity{TaskName: task.Name, IdentityName: task.Identity.Name})
	must.Nil(t, err)
	must.Eq(t, sid.IdentityName, task.Identity.Name)
	must.NotEq(t, sid.JWT, "")

	// pad expiry time with a second to be safe
	must.True(t, inTimeSpan(start.Add(ttl).Add(-1*time.Second), start.Add(ttl).Add(1*time.Second), sid.Expiration))

	must.NoError(t, hook.Stop(context.Background(), nil, nil))
}

func inTimeSpan(start, end, check time.Time) bool {
	return check.After(start) && check.Before(end)
}
