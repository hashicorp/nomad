// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-set"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/lib/lang"
	"golang.org/x/crypto/blake2b"
	"oss.indeed.com/go/libtime"
)

const (
	// ACLUpsertPoliciesRPCMethod is the RPC method for batch creating or
	// modifying ACL policies.
	//
	// Args: ACLPolicyUpsertRequest
	// Reply: GenericResponse
	ACLUpsertPoliciesRPCMethod = "ACL.UpsertPolicies"

	// ACLUpsertTokensRPCMethod is the RPC method for batch creating or
	// modifying ACL tokens.
	//
	// Args: ACLTokenUpsertRequest
	// Reply: ACLTokenUpsertResponse
	ACLUpsertTokensRPCMethod = "ACL.UpsertTokens"

	// ACLDeleteTokensRPCMethod is the RPC method for batch deleting ACL
	// tokens.
	//
	// Args: ACLTokenDeleteRequest
	// Reply: GenericResponse
	ACLDeleteTokensRPCMethod = "ACL.DeleteTokens"

	// ACLUpsertRolesRPCMethod is the RPC method for batch creating or
	// modifying ACL roles.
	//
	// Args: ACLRolesUpsertRequest
	// Reply: ACLRolesUpsertResponse
	ACLUpsertRolesRPCMethod = "ACL.UpsertRoles"

	// ACLDeleteRolesByIDRPCMethod the RPC method for batch deleting ACL
	// roles by their ID.
	//
	// Args: ACLRolesDeleteByIDRequest
	// Reply: ACLRolesDeleteByIDResponse
	ACLDeleteRolesByIDRPCMethod = "ACL.DeleteRolesByID"

	// ACLListRolesRPCMethod is the RPC method for listing ACL roles.
	//
	// Args: ACLRolesListRequest
	// Reply: ACLRolesListResponse
	ACLListRolesRPCMethod = "ACL.ListRoles"

	// ACLGetRolesByIDRPCMethod is the RPC method for detailing a number of ACL
	// roles using their ID. This is an internal only RPC endpoint and used by
	// the ACL Role replication process.
	//
	// Args: ACLRolesByIDRequest
	// Reply: ACLRolesByIDResponse
	ACLGetRolesByIDRPCMethod = "ACL.GetRolesByID"

	// ACLGetRoleByIDRPCMethod is the RPC method for detailing an individual
	// ACL role using its ID.
	//
	// Args: ACLRoleByIDRequest
	// Reply: ACLRoleByIDResponse
	ACLGetRoleByIDRPCMethod = "ACL.GetRoleByID"

	// ACLGetRoleByNameRPCMethod is the RPC method for detailing an individual
	// ACL role using its name.
	//
	// Args: ACLRoleByNameRequest
	// Reply: ACLRoleByNameResponse
	ACLGetRoleByNameRPCMethod = "ACL.GetRoleByName"

	// ACLUpsertAuthMethodsRPCMethod is the RPC method for batch creating or
	// modifying auth methods.
	//
	// Args: ACLAuthMethodsUpsertRequest
	// Reply: ACLAuthMethodUpsertResponse
	ACLUpsertAuthMethodsRPCMethod = "ACL.UpsertAuthMethods"

	// ACLDeleteAuthMethodsRPCMethod is the RPC method for batch deleting auth
	// methods.
	//
	// Args: ACLAuthMethodDeleteRequest
	// Reply: ACLAuthMethodDeleteResponse
	ACLDeleteAuthMethodsRPCMethod = "ACL.DeleteAuthMethods"

	// ACLListAuthMethodsRPCMethod is the RPC method for listing auth methods.
	//
	// Args: ACLAuthMethodListRequest
	// Reply: ACLAuthMethodListResponse
	ACLListAuthMethodsRPCMethod = "ACL.ListAuthMethods"

	// ACLGetAuthMethodRPCMethod is the RPC method for detailing an individual
	// auth method using its name.
	//
	// Args: ACLAuthMethodGetRequest
	// Reply: ACLAuthMethodGetResponse
	ACLGetAuthMethodRPCMethod = "ACL.GetAuthMethod"

	// ACLGetAuthMethodsRPCMethod is the RPC method for getting multiple auth
	// methods using their names.
	//
	// Args: ACLAuthMethodsGetRequest
	// Reply: ACLAuthMethodsGetResponse
	ACLGetAuthMethodsRPCMethod = "ACL.GetAuthMethods"

	// ACLUpsertBindingRulesRPCMethod is the RPC method for batch creating or
	// modifying binding rules.
	//
	// Args: ACLBindingRulesUpsertRequest
	// Reply: ACLBindingRulesUpsertResponse
	ACLUpsertBindingRulesRPCMethod = "ACL.UpsertBindingRules"

	// ACLDeleteBindingRulesRPCMethod is the RPC method for batch deleting
	// binding rules.
	//
	// Args: ACLBindingRulesDeleteRequest
	// Reply: ACLBindingRulesDeleteResponse
	ACLDeleteBindingRulesRPCMethod = "ACL.DeleteBindingRules"

	// ACLListBindingRulesRPCMethod is the RPC method listing binding rules.
	//
	// Args: ACLBindingRulesListRequest
	// Reply: ACLBindingRulesListResponse
	ACLListBindingRulesRPCMethod = "ACL.ListBindingRules"

	// ACLGetBindingRulesRPCMethod is the RPC method for getting multiple
	// binding rules using their IDs.
	//
	// Args: ACLBindingRulesRequest
	// Reply: ACLBindingRulesResponse
	ACLGetBindingRulesRPCMethod = "ACL.GetBindingRules"

	// ACLGetBindingRuleRPCMethod is the RPC method for detailing an individual
	// binding rule using its ID.
	//
	// Args: ACLBindingRuleRequest
	// Reply: ACLBindingRuleResponse
	ACLGetBindingRuleRPCMethod = "ACL.GetBindingRule"

	// ACLOIDCAuthURLRPCMethod is the RPC method for starting the OIDC login
	// workflow. It generates the OIDC provider URL which will be used for user
	// authentication.
	//
	// Args: ACLOIDCAuthURLRequest
	// Reply: ACLOIDCAuthURLResponse
	ACLOIDCAuthURLRPCMethod = "ACL.OIDCAuthURL"

	// ACLOIDCCompleteAuthRPCMethod is the RPC method for completing the OIDC
	// login workflow. It exchanges the OIDC provider token for a Nomad ACL
	// token with roles as defined within the remote provider.
	//
	// Args: ACLOIDCCompleteAuthRequest
	// Reply: ACLOIDCCompleteAuthResponse
	ACLOIDCCompleteAuthRPCMethod = "ACL.OIDCCompleteAuth"

	// ACLLoginRPCMethod is the RPC method for performing a non-OIDC login
	// workflow. It exchanges the provided token for a Nomad ACL token with
	// roles as defined within the remote provider.
	//
	// Args: ACLLoginRequest
	// Reply: ACLLoginResponse
	ACLLoginRPCMethod = "ACL.Login"
)

