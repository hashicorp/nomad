// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"errors"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/uuid"
)

var _ ACLsAPI = (*MockACLsAPI)(nil)

// MockACLsAPI is a mock of consul.ACLsAPI
type MockACLsAPI struct {
	logger hclog.Logger

	lock  sync.Mutex
	state struct {
		index  uint64
		error  error
		tokens map[string]*api.ACLToken
	}
}

func NewMockACLsAPI(l hclog.Logger) *MockACLsAPI {
	return &MockACLsAPI{
		logger: l.Named("mock_consul"),
		state: struct {
			index  uint64
			error  error
			tokens map[string]*api.ACLToken
		}{tokens: make(map[string]*api.ACLToken)},
	}
}

// Example Consul policies for use in tests.
const (
	ExamplePolicyID1 = "a7c86856-0af5-4ab5-8834-03f4517e5564"
	ExamplePolicyID2 = "ffa1b66c-967d-4468-8775-c687b5cfc16e"
	ExamplePolicyID3 = "f68f0c36-51f8-4343-97dd-f0d4816c915f"
	ExamplePolicyID4 = "1087ff34-b8a0-9bb3-9430-d2f758f52bd3"
)

func (m *MockACLsAPI) PolicyRead(policyID string, _ *api.QueryOptions) (*api.ACLPolicy, *api.QueryMeta, error) {
	switch policyID {

	case ExamplePolicyID1:
		return &api.ACLPolicy{
			ID:    ExamplePolicyID1,
			Name:  "example-policy-1",
			Rules: `service "service1" { policy = "write" }`,
		}, nil, nil

	case ExamplePolicyID2:
		return &api.ACLPolicy{
			ID:    ExamplePolicyID2,
			Rules: `service_prefix "foo-" { policy = "write" }`,
		}, nil, nil

	case ExamplePolicyID3:
		return &api.ACLPolicy{
			ID: ExamplePolicyID3,
			Rules: `
service "service1" { policy = "read" }
service "service2" { policy = "write" }`,
		}, nil, nil

	case ExamplePolicyID4:
		return &api.ACLPolicy{
			ID:    ExamplePolicyID4,
			Rules: `key_prefix "" { policy = "read" }`,
		}, nil, nil

	default:
		return nil, nil, errors.New("no such policy")
	}
}

// Example Consul roles for use in tests.
const (
	ExampleRoleID1 = "e569a3a8-7dfb-b024-e492-e790fe3c4183"
	ExampleRoleID2 = "88c825f4-d0da-1c2b-0c1c-cc9fe84c4468"
	ExampleRoleID3 = "b19b2058-6205-6dff-d2b0-470f29b8e627"
)

func (m *MockACLsAPI) RoleRead(roleID string, _ *api.QueryOptions) (*api.ACLRole, *api.QueryMeta, error) {
	switch roleID {
	case ExampleRoleID1:
		return &api.ACLRole{
			ID:   ExampleRoleID1,
			Name: "example-role-1",
			Policies: []*api.ACLRolePolicyLink{{
				ID:   ExamplePolicyID1,
				Name: "example-policy-1",
			}},
			ServiceIdentities: nil,
		}, nil, nil
	case ExampleRoleID2:
		return &api.ACLRole{
			ID:   ExampleRoleID2,
			Name: "example-role-2",
			Policies: []*api.ACLRolePolicyLink{{
				ID:   ExamplePolicyID2,
				Name: "example-policy-2",
			}},
			ServiceIdentities: nil,
		}, nil, nil
	case ExampleRoleID3:
		return &api.ACLRole{
			ID:                ExampleRoleID3,
			Name:              "example-role-3",
			Policies:          nil, // todo add more if needed
			ServiceIdentities: nil, // todo add more if needed
		}, nil, nil
	default:
		return nil, nil, nil
	}
}

// Example Consul ACL tokens for use in tests. These tokens belong to the
// default Consul namespace.
const (
	ExampleOperatorTokenID0 = "de591604-86eb-1e6f-8b44-d4db752921ae"
	ExampleOperatorTokenID1 = "59c219c2-47e4-43f3-bb45-258fd13f59d5"
	ExampleOperatorTokenID2 = "868cc216-e123-4c2b-b362-f4d4c087de8e"
	ExampleOperatorTokenID3 = "6177d1b9-c0f6-4118-b891-d818a3cb80b1"
	ExampleOperatorTokenID4 = "754ae26c-f3cc-e088-d486-9c0d20f5eaea"
	ExampleOperatorTokenID5 = "097cbb45-506b-c79c-ec38-82eb0dc0794a"
	ExampleOperatorTokenID6 = "6268bd42-6f72-4c90-9c83-90ed6336dcf9"
)

