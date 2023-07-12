package structs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-set"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/exp/slices"
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
)

var (
	// validACLRoleName is used to validate an ACL role name.
	validACLRoleName = regexp.MustCompile("^[a-zA-Z0-9-]{1,128}$")
)

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

	if !validACLRoleName.MatchString(a.Name) {
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

// Equals performs an equality check on the two service registrations. It
// handles nil objects.
func (a *ACLRole) Equals(o *ACLRole) bool {
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
