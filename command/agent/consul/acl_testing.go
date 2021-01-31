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

// Example Consul "operator" tokens for use in tests.

const (
	ExampleOperatorTokenID0 = "de591604-86eb-1e6f-8b44-d4db752921ae"
	ExampleOperatorTokenID1 = "59c219c2-47e4-43f3-bb45-258fd13f59d5"
	ExampleOperatorTokenID2 = "868cc216-e123-4c2b-b362-f4d4c087de8e"
	ExampleOperatorTokenID3 = "6177d1b9-c0f6-4118-b891-d818a3cb80b1"
	ExampleOperatorTokenID4 = "754ae26c-f3cc-e088-d486-9c0d20f5eaea"
)

var (
	ExampleOperatorToken0 = &api.ACLToken{
		SecretID:    ExampleOperatorTokenID0,
		AccessorID:  "228865c6-3bf6-6683-df03-06dea2779088 ",
		Description: "Operator Token 0",
	}

	ExampleOperatorToken1 = &api.ACLToken{
		SecretID:    ExampleOperatorTokenID1,
		AccessorID:  "e341bacd-535e-417c-8f45-f88d7faffcaf",
		Description: "Operator Token 1",
		Policies: []*api.ACLTokenPolicyLink{{
			ID: ExamplePolicyID1,
		}},
	}

	ExampleOperatorToken2 = &api.ACLToken{
		SecretID:    ExampleOperatorTokenID2,
		AccessorID:  "615b4d77-5164-4ec6-b616-24c0b24ac9cb",
		Description: "Operator Token 2",
		Policies: []*api.ACLTokenPolicyLink{{
			ID: ExamplePolicyID2,
		}},
	}

	ExampleOperatorToken3 = &api.ACLToken{
		SecretID:    ExampleOperatorTokenID3,
		AccessorID:  "6b7de0d7-15f7-45b4-95eb-fb775bfe3fdc",
		Description: "Operator Token 3",
		Policies: []*api.ACLTokenPolicyLink{{
			ID: ExamplePolicyID3,
		}},
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