// Example Consul ACL tokens for use in tests that match the policies as the
// tokens above, but these belong to the "banana" Consul namespace.
const (
	ExampleOperatorTokenID10 = "ddfe688f-655f-e8dd-1db5-5650eed00aeb"
	ExampleOperatorTokenID11 = "46d09394-598c-1e55-b7fd-64cd2f409707"
	ExampleOperatorTokenID12 = "a041cb88-0f4b-0314-89f6-10e1e093d2e5"
	ExampleOperatorTokenID13 = "cc22a583-243f-3258-14ad-db0e56749657"
	ExampleOperatorTokenID14 = "5b6d0508-13a6-4bc3-33a1-ba1941e1175b"
	ExampleOperatorTokenID15 = "e9db1754-c075-d0fc-0a7e-de1e9e7bff98"
)

// Example Consul ACL tokens for use in tests that match the policies as the
// tokens above, but these belong to the "default" Consul namespace.
const (
	ExampleOperatorTokenID20 = "937b3287-557c-5af8-beb0-d62191988719"
	ExampleOperatorTokenID21 = "067fd927-abfb-d98f-b693-bb05dccea565"
	ExampleOperatorTokenID22 = "71f8030f-f6bd-6157-6614-ba6a0bbfba9f"
	ExampleOperatorTokenID23 = "1dfd2982-b7a1-89ec-09b4-74712983d13c"
	ExampleOperatorTokenID24 = "d26dbc2a-d5d8-e3d9-8a38-e05dec499124"
	ExampleOperatorTokenID25 = "dd5a8eef-554c-a1f9-fdb8-f25eb77258bc"
)

