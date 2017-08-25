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

// ACLTokens is used to query the ACL token endpoints.
type ACLTokens struct {
	client *Client
}

// ACLTokens returns a new handle on the ACL tokens.
func (c *Client) ACLTokens() *ACLTokens {
	return &ACLTokens{client: c}
}

// Bootstrap is used to get the initial bootstrap token
func (a *ACLTokens) Bootstrap(q *WriteOptions) (*ACLToken, *WriteMeta, error) {
	var resp ACLToken
	wm, err := a.client.write("/v1/acl/bootstrap", nil, &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return &resp, wm, nil
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
