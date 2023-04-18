package api

import (
	"encoding/json"
	"errors"
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
		return nil, errors.New("missing policy name")
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
		return nil, errors.New("missing policy name")
	}
	wm, err := a.client.delete("/v1/acl/policy/"+policyName, nil, nil, q)
	if err != nil {
		return nil, err
	}
	return wm, nil
}

// Info is used to query a specific policy
func (a *ACLPolicies) Info(policyName string, q *QueryOptions) (*ACLPolicy, *QueryMeta, error) {
	if policyName == "" {
		return nil, nil, errors.New("missing policy name")
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
		return nil, nil, errors.New("cannot specify Accessor ID")
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
		return nil, nil, errors.New("missing accessor ID")
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
		return nil, errors.New("missing accessor ID")
	}
	wm, err := a.client.delete("/v1/acl/token/"+accessorID, nil, nil, q)
	if err != nil {
		return nil, err
	}
	return wm, nil
}

// Info is used to query a token
func (a *ACLTokens) Info(accessorID string, q *QueryOptions) (*ACLToken, *QueryMeta, error) {
	if accessorID == "" {
		return nil, nil, errors.New("missing accessor ID")
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
		return nil, nil, errors.New("no one-time token returned")
	}
	return resp.OneTimeToken, wm, nil
}

// ExchangeOneTimeToken is used to create a one-time token
func (a *ACLTokens) ExchangeOneTimeToken(secret string, q *WriteOptions) (*ACLToken, *WriteMeta, error) {
	if secret == "" {
		return nil, nil, errors.New("missing secret ID")
	}
	req := &OneTimeTokenExchangeRequest{OneTimeSecretID: secret}
	var resp *OneTimeTokenExchangeResponse
	wm, err := a.client.write("/v1/acl/token/onetime/exchange", req, &resp, q)
	if err != nil {
		return nil, nil, err
	}
	if resp == nil {
		return nil, nil, errors.New("no ACL token returned")
	}
	return resp.Token, wm, nil
}

var (
	// errMissingACLRoleID is the generic errors to use when a call is missing
	// the required ACL Role ID parameter.
	errMissingACLRoleID = errors.New("missing ACL role ID")
)

// ACLRoles is used to query the ACL Role endpoints.
type ACLRoles struct {
	client *Client
}

// ACLRoles returns a new handle on the ACL roles API client.
func (c *Client) ACLRoles() *ACLRoles {
	return &ACLRoles{client: c}
}

