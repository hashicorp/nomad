// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package widmgr

import (
	"crypto/ed25519"
	"fmt"
	"slices"
	"time"

	"github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
)

// MockWIDSigner allows TaskRunner unit tests to avoid having to setup a Server,
// Client, and Allocation.
type MockWIDSigner struct {
	// wids maps identity names to workload identities. If wids is non-nil then
	// SignIdentities will use it to find expirations or reject invalid identity
	// names
	wids  map[string]*structs.WorkloadIdentity
	key   ed25519.PrivateKey
	keyID string
}

func NewMockWIDSigner(wids []*structs.WorkloadIdentity) *MockWIDSigner {
	_, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		panic(err)
	}
	m := &MockWIDSigner{
		key:   privKey,
		keyID: uuid.Generate(),
	}
	if wids != nil {
		m.setWIDs(wids)
	}
	return m
}

// setWIDs is a test helper to use Task.Identities in the MockWIDSigner for
// sharing TTLs and validating names.
func (m *MockWIDSigner) setWIDs(wids []*structs.WorkloadIdentity) {
	m.wids = make(map[string]*structs.WorkloadIdentity, len(wids))
	for _, wid := range wids {
		m.wids[wid.Name] = wid
	}
}
func (m *MockWIDSigner) SignIdentities(minIndex uint64, req []*structs.WorkloadIdentityRequest) ([]*structs.SignedWorkloadIdentity, error) {
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

// MockWIDMgr mocks IdentityManager interface allowing to only get identities
// signed by the mock signer.
type MockWIDMgr struct {
	swids map[structs.WIHandle]*structs.SignedWorkloadIdentity
}

func NewMockWIDMgr(swids map[structs.WIHandle]*structs.SignedWorkloadIdentity) *MockWIDMgr {
	return &MockWIDMgr{swids: swids}
}

// Run does not run a renewal loop in this mock
func (m MockWIDMgr) Run() error { return nil }

func (m MockWIDMgr) Get(identity structs.WIHandle) (*structs.SignedWorkloadIdentity, error) {
	sid, ok := m.swids[identity]
	if !ok {
		return nil, fmt.Errorf("identity not found")
	}
	return sid, nil
}

// Watch does not do anything, this mock doesn't support watching.
func (m MockWIDMgr) Watch(identity structs.WIHandle) (<-chan *structs.SignedWorkloadIdentity, func()) {
	return nil, nil
}

func (m MockWIDMgr) Shutdown() {}