const (
	// ACLMaxExpiredBatchSize is the maximum number of expired ACL tokens that
	// will be garbage collected in a single trigger. This number helps limit
	// the replication pressure due to expired token deletion. If there are a
	// large number of expired tokens pending garbage collection, this value is
	// a potential limiting factor.
	ACLMaxExpiredBatchSize = 4096

	// maxACLRoleDescriptionLength limits an ACL roles description length.
	maxACLRoleDescriptionLength = 256

	// maxACLBindingRuleDescriptionLength limits an ACL binding rules
	// description length and should be used to validate the object.
	maxACLBindingRuleDescriptionLength = 256

	// ACLAuthMethodTokenLocalityLocal is the ACLAuthMethod.TokenLocality that
	// will generate ACL tokens which can only be used on the local cluster the
	// request was made.
	ACLAuthMethodTokenLocalityLocal = "local"

	// ACLAuthMethodTokenLocalityGlobal is the ACLAuthMethod.TokenLocality that
	// will generate ACL tokens which can be used on all federated clusters.
	ACLAuthMethodTokenLocalityGlobal = "global"

	// ACLAuthMethodTypeOIDC the ACLAuthMethod.Type and represents an
	// auth-method which uses the OIDC protocol.
	ACLAuthMethodTypeOIDC = "OIDC"

	// ACLAuthMethodTypeJWT the ACLAuthMethod.Type and represents an auth-method
	// which uses the JWT type.
	ACLAuthMethodTypeJWT = "JWT"
)

var (
	// ValidACLRoleName is used to validate an ACL role name.
	ValidACLRoleName = regexp.MustCompile("^[a-zA-Z0-9-]{1,128}$")

	// ValidACLAuthMethod is used to validate an ACL auth method name.
	ValidACLAuthMethod = regexp.MustCompile("^[a-zA-Z0-9-]{1,128}$")

	// ValitACLAuthMethodTypes lists supported auth method types.
	ValidACLAuthMethodTypes = []string{ACLAuthMethodTypeOIDC, ACLAuthMethodTypeJWT}
)

type ACLCacheEntry[T any] lang.Pair[T, time.Time]

func (e ACLCacheEntry[T]) Age() time.Duration {
	return time.Since(e.Second)
}

func (e ACLCacheEntry[T]) Get() T {
	return e.First
}

// An ACLCache caches ACL tokens by their policy content.
type ACLCache[T any] struct {
	*lru.TwoQueueCache[string, ACLCacheEntry[T]]
	clock libtime.Clock
}

func (c *ACLCache[T]) Add(key string, item T) {
	c.AddAtTime(key, item, c.clock.Now())
}

func (c *ACLCache[T]) AddAtTime(key string, item T, now time.Time) {
	c.TwoQueueCache.Add(key, ACLCacheEntry[T]{
		First:  item,
		Second: now,
	})
}

func NewACLCache[T any](size int) *ACLCache[T] {
	c, err := lru.New2Q[string, ACLCacheEntry[T]](size)
	if err != nil {
		panic(err) // not possible
	}
	return &ACLCache[T]{
		TwoQueueCache: c,
		clock:         libtime.SystemClock(),
	}
}

// ACLTokenRoleLink is used to link an ACL token to an ACL role. The ACL token
// can therefore inherit all the ACL policy permissions that the ACL role
// contains.
type ACLTokenRoleLink struct {

	// ID is the ACLRole.ID UUID. This field is immutable and represents the
	// absolute truth for the link.
	ID string

	// Name is the human friendly identifier for the ACL role and is a
	// convenience field for operators. This field is always resolved to the
	// ID and discarded before the token is stored in state. This is because
	// operators can change the name of an ACL role.
	Name string
}

// Canonicalize performs basic canonicalization on the ACL token object. It is
// important for callers to understand certain fields such as AccessorID are
// set if it is empty, so copies should be taken if needed before calling this
// function.
func (a *ACLToken) Canonicalize() {

	// If the accessor ID is empty, it means this is creation of a new token,
	// therefore we need to generate base information.
	if a.AccessorID == "" {

		a.AccessorID = uuid.Generate()
		a.SecretID = uuid.Generate()
		a.CreateTime = time.Now().UTC()

		// If the user has not set the expiration time, but has provided a TTL, we
		// calculate and populate the former filed.
		if a.ExpirationTime == nil && a.ExpirationTTL != 0 {
			a.ExpirationTime = pointer.Of(a.CreateTime.Add(a.ExpirationTTL))
		}
	}
}

