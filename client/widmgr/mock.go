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
	wids    map[string]*structs.WorkloadIdentity
	key     ed25519.PrivateKey
	keyID   string
	mockNow time.Time // allows moving the clock
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

// now returns the mocked time or falls back to the clock
func (m *MockWIDSigner) now() time.Time {
	if m.mockNow.IsZero() {
		return time.Now()
	}
	return m.mockNow
}

func (m *MockWIDSigner) JSONWebKeySet() *jose.JSONWebKeySet {
	jwk := jose.JSONWebKey{
		Key:       m.key.Public(),
		KeyID:     m.keyID,
		Algorithm: "EdDSA",
		Use:       "sig",
	}
	return &jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{jwk},
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
			TaskName:     idReq.WorkloadIdentifier,
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
				claims.Expiry = jwt.NewNumericDate(m.now().Add(wid.TTL))
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

func NewMockWIDMgr(swids []*structs.SignedWorkloadIdentity) *MockWIDMgr {
	swidmap := map[structs.WIHandle]*structs.SignedWorkloadIdentity{}
	for _, id := range swids {
		swidmap[id.WIHandle] = id
	}
	return &MockWIDMgr{swids: swidmap}
}

// Run does not run a renewal loop in this mock
func (m MockWIDMgr) Run() error { return nil }

func (m MockWIDMgr) Get(id structs.WIHandle) (*structs.SignedWorkloadIdentity, error) {
	sid, ok := m.swids[id]
	if !ok {
		return nil, fmt.Errorf("unable to find token for workload %q and identity %q", id.WorkloadIdentifier, id.IdentityName)
	}
	return sid, nil
}

// Watch does not do anything, this mock doesn't support watching.
func (m MockWIDMgr) Watch(identity structs.WIHandle) (<-chan *structs.SignedWorkloadIdentity, func()) {
	return nil, nil
}

func (m MockWIDMgr) Shutdown() {}