// List is used to detail all the ACL roles currently stored within state.
func (a *ACLRoles) List(q *QueryOptions) ([]*ACLRoleListStub, *QueryMeta, error) {
	var resp []*ACLRoleListStub
	qm, err := a.client.query("/v1/acl/roles", &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return resp, qm, nil
}

// Create is used to create an ACL role.
func (a *ACLRoles) Create(role *ACLRole, w *WriteOptions) (*ACLRole, *WriteMeta, error) {
	if role.ID != "" {
		return nil, nil, errors.New("cannot specify ACL role ID")
	}
	var resp ACLRole
	wm, err := a.client.write("/v1/acl/role", role, &resp, w)
	if err != nil {
		return nil, nil, err
	}
	return &resp, wm, nil
}

// Update is used to update an existing ACL role.
func (a *ACLRoles) Update(role *ACLRole, w *WriteOptions) (*ACLRole, *WriteMeta, error) {
	if role.ID == "" {
		return nil, nil, errMissingACLRoleID
	}
	var resp ACLRole
	wm, err := a.client.write("/v1/acl/role/"+role.ID, role, &resp, w)
	if err != nil {
		return nil, nil, err
	}
	return &resp, wm, nil
}

// Delete is used to delete an ACL role.
func (a *ACLRoles) Delete(roleID string, w *WriteOptions) (*WriteMeta, error) {
	if roleID == "" {
		return nil, errMissingACLRoleID
	}
	wm, err := a.client.delete("/v1/acl/role/"+roleID, nil, nil, w)
	if err != nil {
		return nil, err
	}
	return wm, nil
}

// Get is used to look up an ACL role.
func (a *ACLRoles) Get(roleID string, q *QueryOptions) (*ACLRole, *QueryMeta, error) {
	if roleID == "" {
		return nil, nil, errMissingACLRoleID
	}
	var resp ACLRole
	qm, err := a.client.query("/v1/acl/role/"+roleID, &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return &resp, qm, nil
}

// GetByName is used to look up an ACL role using its name.
func (a *ACLRoles) GetByName(roleName string, q *QueryOptions) (*ACLRole, *QueryMeta, error) {
	if roleName == "" {
		return nil, nil, errors.New("missing ACL role name")
	}
	var resp ACLRole
	qm, err := a.client.query("/v1/acl/role/name/"+roleName, &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return &resp, qm, nil
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
	JobACL      *JobACL

	CreateIndex uint64
	ModifyIndex uint64
}

// JobACL represents an ACL policy's attachment to a job, group, or task.
type JobACL struct {
	Namespace string
	JobID     string
	Group     string
	Task      string
}

// ACLToken represents a client token which is used to Authenticate
type ACLToken struct {
	AccessorID string
	SecretID   string
	Name       string
	Type       string
	Policies   []string

	// Roles represents the ACL roles that this token is tied to. The token
	// will inherit the permissions of all policies detailed within the role.
	Roles []*ACLTokenRoleLink

	Global     bool
	CreateTime time.Time

	// ExpirationTime represents the point after which a token should be
	// considered revoked and is eligible for destruction. The zero value of
	// time.Time does not respect json omitempty directives, so we must use a
	// pointer.
	ExpirationTime *time.Time `json:",omitempty"`

	// ExpirationTTL is a convenience field for helping set ExpirationTime to a
	// value of CreateTime+ExpirationTTL. This can only be set during token
	// creation. This is a string version of a time.Duration like "2m".
	ExpirationTTL time.Duration `json:",omitempty"`

	CreateIndex uint64
	ModifyIndex uint64
}

// ACLTokenRoleLink is used to link an ACL token to an ACL role. The ACL token
// can therefore inherit all the ACL policy permissions that the ACL role
// contains.
type ACLTokenRoleLink struct {

	// ID is the ACLRole.ID UUID. This field is immutable and represents the
	// absolute truth for the link.
	ID string

	// Name is the human friendly identifier for the ACL role and is a
	// convenience field for operators.
	Name string
}

// MarshalJSON implements the json.Marshaler interface and allows
// ACLToken.ExpirationTTL to be marshaled correctly.
func (a *ACLToken) MarshalJSON() ([]byte, error) {
	type Alias ACLToken
	exported := &struct {
		ExpirationTTL string
		*Alias
	}{
		ExpirationTTL: a.ExpirationTTL.String(),
		Alias:         (*Alias)(a),
	}
	if a.ExpirationTTL == 0 {
		exported.ExpirationTTL = ""
	}
	return json.Marshal(exported)
}

// UnmarshalJSON implements the json.Unmarshaler interface and allows
// ACLToken.ExpirationTTL to be unmarshalled correctly.
func (a *ACLToken) UnmarshalJSON(data []byte) (err error) {
	type Alias ACLToken
	aux := &struct {
		ExpirationTTL any
		*Alias
	}{
		Alias: (*Alias)(a),
	}

	if err = json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.ExpirationTTL != nil {
		switch v := aux.ExpirationTTL.(type) {
		case string:
			if v != "" {
				if a.ExpirationTTL, err = time.ParseDuration(v); err != nil {
					return err
				}
			}
		case float64:
			a.ExpirationTTL = time.Duration(v)
		}

	}
	return nil
}

type ACLTokenListStub struct {
	AccessorID string
	Name       string
	Type       string
	Policies   []string
	Roles      []*ACLTokenRoleLink
	Global     bool
	CreateTime time.Time

	// ExpirationTime represents the point after which a token should be
	// considered revoked and is eligible for destruction. A nil value
	// indicates no expiration has been set on the token.
	ExpirationTime *time.Time `json:",omitempty"`

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

// ACLRole is an abstraction for the ACL system which allows the grouping of
// ACL policies into a single object. ACL tokens can be created and linked to
// a role; the token then inherits all the permissions granted by the policies.
type ACLRole struct {

	// ID is an internally generated UUID for this role and is controlled by
	// Nomad. It can be used after role creation to update the existing role.
	ID string

	// Name is unique across the entire set of federated clusters and is
	// supplied by the operator on role creation. The name can be modified by
	// updating the role and including the Nomad generated ID. This update will
	// not affect tokens created and linked to this role. This is a required
	// field.
	Name string

	// Description is a human-readable, operator set description that can
	// provide additional context about the role. This is an optional field.
	Description string

	// Policies is an array of ACL policy links. Although currently policies
	// can only be linked using their name, in the future we will want to add
	// IDs also and thus allow operators to specify either a name, an ID, or
	// both. At least one entry is required.
	Policies []*ACLRolePolicyLink

	CreateIndex uint64
	ModifyIndex uint64
}

// ACLRolePolicyLink is used to link a policy to an ACL role. We use a struct
// rather than a list of strings as in the future we will want to add IDs to
// policies and then link via these.
type ACLRolePolicyLink struct {

	// Name is the ACLPolicy.Name value which will be linked to the ACL role.
	Name string
}

// ACLRoleListStub is the stub object returned when performing a listing of ACL
// roles. While it might not currently be different to the full response
// object, it allows us to future-proof the RPC in the event the ACLRole object
// grows over time.
type ACLRoleListStub struct {

	// ID is an internally generated UUID for this role and is controlled by
	// Nomad.
	ID string

	// Name is unique across the entire set of federated clusters and is
	// supplied by the operator on role creation. The name can be modified by
	// updating the role and including the Nomad generated ID. This update will
	// not affect tokens created and linked to this role. This is a required
	// field.
	Name string

	// Description is a human-readable, operator set description that can
	// provide additional context about the role. This is an operational field.
	Description string

	// Policies is an array of ACL policy links. Although currently policies
	// can only be linked using their name, in the future we will want to add
	// IDs also and thus allow operators to specify either a name, an ID, or
	// both.
	Policies []*ACLRolePolicyLink

	CreateIndex uint64
	ModifyIndex uint64
}