// Validate is used to check a token for reasonableness
func (a *ACLToken) Validate(minTTL, maxTTL time.Duration, existing *ACLToken) error {
	var mErr multierror.Error

	// The human friendly name of an ACL token cannot exceed 256 characters.
	if len(a.Name) > maxTokenNameLength {
		mErr.Errors = append(mErr.Errors, errors.New("token name too long"))
	}

	// The type of an ACL token must be set. An ACL token of type client must
	// have associated policies or roles, whereas a management token cannot be
	// associated with policies.
	switch a.Type {
	case ACLClientToken:
		if len(a.Policies) == 0 && len(a.Roles) == 0 {
			mErr.Errors = append(mErr.Errors, errors.New("client token missing policies or roles"))
		}
	case ACLManagementToken:
		if len(a.Policies) != 0 || len(a.Roles) != 0 {
			mErr.Errors = append(mErr.Errors, errors.New("management token cannot be associated with policies or roles"))
		}
	default:
		mErr.Errors = append(mErr.Errors, errors.New("token type must be client or management"))
	}

	// There are different validation rules depending on whether the ACL token
	// is being created or updated.
	switch existing {
	case nil:
		if a.ExpirationTTL < 0 {
			mErr.Errors = append(mErr.Errors,
				fmt.Errorf("token expiration TTL '%s' should not be negative", a.ExpirationTTL))
		}

		if a.ExpirationTime != nil && !a.ExpirationTime.IsZero() {

			if a.CreateTime.After(*a.ExpirationTime) {
				mErr.Errors = append(mErr.Errors, errors.New("expiration time cannot be before create time"))
			}

			// Create a time duration which details the time-til-expiry, so we can
			// check this against the regions max and min values.
			expiresIn := a.ExpirationTime.Sub(a.CreateTime)
			if expiresIn > maxTTL {
				mErr.Errors = append(mErr.Errors,
					fmt.Errorf("expiration time cannot be more than %s in the future (was %s)",
						maxTTL, expiresIn))

			} else if expiresIn < minTTL {
				mErr.Errors = append(mErr.Errors,
					fmt.Errorf("expiration time cannot be less than %s in the future (was %s)",
						minTTL, expiresIn))
			}
		}
	default:
		if existing.Global != a.Global {
			mErr.Errors = append(mErr.Errors, errors.New("cannot toggle global mode"))
		}
		if existing.ExpirationTTL != a.ExpirationTTL {
			mErr.Errors = append(mErr.Errors, errors.New("cannot update expiration TTL"))
		}
		if existing.ExpirationTime != a.ExpirationTime {
			mErr.Errors = append(mErr.Errors, errors.New("cannot update expiration time"))
		}
	}

	return mErr.ErrorOrNil()
}

// HasExpirationTime checks whether the ACL token has an expiration time value
// set.
func (a *ACLToken) HasExpirationTime() bool {
	if a == nil || a.ExpirationTime == nil {
		return false
	}
	return !a.ExpirationTime.IsZero()
}

// IsExpired compares the ACLToken.ExpirationTime against the passed t to
// identify whether the token is considered expired. The function can be called
// without checking whether the ACL token has an expiry time.
func (a *ACLToken) IsExpired(t time.Time) bool {

	// Check the token has an expiration time before potentially modifying the
	// supplied time. This allows us to avoid extra work, if it isn't needed.
	if !a.HasExpirationTime() {
		return false
	}

	// Check and ensure the time location is set to UTC. This is vital for
	// consistency with multi-region global tokens.
	if t.Location() != time.UTC {
		t = t.UTC()
	}

	return a.ExpirationTime.Before(t) || t.IsZero()
}

// HasRoles checks if a given set of role IDs are assigned to the ACL token. It
// does not account for management tokens, therefore it is the responsibility
// of the caller to perform this check, if required.
func (a *ACLToken) HasRoles(roleIDs []string) bool {

	// Generate a set of role IDs that the token is assigned.
	roleSet := set.FromFunc(a.Roles, func(roleLink *ACLTokenRoleLink) string { return roleLink.ID })

	// Iterate the role IDs within the request and check whether these are
	// present within the token assignment.
	for _, roleID := range roleIDs {
		if !roleSet.Contains(roleID) {
			return false
		}
	}
	return true
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
		ExpirationTTL interface{}
		Hash          string
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
	if aux.Hash != "" {
		a.Hash = []byte(aux.Hash)
	}
	return nil
}

// ACLRole is an abstraction for the ACL system which allows the grouping of
// ACL policies into a single object. ACL tokens can be created and linked to
// a role; the token then inherits all the permissions granted by the policies.
type ACLRole struct {

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

	// Hash is the hashed value of the role and is generated using all fields
	// above this point.
	Hash []byte

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

// SetHash is used to compute and set the hash of the ACL role. This should be
// called every and each time a user specified field on the role is changed
// before updating the Nomad state store.
func (a *ACLRole) SetHash() []byte {

	// Initialize a 256bit Blake2 hash (32 bytes).
	hash, err := blake2b.New256(nil)
	if err != nil {
		panic(err)
	}

	// Write all the user set fields.
	_, _ = hash.Write([]byte(a.Name))
	_, _ = hash.Write([]byte(a.Description))

	for _, policyLink := range a.Policies {
		_, _ = hash.Write([]byte(policyLink.Name))
	}

	// Finalize the hash.
	hashVal := hash.Sum(nil)

	// Set and return the hash.
	a.Hash = hashVal
	return hashVal
}

// Validate ensure the ACL role contains valid information which meets Nomad's
// internal requirements. This does not include any state calls, such as
// ensuring the linked policies exist.
func (a *ACLRole) Validate() error {

	var mErr multierror.Error

	if !ValidACLRoleName.MatchString(a.Name) {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("invalid name '%s'", a.Name))
	}

	if len(a.Description) > maxACLRoleDescriptionLength {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("description longer than %d", maxACLRoleDescriptionLength))
	}

	if len(a.Policies) < 1 {
		mErr.Errors = append(mErr.Errors, errors.New("at least one policy should be specified"))
	}

	return mErr.ErrorOrNil()
}

// Canonicalize performs basic canonicalization on the ACL role object. It is
// important for callers to understand certain fields such as ID are set if it
// is empty, so copies should be taken if needed before calling this function.
func (a *ACLRole) Canonicalize() {
	if a.ID == "" {
		a.ID = uuid.Generate()
	}
}

// Equal performs an equality check on the two service registrations. It
// handles nil objects.
func (a *ACLRole) Equal(o *ACLRole) bool {
	if a == nil || o == nil {
		return a == o
	}
	if len(a.Hash) == 0 {
		a.SetHash()
	}
	if len(o.Hash) == 0 {
		o.SetHash()
	}
	return bytes.Equal(a.Hash, o.Hash)
}

// Copy creates a deep copy of the ACL role. This copy can then be safely
// modified. It handles nil objects.
func (a *ACLRole) Copy() *ACLRole {
	if a == nil {
		return nil
	}

	c := new(ACLRole)
	*c = *a

	c.Policies = slices.Clone(a.Policies)
	c.Hash = slices.Clone(a.Hash)

	return c
}

