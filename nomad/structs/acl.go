package structs

import (
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
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
)

const (
	// ACLMaxExpiredBatchSize is the maximum number of expired ACL tokens that
	// will be garbage collected in a single trigger. This number helps limit
	// the replication pressure due to expired token deletion. If there are a
	// large number of expired tokens pending garbage collection, this value is
	// a potential limiting factor.
	ACLMaxExpiredBatchSize = 4096
)

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
	// have associated policies, whereas a management token cannot be
	// associated with policies.
	switch a.Type {
	case ACLClientToken:
		if len(a.Policies) == 0 {
			mErr.Errors = append(mErr.Errors, errors.New("client token missing policies"))
		}
	case ACLManagementToken:
		if len(a.Policies) != 0 {
			mErr.Errors = append(mErr.Errors, errors.New("management token cannot be associated with policies"))
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
