// +build ent

package structs

import (
	"encoding/binary"
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
	// SentinelEnforcementLevelAdvisory allows a policy to fail and issues a warning
	SentinelEnforcementLevelAdvisory = "advisory"

	// SentinelEnforcementLevelSoftMandatory prevents an operation unless an override is set, and then warns
	SentinelEnforcementLevelSoftMandatory = "soft-mandatory"

	// SentinelEnforcementLevelHardMandatory prevents an operation on failure
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

// QuotaSpec specifies the allowed resource usage across regions.
type QuotaSpec struct {
	// Name is the name for the quota object
	Name string

	// Description is an optional description for the quota object
	Description string

	// Limits is the set of quota limits encapsulated by this quota object. Each
	// limit applies quota in a particular region and in the future over a
	// particular priority range and datacenter set.
	Limits []*QuotaLimit

	// Hash is the hash of the object and is used to make replication efficient.
	Hash []byte

	// Raft indexes to track creation and modification
	CreateIndex uint64
	ModifyIndex uint64
}

// SetHash is used to compute and set the hash of the QuotaSpec
func (q *QuotaSpec) SetHash() []byte {
	// Initialize a 256bit Blake2 hash (32 bytes)
	hash, err := blake2b.New256(nil)
	if err != nil {
		panic(err)
	}

	// Write all the user set fields
	hash.Write([]byte(q.Name))
	hash.Write([]byte(q.Description))

	for _, l := range q.Limits {
		hash.Write(l.SetHash())
	}

	// Finalize the hash
	hashVal := hash.Sum(nil)

	// Set and return the hash
	q.Hash = hashVal
	return hashVal
}

func (q *QuotaSpec) Validate() error {
	var mErr multierror.Error
	if !validPolicyName.MatchString(q.Name) {
		err := fmt.Errorf("invalid name %q", q.Name)
		mErr.Errors = append(mErr.Errors, err)
	}
	if len(q.Description) > maxPolicyDescriptionLength {
		err := fmt.Errorf("description longer than %d", maxPolicyDescriptionLength)
		mErr.Errors = append(mErr.Errors, err)
	}

	if len(q.Limits) == 0 {
		err := fmt.Errorf("must provide at least one quota limit")
		mErr.Errors = append(mErr.Errors, err)
	} else {
		for i, l := range q.Limits {
			if err := l.Validate(); err != nil {
				wrapped := fmt.Errorf("invalid quota limit %d: %v", i, err)
				mErr.Errors = append(mErr.Errors, wrapped)
			}
		}
	}

	return mErr.ErrorOrNil()
}

// QuotaLimit describes the resource limit in a particular region.
type QuotaLimit struct {
	// Region is the region in which this limit has affect
	Region string

	// RegionLimit is the quota limit that applies to any allocation within a
	// referencing namespace in the region.
	RegionLimit *Resources

	// Hash is the hash of the object and is used to make replication efficient.
	Hash []byte
}

// SetHash is used to compute and set the hash of the QuotaLimit
func (q *QuotaLimit) SetHash() []byte {
	// Initialize a 256bit Blake2 hash (32 bytes)
	hash, err := blake2b.New256(nil)
	if err != nil {
		panic(err)
	}

	// Write all the user set fields
	hash.Write([]byte(q.Region))

	if q.RegionLimit != nil {
		binary.Write(hash, binary.LittleEndian, q.RegionLimit.CPU)
		binary.Write(hash, binary.LittleEndian, q.RegionLimit.MemoryMB)
	}

	// Finalize the hash
	hashVal := hash.Sum(nil)
	q.Hash = hashVal
	return hashVal
}

// Validate validates the QuotaLimit
func (q *QuotaLimit) Validate() error {
	var mErr multierror.Error

	if q.Region == "" {
		err := fmt.Errorf("must provide a region")
		mErr.Errors = append(mErr.Errors, err)
	}

	if q.RegionLimit == nil {
		err := fmt.Errorf("must provide a region limit")
		mErr.Errors = append(mErr.Errors, err)
	} else {
		if q.RegionLimit.DiskMB > 0 {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("quota can not limit disk"))
		}
		if len(q.RegionLimit.Networks) > 0 {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("quota can not limit networks"))
		}
	}

	return mErr.ErrorOrNil()
}

// QuotaUsage is local to a region and is used to track current
// resource usage for the quota object.
type QuotaUsage struct {
	// Name is a uniquely identifying name that is shared with the spec
	Name string

	// Used is the currently used resources for each quota limit. The map is
	// keyed by the QuotaLimit hash.
	Used map[string]*QuotaLimit

	// Raft indexes to track creation and modification
	CreateIndex uint64
	ModifyIndex uint64
}

// QuotaSpecListRequest is used to request a list of quota specifications
type QuotaSpecListRequest struct {
	QueryOptions
}

// QuotaSpecSpecificRequest is used to query a specific quota specification
type QuotaSpecSpecificRequest struct {
	Name string
	QueryOptions
}

// QuotaSpecSetRequest is used to query a set of quota specs
type QuotaSpecSetRequest struct {
	Names []string
	QueryOptions
}

// QuotaSpecListResponse is used for a list request
type QuotaSpecListResponse struct {
	Quotas []*QuotaSpec
	QueryMeta
}

// SingleQuotaSpecResponse is used to return a single quota specification
type SingleQuotaSpecResponse struct {
	Quota *QuotaSpec
	QueryMeta
}

// QuotaSpecSetResponse is used to return a set of quota specifications
type QuotaSpecSetResponse struct {
	Quotas map[string]*QuotaSpec
	QueryMeta
}

// QuotaSpecDeleteRequest is used to delete a set of quota specifications
type QuotaSpecDeleteRequest struct {
	Names []string
	WriteRequest
}

// QuotaSpecUpsertRequest is used to upsert a set of quota specifications
type QuotaSpecUpsertRequest struct {
	Quotas []*QuotaSpec
	WriteRequest
}

// QuotaUsageListRequest is used to request a list of quota usages
type QuotaUsageListRequest struct {
	QueryOptions
}

// QuotaUsageSpecificRequest is used to query a specific quota usage
type QuotaUsageSpecificRequest struct {
	Name string
	QueryOptions
}

// QuotaUsageListResponse is used for a list request
type QuotaUsageListResponse struct {
	Usages []*QuotaUsage
	QueryMeta
}

// SingleQuotaUsageResponse is used to return a single quota usage
type SingleQuotaUsageResponse struct {
	Usage *QuotaUsage
	QueryMeta
}