var (
	// In no Consul namespace (OSS, ENT w/o Namespaces)

	ExampleOperatorToken0 = &api.ACLToken{
		SecretID:    ExampleOperatorTokenID0,
		AccessorID:  "228865c6-3bf6-6683-df03-06dea2779088 ",
		Description: "Operator Token 0",
		Namespace:   "",
	}

	ExampleOperatorToken1 = &api.ACLToken{
		SecretID:    ExampleOperatorTokenID1,
		AccessorID:  "e341bacd-535e-417c-8f45-f88d7faffcaf",
		Description: "Operator Token 1",
		Policies: []*api.ACLTokenPolicyLink{{
			ID: ExamplePolicyID1,
		}},
		Namespace: "",
	}

	ExampleOperatorToken2 = &api.ACLToken{
		SecretID:    ExampleOperatorTokenID2,
		AccessorID:  "615b4d77-5164-4ec6-b616-24c0b24ac9cb",
		Description: "Operator Token 2",
		Policies: []*api.ACLTokenPolicyLink{{
			ID: ExamplePolicyID2,
		}},
		Namespace: "",
	}

	ExampleOperatorToken3 = &api.ACLToken{
		SecretID:    ExampleOperatorTokenID3,
		AccessorID:  "6b7de0d7-15f7-45b4-95eb-fb775bfe3fdc",
		Description: "Operator Token 3",
		Policies: []*api.ACLTokenPolicyLink{{
			ID: ExamplePolicyID3,
		}},
		Namespace: "",
	}

	ExampleOperatorToken4 = &api.ACLToken{
		SecretID:    ExampleOperatorTokenID4,
		AccessorID:  "7b5fdb1a-71e5-f3d8-2cfe-448d973f327d",
		Description: "Operator Token 4",
		Policies:    nil, // no direct policy, only roles
		Roles: []*api.ACLTokenRoleLink{{
			ID:   ExampleRoleID1,
			Name: "example-role-1",
		}},
		Namespace: "",
	}

	ExampleOperatorToken5 = &api.ACLToken{
		SecretID:    ExampleOperatorTokenID5,
		AccessorID:  "cf39aad5-00c3-af23-cf0b-75d41e12f28d",
		Description: "Operator Token 5",
		Policies: []*api.ACLTokenPolicyLink{{
			ID: ExamplePolicyID4,
		}},
		Namespace: "",
	}

	ExampleOperatorToken6 = &api.ACLToken{
		SecretID:    ExampleOperatorTokenID6,
		AccessorID:  "93786935-8856-6e17-0488-c5370a1f044e",
		Description: "Operator Token 6",
		ServiceIdentities: []*api.ACLServiceIdentity{
			{ServiceName: "service1"},
		},
		Namespace: "",
	}

	// In Consul namespace "banana"

	ExampleOperatorToken10 = &api.ACLToken{
		SecretID:    ExampleOperatorTokenID10,
		AccessorID:  "76a2c3b5-5d64-9089-f701-660eec2d3554",
		Description: "Operator Token 0",
		Namespace:   "banana",
	}

	ExampleOperatorToken11 = &api.ACLToken{
		SecretID:    ExampleOperatorTokenID11,
		AccessorID:  "40f2a36a-0a65-1972-106c-b2e5dd46d6e8",
		Description: "Operator Token 1",
		Policies: []*api.ACLTokenPolicyLink{{
			ID: ExamplePolicyID1,
		}},
		Namespace: "banana",
	}

	ExampleOperatorToken12 = &api.ACLToken{
		SecretID:    ExampleOperatorTokenID12,
		AccessorID:  "894f2c5c-b285-71bf-4acb-6344cecf71f3",
		Description: "Operator Token 2",
		Policies: []*api.ACLTokenPolicyLink{{
			ID: ExamplePolicyID2,
		}},
		Namespace: "banana",
	}

	ExampleOperatorToken13 = &api.ACLToken{
		SecretID:    ExampleOperatorTokenID13,
		AccessorID:  "2a81ec0b-692e-845e-f5b8-c33c05e5af22",
		Description: "Operator Token 3",
		Policies: []*api.ACLTokenPolicyLink{{
			ID: ExamplePolicyID3,
		}},
		Namespace: "banana",
	}

	ExampleOperatorToken14 = &api.ACLToken{
		SecretID:    ExampleOperatorTokenID14,
		AccessorID:  "4273f1cc-5626-7a77-dc65-1f24af035ed5d",
		Description: "Operator Token 4",
		Policies:    nil, // no direct policy, only roles
		Roles: []*api.ACLTokenRoleLink{{
			ID:   ExampleRoleID1,
			Name: "example-role-1",
		}},
		Namespace: "banana",
	}

	ExampleOperatorToken15 = &api.ACLToken{
		SecretID:    ExampleOperatorTokenID15,
		AccessorID:  "5b78e186-87d8-c1ad-966f-f5fa87b05c9a",
		Description: "Operator Token 5",
		Policies: []*api.ACLTokenPolicyLink{{
			ID: ExamplePolicyID4,
		}},
		Namespace: "banana",
	}

	// In Consul namespace "default"

	ExampleOperatorToken20 = &api.ACLToken{
		SecretID:    ExampleOperatorTokenID20,
		AccessorID:  "228865c6-3bf6-6683-df03-06dea2779088",
		Description: "Operator Token 0",
		// Should still be able to register jobs where no namespace was set
		Namespace: "default",
	}

	ExampleOperatorToken21 = &api.ACLToken{
		SecretID:    ExampleOperatorTokenID21,
		AccessorID:  "54d01af9-5036-31d3-296b-b15b941d7aa2",
		Description: "Operator Token 1",
		Policies: []*api.ACLTokenPolicyLink{{
			ID: ExamplePolicyID1,
		}},
		// Should still be able to register jobs where no namespace was set
		Namespace: "default",
	}

	ExampleOperatorToken22 = &api.ACLToken{
		SecretID:    ExampleOperatorTokenID22,
		AccessorID:  "894f2c5c-b285-71bf-4acb-6344cecf71f3",
		Description: "Operator Token 2",
		Policies: []*api.ACLTokenPolicyLink{{
			ID: ExamplePolicyID2,
		}},
		Namespace: "default",
	}

	ExampleOperatorToken23 = &api.ACLToken{
		SecretID:    ExampleOperatorTokenID23,
		AccessorID:  "2a81ec0b-692e-845e-f5b8-c33c05e5af22",
		Description: "Operator Token 3",
		Policies: []*api.ACLTokenPolicyLink{{
			ID: ExamplePolicyID3,
		}},
		Namespace: "default",
	}

	ExampleOperatorToken24 = &api.ACLToken{
		SecretID:    ExampleOperatorTokenID24,
		AccessorID:  "4273f1cc-5626-7a77-dc65-1f24af035ed5d",
		Description: "Operator Token 4",
		Policies:    nil, // no direct policy, only roles
		Roles: []*api.ACLTokenRoleLink{{
			ID:   ExampleRoleID1,
			Name: "example-role-1",
		}},
		Namespace: "default",
	}

	ExampleOperatorToken25 = &api.ACLToken{
		SecretID:    ExampleOperatorTokenID25,
		AccessorID:  "5b78e186-87d8-c1ad-966f-f5fa87b05c9a",
		Description: "Operator Token 5",
		Policies: []*api.ACLTokenPolicyLink{{
			ID: ExamplePolicyID4,
		}},
		Namespace: "default",
	}
)