// Stub converts the ACLRole object into a ACLRoleListStub object.
func (a *ACLRole) Stub() *ACLRoleListStub {
	return &ACLRoleListStub{
		ID:          a.ID,
		Name:        a.Name,
		Description: a.Description,
		Policies:    a.Policies,
		Hash:        a.Hash,
		CreateIndex: a.CreateIndex,
		ModifyIndex: a.ModifyIndex,
	}
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

	// Hash is the hashed value of the role and is generated using all fields
	// above this point.
	Hash []byte

	CreateIndex uint64
	ModifyIndex uint64
}

// ACLRolesUpsertRequest is the request object used to upsert one or more ACL
// roles.
type ACLRolesUpsertRequest struct {
	ACLRoles []*ACLRole

	// AllowMissingPolicies skips the ACL Role policy link verification and is
	// used by the replication process. The replication cannot ensure policies
	// are present before ACL Roles are replicated.
	AllowMissingPolicies bool

	WriteRequest
}

// ACLRolesUpsertResponse is the response object when one or more ACL roles
// have been successfully upserted into state.
type ACLRolesUpsertResponse struct {
	ACLRoles []*ACLRole
	WriteMeta
}

// ACLRolesDeleteByIDRequest is the request object to delete one or more ACL
// roles using the role ID.
type ACLRolesDeleteByIDRequest struct {
	ACLRoleIDs []string
	WriteRequest
}

// ACLRolesDeleteByIDResponse is the response object when performing a deletion
// of one or more ACL roles using the role ID.
type ACLRolesDeleteByIDResponse struct {
	WriteMeta
}

// ACLRolesListRequest is the request object when performing ACL role listings.
type ACLRolesListRequest struct {
	QueryOptions
}

// ACLRolesListResponse is the response object when performing ACL role
// listings.
type ACLRolesListResponse struct {
	ACLRoles []*ACLRoleListStub
	QueryMeta
}

// ACLRolesByIDRequest is the request object when performing a lookup of
// multiple roles by the ID.
type ACLRolesByIDRequest struct {
	ACLRoleIDs []string
	QueryOptions
}

// ACLRolesByIDResponse is the response object when performing a lookup of
// multiple roles by their IDs.
type ACLRolesByIDResponse struct {
	ACLRoles map[string]*ACLRole
	QueryMeta
}

// ACLRoleByIDRequest is the request object to perform a lookup of an ACL
// role using a specific ID.
type ACLRoleByIDRequest struct {
	RoleID string
	QueryOptions
}

// ACLRoleByIDResponse is the response object when performing a lookup of an
// ACL role matching a specific ID.
type ACLRoleByIDResponse struct {
	ACLRole *ACLRole
	QueryMeta
}

// ACLRoleByNameRequest is the request object to perform a lookup of an ACL
// role using a specific name.
type ACLRoleByNameRequest struct {
	RoleName string
	QueryOptions
}

// ACLRoleByNameResponse is the response object when performing a lookup of an
// ACL role matching a specific name.
type ACLRoleByNameResponse struct {
	ACLRole *ACLRole
	QueryMeta
}

// ACLAuthMethod is used to capture the properties of an authentication method
// used for single sing-on
type ACLAuthMethod struct {
	Name          string
	Type          string
	TokenLocality string // is the token valid locally or globally?
	MaxTokenTTL   time.Duration
	Default       bool
	Config        *ACLAuthMethodConfig

	Hash []byte

	CreateTime  time.Time
	ModifyTime  time.Time
	CreateIndex uint64
	ModifyIndex uint64
}

// SetHash is used to compute and set the hash of the ACL auth method. This
// should be called every and each time a user specified field on the method is
// changed before updating the Nomad state store.
func (a *ACLAuthMethod) SetHash() []byte {

	// Initialize a 256bit Blake2 hash (32 bytes).
	hash, err := blake2b.New256(nil)
	if err != nil {
		panic(err)
	}

	_, _ = hash.Write([]byte(a.Name))
	_, _ = hash.Write([]byte(a.Type))
	_, _ = hash.Write([]byte(a.TokenLocality))
	_, _ = hash.Write([]byte(a.MaxTokenTTL.String()))
	_, _ = hash.Write([]byte(strconv.FormatBool(a.Default)))

	if a.Config != nil {
		_, _ = hash.Write([]byte(a.Config.OIDCDiscoveryURL))
		_, _ = hash.Write([]byte(a.Config.OIDCClientID))
		_, _ = hash.Write([]byte(a.Config.OIDCClientSecret))
		for _, ba := range a.Config.BoundAudiences {
			_, _ = hash.Write([]byte(ba))
		}
		for _, uri := range a.Config.AllowedRedirectURIs {
			_, _ = hash.Write([]byte(uri))
		}
		for _, pem := range a.Config.DiscoveryCaPem {
			_, _ = hash.Write([]byte(pem))
		}
		for _, sa := range a.Config.SigningAlgs {
			_, _ = hash.Write([]byte(sa))
		}
		for k, v := range a.Config.ClaimMappings {
			_, _ = hash.Write([]byte(k))
			_, _ = hash.Write([]byte(v))
		}
		for k, v := range a.Config.ListClaimMappings {
			_, _ = hash.Write([]byte(k))
			_, _ = hash.Write([]byte(v))
		}
	}

	// Finalize the hash.
	hashVal := hash.Sum(nil)

	// Set and return the hash.
	a.Hash = hashVal
	return hashVal
}

// MarshalJSON implements the json.Marshaler interface and allows
// ACLAuthMethod.MaxTokenTTL to be marshaled correctly.
func (a *ACLAuthMethod) MarshalJSON() ([]byte, error) {
	type Alias ACLAuthMethod
	exported := &struct {
		MaxTokenTTL string
		*Alias
	}{
		MaxTokenTTL: a.MaxTokenTTL.String(),
		Alias:       (*Alias)(a),
	}
	if a.MaxTokenTTL == 0 {
		exported.MaxTokenTTL = ""
	}
	return json.Marshal(exported)
}

// UnmarshalJSON implements the json.Unmarshaler interface and allows
// ACLAuthMethod.MaxTokenTTL to be unmarshalled correctly.
func (a *ACLAuthMethod) UnmarshalJSON(data []byte) (err error) {
	type Alias ACLAuthMethod
	aux := &struct {
		MaxTokenTTL interface{}
		*Alias
	}{
		Alias: (*Alias)(a),
	}
	if err = json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.MaxTokenTTL != nil {
		switch v := aux.MaxTokenTTL.(type) {
		case string:
			if a.MaxTokenTTL, err = time.ParseDuration(v); err != nil {
				return err
			}
		case float64:
			a.MaxTokenTTL = time.Duration(v)
		}
	}
	return nil
}

