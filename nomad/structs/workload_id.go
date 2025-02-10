// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	jwt "github.com/go-jose/go-jose/v3/jwt"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/helper/uuid"
)

const (
	// WorkloadIdentityDefaultName is the name of the default (builtin) Workload
	// Identity.
	WorkloadIdentityDefaultName = "default"

	// WorkloadIdentityDefaultAud is the audience of the default identity.
	WorkloadIdentityDefaultAud = "nomadproject.io"

	// WorkloadIdentityVaultPrefix is the name prefix of workload identities
	// used to derive Vault tokens.
	WorkloadIdentityVaultPrefix = "vault_"

	// WIRejectionReasonMissingAlloc is the WorkloadIdentityRejection.Reason
	// returned when an allocation longer exists. This may be due to the alloc
	// being GC'd or the job being updated.
	WIRejectionReasonMissingAlloc = "allocation not found"

	// WIRejectionReasonMissingTask is the WorkloadIdentityRejection.Reason
	// returned when the requested task no longer exists on the allocation.
	WIRejectionReasonMissingTask = "task not found"

	// WIRejectionReasonMissingIdentity is the WorkloadIdentityRejection.Reason
	// returned when the requested identity does not exist on the allocation.
	WIRejectionReasonMissingIdentity = "identity not found"

	// WIChangeModeNoop takes no action when a new token is retrieved.
	WIChangeModeNoop = "noop"

	// WIChangeModeSignal signals the task when a new token is retrieved.
	WIChangeModeSignal = "signal"

	// WIChangeModeRestart restarts the task when a new token is retrieved.
	WIChangeModeRestart = "restart"
)

var (
	// validIdentityName is used to validate workload identity Name fields. Must
	// be safe to use in filenames.
	validIdentityName = regexp.MustCompile("^[a-zA-Z0-9-_]{1,128}$")

	// MinNomadVersionVaultWID is the minimum version of Nomad that supports
	// workload identities for Vault.
	// "-a" is used here so that it is "less than" all pre-release versions of
	// Nomad 1.7.0 as well
	MinNomadVersionVaultWID = version.Must(version.NewVersion("1.7.0-a"))
)

// IdentityClaims are the input to a JWT identifying a workload. It
// should never be serialized to msgpack unsigned.
type IdentityClaims struct {
	Namespace    string `json:"nomad_namespace"`
	JobID        string `json:"nomad_job_id"`
	AllocationID string `json:"nomad_allocation_id"`
	TaskName     string `json:"nomad_task,omitempty"`
	ServiceName  string `json:"nomad_service,omitempty"`

	ConsulNamespace string `json:"consul_namespace,omitempty"`
	VaultNamespace  string `json:"vault_namespace,omitempty"`
	VaultRole       string `json:"vault_role,omitempty"`

	// ExtraClaims are added based on this identity's
	// WorkloadIdentityConfiguration, controlled by server configuration
	ExtraClaims map[string]string `json:"extra_claims,omitempty"`

	jwt.Claims
}

// IdentityClaimsBuilder is used to build up all the context we need to create
// IdentityClaims from jobs, allocs, tasks, services, Vault and Consul
// configurations, etc. This lets us treat IdentityClaims as the immutable
// output of that process.
type IdentityClaimsBuilder struct {
	wid         *WorkloadIdentity // from jobspec
	wihandle    *WIHandle
	alloc       *Allocation
	job         *Job
	tg          *TaskGroup
	task        *Task
	serviceName string
	consul      *Consul
	vault       *Vault
	node        *Node
	extras      map[string]string
}

// NewIdentityClaimsBuilder returns an initialized IdentityClaimsBuilder for the
// allocation and identity request. Because it may be called with a denormalized
// Allocation in the plan applier, the Job must be passed in as a separate
// parameter.
func NewIdentityClaimsBuilder(job *Job, alloc *Allocation, wihandle *WIHandle, wid *WorkloadIdentity) *IdentityClaimsBuilder {
	tg := job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil {
		return nil
	}
	if wid == nil {
		wid = DefaultWorkloadIdentity()
	}

	return &IdentityClaimsBuilder{
		alloc:    alloc,
		job:      job,
		wihandle: wihandle,
		wid:      wid,
		tg:       tg,
		extras:   map[string]string{},
	}
}

// WithTask adds a task to the builder context.
func (b *IdentityClaimsBuilder) WithTask(task *Task) *IdentityClaimsBuilder {
	if task == nil {
		return b
	}
	b.task = task
	return b
}

