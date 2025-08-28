// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package widmgr

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"slices"
	"sync"
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
	key     *rsa.PrivateKey
	keyID   string
	mockNow time.Time // allows moving the clock
}

func NewMockWIDSigner(wids []*structs.WorkloadIdentity) *MockWIDSigner {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
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
		Algorithm: "RS256",
		Use:       "sig",
	}
	return &jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{jwk},
	}
}

func (m *MockWIDSigner) SignIdentities(_ uint64, req []*structs.WorkloadIdentityRequest) ([]*structs.SignedWorkloadIdentity, error) {
	swids := make([]*structs.SignedWorkloadIdentity, 0, len(req))
	for _, idReq := range req {
		// Set test values for default claims
		claims := &structs.IdentityClaims{
			WorkloadIdentityClaims: &structs.WorkloadIdentityClaims{
				Namespace:    "default",
				JobID:        "test",
				AllocationID: idReq.AllocID,
				TaskName:     idReq.WorkloadIdentifier,
			},
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
		sig, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: m.key}, opts)
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

type MockIdentityManager struct {
	lastToken     map[structs.WIHandle]*structs.SignedWorkloadIdentity
	lastTokenLock sync.RWMutex
}

// NewMockIdentityManager returns an implementation of the IdentityManager
// interface which supports data manipulation for testing.
func NewMockIdentityManager() IdentityManager {
	return &MockIdentityManager{
		lastToken: make(map[structs.WIHandle]*structs.SignedWorkloadIdentity),
	}
}

// Get implements the IdentityManager.Get functionality. This should be used
// along with SetIdentity for testing.
func (m *MockIdentityManager) Get(handle structs.WIHandle) (*structs.SignedWorkloadIdentity, error) {
	m.lastTokenLock.RLock()
	defer m.lastTokenLock.RUnlock()

	token := m.lastToken[handle]
	if token == nil {
		return nil, fmt.Errorf("no token for handle name:%s wid:%s type:%v",
			handle.IdentityName, handle.WorkloadIdentifier, handle.WorkloadType)
	}

	return token, nil
}

// Run implements the IdentityManager.Run functionality. It currently does
// nothing.
func (m *MockIdentityManager) Run() error { return nil }

// Watch implements the IdentityManager.Watch functionality. It currently does
// nothing.
func (m *MockIdentityManager) Watch(_ structs.WIHandle) (<-chan *structs.SignedWorkloadIdentity, func()) {
	return nil, nil
}

// Shutdown implements the IdentityManager.Shutdown functionality. It currently
// does nothing.
func (m *MockIdentityManager) Shutdown() {}

// SetIdentity is a helper function that allows testing callers to set custom
// identity information. The constructor function returns the interface name,
// therefore to call this you will need assert the type like
// ".(*widmgr.MockIdentityManager).SetIdentity(...)".
func (m *MockIdentityManager) SetIdentity(handle structs.WIHandle, token *structs.SignedWorkloadIdentity) {
	m.lastTokenLock.Lock()
	m.lastToken[handle] = token
	m.lastTokenLock.Unlock()
}