func (a *ACLAuthMethod) Stub() *ACLAuthMethodStub {
	return &ACLAuthMethodStub{
		Name:        a.Name,
		Type:        a.Type,
		Default:     a.Default,
		Hash:        a.Hash,
		CreateIndex: a.CreateIndex,
		ModifyIndex: a.ModifyIndex,
	}
}

func (a *ACLAuthMethod) Equal(other *ACLAuthMethod) bool {
	if a == nil || other == nil {
		return a == other
	}
	if len(a.Hash) == 0 {
		a.SetHash()
	}
	if len(other.Hash) == 0 {
		other.SetHash()
	}
	return bytes.Equal(a.Hash, other.Hash)

}

// Copy creates a deep copy of the ACL auth method. This copy can then be safely
// modified. It handles nil objects.
func (a *ACLAuthMethod) Copy() *ACLAuthMethod {
	if a == nil {
		return nil
	}

	c := new(ACLAuthMethod)
	*c = *a

	c.Hash = slices.Clone(a.Hash)
	c.Config = a.Config.Copy()

	return c
}

// Canonicalize performs basic canonicalization on the ACL auth method object.
func (a *ACLAuthMethod) Canonicalize() {
	t := time.Now().UTC()

	if a.CreateTime.IsZero() {
		a.CreateTime = t
	}
	a.ModifyTime = t
}

// Merge merges auth method a with method b. It sets all required empty fields
// of method a to corresponding values of method b, except for "default" and
// "name."
func (a *ACLAuthMethod) Merge(b *ACLAuthMethod) {
	if b != nil {
		a.Type = helper.Merge(a.Type, b.Type)
		a.TokenLocality = helper.Merge(a.TokenLocality, b.TokenLocality)
		a.MaxTokenTTL = helper.Merge(a.MaxTokenTTL, b.MaxTokenTTL)
		a.Config = helper.Merge(a.Config, b.Config)
	}
}

// Validate returns an error is the ACLAuthMethod is invalid.
//
// TODO revisit possible other validity conditions in the future
func (a *ACLAuthMethod) Validate(minTTL, maxTTL time.Duration) error {
	var mErr multierror.Error

	if !ValidACLAuthMethod.MatchString(a.Name) {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("invalid name '%s'", a.Name))
	}

	if !slices.Contains([]string{"local", "global"}, a.TokenLocality) {
		mErr.Errors = append(
			mErr.Errors, fmt.Errorf("invalid token locality '%s'", a.TokenLocality))
	}

	if !slices.Contains(ValidACLAuthMethodTypes, a.Type) {
		mErr.Errors = append(
			mErr.Errors, fmt.Errorf("invalid token type '%s'", a.Type))
	}

	if minTTL > a.MaxTokenTTL || a.MaxTokenTTL > maxTTL {
		mErr.Errors = append(mErr.Errors, fmt.Errorf(
			"invalid MaxTokenTTL value '%s' (should be between %s and %s)",
			a.MaxTokenTTL.String(), minTTL.String(), maxTTL.String()))
	}

	return mErr.ErrorOrNil()
}

// TokenLocalityIsGlobal returns whether the auth method creates global ACL
// tokens or not.
func (a *ACLAuthMethod) TokenLocalityIsGlobal() bool { return a.TokenLocality == "global" }

// ACLAuthMethodConfig is used to store configuration of an auth method
type ACLAuthMethodConfig struct {
	// A list of PEM-encoded public keys to use to authenticate signatures
	// locally
	JWTValidationPubKeys []string

	// JSON Web Key Sets url for authenticating signatures
	JWKSURL string

	// The OIDC Discovery URL, without any .well-known component (base path)
	OIDCDiscoveryURL string

	// The OAuth Client ID configured with the OIDC provider
	OIDCClientID string

	// The OAuth Client Secret configured with the OIDC provider
	OIDCClientSecret string

	// List of OIDC scopes
	OIDCScopes []string

	// List of auth claims that are valid for login
	BoundAudiences []string

	// The value against which to match the iss claim in a JWT
	BoundIssuer []string

	// A list of allowed values for redirect_uri
	AllowedRedirectURIs []string

	// PEM encoded CA certs for use by the TLS client used to talk with the
	// OIDC Discovery URL.
	DiscoveryCaPem []string

	// PEM encoded CA cert for use by the TLS client used to talk with the JWKS
	// URL
	JWKSCACert string

	// A list of supported signing algorithms
	SigningAlgs []string

	// Duration in seconds of leeway when validating expiration of a token to
	// account for clock skew
	ExpirationLeeway time.Duration

	// Duration in seconds of leeway when validating not before values of a
	// token to account for clock skew.
	NotBeforeLeeway time.Duration

	// Duration in seconds of leeway when validating all claims to account for
	// clock skew.
	ClockSkewLeeway time.Duration

	// Mappings of claims (key) that will be copied to a metadata field
	// (value).
	ClaimMappings     map[string]string
	ListClaimMappings map[string]string
}

func (a *ACLAuthMethodConfig) Copy() *ACLAuthMethodConfig {
	if a == nil {
		return nil
	}

	c := new(ACLAuthMethodConfig)
	*c = *a

	c.JWTValidationPubKeys = slices.Clone(a.JWTValidationPubKeys)
	c.OIDCScopes = slices.Clone(a.OIDCScopes)
	c.BoundAudiences = slices.Clone(a.BoundAudiences)
	c.BoundIssuer = slices.Clone(a.BoundIssuer)
	c.AllowedRedirectURIs = slices.Clone(a.AllowedRedirectURIs)
	c.DiscoveryCaPem = slices.Clone(a.DiscoveryCaPem)
	c.SigningAlgs = slices.Clone(a.SigningAlgs)

	return c
}