// WithVault adds the task's vault block to the builder context. This should
// only be called after WithTask.
func (b *IdentityClaimsBuilder) WithVault(extraClaims map[string]string) *IdentityClaimsBuilder {
	if !b.wid.IsVault() || b.task == nil {
		return b
	}
	b.vault = b.task.Vault
	for k, v := range extraClaims {
		b.extras[k] = v
	}
	return b
}

// WithConsul adds the group or task's consul block to the builder context. For
// task identities, this should only be called after WithTask.
func (b *IdentityClaimsBuilder) WithConsul() *IdentityClaimsBuilder {
	if !b.wid.IsConsul() {
		return b
	}
	if b.task != nil && b.task.Consul != nil {
		b.consul = b.task.Consul
	} else if b.tg.Consul != nil {
		b.consul = b.tg.Consul
	}
	return b
}

// WithService adds a service block to the builder context. This should only be
// called for service identities, and a builder for service identities will
// never set the task_name claim.
func (b *IdentityClaimsBuilder) WithService(service *Service) *IdentityClaimsBuilder {
	if b.wihandle.WorkloadType != WorkloadTypeService {
		return b
	}
	serviceName := b.wihandle.WorkloadIdentifier
	if b.wihandle.InterpolatedWorkloadIdentifier != "" {
		serviceName = b.wihandle.InterpolatedWorkloadIdentifier
	}
	b.serviceName = serviceName
	return b
}

// WithNode add the allocation's node to the builder context.
func (b *IdentityClaimsBuilder) WithNode(node *Node) *IdentityClaimsBuilder {
	b.node = node
	return b
}

// Build is the terminal method for the builder and sets all the derived values
// on the claim. The claim ID is random (nondeterministic) so multiple calls
// with the same values will not return equal claims by design. JWT IDs should
// never collide.
func (b *IdentityClaimsBuilder) Build(now time.Time) *IdentityClaims {
	b.interpolate()

	jwtnow := jwt.NewNumericDate(now.UTC())
	claims := &IdentityClaims{
		Namespace:    b.alloc.Namespace,
		JobID:        b.job.GetIDforWorkloadIdentity(),
		AllocationID: b.alloc.ID,
		ServiceName:  b.serviceName,
		Claims: jwt.Claims{
			NotBefore: jwtnow,
			IssuedAt:  jwtnow,
		},
		ExtraClaims: b.extras,
	}
	if b.task != nil && b.wihandle.WorkloadType != WorkloadTypeService {
		claims.TaskName = b.task.Name
	}
	if b.consul != nil {
		claims.ConsulNamespace = b.consul.Namespace
	}
	if b.vault != nil {
		claims.VaultNamespace = b.vault.Namespace
		claims.VaultRole = b.vault.Role
	}

	claims.Audience = slices.Clone(b.wid.Audience)
	claims.setSubject(b.job, b.alloc.TaskGroup, b.wihandle.WorkloadIdentifier, b.wid.Name)
	claims.setExp(now, b.wid)

	claims.ID = uuid.Generate()

	return claims
}

func strAttrGet[T any](x *T, fn func(x *T) string) string {
	if x != nil {
		return fn(x)
	}
	return ""
}

func (b *IdentityClaimsBuilder) interpolate() {
	if len(b.extras) == 0 {
		return
	}

	r := strings.NewReplacer(
		// attributes that always exist
		"${job.region}", b.job.Region,
		"${job.namespace}", b.job.Namespace,
		"${job.id}", b.job.GetIDforWorkloadIdentity(),
		"${job.node_pool}", b.job.NodePool,
		"${group.name}", b.tg.Name,
		"${alloc.id}", b.alloc.ID,

		// attributes that conditionally exist
		"${node.id}", strAttrGet(b.node, func(n *Node) string { return n.ID }),
		"${node.datacenter}", strAttrGet(b.node, func(n *Node) string { return n.Datacenter }),
		"${node.pool}", strAttrGet(b.node, func(n *Node) string { return n.NodePool }),
		"${node.class}", strAttrGet(b.node, func(n *Node) string { return n.NodeClass }),
		"${task.name}", strAttrGet(b.task, func(t *Task) string { return t.Name }),
		"${vault.cluster}", strAttrGet(b.vault, func(v *Vault) string { return v.Cluster }),
		"${vault.namespace}", strAttrGet(b.vault, func(v *Vault) string { return v.Namespace }),
		"${vault.role}", strAttrGet(b.vault, func(v *Vault) string { return v.Role }),
	)
	for k, v := range b.extras {
		b.extras[k] = r.Replace(v)
	}
}

