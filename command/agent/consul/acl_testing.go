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

const (
	ExampleOperatorToken1 = "59c219c2-47e4-43f3-bb45-258fd13f59d5"
	ExampleOperatorToken2 = "868cc216-e123-4c2b-b362-f4d4c087de8e"
	ExampleOperatorToken3 = "6177d1b9-c0f6-4118-b891-d818a3cb80b1"
)

func (m *MockACLsAPI) TokenReadSelf(q *api.QueryOptions) (*api.ACLToken, *api.QueryMeta, error) {
	switch q.Token {

	case ExampleOperatorToken1:
		return &api.ACLToken{
			SecretID:    ExampleOperatorToken1,
			AccessorID:  "e341bacd-535e-417c-8f45-f88d7faffcaf",
			Description: "operator token 1",
			Policies: []*api.ACLTokenPolicyLink{{
				ID: ExamplePolicyID1,
			}},
		}, nil, nil

	case ExampleOperatorToken2:
		return &api.ACLToken{
			SecretID:    ExampleOperatorToken2,
			AccessorID:  "615b4d77-5164-4ec6-b616-24c0b24ac9cb",
			Description: "operator token 2",
			Policies: []*api.ACLTokenPolicyLink{{
				ID: ExamplePolicyID2,
			}},
		}, nil, nil

	case ExampleOperatorToken3:
		return &api.ACLToken{
			SecretID:    ExampleOperatorToken3,
			AccessorID:  "6b7de0d7-15f7-45b4-95eb-fb775bfe3fdc",
			Description: "operator token 3",
			Policies: []*api.ACLTokenPolicyLink{{
				ID: ExamplePolicyID3,
			}},
		}, nil, nil

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
		return nil, errors.New("token does not exist")
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