// MarshalJSON implements the json.Marshaler interface and allows
// time.Diration fields to be marshaled correctly.
func (a *ACLAuthMethodConfig) MarshalJSON() ([]byte, error) {
	type Alias ACLAuthMethodConfig
	exported := &struct {
		ExpirationLeeway string
		NotBeforeLeeway  string
		ClockSkewLeeway  string
		*Alias
	}{
		ExpirationLeeway: a.ExpirationLeeway.String(),
		NotBeforeLeeway:  a.NotBeforeLeeway.String(),
		ClockSkewLeeway:  a.ClockSkewLeeway.String(),
		Alias:            (*Alias)(a),
	}
	if a.ExpirationLeeway == 0 {
		exported.ExpirationLeeway = ""
	}
	if a.NotBeforeLeeway == 0 {
		exported.NotBeforeLeeway = ""
	}
	if a.ClockSkewLeeway == 0 {
		exported.ClockSkewLeeway = ""
	}
	return json.Marshal(exported)
}

// UnmarshalJSON implements the json.Unmarshaler interface and allows
// time.Duration fields to be unmarshalled correctly.
func (a *ACLAuthMethodConfig) UnmarshalJSON(data []byte) (err error) {
	type Alias ACLAuthMethodConfig
	aux := &struct {
		ExpirationLeeway any
		NotBeforeLeeway  any
		ClockSkewLeeway  any
		*Alias
	}{
		Alias: (*Alias)(a),
	}
	if err = json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.ExpirationLeeway != nil {
		switch v := aux.ExpirationLeeway.(type) {
		case string:
			if v != "" {
				if a.ExpirationLeeway, err = time.ParseDuration(v); err != nil {
					return err
				}
			}
		case float64:
			a.ExpirationLeeway = time.Duration(v)
		default:
			return fmt.Errorf("unexpected ExpirationLeeway type: %v", v)
		}
	}
	if aux.NotBeforeLeeway != nil {
		switch v := aux.NotBeforeLeeway.(type) {
		case string:
			if v != "" {
				if a.NotBeforeLeeway, err = time.ParseDuration(v); err != nil {
					return err
				}
			}
		case float64:
			a.NotBeforeLeeway = time.Duration(v)
		default:
			return fmt.Errorf("unexpected NotBeforeLeeway type: %v", v)
		}
	}
	if aux.ClockSkewLeeway != nil {
		switch v := aux.ClockSkewLeeway.(type) {
		case string:
			if v != "" {
				if a.ClockSkewLeeway, err = time.ParseDuration(v); err != nil {
					return err
				}
			}
		case float64:
			a.ClockSkewLeeway = time.Duration(v)
		default:
			return fmt.Errorf("unexpected ClockSkewLeeway type: %v", v)
		}
	}
	return nil
}

// ACLAuthClaims is the claim mapping of the OIDC auth method in a format that
// can be used with go-bexpr. This structure is used during rule binding
// evaluation.
type ACLAuthClaims struct {
	Value map[string]string   `bexpr:"value"`
	List  map[string][]string `bexpr:"list"`
}

// ACLAuthMethodStub is used for listing ACL auth methods
type ACLAuthMethodStub struct {
	Name    string
	Type    string
	Default bool

	// Hash is the hashed value of the auth-method and is generated using all
	// fields from the full object except the create and modify times and
	// indexes.
	Hash []byte

	CreateIndex uint64
	ModifyIndex uint64
}

// ACLAuthMethodListRequest is used to list auth methods
type ACLAuthMethodListRequest struct {
	QueryOptions
}

// ACLAuthMethodListResponse is used to list auth methods
type ACLAuthMethodListResponse struct {
	AuthMethods []*ACLAuthMethodStub
	QueryMeta
}

// ACLAuthMethodGetRequest is used to query a specific auth method
type ACLAuthMethodGetRequest struct {
	MethodName string
	QueryOptions
}

// ACLAuthMethodGetResponse is used to return a single auth method
type ACLAuthMethodGetResponse struct {
	AuthMethod *ACLAuthMethod
	QueryMeta
}

// ACLAuthMethodsGetRequest is used to query a set of auth methods
type ACLAuthMethodsGetRequest struct {
	Names []string
	QueryOptions
}

// ACLAuthMethodsGetResponse is used to return a set of auth methods
type ACLAuthMethodsGetResponse struct {
	AuthMethods map[string]*ACLAuthMethod
	QueryMeta
}

// ACLAuthMethodUpsertRequest is used to upsert a set of auth methods
type ACLAuthMethodUpsertRequest struct {
	AuthMethods []*ACLAuthMethod
	WriteRequest
}

// ACLAuthMethodUpsertResponse is a response of the upsert ACL auth methods
// operation
type ACLAuthMethodUpsertResponse struct {
	AuthMethods []*ACLAuthMethod
	WriteMeta
}

// ACLAuthMethodDeleteRequest is used to delete a set of auth methods by their
// name
type ACLAuthMethodDeleteRequest struct {
	Names []string
	WriteRequest
}

// ACLAuthMethodDeleteResponse is a response of the delete ACL auth methods
// operation
type ACLAuthMethodDeleteResponse struct {
	WriteMeta
}

type ACLWhoAmIResponse struct {
	Identity *AuthenticatedIdentity
	QueryMeta
}

// ACLBindingRule contains a direct relation to an ACLAuthMethod and represents
// a rule to apply when logging in via the named AuthMethod. This allows the
// transformation of OIDC provider claims, to Nomad based ACL concepts such as
// ACL Roles and Policies.
type ACLBindingRule struct {

	// ID is an internally generated UUID for this rule and is controlled by
	// Nomad.
	ID string

	// Description is a human-readable, operator set description that can
	// provide additional context about the binding role. This is an
	// operational field.
	Description string

	// AuthMethod is the name of the auth method for which this rule applies
	// to. This is required and the method must exist within state before the
	// cluster administrator can create the rule.
	AuthMethod string

	// Selector is an expression that matches against verified identity
	// attributes returned from the auth method during login. This is optional
	// and when not set, provides a catch-all rule.
	Selector string

	// BindType adjusts how this binding rule is applied at login time. The
	// valid values are ACLBindingRuleBindTypeRole,
	// ACLBindingRuleBindTypePolicy, and ACLBindingRuleBindTypeManagement.
	BindType string

	// BindName is the target of the binding. Can be lightly templated using
	// HIL ${foo} syntax from available field names. How it is used depends
	// upon the BindType.
	BindName string

	// Hash is the hashed value of the binding rule and is generated using all
	// fields from the full object except the create and modify times and
	// indexes.
	Hash []byte

	CreateTime  time.Time
	ModifyTime  time.Time
	CreateIndex uint64
	ModifyIndex uint64
}