// setSubject creates the standard subject claim for workload identities.
func (claims *IdentityClaims) setSubject(job *Job, group, widentifier, id string) {
	claims.Subject = strings.Join([]string{
		job.Region,
		job.Namespace,
		job.GetIDforWorkloadIdentity(),
		group,
		widentifier,
		id,
	}, ":")
}

// setExp sets the absolute time at which these identity claims expire.
func (claims *IdentityClaims) setExp(now time.Time, wid *WorkloadIdentity) {
	if wid.TTL == 0 {
		// No expiry
		return
	}

	claims.Expiry = jwt.NewNumericDate(now.Add(wid.TTL))
}

// WorkloadIdentity is the jobspec block which determines if and how a workload
// identity is exposed to tasks similar to the Vault block.
//
// CAUTION: a similar struct called WorkloadIdentityConfig lives in
// nomad/structs/config/workload_id.go and is used for agent configuration.
// Updates here may need to be applied there as well.
type WorkloadIdentity struct {
	Name string

	// Audience is the valid recipients for this identity (the "aud" JWT claim)
	// and defaults to the identity's name.
	Audience []string

	// ChangeMode is used to configure the task's behavior when the identity
	// token changes.
	ChangeMode string

	// ChangeSignal is the signal sent to the task when a new token is
	// retrieved. This is only valid when using the signal change mode.
	ChangeSignal string

	// Env injects the Workload Identity into the Task's environment if
	// set.
	Env bool

	// File writes the Workload Identity into the Task's secrets directory
	// or path specified by Filepath if set.
	File bool

	// Filepath is used to specify a custom path for the Task's Workload
	// Identity JWT.
	Filepath string

	// ServiceName is used to bind the identity to a correct Consul service.
	ServiceName string

	// TTL is used to determine the expiration of the credentials created for
	// this identity (eg the JWT "exp" claim).
	TTL time.Duration

	// Note: ExtraClaims is available on config/WorkloadIdentity but not
	// available here on jobspecs because that might allow a job author to
	// escalate their privileges if they know what claim mappings to expect.
}

func DefaultWorkloadIdentity() *WorkloadIdentity {
	return &WorkloadIdentity{
		Name:     WorkloadIdentityDefaultName,
		Audience: []string{WorkloadIdentityDefaultAud},
	}
}

// IsConsul returns true if the identity name starts with the standard prefix
// for Consul tasks and services.
func (wi *WorkloadIdentity) IsConsul() bool {
	if wi == nil {
		return false
	}
	return strings.HasPrefix(wi.Name, ConsulTaskIdentityNamePrefix) ||
		strings.HasPrefix(wi.Name, ConsulServiceIdentityNamePrefix)
}

// IsVault returns true if the identity name starts with the standard prefix
// for Vault tasks.
func (wi *WorkloadIdentity) IsVault() bool {
	if wi == nil {
		return false
	}
	return strings.HasPrefix(wi.Name, WorkloadIdentityVaultPrefix)
}

func (wi *WorkloadIdentity) Copy() *WorkloadIdentity {
	if wi == nil {
		return nil
	}
	return &WorkloadIdentity{
		Name:         wi.Name,
		Audience:     slices.Clone(wi.Audience),
		ChangeMode:   wi.ChangeMode,
		ChangeSignal: wi.ChangeSignal,
		Env:          wi.Env,
		File:         wi.File,
		Filepath:     wi.Filepath,
		ServiceName:  wi.ServiceName,
		TTL:          wi.TTL,
	}
}

func (wi *WorkloadIdentity) Equal(other *WorkloadIdentity) bool {
	if wi == nil || other == nil {
		return wi == other
	}

	if wi.Name != other.Name {
		return false
	}

	if !slices.Equal(wi.Audience, other.Audience) {
		return false
	}

	if wi.ChangeMode != other.ChangeMode {
		return false
	}

	if wi.ChangeSignal != other.ChangeSignal {
		return false
	}

	if wi.Env != other.Env {
		return false
	}

	if wi.File != other.File {
		return false
	}

	if wi.Filepath != other.Filepath {
		return false
	}

	if wi.ServiceName != other.ServiceName {
		return false
	}

	if wi.TTL != other.TTL {
		return false
	}

	return true
}

func (wi *WorkloadIdentity) Canonicalize() {
	if wi == nil {
		return
	}

	if wi.Name == "" {
		wi.Name = WorkloadIdentityDefaultName
	}

	// The default identity is only valid for use with Nomad itself.
	if wi.Name == WorkloadIdentityDefaultName {
		wi.Audience = []string{WorkloadIdentityDefaultAud}
	}

	if wi.ChangeSignal != "" {
		wi.ChangeSignal = strings.ToUpper(wi.ChangeSignal)
	}
}

