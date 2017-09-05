// +build ent

package structs

import (
	"fmt"

	"github.com/hashicorp/errwrap"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/sentinel/lang/ast"
	"github.com/hashicorp/sentinel/lang/parser"
	"github.com/hashicorp/sentinel/lang/semantic"
	"github.com/hashicorp/sentinel/lang/token"
	"golang.org/x/crypto/blake2b"
)

// Restrict the possible Sentinel policy types
const (
	SentinelEnforcementLevelAdvisory      = "advisory"
	SentinelEnforcementLevelSoftMandatory = "soft-mandatory"
	SentinelEnforcementLevelHardMandatory = "hard-mandatory"
)

// Restrict the possible Sentinel scopes
const (
	SentinelScopeSubmitJob = "submit-job"
)

// SentinelPolicy is used to represent a Sentinel policy
type SentinelPolicy struct {
	Name             string // Unique name
	Description      string // Human readable
	Scope            string // Where should this policy be executed
	EnforcementLevel string // Enforcement Level
	Policy           string
	Hash             []byte
	CreateIndex      uint64
	ModifyIndex      uint64
}

type SentinelPolicyListStub struct {
	Name             string
	Description      string
	Scope            string
	EnforcementLevel string
	Hash             []byte
	CreateIndex      uint64
	ModifyIndex      uint64
}

func (s *SentinelPolicy) Stub() *SentinelPolicyListStub {
	return &SentinelPolicyListStub{
		Name:             s.Name,
		Description:      s.Description,
		Scope:            s.Scope,
		EnforcementLevel: s.EnforcementLevel,
		Hash:             s.Hash,
		CreateIndex:      s.CreateIndex,
		ModifyIndex:      s.ModifyIndex,
	}
}

// SetHash is used to compute and set the hash of the ACL policy
func (s *SentinelPolicy) SetHash() []byte {
	// Initialize a 256bit Blake2 hash (32 bytes)
	hash, err := blake2b.New256(nil)
	if err != nil {
		panic(err)
	}

	// Write all the user set fields
	hash.Write([]byte(s.Name))
	hash.Write([]byte(s.Description))
	hash.Write([]byte(s.Scope))
	hash.Write([]byte(s.EnforcementLevel))
	hash.Write([]byte(s.Policy))

	// Finalize the hash
	hashVal := hash.Sum(nil)

	// Set and return the hash
	s.Hash = hashVal
	return hashVal
}

func (s *SentinelPolicy) Validate() error {
	var mErr multierror.Error
	if !validPolicyName.MatchString(s.Name) {
		err := fmt.Errorf("invalid name %q", s.Name)
		mErr.Errors = append(mErr.Errors, err)
	}
	if len(s.Description) > maxPolicyDescriptionLength {
		err := fmt.Errorf("description longer than %d", maxPolicyDescriptionLength)
		mErr.Errors = append(mErr.Errors, err)
	}
	if s.Scope != SentinelScopeSubmitJob {
		err := fmt.Errorf("invalid scope %q", s.Scope)
		mErr.Errors = append(mErr.Errors, err)
	}
	switch s.EnforcementLevel {
	case SentinelEnforcementLevelAdvisory, SentinelEnforcementLevelSoftMandatory, SentinelEnforcementLevelHardMandatory:
	default:
		err := fmt.Errorf("invalid enforcement level %q",
			s.EnforcementLevel)
		mErr.Errors = append(mErr.Errors, err)
	}

	// Validate that policy compiles
	if _, _, err := s.Compile(); err != nil {
		err = errwrap.Wrapf("policy compile error: {{err}}", err)
		mErr.Errors = append(mErr.Errors, err)
	}
	return mErr.ErrorOrNil()
}

// CacheKey returns a key that gets invalidated on changes
func (s *SentinelPolicy) CacheKey() string {
	return fmt.Sprintf("%s:%d", s.Name, s.ModifyIndex)
}

// Compile is used to compile the Sentinel policy for policy.SetPolicy
func (s *SentinelPolicy) Compile() (*ast.File, *token.FileSet, error) {
	// Parse
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, s.Name, s.Policy, 0)
	if err != nil {
		return nil, nil, err
	}

	// Perform semantic checks
	if err := semantic.Check(f, fset); err != nil {
		return nil, nil, err
	}

	// Return the reuslt
	return f, fset, nil
}

// SentinelPolicyListRequest is used to request a list of policies
type SentinelPolicyListRequest struct {
	QueryOptions
}

// SentinelPolicySpecificRequest is used to query a specific policy
type SentinelPolicySpecificRequest struct {
	Name string
	QueryOptions
}

// SentinelPolicySetRequest is used to query a set of policies
type SentinelPolicySetRequest struct {
	Names []string
	QueryOptions
}

// SentinelPolicyListResponse is used for a list request
type SentinelPolicyListResponse struct {
	Policies []*SentinelPolicyListStub
	QueryMeta
}

// SingleSentinelPolicyResponse is used to return a single policy
type SingleSentinelPolicyResponse struct {
	Policy *SentinelPolicy
	QueryMeta
}

// SentinelPolicySetResponse is used to return a set of policies
type SentinelPolicySetResponse struct {
	Policies map[string]*SentinelPolicy
	QueryMeta
}

// SentinelPolicyDeleteRequest is used to delete a set of policies
type SentinelPolicyDeleteRequest struct {
	Names []string
	WriteRequest
}

// SentinelPolicyUpsertRequest is used to upsert a set of policies
type SentinelPolicyUpsertRequest struct {
	Policies []*SentinelPolicy
	WriteRequest
}