func (m *MockACLsAPI) TokenReadSelf(q *api.QueryOptions) (*api.ACLToken, *api.QueryMeta, error) {
	switch q.Token {

	case ExampleOperatorTokenID1:
		return ExampleOperatorToken1, nil, nil

	case ExampleOperatorTokenID2:
		return ExampleOperatorToken2, nil, nil

	case ExampleOperatorTokenID3:
		return ExampleOperatorToken3, nil, nil

	case ExampleOperatorTokenID4:
		return ExampleOperatorToken4, nil, nil

	case ExampleOperatorTokenID5:
		return ExampleOperatorToken5, nil, nil

	case ExampleOperatorTokenID10:
		return ExampleOperatorToken10, nil, nil

	case ExampleOperatorTokenID11:
		return ExampleOperatorToken11, nil, nil

	case ExampleOperatorTokenID12:
		return ExampleOperatorToken12, nil, nil

	case ExampleOperatorTokenID13:
		return ExampleOperatorToken13, nil, nil

	case ExampleOperatorTokenID14:
		return ExampleOperatorToken14, nil, nil

	case ExampleOperatorTokenID15:
		return ExampleOperatorToken15, nil, nil

	case ExampleOperatorTokenID20:
		return ExampleOperatorToken20, nil, nil

	case ExampleOperatorTokenID21:
		return ExampleOperatorToken21, nil, nil

	case ExampleOperatorTokenID22:
		return ExampleOperatorToken22, nil, nil

	case ExampleOperatorTokenID23:
		return ExampleOperatorToken23, nil, nil

	case ExampleOperatorTokenID24:
		return ExampleOperatorToken24, nil, nil

	case ExampleOperatorTokenID25:
		return ExampleOperatorToken25, nil, nil

	default:
		return nil, nil, errors.New("no such token")
	}
}

// SetError is a helper method for configuring an error that will be returned
// on future calls to mocked methods.
func (m *MockACLsAPI) SetError(err error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.state.error = err
}

// TokenCreate is a mock of ACLsAPI.TokenCreate
func (m *MockACLsAPI) TokenCreate(token *api.ACLToken, opts *api.WriteOptions) (*api.ACLToken, *api.WriteMeta, error) {
	index, created, meta, err := m.tokenCreate(token, opts)

	services := func(token *api.ACLToken) []string {
		if token == nil {
			return nil
		}
		var names []string
		for _, id := range token.ServiceIdentities {
			names = append(names, id.ServiceName)
		}
		return names
	}(created)

	description := func(token *api.ACLToken) string {
		if token == nil {
			return "<nil>"
		}
		return token.Description
	}(created)

	accessor := func(token *api.ACLToken) string {
		if token == nil {
			return "<nil>"
		}
		return token.AccessorID
	}(created)

	secret := func(token *api.ACLToken) string {
		if token == nil {
			return "<nil>"
		}
		return token.SecretID
	}(created)

	m.logger.Trace("TokenCreate()", "description", description, "service_identities", services, "accessor", accessor, "secret", secret, "index", index, "error", err)
	return created, meta, err
}

func (m *MockACLsAPI) tokenCreate(token *api.ACLToken, _ *api.WriteOptions) (uint64, *api.ACLToken, *api.WriteMeta, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.state.index++

	if m.state.error != nil {
		return m.state.index, nil, nil, m.state.error
	}

	secret := &api.ACLToken{
		CreateIndex:       m.state.index,
		ModifyIndex:       m.state.index,
		AccessorID:        uuid.Generate(),
		SecretID:          uuid.Generate(),
		Description:       token.Description,
		ServiceIdentities: token.ServiceIdentities,
		Namespace:         token.Namespace,
		CreateTime:        time.Now(),
	}

	m.state.tokens[secret.AccessorID] = secret

	w := &api.WriteMeta{
		RequestTime: 1 * time.Millisecond,
	}

	return m.state.index, secret, w, nil
}

// TokenDelete is a mock of ACLsAPI.TokenDelete
func (m *MockACLsAPI) TokenDelete(accessorID string, opts *api.WriteOptions) (*api.WriteMeta, error) {
	meta, err := m.tokenDelete(accessorID, opts)
	m.logger.Trace("TokenDelete()", "accessor", accessorID, "error", err)
	return meta, err
}

func (m *MockACLsAPI) tokenDelete(tokenID string, _ *api.WriteOptions) (*api.WriteMeta, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.state.index++

	if m.state.error != nil {
		return nil, m.state.error
	}

	if _, exists := m.state.tokens[tokenID]; !exists {
		return nil, nil // consul no-ops delete of non-existent token
	}

	delete(m.state.tokens, tokenID)

	m.logger.Trace("TokenDelete()")

	return nil, nil
}

// TokenList is a mock of ACLsAPI.TokenList
func (m *MockACLsAPI) TokenList(_ *api.QueryOptions) ([]*api.ACLTokenListEntry, *api.QueryMeta, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	//todo(shoenig): will need this for background token reconciliation
	// coming in another issue

	return nil, nil, nil
}