const (
	// ACLBindingRuleBindTypeRole is the ACL binding rule bind type that only
	// allows the binding rule to function if a role exists at login-time. The
	// role will be specified within the ACLBindingRule.BindName parameter, and
	// will identify whether this is an ID or Name.
	ACLBindingRuleBindTypeRole = "role"

	// ACLBindingRuleBindTypePolicy is the ACL binding rule bind type that
	// assigns a policy to the generate ACL token. The role will be specified
	// within the ACLBindingRule.BindName parameter, and will be the policy
	// name.
	ACLBindingRuleBindTypePolicy = "policy"

	// ACLBindingRuleBindTypeManagement is the ACL binding rule bind type that
	// will generate management ACL tokens when matched.
	ACLBindingRuleBindTypeManagement = "management"
)

// Canonicalize performs basic canonicalization on the ACL token object. It is
// important for callers to understand certain fields such as ID are set if it
// is empty, so copies should be taken if needed before calling this function.
func (a *ACLBindingRule) Canonicalize() {

	now := time.Now().UTC()

	// If the ID is empty, it means this is creation of a new binding rule,
	// therefore we need to generate base information.
	if a.ID == "" {
		a.ID = uuid.Generate()
		a.CreateTime = now
	}

	// The fact this function is being called indicates we are attempting an
	// upsert into state. Therefore, update the modify time.
	a.ModifyTime = now
}

// Validate ensures the ACL binding rule contains valid information which meets
// Nomad's internal requirements.
func (a *ACLBindingRule) Validate() error {

	var mErr multierror.Error

	if a.AuthMethod == "" {
		mErr.Errors = append(mErr.Errors, errors.New("auth method is missing"))
	}
	if len(a.Description) > maxACLBindingRuleDescriptionLength {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("description longer than %d", maxACLRoleDescriptionLength))
	}

	// Depending on the bind type, we have some specific validation. Catching
	// the empty string also provides easier to understand feedback to the
	// user.
	switch a.BindType {
	case "":
		mErr.Errors = append(mErr.Errors, errors.New("bind type is missing"))
	case ACLBindingRuleBindTypeRole, ACLBindingRuleBindTypePolicy:
		if a.BindName == "" {
			mErr.Errors = append(mErr.Errors, errors.New("bind name is missing"))
		}
	case ACLBindingRuleBindTypeManagement:
		if a.BindName != "" {
			mErr.Errors = append(mErr.Errors, errors.New("bind name should be empty"))
		}
	default:
		mErr.Errors = append(mErr.Errors, fmt.Errorf("unsupported bind type: %q", a.BindType))
	}

	// If there is a selector configured, ensure that go-bexpr can parse this.
	// Otherwise, the user will get an ambiguous failure when attempting to
	// login.
	if a.Selector != "" {
		if _, err := bexpr.CreateEvaluator(a.Selector, nil); err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("selector is invalid: %v", err))
		}
	}

	return mErr.ErrorOrNil()
}

// Merge merges binding rule a with b. It sets all required empty fields of rule
// a to corresponding values of rule b, except for "ID" which must be provided.
func (a *ACLBindingRule) Merge(b *ACLBindingRule) {
	a.BindName = helper.Merge(a.BindName, b.BindName)
	a.BindType = helper.Merge(a.BindType, b.BindType)
	a.AuthMethod = helper.Merge(a.AuthMethod, b.AuthMethod)
}

// SetHash is used to compute and set the hash of the ACL binding rule. This
// should be called every and each time a user specified field on the method is
// changed before updating the Nomad state store.
func (a *ACLBindingRule) SetHash() []byte {

	// Initialize a 256bit Blake2 hash (32 bytes).
	hash, err := blake2b.New256(nil)
	if err != nil {
		panic(err)
	}

	_, _ = hash.Write([]byte(a.ID))
	_, _ = hash.Write([]byte(a.Description))
	_, _ = hash.Write([]byte(a.AuthMethod))
	_, _ = hash.Write([]byte(a.Selector))
	_, _ = hash.Write([]byte(a.BindType))
	_, _ = hash.Write([]byte(a.BindName))

	// Finalize the hash.
	hashVal := hash.Sum(nil)

	// Set and return the hash.
	a.Hash = hashVal
	return hashVal
}

// Equal performs an equality check on the two ACL binding rules. It handles
// nil objects.
func (a *ACLBindingRule) Equal(other *ACLBindingRule) bool {
	if a == nil || other == nil {
		return a == other
	}
	if len(a.Hash) == 0 {
		a.SetHash()
	}
	if len(other.Hash) == 0 {
		other.SetHash()
	}
	return bytes.Equal(a.Hash, other.Hash)
}

// Copy creates a deep copy of the ACL binding rule. This copy can then be
// safely modified. It handles nil objects.
func (a *ACLBindingRule) Copy() *ACLBindingRule {
	if a == nil {
		return nil
	}

	c := new(ACLBindingRule)
	*c = *a
	c.Hash = slices.Clone(a.Hash)

	return c
}

// Stub converts the ACLBindingRule object into a ACLBindingRuleListStub
// object.
func (a *ACLBindingRule) Stub() *ACLBindingRuleListStub {
	return &ACLBindingRuleListStub{
		ID:          a.ID,
		Description: a.Description,
		AuthMethod:  a.AuthMethod,
		Hash:        a.Hash,
		CreateIndex: a.CreateIndex,
		ModifyIndex: a.ModifyIndex,
	}
}

// ACLBindingRuleListStub is the stub object returned when performing a listing
// of ACL binding rules.
type ACLBindingRuleListStub struct {

	// ID is an internally generated UUID for this role and is controlled by
	// Nomad.
	ID string

	// Description is a human-readable, operator set description that can
	// provide additional context about the binding role. This is an
	// operational field.
	Description string

	// AuthMethod is the name of the auth method for which this rule applies
	// to. This is required and the method must exist within state before the
	// cluster administrator can create the rule.
	AuthMethod string

	// Hash is the hashed value of the binding rule and is generated using all
	// fields from the full object except the create and modify times and
	// indexes.
	Hash []byte

	CreateIndex uint64
	ModifyIndex uint64
}