func (wi *WorkloadIdentity) Validate() error {
	if wi == nil {
		return fmt.Errorf("must not be nil")
	}

	var mErr multierror.Error

	if !validIdentityName.MatchString(wi.Name) {
		err := fmt.Errorf("invalid name %q. Must match regex %s", wi.Name, validIdentityName)
		mErr.Errors = append(mErr.Errors, err)
	}

	for i, aud := range wi.Audience {
		if aud == "" {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("an empty string is an invalid audience (%d)", i+1))
		}
	}

	switch wi.ChangeMode {
	case "", WIChangeModeNoop, WIChangeModeRestart:
		// Treat "" as noop. Make sure signal isn't set.
		if wi.ChangeSignal != "" {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("can only use change_signal=%q with change_mode=%q",
				wi.ChangeSignal, WIChangeModeSignal))
		}
	case WIChangeModeSignal:
		if wi.ChangeSignal == "" {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("change_signal must be specified when using change_mode=%q", WIChangeModeSignal))
		}
	default:
		// Unknown change_mode
		mErr.Errors = append(mErr.Errors, fmt.Errorf("invalid change_mode: %s", wi.ChangeMode))
	}

	if wi.TTL > 0 && (wi.Name == "" || wi.Name == WorkloadIdentityDefaultName) {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("ttl for default identity not yet supported"))
	}

	if wi.TTL < 0 {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("ttl must be >= 0"))
	}

	if wi.Filepath != "" && !wi.File {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("file parameter must be true in order to specify filepath"))
	}

	return mErr.ErrorOrNil()
}

func (wi *WorkloadIdentity) Warnings() error {
	if wi == nil {
		return fmt.Errorf("must not be nil")
	}

	var mErr multierror.Error

	if n := len(wi.Audience); n == 0 {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("identities without an audience are insecure"))
	} else if n > 1 {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("while multiple audiences is allowed, it is more secure to use 1 audience per identity"))
	}

	if wi.Name != "" && wi.Name != WorkloadIdentityDefaultName {
		if wi.TTL == 0 {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("identities without an expiration are insecure"))
		}
	}

	// Warn users about using env vars without restarts
	if wi.Env && wi.ChangeMode != WIChangeModeRestart {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("using env=%t without change_mode=%q may result in task not getting updated identity",
			wi.Env, WIChangeModeRestart))
	}

	return mErr.ErrorOrNil()
}

// WorkloadIdentityRequest encapsulates the 3 parameters used to generated a
// signed workload identity: the alloc, task, and specific identity's name.
type WorkloadIdentityRequest struct {
	AllocID string
	WIHandle
}

// SignedWorkloadIdentity is the response to a WorkloadIdentityRequest and
// includes the JWT for the requested workload identity.
type SignedWorkloadIdentity struct {
	WorkloadIdentityRequest
	JWT        string
	Expiration time.Time
}

// WorkloadIdentityRejection is the response to a WorkloadIdentityRequest that
// is rejected and includes a reason.
type WorkloadIdentityRejection struct {
	WorkloadIdentityRequest
	Reason string
}

// AllocIdentitiesRequest is the RPC arguments for requesting signed workload
// identities.
type AllocIdentitiesRequest struct {
	Identities []*WorkloadIdentityRequest
	QueryOptions
}

// AllocIdentitiesResponse is the RPC response for requested workload
// identities including any rejections.
type AllocIdentitiesResponse struct {
	SignedIdentities []*SignedWorkloadIdentity
	Rejections       []*WorkloadIdentityRejection
	QueryMeta
}

type WorkloadType int

const (
	WorkloadTypeTask WorkloadType = iota
	WorkloadTypeService
)

// WIHandle is used by code that needs to uniquely match a workload identity
// with the task or service it belongs to.
type WIHandle struct {
	IdentityName string
	// WorkloadIdentifier is either a ServiceName or a TaskName
	WorkloadIdentifier string
	WorkloadType       WorkloadType

	// InterpolatedWorkloadIdentifier is the WorkloadIdentifier, interpolated by
	// the client. It is used only to provide an override for the identity
	// claims
	InterpolatedWorkloadIdentifier string
}

func (w *WIHandle) Equal(o WIHandle) bool {
	if w == nil {
		return false
	}
	// note: we're intentionally ignoring InterpolatedWorkloadIdentifier here
	return w.IdentityName == o.IdentityName &&
		w.WorkloadIdentifier == o.WorkloadIdentifier &&
		w.WorkloadType == o.WorkloadType
}
