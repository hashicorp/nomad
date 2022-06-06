package api

import (
	"fmt"
	"time"
)

// ACLPolicies is used to query the ACL Policy endpoints.
type ACLPolicies struct {
	client *Client
}

// ACLPolicies returns a new handle on the ACL policies.
func (c *Client) ACLPolicies() *ACLPolicies {
	return &ACLPolicies{client: c}
}

// List is used to dump all of the policies.
func (a *ACLPolicies) List(q *QueryOptions) ([]*ACLPolicyListStub, *QueryMeta, error) {
	var resp []*ACLPolicyListStub
	qm, err := a.client.query("/v1/acl/policies", &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return resp, qm, nil
}

// Upsert is used to create or update a policy
func (a *ACLPolicies) Upsert(policy *ACLPolicy, q *WriteOptions) (*WriteMeta, error) {
	if policy == nil || policy.Name == "" {
		return nil, fmt.Errorf("missing policy name")
	}
	wm, err := a.client.write("/v1/acl/policy/"+policy.Name, policy, nil, q)
	if err != nil {
		return nil, err
	}
	return wm, nil
}

// Delete is used to delete a policy
func (a *ACLPolicies) Delete(policyName string, q *WriteOptions) (*WriteMeta, error) {
	if policyName == "" {
		return nil, fmt.Errorf("missing policy name")
	}
	wm, err := a.client.delete("/v1/acl/policy/"+policyName, nil, q)
	if err != nil {
		return nil, err
	}
	return wm, nil
}

// Info is used to query a specific policy
func (a *ACLPolicies) Info(policyName string, q *QueryOptions) (*ACLPolicy, *QueryMeta, error) {
	if policyName == "" {
		return nil, nil, fmt.Errorf("missing policy name")
	}
	var resp ACLPolicy
	wm, err := a.client.query("/v1/acl/policy/"+policyName, &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return &resp, wm, nil
}

// ACLTokens is used to query the ACL token endpoints.
type ACLTokens struct {
	client *Client
}

// ACLTokens returns a new handle on the ACL tokens.
func (c *Client) ACLTokens() *ACLTokens {
	return &ACLTokens{client: c}
}

// DEPRECATED: will be removed in Nomad 1.5.0
// Bootstrap is used to get the initial bootstrap token
func (a *ACLTokens) Bootstrap(q *WriteOptions) (*ACLToken, *WriteMeta, error) {
	var resp ACLToken
	wm, err := a.client.write("/v1/acl/bootstrap", nil, &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return &resp, wm, nil
}

// BootstrapOpts is used to get the initial bootstrap token or pass in the one that was provided in the API
func (a *ACLTokens) BootstrapOpts(btoken string, q *WriteOptions) (*ACLToken, *WriteMeta, error) {
	if q == nil {
		q = &WriteOptions{}
	}
	req := &BootstrapRequest{
		BootstrapSecret: btoken,
	}

	var resp ACLToken
	wm, err := a.client.write("/v1/acl/bootstrap", req, &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return &resp, wm, nil
}

// List is used to dump all of the tokens.
func (a *ACLTokens) List(q *QueryOptions) ([]*ACLTokenListStub, *QueryMeta, error) {
	var resp []*ACLTokenListStub
	qm, err := a.client.query("/v1/acl/tokens", &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return resp, qm, nil
}

// Create is used to create a token
func (a *ACLTokens) Create(token *ACLToken, q *WriteOptions) (*ACLToken, *WriteMeta, error) {
	if token.AccessorID != "" {
		return nil, nil, fmt.Errorf("cannot specify Accessor ID")
	}
	var resp ACLToken
	wm, err := a.client.write("/v1/acl/token", token, &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return &resp, wm, nil
}

// Update is used to update an existing token
func (a *ACLTokens) Update(token *ACLToken, q *WriteOptions) (*ACLToken, *WriteMeta, error) {
	if token.AccessorID == "" {
		return nil, nil, fmt.Errorf("missing accessor ID")
	}
	var resp ACLToken
	wm, err := a.client.write("/v1/acl/token/"+token.AccessorID,
		token, &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return &resp, wm, nil
}

// Delete is used to delete a token
func (a *ACLTokens) Delete(accessorID string, q *WriteOptions) (*WriteMeta, error) {
	if accessorID == "" {
		return nil, fmt.Errorf("missing accessor ID")
	}
	wm, err := a.client.delete("/v1/acl/token/"+accessorID, nil, q)
	if err != nil {
		return nil, err
	}
	return wm, nil
}

// Info is used to query a token
func (a *ACLTokens) Info(accessorID string, q *QueryOptions) (*ACLToken, *QueryMeta, error) {
	if accessorID == "" {
		return nil, nil, fmt.Errorf("missing accessor ID")
	}
	var resp ACLToken
	wm, err := a.client.query("/v1/acl/token/"+accessorID, &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return &resp, wm, nil
}

// Self is used to query our own token
func (a *ACLTokens) Self(q *QueryOptions) (*ACLToken, *QueryMeta, error) {
	var resp ACLToken
	wm, err := a.client.query("/v1/acl/token/self", &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return &resp, wm, nil
}

// UpsertOneTimeToken is used to create a one-time token
func (a *ACLTokens) UpsertOneTimeToken(q *WriteOptions) (*OneTimeToken, *WriteMeta, error) {
	var resp *OneTimeTokenUpsertResponse
	wm, err := a.client.write("/v1/acl/token/onetime", nil, &resp, q)
	if err != nil {
		return nil, nil, err
	}
	if resp == nil {
		return nil, nil, fmt.Errorf("no one-time token returned")
	}
	return resp.OneTimeToken, wm, nil
}

// ExchangeOneTimeToken is used to create a one-time token
func (a *ACLTokens) ExchangeOneTimeToken(secret string, q *WriteOptions) (*ACLToken, *WriteMeta, error) {
	if secret == "" {
		return nil, nil, fmt.Errorf("missing secret ID")
	}
	req := &OneTimeTokenExchangeRequest{OneTimeSecretID: secret}
	var resp *OneTimeTokenExchangeResponse
	wm, err := a.client.write("/v1/acl/token/onetime/exchange", req, &resp, q)
	if err != nil {
		return nil, nil, err
	}
	if resp == nil {
		return nil, nil, fmt.Errorf("no ACL token returned")
	}
	return resp.Token, wm, nil
}

// ACLPolicyListStub is used to for listing ACL policies
type ACLPolicyListStub struct {
	Name        string
	Description string
	CreateIndex uint64
	ModifyIndex uint64
}

// ACLPolicy is used to represent an ACL policy
type ACLPolicy struct {
	Name        string
	Description string
	Rules       string
	CreateIndex uint64
	ModifyIndex uint64
}

// ACLToken represents a client token which is used to Authenticate
type ACLToken struct {
	AccessorID  string
	SecretID    string
	Name        string
	Type        string
	Policies    []string
	Global      bool
	CreateTime  time.Time
	CreateIndex uint64
	ModifyIndex uint64
}

type ACLTokenListStub struct {
	AccessorID  string
	Name        string
	Type        string
	Policies    []string
	Global      bool
	CreateTime  time.Time
	CreateIndex uint64
	ModifyIndex uint64
}

type OneTimeToken struct {
	OneTimeSecretID string
	AccessorID      string
	ExpiresAt       time.Time
	CreateIndex     uint64
	ModifyIndex     uint64
}

type OneTimeTokenUpsertResponse struct {
	OneTimeToken *OneTimeToken
}

type OneTimeTokenExchangeRequest struct {
	OneTimeSecretID string
}

type OneTimeTokenExchangeResponse struct {
	Token *ACLToken
}

// BootstrapRequest is used for when operators provide an ACL Bootstrap Token
type BootstrapRequest struct {
	BootstrapSecret string
}