// ACLBindingRulesUpsertRequest is used to upsert a set of ACL binding rules.
type ACLBindingRulesUpsertRequest struct {
	ACLBindingRules []*ACLBindingRule

	// AllowMissingAuthMethods skips the ACL binding rule auth method link
	// verification and is used by the replication process. The replication
	// cannot ensure auth methods are present before ACL binding rules are
	// replicated.
	AllowMissingAuthMethods bool

	WriteRequest
}

// ACLBindingRulesUpsertResponse is a response of the upsert ACL binding rules
// operation.
type ACLBindingRulesUpsertResponse struct {
	ACLBindingRules []*ACLBindingRule
	WriteMeta
}

// ACLBindingRulesDeleteRequest is used to delete a set of ACL binding rules by
// their IDs.
type ACLBindingRulesDeleteRequest struct {
	ACLBindingRuleIDs []string
	WriteRequest
}

// ACLBindingRulesDeleteResponse is a response of the delete ACL binding rules
// operation.
type ACLBindingRulesDeleteResponse struct {
	WriteMeta
}

// ACLBindingRulesListRequest  is the request object when performing ACL
// binding rules listings.
type ACLBindingRulesListRequest struct {
	QueryOptions
}

// ACLBindingRulesListResponse is the response object when performing ACL
// binding rule listings.
type ACLBindingRulesListResponse struct {
	ACLBindingRules []*ACLBindingRuleListStub
	QueryMeta
}

// ACLBindingRulesRequest is the request object when performing a lookup of
// multiple binding rules by the ID.
type ACLBindingRulesRequest struct {
	ACLBindingRuleIDs []string
	QueryOptions
}

// ACLBindingRulesResponse is the response object when performing a lookup of
// multiple binding rules by their IDs.
type ACLBindingRulesResponse struct {
	ACLBindingRules map[string]*ACLBindingRule
	QueryMeta
}

// ACLBindingRuleRequest is the request object to perform a lookup of an ACL
// binding rule using a specific ID.
type ACLBindingRuleRequest struct {
	ACLBindingRuleID string
	QueryOptions
}

// ACLBindingRuleResponse is the response object when performing a lookup of an
// ACL binding rule matching a specific ID.
type ACLBindingRuleResponse struct {
	ACLBindingRule *ACLBindingRule
	QueryMeta
}

// ACLOIDCAuthURLRequest is the request to make when starting the OIDC
// authentication login flow.
type ACLOIDCAuthURLRequest struct {

	// AuthMethodName is the OIDC auth-method to use. This is a required
	// parameter.
	AuthMethodName string

	// RedirectURI is the URL that authorization should redirect to. This is a
	// required parameter.
	RedirectURI string

	// ClientNonce is a randomly generated string to prevent replay attacks. It
	// is up to the client to generate this and Go integrations should use the
	// oidc.NewID function within the hashicorp/cap library. This must then be
	// passed back to ACLOIDCCompleteAuthRequest. This is a required parameter.
	ClientNonce string

	// WriteRequest is used due to the requirement by the RPC forwarding
	// mechanism. This request doesn't write anything to Nomad's internal
	// state.
	WriteRequest
}

// Validate ensures the request object contains all the required fields in
// order to start the OIDC authentication flow.
func (a *ACLOIDCAuthURLRequest) Validate() error {

	var mErr multierror.Error

	if a.AuthMethodName == "" {
		mErr.Errors = append(mErr.Errors, errors.New("missing auth method name"))
	}
	if a.ClientNonce == "" {
		mErr.Errors = append(mErr.Errors, errors.New("missing client nonce"))
	}
	if a.RedirectURI == "" {
		mErr.Errors = append(mErr.Errors, errors.New("missing redirect URI"))
	}
	return mErr.ErrorOrNil()
}

// ACLOIDCAuthURLResponse is the response when starting the OIDC authentication
// login flow.
type ACLOIDCAuthURLResponse struct {

	// AuthURL is URL to begin authorization and is where the user logging in
	// should go.
	AuthURL string
}

// ACLOIDCCompleteAuthRequest is the request object to begin completing the
// OIDC auth cycle after receiving the callback from the OIDC provider.
type ACLOIDCCompleteAuthRequest struct {

	// AuthMethodName is the name of the auth method being used to login via
	// OIDC. This will match ACLOIDCAuthURLRequest.AuthMethodName. This is a
	// required parameter.
	AuthMethodName string

	// ClientNonce, State, and Code are provided from the parameters given to
	// the redirect URL. These are all required parameters.
	ClientNonce string
	State       string
	Code        string

	// RedirectURI is the URL that authorization should redirect to. This is a
	// required parameter.
	RedirectURI string

	WriteRequest
}

// Validate ensures the request object contains all the required fields in
// order to complete the OIDC authentication flow.
func (a *ACLOIDCCompleteAuthRequest) Validate() error {

	var mErr multierror.Error

	if a.AuthMethodName == "" {
		mErr.Errors = append(mErr.Errors, errors.New("missing auth method name"))
	}
	if a.ClientNonce == "" {
		mErr.Errors = append(mErr.Errors, errors.New("missing client nonce"))
	}
	if a.State == "" {
		mErr.Errors = append(mErr.Errors, errors.New("missing state"))
	}
	if a.Code == "" {
		mErr.Errors = append(mErr.Errors, errors.New("missing code"))
	}
	if a.RedirectURI == "" {
		mErr.Errors = append(mErr.Errors, errors.New("missing redirect URI"))
	}
	return mErr.ErrorOrNil()
}

// ACLLoginResponse is the response when the auth flow has been
// completed successfully.
type ACLLoginResponse struct {
	ACLToken *ACLToken
	WriteMeta
}

// ACLLoginRequest is the request object to begin auth with an external
// token provider.
type ACLLoginRequest struct {

	// AuthMethodName is the name of the auth method being used to login. This
	// is a required parameter.
	AuthMethodName string

	// LoginToken is the 3rd party token that we use to exchange for Nomad ACL
	// Token in order to authenticate. This is a required parameter.
	LoginToken string

	WriteRequest
}

// Validate ensures the request object contains all the required fields in
// order to complete the authentication flow.
func (a *ACLLoginRequest) Validate() error {

	var mErr multierror.Error

	if a.AuthMethodName == "" {
		mErr.Errors = append(mErr.Errors, errors.New("missing auth method name"))
	}
	if a.LoginToken == "" {
		mErr.Errors = append(mErr.Errors, errors.New("missing login token"))
	}
	return mErr.ErrorOrNil()
}
