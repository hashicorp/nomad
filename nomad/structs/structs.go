// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"bytes"
	"container/heap"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"maps"
	"math"
	"net"
	"os"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/cronexpr"
	"github.com/hashicorp/go-msgpack/v2/codec"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
	"github.com/hashicorp/nomad/command/agent/host"
	"github.com/hashicorp/nomad/command/agent/pprof"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/args"
	"github.com/hashicorp/nomad/helper/constraints/semver"
	"github.com/hashicorp/nomad/helper/escapingfs"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/lib/kheap"
	psstructs "github.com/hashicorp/nomad/plugins/shared/structs"
	"github.com/miekg/dns"
	"github.com/mitchellh/copystructure"
	"github.com/ryanuber/go-glob"
	"golang.org/x/crypto/blake2b"
)

var (
	// ValidPolicyName is used to validate a policy name
	ValidPolicyName = regexp.MustCompile("^[a-zA-Z0-9-]{1,128}$")

	// b32 is a lowercase base32 encoding for use in URL friendly service hashes
	b32 = base32.NewEncoding(strings.ToLower("abcdefghijklmnopqrstuvwxyz234567"))
)

type MessageType uint8

// note: new raft message types need to be added to the end of this
// list of contents
const (
	NodeRegisterRequestType                      MessageType = 0
	NodeDeregisterRequestType                    MessageType = 1
	NodeUpdateStatusRequestType                  MessageType = 2
	NodeUpdateDrainRequestType                   MessageType = 3
	JobRegisterRequestType                       MessageType = 4
	JobDeregisterRequestType                     MessageType = 5
	EvalUpdateRequestType                        MessageType = 6
	EvalDeleteRequestType                        MessageType = 7
	AllocUpdateRequestType                       MessageType = 8
	AllocClientUpdateRequestType                 MessageType = 9
	ReconcileJobSummariesRequestType             MessageType = 10
	VaultAccessorRegisterRequestType             MessageType = 11
	VaultAccessorDeregisterRequestType           MessageType = 12
	ApplyPlanResultsRequestType                  MessageType = 13
	DeploymentStatusUpdateRequestType            MessageType = 14
	DeploymentPromoteRequestType                 MessageType = 15
	DeploymentAllocHealthRequestType             MessageType = 16
	DeploymentDeleteRequestType                  MessageType = 17
	JobStabilityRequestType                      MessageType = 18
	ACLPolicyUpsertRequestType                   MessageType = 19
	ACLPolicyDeleteRequestType                   MessageType = 20
	ACLTokenUpsertRequestType                    MessageType = 21
	ACLTokenDeleteRequestType                    MessageType = 22
	ACLTokenBootstrapRequestType                 MessageType = 23
	AutopilotRequestType                         MessageType = 24
	UpsertNodeEventsType                         MessageType = 25
	JobBatchDeregisterRequestType                MessageType = 26
	AllocUpdateDesiredTransitionRequestType      MessageType = 27
	NodeUpdateEligibilityRequestType             MessageType = 28
	BatchNodeUpdateDrainRequestType              MessageType = 29
	SchedulerConfigRequestType                   MessageType = 30
	NodeBatchDeregisterRequestType               MessageType = 31
	ClusterMetadataRequestType                   MessageType = 32
	ServiceIdentityAccessorRegisterRequestType   MessageType = 33
	ServiceIdentityAccessorDeregisterRequestType MessageType = 34
	CSIVolumeRegisterRequestType                 MessageType = 35
	CSIVolumeDeregisterRequestType               MessageType = 36
	CSIVolumeClaimRequestType                    MessageType = 37
	ScalingEventRegisterRequestType              MessageType = 38
	CSIVolumeClaimBatchRequestType               MessageType = 39
	CSIPluginDeleteRequestType                   MessageType = 40
	EventSinkUpsertRequestType                   MessageType = 41
	EventSinkDeleteRequestType                   MessageType = 42
	BatchEventSinkUpdateProgressType             MessageType = 43
	OneTimeTokenUpsertRequestType                MessageType = 44
	OneTimeTokenDeleteRequestType                MessageType = 45
	OneTimeTokenExpireRequestType                MessageType = 46
	ServiceRegistrationUpsertRequestType         MessageType = 47
	ServiceRegistrationDeleteByIDRequestType     MessageType = 48
	ServiceRegistrationDeleteByNodeIDRequestType MessageType = 49
	VarApplyStateRequestType                     MessageType = 50
	RootKeyMetaUpsertRequestType                 MessageType = 51 // DEPRECATED
	WrappedRootKeysDeleteRequestType             MessageType = 52
	ACLRolesUpsertRequestType                    MessageType = 53
	ACLRolesDeleteByIDRequestType                MessageType = 54
	ACLAuthMethodsUpsertRequestType              MessageType = 55
	ACLAuthMethodsDeleteRequestType              MessageType = 56
	ACLBindingRulesUpsertRequestType             MessageType = 57
	ACLBindingRulesDeleteRequestType             MessageType = 58
	NodePoolUpsertRequestType                    MessageType = 59
	NodePoolDeleteRequestType                    MessageType = 60
	JobVersionTagRequestType                     MessageType = 61
	WrappedRootKeysUpsertRequestType             MessageType = 62
	NamespaceUpsertRequestType                   MessageType = 64
	NamespaceDeleteRequestType                   MessageType = 65

	// NOTE: MessageTypes are shared between CE and ENT. If you need to add a
	// new type, check that ENT is not already using that value.
)

const (

	// SystemInitializationType is used for messages that initialize parts of
	// the system, such as the state store. These messages are not included in
	// the event stream.
	SystemInitializationType MessageType = 127

	// IgnoreUnknownTypeFlag is set along with a MessageType
	// to indicate that the message type can be safely ignored
	// if it is not recognized. This is for future proofing, so
	// that new commands can be added in a way that won't cause
	// old servers to crash when the FSM attempts to process them.
	IgnoreUnknownTypeFlag MessageType = 128

	// MsgTypeTestSetup is used during testing when calling state store
	// methods directly that require an FSM MessageType
	MsgTypeTestSetup MessageType = IgnoreUnknownTypeFlag

	GetterModeAny  = "any"
	GetterModeFile = "file"
	GetterModeDir  = "dir"

	// maxPolicyDescriptionLength limits a policy description length
	maxPolicyDescriptionLength = 256

	// maxTokenNameLength limits a ACL token name length
	maxTokenNameLength = 256

	// ACLClientToken and ACLManagementToken are the only types of tokens
	ACLClientToken     = "client"
	ACLManagementToken = "management"

	// DefaultNamespace is the default namespace.
	DefaultNamespace            = "default"
	DefaultNamespaceDescription = "Default shared namespace"

	// AllNamespacesSentinel is the value used as a namespace RPC value
	// to indicate that endpoints must search in all namespaces
	//
	// Also defined in acl/acl.go to avoid circular dependencies. If modified
	// it should be updated there as well.
	AllNamespacesSentinel = "*"

	// maxNamespaceDescriptionLength limits a namespace description length
	maxNamespaceDescriptionLength = 256

	// JitterFraction is a the limit to the amount of jitter we apply
	// to a user specified MaxQueryTime. We divide the specified time by
	// the fraction. So 16 == 6.25% limit of jitter. This jitter is also
	// applied to RPCHoldTimeout.
	JitterFraction = 16

	// MaxRetainedNodeEvents is the maximum number of node events that will be
	// retained for a single node
	MaxRetainedNodeEvents = 10

	// MaxRetainedNodeScores is the number of top scoring nodes for which we
	// retain scoring metadata
	MaxRetainedNodeScores = 5

	// Normalized scorer name
	NormScorerName = "normalized-score"

	// MaxBlockingRPCQueryTime is used to bound the limit of a blocking query
	MaxBlockingRPCQueryTime = 300 * time.Second

	// DefaultBlockingRPCQueryTime is the amount of time we block waiting for a change
	// if no time is specified. Previously we would wait the MaxBlockingRPCQueryTime.
	DefaultBlockingRPCQueryTime = 300 * time.Second

	// RateMetric constants are used as labels in RPC rate metrics
	RateMetricRead  = "read"
	RateMetricList  = "list"
	RateMetricWrite = "write"
)

var (
	// validNamespaceName is used to validate a namespace name
	validNamespaceName = regexp.MustCompile("^[a-zA-Z0-9-]{1,128}$")
)

// NamespacedID is a tuple of an ID and a namespace
type NamespacedID struct {
	ID        string
	Namespace string
}

// NewNamespacedID returns a new namespaced ID given the ID and namespace
func NewNamespacedID(id, ns string) NamespacedID {
	return NamespacedID{
		ID:        id,
		Namespace: ns,
	}
}

func (n NamespacedID) String() string {
	return fmt.Sprintf("<ns: %q, id: %q>", n.Namespace, n.ID)
}

// RPCInfo is used to describe common information about query
type RPCInfo interface {
	RequestRegion() string
	IsRead() bool
	AllowStaleRead() bool
	IsForwarded() bool
	SetForwarded()
	TimeToBlock() time.Duration
	// SetTimeToBlock sets how long this request can block. The requested time may not be possible,
	// so Callers should readback TimeToBlock. E.g. you cannot set time to block at all on WriteRequests
	// and it cannot exceed MaxBlockingRPCQueryTime
	SetTimeToBlock(t time.Duration)
}

// InternalRpcInfo allows adding internal RPC metadata to an RPC. This struct
// should NOT be replicated in the API package as it is internal only.
type InternalRpcInfo struct {
	// Forwarded marks whether the RPC has been forwarded.
	Forwarded bool
}

// IsForwarded returns whether the RPC is forwarded from another server.
func (i *InternalRpcInfo) IsForwarded() bool {
	return i.Forwarded
}

// SetForwarded marks that the RPC is being forwarded from another server.
func (i *InternalRpcInfo) SetForwarded() {
	i.Forwarded = true
}

// QueryOptions is used to specify various flags for read queries
type QueryOptions struct {
	// The target region for this query
	Region string

	// Namespace is the target namespace for the query.
	//
	// Since handlers do not have a default value set they should access
	// the Namespace via the RequestNamespace method.
	//
	// Requests accessing specific namespaced objects must check ACLs
	// against the namespace of the object, not the namespace in the
	// request.
	Namespace string

	// If set, wait until query exceeds given index. Must be provided
	// with MaxQueryTime.
	MinQueryIndex uint64

	// Provided with MinQueryIndex to wait for change.
	MaxQueryTime time.Duration

	// If set, any follower can service the request. Results
	// may be arbitrarily stale.
	AllowStale bool

	// If set, used as prefix for resource list searches
	Prefix string

	// AuthToken is secret portion of the ACL token or workload identity used for
	// the request.
	AuthToken string

	// Filter specifies the go-bexpr filter expression to be used for
	// filtering the data prior to returning a response
	Filter string

	// PerPage is the number of entries to be returned in queries that support
	// paginated lists.
	PerPage int32

	// NextToken is the token used to indicate where to start paging
	// for queries that support paginated lists. This token should be
	// the ID of the next object after the last one seen in the
	// previous response.
	NextToken string

	// Reverse is used to reverse the default order of list results.
	Reverse bool

	identity *AuthenticatedIdentity

	InternalRpcInfo
}

// TimeToBlock returns MaxQueryTime adjusted for maximums and defaults
// it will return 0 if this is not a blocking query
func (q QueryOptions) TimeToBlock() time.Duration {
	if q.MinQueryIndex == 0 {
		return 0
	}
	if q.MaxQueryTime > MaxBlockingRPCQueryTime {
		return MaxBlockingRPCQueryTime
	} else if q.MaxQueryTime <= 0 {
		return DefaultBlockingRPCQueryTime
	}
	return q.MaxQueryTime
}

func (q *QueryOptions) SetTimeToBlock(t time.Duration) {
	q.MaxQueryTime = t
}

func (q QueryOptions) RequestRegion() string {
	return q.Region
}

// RequestNamespace returns the request's namespace or the default namespace if
// no explicit namespace was sent.
//
// Requests accessing specific namespaced objects must check ACLs against the
// namespace of the object, not the namespace in the request.
func (q QueryOptions) RequestNamespace() string {
	if q.Namespace == "" {
		return DefaultNamespace
	}
	return q.Namespace
}

// IsRead only applies to reads, so always true.
func (q QueryOptions) IsRead() bool {
	return true
}

func (q QueryOptions) AllowStaleRead() bool {
	return q.AllowStale
}

func (q *QueryOptions) GetAuthToken() string {
	return q.AuthToken
}

func (q *QueryOptions) SetIdentity(identity *AuthenticatedIdentity) {
	q.identity = identity
}

func (q QueryOptions) GetIdentity() *AuthenticatedIdentity {
	return q.identity
}

// AgentPprofRequest is used to request a pprof report for a given node.
type AgentPprofRequest struct {
	// ReqType specifies the profile to use
	ReqType pprof.ReqType

	// Profile specifies the runtime/pprof profile to lookup and generate.
	Profile string

	// Seconds is the number of seconds to capture a profile
	Seconds int

	// Debug specifies if pprof profile should inclue debug output
	Debug int

	// GC specifies if the profile should call runtime.GC() before
	// running its profile. This is only used for "heap" profiles
	GC int

	// NodeID is the node we want to track the logs of
	NodeID string

	// ServerID is the server we want to track the logs of
	ServerID string

	QueryOptions
}

// AgentPprofResponse is used to return a generated pprof profile
type AgentPprofResponse struct {
	// ID of the agent that fulfilled the request
	AgentID string

	// Payload is the generated pprof profile
	Payload []byte

	// HTTPHeaders are a set of key value pairs to be applied as
	// HTTP headers for a specific runtime profile
	HTTPHeaders map[string]string
}

type WriteRequest struct {
	// The target region for this write
	Region string

	// Namespace is the target namespace for the write.
	//
	// Since RPC handlers do not have a default value set they should
	// access the Namespace via the RequestNamespace method.
	//
	// Requests accessing specific namespaced objects must check ACLs
	// against the namespace of the object, not the namespace in the
	// request.
	Namespace string

	// AuthToken is secret portion of the ACL token used for the request
	AuthToken string

	// IdempotencyToken can be used to ensure the write is idempotent.
	IdempotencyToken string

	identity *AuthenticatedIdentity

	InternalRpcInfo
}

func (w WriteRequest) TimeToBlock() time.Duration {
	return 0
}

func (w WriteRequest) SetTimeToBlock(_ time.Duration) {
}

func (w WriteRequest) RequestRegion() string {
	// The target region for this request
	return w.Region
}

// RequestNamespace returns the request's namespace or the default namespace if
// no explicit namespace was sent.
//
// Requests accessing specific namespaced objects must check ACLs against the
// namespace of the object, not the namespace in the request.
func (w WriteRequest) RequestNamespace() string {
	if w.Namespace == "" {
		return DefaultNamespace
	}
	return w.Namespace
}

// IsRead only applies to writes, always false.
func (w WriteRequest) IsRead() bool {
	return false
}

func (w WriteRequest) AllowStaleRead() bool {
	return false
}

func (w *WriteRequest) GetAuthToken() string {
	return w.AuthToken
}

func (w *WriteRequest) SetIdentity(identity *AuthenticatedIdentity) {
	w.identity = identity
}

func (w WriteRequest) GetIdentity() *AuthenticatedIdentity {
	return w.identity
}

// AuthenticatedIdentity is returned by the Authenticate method on server to
// return a wrapper around the various elements that can be resolved as an
// identity. RPC handlers will use the relevant fields for performing
// authorization.
//
// Keeping these fields independent rather than merging them into an ephemeral
// ACLToken makes the original of the credential clear to RPC handlers, who may
// have different behavior for internal vs external origins.
type AuthenticatedIdentity struct {
	// ACLToken authenticated. Claims and ClientID will be unset if this is set.
	ACLToken *ACLToken

	// Claims authenticated by workload identity. ACLToken and ClientID will be
	// unset if this is set.
	Claims *IdentityClaims

	// ClientID is the Nomad client node ID. ACLToken and Claims will be nil if
	// this is set.
	ClientID string

	// TLSName is the name of the TLS certificate, if any. Outside of the
	// AuthenticateServerOnly and AuthenticateClientOnly methods, this should be
	// used only to identify the request for metrics, not authorization
	TLSName string

	// RemoteIP is the name of the connection's IP address; this should be used
	// only to identify the request for metrics, not authorization
	RemoteIP net.IP
}

func (ai *AuthenticatedIdentity) GetACLToken() *ACLToken {
	if ai == nil {
		return nil
	}
	return ai.ACLToken
}

func (ai *AuthenticatedIdentity) GetClaims() *IdentityClaims {
	if ai == nil {
		return nil
	}
	return ai.Claims
}

func (ai *AuthenticatedIdentity) String() string {
	if ai == nil {
		return "unauthenticated"
	}
	if ai.ACLToken != nil && ai.ACLToken != AnonymousACLToken {
		return "token:" + ai.ACLToken.AccessorID
	}
	if ai.Claims != nil {
		return "alloc:" + ai.Claims.AllocationID
	}
	if ai.ClientID != "" {
		return "client:" + ai.ClientID
	}
	return ai.TLSName + ":" + ai.RemoteIP.String()
}

func (ai *AuthenticatedIdentity) IsExpired(now time.Time) bool {
	// Only ACLTokens currently support expiry so return unexpired if there isn't
	// one.
	if ai.ACLToken == nil {
		return false
	}

	return ai.ACLToken.IsExpired(now)
}

type RequestWithIdentity interface {
	GetAuthToken() string
	SetIdentity(identity *AuthenticatedIdentity)
	GetIdentity() *AuthenticatedIdentity
}

// QueryMeta allows a query response to include potentially
// useful metadata about a query
type QueryMeta struct {
	// This is the index associated with the read
	Index uint64

	// If AllowStale is used, this is time elapsed since
	// last contact between the follower and leader. This
	// can be used to gauge staleness.
	LastContact time.Duration

	// Used to indicate if there is a known leader node
	KnownLeader bool

	// NextToken is the token returned with queries that support
	// paginated lists. To resume paging from this point, pass
	// this token in the next request's QueryOptions.
	NextToken string
}

// WriteMeta allows a write response to include potentially
// useful metadata about the write
type WriteMeta struct {
	// This is the index associated with the write
	Index uint64
}

// NodeRegisterRequest is used for Node.Register endpoint
// to register a node as being a schedulable entity.
type NodeRegisterRequest struct {
	Node      *Node
	NodeEvent *NodeEvent

	// CreateNodePool is used to indicate that the node's node pool should be
	// create along with the node registration if it doesn't exist.
	CreateNodePool bool

	WriteRequest
}

// NodeDeregisterRequest is used for Node.Deregister endpoint
// to deregister a node as being a schedulable entity.
type NodeDeregisterRequest struct {
	NodeID string
	WriteRequest
}

// NodeBatchDeregisterRequest is used for Node.BatchDeregister endpoint
// to deregister a batch of nodes from being schedulable entities.
type NodeBatchDeregisterRequest struct {
	NodeIDs []string
	WriteRequest
}

// NodeServerInfo is used to in NodeUpdateResponse to return Nomad server
// information used in RPC server lists.
type NodeServerInfo struct {
	// RPCAdvertiseAddr is the IP endpoint that a Nomad Server wishes to
	// be contacted at for RPCs.
	RPCAdvertiseAddr string

	// RpcMajorVersion is the major version number the Nomad Server
	// supports
	RPCMajorVersion int32

	// RpcMinorVersion is the minor version number the Nomad Server
	// supports
	RPCMinorVersion int32

	// Datacenter is the datacenter that a Nomad server belongs to
	Datacenter string
}

// NodeUpdateStatusRequest is used for Node.UpdateStatus endpoint
// to update the status of a node.
type NodeUpdateStatusRequest struct {
	NodeID    string
	Status    string
	NodeEvent *NodeEvent
	UpdatedAt int64
	WriteRequest
}

// NodeUpdateDrainRequest is used for updating the drain strategy
type NodeUpdateDrainRequest struct {
	NodeID        string
	DrainStrategy *DrainStrategy

	// MarkEligible marks the node as eligible if removing the drain strategy.
	MarkEligible bool

	// NodeEvent is the event added to the node
	NodeEvent *NodeEvent

	// UpdatedAt represents server time of receiving request
	UpdatedAt int64

	// Meta is user-provided metadata relating to the drain operation
	Meta map[string]string

	// UpdatedBy represents the AuthenticatedIdentity of the request, so that we
	// can record it in the LastDrain data without re-authenticating in the FSM.
	UpdatedBy string

	WriteRequest
}

// BatchNodeUpdateDrainRequest is used for updating the drain strategy for a
// batch of nodes
type BatchNodeUpdateDrainRequest struct {
	// Updates is a mapping of nodes to their updated drain strategy
	Updates map[string]*DrainUpdate

	// NodeEvents is a mapping of the node to the event to add to the node
	NodeEvents map[string]*NodeEvent

	// UpdatedAt represents server time of receiving request
	UpdatedAt int64

	WriteRequest
}

// DrainUpdate is used to update the drain of a node
type DrainUpdate struct {
	// DrainStrategy is the new strategy for the node
	DrainStrategy *DrainStrategy

	// MarkEligible marks the node as eligible if removing the drain strategy.
	MarkEligible bool
}

// NodeUpdateEligibilityRequest is used for updating the scheduling	eligibility
type NodeUpdateEligibilityRequest struct {
	NodeID      string
	Eligibility string

	// NodeEvent is the event added to the node
	NodeEvent *NodeEvent

	// UpdatedAt represents server time of receiving request
	UpdatedAt int64

	WriteRequest
}

// NodeEvaluateRequest is used to re-evaluate the node
type NodeEvaluateRequest struct {
	NodeID string
	WriteRequest
}

// NodeSpecificRequest is used when we just need to specify a target node
type NodeSpecificRequest struct {
	NodeID   string
	SecretID string
	QueryOptions
}

// JobRegisterRequest is used for Job.Register endpoint
// to register a job as being a schedulable entity.
type JobRegisterRequest struct {
	Submission *JobSubmission

	// Job is the parsed job, no matter what form the input was in.
	Job *Job

	// If EnforceIndex is set then the job will only be registered if the passed
	// JobModifyIndex matches the current Jobs index. If the index is zero, the
	// register only occurs if the job is new.
	EnforceIndex   bool
	JobModifyIndex uint64

	// PreserveCounts indicates that during job update, existing task group
	// counts should be preserved, over those specified in the new job spec
	// PreserveCounts is ignored for newly created jobs.
	PreserveCounts bool

	// PolicyOverride is set when the user is attempting to override any policies
	PolicyOverride bool

	// EvalPriority is an optional priority to use on any evaluation created as
	// a result on this job registration. This value must be between 1-100
	// inclusively, where a larger value corresponds to a higher priority. This
	// is useful when an operator wishes to push through a job registration in
	// busy clusters with a large evaluation backlog. This avoids needing to
	// change the job priority which also impacts preemption.
	EvalPriority int

	// Eval is the evaluation that is associated with the job registration
	Eval *Evaluation

	// Deployment is the deployment to be create when the job is registered. If
	// there is an active deployment for the job it will be canceled.
	Deployment *Deployment

	WriteRequest
}

// JobDeregisterRequest is used for Job.Deregister endpoint
// to deregister a job as being a schedulable entity.
type JobDeregisterRequest struct {
	JobID string

	// Purge controls whether the deregister purges the job from the system or
	// whether the job is just marked as stopped and will be removed by the
	// garbage collector
	Purge bool

	// Global controls whether all regions of a multi-region job are
	// deregistered. It is ignored for single-region jobs.
	Global bool

	// EvalPriority is an optional priority to use on any evaluation created as
	// a result on this job deregistration. This value must be between 1-100
	// inclusively, where a larger value corresponds to a higher priority. This
	// is useful when an operator wishes to push through a job deregistration
	// in busy clusters with a large evaluation backlog.
	EvalPriority int

	// NoShutdownDelay, if set to true, will override the group and
	// task shutdown_delay configuration and ignore the delay for any
	// allocations stopped as a result of this Deregister call.
	NoShutdownDelay bool

	// Eval is the evaluation to create that's associated with job deregister
	Eval *Evaluation

	// SubmitTime is the time at which the job was requested to be stopped
	SubmitTime int64

	WriteRequest
}

// JobEvaluateRequest is used when we just need to re-evaluate a target job
type JobEvaluateRequest struct {
	JobID       string
	EvalOptions EvalOptions
	WriteRequest
}

// EvalOptions is used to encapsulate options when forcing a job evaluation
type EvalOptions struct {
	ForceReschedule bool
}

// JobSubmissionRequest is used to query a JobSubmission object associated with a
// job at a specific version.
type JobSubmissionRequest struct {
	JobID   string
	Version uint64

	QueryOptions
}

// JobSubmissionResponse contains a JobSubmission object, which may be nil
// if no submission data is available.
type JobSubmissionResponse struct {
	Submission *JobSubmission

	QueryMeta
}

// JobSpecificRequest is used when we just need to specify a target job
type JobSpecificRequest struct {
	JobID string
	All   bool
	QueryOptions
}

// JobListRequest is used to parameterize a list request
type JobListRequest struct {
	QueryOptions
	Fields *JobStubFields
}

// Stub returns a summarized version of the job
type JobStubFields struct {
	Meta bool
}

// JobPlanRequest is used for the Job.Plan endpoint to trigger a dry-run
// evaluation of the Job.
type JobPlanRequest struct {
	Job  *Job
	Diff bool // Toggles an annotated diff
	// PolicyOverride is set when the user is attempting to override any policies
	PolicyOverride bool
	WriteRequest
}

// JobScaleRequest is used for the Job.Scale endpoint to scale one of the
// scaling targets in a job
type JobScaleRequest struct {
	JobID   string
	Target  map[string]string
	Count   *int64
	Message string
	Error   bool
	Meta    map[string]interface{}

	// PolicyOverride is set when the user is attempting to override any policies
	PolicyOverride bool

	// If EnforceIndex is set then the job will only be scaled if the passed
	// JobModifyIndex matches the current Jobs index. If the index is zero,
	// EnforceIndex is ignored.
	EnforceIndex   bool
	JobModifyIndex uint64

	WriteRequest
}

// Validate is used to validate the arguments in the request
func (r *JobScaleRequest) Validate() error {
	namespace := r.Target[ScalingTargetNamespace]
	if namespace != "" && namespace != r.RequestNamespace() {
		return NewErrRPCCoded(400, "namespace in payload did not match header")
	}

	jobID := r.Target[ScalingTargetJob]
	if jobID != "" && jobID != r.JobID {
		return fmt.Errorf("job ID in payload did not match URL")
	}

	groupName := r.Target[ScalingTargetGroup]
	if groupName == "" {
		return NewErrRPCCoded(400, "missing task group name for scaling action")
	}

	if r.Count != nil {
		if *r.Count < 0 {
			return NewErrRPCCoded(400, "scaling action count can't be negative")
		}

		if r.Error {
			return NewErrRPCCoded(400, "scaling action should not contain count if error is true")
		}

		truncCount := int(*r.Count)
		if int64(truncCount) != *r.Count {
			return NewErrRPCCoded(400,
				fmt.Sprintf("new scaling count is too large for TaskGroup.Count (int): %v", r.Count))
		}
	}

	return nil
}

// JobSummaryRequest is used when we just need to get a specific job summary
type JobSummaryRequest struct {
	JobID string
	QueryOptions
}

// JobScaleStatusRequest is used to get the scale status for a job
type JobScaleStatusRequest struct {
	JobID string
	QueryOptions
}

// JobDispatchRequest is used to dispatch a job based on a parameterized job
type JobDispatchRequest struct {
	JobID   string
	Payload []byte
	Meta    map[string]string
	WriteRequest
	IdPrefixTemplate string
}

// JobValidateRequest is used to validate a job
type JobValidateRequest struct {
	Job *Job
	WriteRequest
}

// JobRevertRequest is used to revert a job to a prior version.
type JobRevertRequest struct {
	// JobID is the ID of the job  being reverted
	JobID string

	// JobVersion the version to revert to.
	JobVersion uint64

	// EnforcePriorVersion if set will enforce that the job is at the given
	// version before reverting.
	EnforcePriorVersion *uint64

	// ConsulToken is the Consul token that proves the submitter of the job revert
	// has access to the Service Identity policies associated with the job's
	// Consul Connect enabled services. This field is only used to transfer the
	// token and is not stored after the Job revert.
	ConsulToken string

	// VaultToken is the Vault token that proves the submitter of the job revert
	// has access to any Vault policies specified in the targeted job version. This
	// field is only used to transfer the token and is not stored after the Job
	// revert.
	VaultToken string

	WriteRequest
}

// JobStabilityRequest is used to marked a job as stable.
type JobStabilityRequest struct {
	// Job to set the stability on
	JobID      string
	JobVersion uint64

	// Set the stability
	Stable bool
	WriteRequest
}

// JobStabilityResponse is the response when marking a job as stable.
type JobStabilityResponse struct {
	WriteMeta
}

// NodeListRequest is used to parameterize a list request
type NodeListRequest struct {
	QueryOptions

	Fields *NodeStubFields
}

// EvalUpdateRequest is used for upserting evaluations.
type EvalUpdateRequest struct {
	Evals     []*Evaluation
	EvalToken string
	WriteRequest
}

// EvalReapRequest is used for reaping evaluations and allocation. This struct
// is used by the Eval.Reap RPC endpoint as a request argument, and also when
// performing eval reap or deletes via Raft. This is because Eval.Reap and
// Eval.Delete use the same Raft message when performing deletes so we do not
// need more Raft message types.
type EvalReapRequest struct {
	Evals  []string // slice of Evaluation IDs
	Allocs []string // slice of Allocation IDs

	// Filter specifies the go-bexpr filter expression to be used for
	// filtering the data prior to returning a response
	Filter    string
	PerPage   int32
	NextToken string

	// UserInitiated tracks whether this reap request is the result of an
	// operator request. If this is true, the FSM needs to ensure the eval
	// broker is paused as the request can include non-terminal allocations.
	UserInitiated bool

	WriteRequest
}

// EvalSpecificRequest is used when we just need to specify a target evaluation
type EvalSpecificRequest struct {
	EvalID         string
	IncludeRelated bool
	QueryOptions
}

// EvalAckRequest is used to Ack/Nack a specific evaluation
type EvalAckRequest struct {
	EvalID string
	Token  string
	WriteRequest
}

// EvalDequeueRequest is used when we want to dequeue an evaluation
type EvalDequeueRequest struct {
	Schedulers       []string
	Timeout          time.Duration
	SchedulerVersion uint16
	WriteRequest
}

// EvalListRequest is used to list the evaluations
type EvalListRequest struct {
	FilterJobID      string
	FilterEvalStatus string
	QueryOptions
}

// ShouldBeFiltered indicates that the eval should be filtered (that
// is, removed) from the results
func (req *EvalListRequest) ShouldBeFiltered(e *Evaluation) bool {
	if req.FilterJobID != "" && req.FilterJobID != e.JobID {
		return true
	}
	if req.FilterEvalStatus != "" && req.FilterEvalStatus != e.Status {
		return true
	}
	return false
}

// EvalCountRequest is used to count evaluations
type EvalCountRequest struct {
	QueryOptions
}

// PlanRequest is used to submit an allocation plan to the leader
type PlanRequest struct {
	Plan *Plan
	WriteRequest
}

// ApplyPlanResultsRequest is used by the planner to apply a Raft transaction
// committing the result of a plan.
type ApplyPlanResultsRequest struct {
	// AllocUpdateRequest holds the allocation updates to be made by the
	// scheduler.
	AllocUpdateRequest

	// Deployment is the deployment created or updated as a result of a
	// scheduling event.
	Deployment *Deployment

	// DeploymentUpdates is a set of status updates to apply to the given
	// deployments. This allows the scheduler to cancel any unneeded deployment
	// because the job is stopped or the update block is removed.
	DeploymentUpdates []*DeploymentStatusUpdate

	// EvalID is the eval ID of the plan being applied. The modify index of the
	// evaluation is updated as part of applying the plan to ensure that subsequent
	// scheduling events for the same job will wait for the index that last produced
	// state changes. This is necessary for blocked evaluations since they can be
	// processed many times, potentially making state updates, without the state of
	// the evaluation itself being updated.
	EvalID string

	// COMPAT 0.11
	// NodePreemptions is a slice of allocations from other lower priority jobs
	// that are preempted. Preempted allocations are marked as evicted.
	// Deprecated: Replaced with AllocsPreempted which contains only the diff
	NodePreemptions []*Allocation

	// AllocsPreempted is a slice of allocation diffs from other lower priority jobs
	// that are preempted. Preempted allocations are marked as evicted.
	AllocsPreempted []*AllocationDiff

	// PreemptionEvals is a slice of follow up evals for jobs whose allocations
	// have been preempted to place allocs in this plan
	PreemptionEvals []*Evaluation

	// IneligibleNodes are nodes the plan applier has repeatedly rejected
	// placements for and should therefore be considered ineligible by workers
	// to avoid retrying them repeatedly.
	IneligibleNodes []string

	// UpdatedAt represents server time of receiving request.
	UpdatedAt int64
}

// AllocUpdateRequest is used to submit changes to allocations, either
// to cause evictions or to assign new allocations. Both can be done
// within a single transaction
type AllocUpdateRequest struct {
	// COMPAT 0.11
	// Alloc is the list of new allocations to assign
	// Deprecated: Replaced with two separate slices, one containing stopped allocations
	// and another containing updated allocations
	Alloc []*Allocation

	// Allocations to stop. Contains only the diff, not the entire allocation
	AllocsStopped []*AllocationDiff

	// New or updated allocations
	AllocsUpdated []*Allocation

	// Evals is the list of new evaluations to create
	// Evals are valid only when used in the Raft RPC
	Evals []*Evaluation

	// Job is the shared parent job of the allocations.
	// It is pulled out since it is common to reduce payload size.
	Job *Job

	WriteRequest
}

// AllocUpdateDesiredTransitionRequest is used to submit changes to allocations
// desired transition state.
type AllocUpdateDesiredTransitionRequest struct {
	// Allocs is the mapping of allocation ids to their desired state
	// transition
	Allocs map[string]*DesiredTransition

	// Evals is the set of evaluations to create
	Evals []*Evaluation

	WriteRequest
}

// AllocStopRequest is used to stop and reschedule a running Allocation.
type AllocStopRequest struct {
	AllocID         string
	NoShutdownDelay bool

	WriteRequest
}

// AllocStopResponse is the response to an `AllocStopRequest`
type AllocStopResponse struct {
	// EvalID is the id of the follow up evalution for the rescheduled alloc.
	EvalID string

	WriteMeta
}

// AllocListRequest is used to request a list of allocations
type AllocListRequest struct {
	QueryOptions

	Fields *AllocStubFields
}

// AllocSpecificRequest is used to query a specific allocation
type AllocSpecificRequest struct {
	AllocID string
	QueryOptions
}

// AllocSignalRequest is used to signal a specific allocation
type AllocSignalRequest struct {
	AllocID string
	Task    string
	Signal  string
	QueryOptions
}

// AllocPauseRequest is used to set the pause state of a task in an allocation.
type AllocPauseRequest struct {
	AllocID       string
	Task          string
	ScheduleState TaskScheduleState
	QueryOptions
}

// AllocGetPauseStateRequest is used to get the pause state of a task in an allocation.
type AllocGetPauseStateRequest struct {
	AllocID string
	Task    string
	QueryOptions
}

// AllocGetPauseStateResponse contains the pause state of a task in an allocation.
type AllocGetPauseStateResponse struct {
	ScheduleState TaskScheduleState
}

// AllocsGetRequest is used to query a set of allocations
type AllocsGetRequest struct {
	AllocIDs []string
	QueryOptions
}

// AllocRestartRequest is used to restart a specific allocations tasks.
type AllocRestartRequest struct {
	AllocID  string
	TaskName string
	AllTasks bool

	QueryOptions
}

// PeriodicForceRequest is used to force a specific periodic job.
type PeriodicForceRequest struct {
	JobID string
	WriteRequest
}

// ServerMembersResponse has the list of servers in a cluster
type ServerMembersResponse struct {
	ServerName   string
	ServerRegion string
	ServerDC     string
	Members      []*ServerMember
}

// ServerMember holds information about a Nomad server agent in a cluster
type ServerMember struct {
	Name        string
	Addr        net.IP
	Port        uint16
	Tags        map[string]string
	Status      string
	ProtocolMin uint8
	ProtocolMax uint8
	ProtocolCur uint8
	DelegateMin uint8
	DelegateMax uint8
	DelegateCur uint8
}

// ClusterMetadata is used to store per-cluster metadata.
type ClusterMetadata struct {
	ClusterID  string
	CreateTime int64
}

// DeriveVaultTokenRequest is used to request wrapped Vault tokens for the
// following tasks in the given allocation
type DeriveVaultTokenRequest struct {
	NodeID   string
	SecretID string
	AllocID  string
	Tasks    []string
	QueryOptions
}

// VaultAccessorsRequest is used to operate on a set of Vault accessors
type VaultAccessorsRequest struct {
	Accessors []*VaultAccessor
}

// VaultAccessor is a reference to a created Vault token on behalf of
// an allocation's task.
type VaultAccessor struct {
	AllocID     string
	Task        string
	NodeID      string
	Accessor    string
	CreationTTL int

	// Raft Indexes
	CreateIndex uint64
}

// DeriveVaultTokenResponse returns the wrapped tokens for each requested task
type DeriveVaultTokenResponse struct {
	// Tasks is a mapping between the task name and the wrapped token
	Tasks map[string]string

	// Error stores any error that occurred. Errors are stored here so we can
	// communicate whether it is retryable
	Error *RecoverableError

	QueryMeta
}

// GenericRequest is used to request where no
// specific information is needed.
type GenericRequest struct {
	QueryOptions
}

// DeploymentListRequest is used to list the deployments
type DeploymentListRequest struct {
	QueryOptions
}

// DeploymentDeleteRequest is used for deleting deployments.
type DeploymentDeleteRequest struct {
	Deployments []string
	WriteRequest
}

// DeploymentStatusUpdateRequest is used to update the status of a deployment as
// well as optionally creating an evaluation atomically.
type DeploymentStatusUpdateRequest struct {
	// Eval, if set, is used to create an evaluation at the same time as
	// updating the status of a deployment.
	Eval *Evaluation

	// DeploymentUpdate is a status update to apply to the given
	// deployment.
	DeploymentUpdate *DeploymentStatusUpdate

	// Job is used to optionally upsert a job. This is used when setting the
	// allocation health results in a deployment failure and the deployment
	// auto-reverts to the latest stable job.
	Job *Job
}

// DeploymentAllocHealthRequest is used to set the health of a set of
// allocations as part of a deployment.
type DeploymentAllocHealthRequest struct {
	DeploymentID string

	// Marks these allocations as healthy, allow further allocations
	// to be rolled.
	HealthyAllocationIDs []string

	// Any unhealthy allocations fail the deployment
	UnhealthyAllocationIDs []string

	WriteRequest
}

// ApplyDeploymentAllocHealthRequest is used to apply an alloc health request via Raft
type ApplyDeploymentAllocHealthRequest struct {
	DeploymentAllocHealthRequest

	// Timestamp is the timestamp to use when setting the allocations health.
	Timestamp time.Time

	// An optional field to update the status of a deployment
	DeploymentUpdate *DeploymentStatusUpdate

	// Job is used to optionally upsert a job. This is used when setting the
	// allocation health results in a deployment failure and the deployment
	// auto-reverts to the latest stable job.
	Job *Job

	// An optional evaluation to create after promoting the canaries
	Eval *Evaluation
}

// DeploymentPromoteRequest is used to promote task groups in a deployment
type DeploymentPromoteRequest struct {
	DeploymentID string

	// All is to promote all task groups
	All bool

	// Groups is used to set the promotion status per task group
	Groups []string

	WriteRequest
}

// ApplyDeploymentPromoteRequest is used to apply a promotion request via Raft
type ApplyDeploymentPromoteRequest struct {
	DeploymentPromoteRequest

	// An optional evaluation to create after promoting the canaries
	Eval *Evaluation
}

// DeploymentPauseRequest is used to pause a deployment
type DeploymentPauseRequest struct {
	DeploymentID string

	// Pause sets the pause status
	Pause bool

	WriteRequest
}

// DeploymentRunRequest is used to remotely start a pending deployment.
// Used only for multiregion deployments.
type DeploymentRunRequest struct {
	DeploymentID string

	WriteRequest
}

// DeploymentUnblockRequest is used to remotely unblock a deployment.
// Used only for multiregion deployments.
type DeploymentUnblockRequest struct {
	DeploymentID string

	WriteRequest
}

// DeploymentCancelRequest is used to remotely cancel a deployment.
// Used only for multiregion deployments.
type DeploymentCancelRequest struct {
	DeploymentID string

	WriteRequest
}

// DeploymentSpecificRequest is used to make a request specific to a particular
// deployment
type DeploymentSpecificRequest struct {
	DeploymentID string
	QueryOptions
}

// DeploymentFailRequest is used to fail a particular deployment
type DeploymentFailRequest struct {
	DeploymentID string
	WriteRequest
}

// ScalingPolicySpecificRequest is used when we just need to specify a target scaling policy
type ScalingPolicySpecificRequest struct {
	ID string
	QueryOptions
}

// SingleScalingPolicyResponse is used to return a single job
type SingleScalingPolicyResponse struct {
	Policy *ScalingPolicy
	QueryMeta
}

// ScalingPolicyListRequest is used to parameterize a scaling policy list request
type ScalingPolicyListRequest struct {
	Job  string
	Type string
	QueryOptions
}

// ScalingPolicyListResponse is used for a list request
type ScalingPolicyListResponse struct {
	Policies []*ScalingPolicyListStub
	QueryMeta
}

// SingleDeploymentResponse is used to respond with a single deployment
type SingleDeploymentResponse struct {
	Deployment *Deployment
	QueryMeta
}

// GenericResponse is used to respond to a request where no
// specific response information is needed.
type GenericResponse struct {
	WriteMeta
}

// VersionResponse is used for the Status.Version response
type VersionResponse struct {
	Build    string
	Versions map[string]int
	QueryMeta
}

// JobRegisterResponse is used to respond to a job registration
type JobRegisterResponse struct {
	EvalID          string
	EvalCreateIndex uint64
	JobModifyIndex  uint64

	// Warnings contains any warnings about the given job. These may include
	// deprecation warnings.
	Warnings string

	QueryMeta
}

// JobDeregisterResponse is used to respond to a job deregistration
type JobDeregisterResponse struct {
	EvalID          string
	EvalCreateIndex uint64
	JobModifyIndex  uint64
	VolumeEvalID    string
	VolumeEvalIndex uint64
	QueryMeta
}

// JobValidateResponse is the response from validate request
type JobValidateResponse struct {
	// DriverConfigValidated indicates whether the agent validated the driver
	// config
	DriverConfigValidated bool

	// ValidationErrors is a list of validation errors
	ValidationErrors []string

	// Error is a string version of any error that may have occurred
	Error string

	// Warnings contains any warnings about the given job. These may include
	// deprecation warnings.
	Warnings string
}

// NodeUpdateResponse is used to respond to a node update
type NodeUpdateResponse struct {
	HeartbeatTTL    time.Duration
	EvalIDs         []string
	EvalCreateIndex uint64
	NodeModifyIndex uint64

	// Features informs clients what enterprise features are allowed
	Features uint64

	// LeaderRPCAddr is the RPC address of the current Raft Leader.  If
	// empty, the current Nomad Server is in the minority of a partition.
	LeaderRPCAddr string

	// NumNodes is the number of Nomad nodes attached to this quorum of
	// Nomad Servers at the time of the response.  This value can
	// fluctuate based on the health of the cluster between heartbeats.
	NumNodes int32

	// Servers is the full list of known Nomad servers in the local
	// region.
	Servers []*NodeServerInfo

	// SchedulingEligibility is used to inform clients what the server-side
	// has for their scheduling status during heartbeats.
	SchedulingEligibility string

	QueryMeta
}

// NodeDrainUpdateResponse is used to respond to a node drain update
type NodeDrainUpdateResponse struct {
	NodeModifyIndex uint64
	EvalIDs         []string
	EvalCreateIndex uint64
	WriteMeta
}

// NodeEligibilityUpdateResponse is used to respond to a node eligibility update
type NodeEligibilityUpdateResponse struct {
	NodeModifyIndex uint64
	EvalIDs         []string
	EvalCreateIndex uint64
	WriteMeta
}

// NodeAllocsResponse is used to return allocs for a single node
type NodeAllocsResponse struct {
	Allocs []*Allocation
	QueryMeta
}

// NodeClientAllocsResponse is used to return allocs meta data for a single node
type NodeClientAllocsResponse struct {
	Allocs map[string]uint64

	// MigrateTokens are used when ACLs are enabled to allow cross node,
	// authenticated access to sticky volumes
	MigrateTokens map[string]string

	QueryMeta
}

// SingleNodeResponse is used to return a single node
type SingleNodeResponse struct {
	Node *Node
	QueryMeta
}

// NodeListResponse is used for a list request
type NodeListResponse struct {
	Nodes []*NodeListStub
	QueryMeta
}

// SingleJobResponse is used to return a single job
type SingleJobResponse struct {
	Job *Job
	QueryMeta
}

// JobSummaryResponse is used to return a single job summary
type JobSummaryResponse struct {
	JobSummary *JobSummary
	QueryMeta
}

// JobScaleStatusResponse is used to return the scale status for a job
type JobScaleStatusResponse struct {
	JobScaleStatus *JobScaleStatus
	QueryMeta
}

type JobScaleStatus struct {
	JobID          string
	Namespace      string
	JobCreateIndex uint64
	JobModifyIndex uint64
	JobStopped     bool
	TaskGroups     map[string]*TaskGroupScaleStatus
}

// TaskGroupScaleStatus is used to return the scale status for a given task group
type TaskGroupScaleStatus struct {
	Desired   int
	Placed    int
	Running   int
	Healthy   int
	Unhealthy int
	Events    []*ScalingEvent
}

type JobDispatchResponse struct {
	DispatchedJobID string
	EvalID          string
	EvalCreateIndex uint64
	JobCreateIndex  uint64
	WriteMeta
}

// JobListResponse is used for a list request
type JobListResponse struct {
	Jobs []*JobListStub
	QueryMeta
}

// JobVersionsRequest is used to get a jobs versions
type JobVersionsRequest struct {
	JobID       string
	Diffs       bool
	DiffVersion *uint64
	DiffTagName string
	QueryOptions
}

// JobVersionsResponse is used for a job get versions request
type JobVersionsResponse struct {
	Versions []*Job
	Diffs    []*JobDiff
	QueryMeta
}

// JobPlanResponse is used to respond to a job plan request
type JobPlanResponse struct {
	// Annotations stores annotations explaining decisions the scheduler made.
	Annotations *PlanAnnotations

	// FailedTGAllocs is the placement failures per task group.
	FailedTGAllocs map[string]*AllocMetric

	// JobModifyIndex is the modification index of the job. The value can be
	// used when running `nomad run` to ensure that the Job wasn’t modified
	// since the last plan. If the job is being created, the value is zero.
	JobModifyIndex uint64

	// CreatedEvals is the set of evaluations created by the scheduler. The
	// reasons for this can be rolling-updates or blocked evals.
	CreatedEvals []*Evaluation

	// Diff contains the diff of the job and annotations on whether the change
	// causes an in-place update or create/destroy
	Diff *JobDiff

	// NextPeriodicLaunch is the time duration till the job would be launched if
	// submitted.
	NextPeriodicLaunch time.Time

	// Warnings contains any warnings about the given job. These may include
	// deprecation warnings.
	Warnings string

	WriteMeta
}

// SingleAllocResponse is used to return a single allocation
type SingleAllocResponse struct {
	Alloc *Allocation
	QueryMeta
}

// AllocsGetResponse is used to return a set of allocations and their workload
// identities.
type AllocsGetResponse struct {
	Allocs []*Allocation

	// SignedIdentities are the alternate workload identities for the Allocs.
	SignedIdentities []SignedWorkloadIdentity

	QueryMeta
}

// JobAllocationsResponse is used to return the allocations for a job
type JobAllocationsResponse struct {
	Allocations []*AllocListStub
	QueryMeta
}

// JobEvaluationsResponse is used to return the evaluations for a job
type JobEvaluationsResponse struct {
	Evaluations []*Evaluation
	QueryMeta
}

// SingleEvalResponse is used to return a single evaluation
type SingleEvalResponse struct {
	Eval *Evaluation
	QueryMeta
}

// EvalDequeueResponse is used to return from a dequeue
type EvalDequeueResponse struct {
	Eval  *Evaluation
	Token string

	// WaitIndex is the Raft index the worker should wait until invoking the
	// scheduler.
	WaitIndex uint64

	QueryMeta
}

// GetWaitIndex is used to retrieve the Raft index in which state should be at
// or beyond before invoking the scheduler.
func (e *EvalDequeueResponse) GetWaitIndex() uint64 {
	// Prefer the wait index sent. This will be populated on all responses from
	// 0.7.0 and above
	if e.WaitIndex != 0 {
		return e.WaitIndex
	} else if e.Eval != nil {
		return e.Eval.ModifyIndex
	}

	// This should never happen
	return 1
}

// PlanResponse is used to return from a PlanRequest
type PlanResponse struct {
	Result *PlanResult
	WriteMeta
}

// AllocListResponse is used for a list request
type AllocListResponse struct {
	Allocations []*AllocListStub
	QueryMeta
}

// DeploymentListResponse is used for a list request
type DeploymentListResponse struct {
	Deployments []*Deployment
	QueryMeta
}

// EvalListResponse is used for a list request
type EvalListResponse struct {
	Evaluations []*Evaluation
	QueryMeta
}

// EvalCountResponse is used for a count request
type EvalCountResponse struct {
	Count int
	QueryMeta
}

// EvalAllocationsResponse is used to return the allocations for an evaluation
type EvalAllocationsResponse struct {
	Allocations []*AllocListStub
	QueryMeta
}

// PeriodicForceResponse is used to respond to a periodic job force launch
type PeriodicForceResponse struct {
	EvalID          string
	EvalCreateIndex uint64
	WriteMeta
}

// DeploymentUpdateResponse is used to respond to a deployment change. The
// response will include the modify index of the deployment as well as details
// of any triggered evaluation.
type DeploymentUpdateResponse struct {
	EvalID                string
	EvalCreateIndex       uint64
	DeploymentModifyIndex uint64

	// RevertedJobVersion is the version the job was reverted to. If unset, the
	// job wasn't reverted
	RevertedJobVersion *uint64

	WriteMeta
}

// NodeConnQueryResponse is used to respond to a query of whether a server has
// a connection to a specific Node
type NodeConnQueryResponse struct {
	// Connected indicates whether a connection to the Client exists
	Connected bool

	// Established marks the time at which the connection was established
	Established time.Time

	QueryMeta
}

// HostDataRequest is used by /agent/host to retrieve data about the agent's host system. If
// ServerID or NodeID is specified, the request is forwarded to the remote agent
type HostDataRequest struct {
	ServerID string
	NodeID   string
	QueryOptions
}

// HostDataResponse contains the HostData content
type HostDataResponse struct {
	AgentID  string
	HostData *host.HostData
}

// EmitNodeEventsRequest is a request to update the node events source
// with a new client-side event
type EmitNodeEventsRequest struct {
	// NodeEvents are a map where the key is a node id, and value is a list of
	// events for that node
	NodeEvents map[string][]*NodeEvent

	WriteRequest
}

// EmitNodeEventsResponse is a response to the client about the status of
// the node event source update.
type EmitNodeEventsResponse struct {
	WriteMeta
}

const (
	NodeEventSubsystemDrain     = "Drain"
	NodeEventSubsystemDriver    = "Driver"
	NodeEventSubsystemHeartbeat = "Heartbeat"
	NodeEventSubsystemCluster   = "Cluster"
	NodeEventSubsystemScheduler = "Scheduler"
	NodeEventSubsystemStorage   = "Storage"
)

// NodeEvent is a single unit representing a node’s state change
type NodeEvent struct {
	Message     string
	Subsystem   string
	Details     map[string]string
	Timestamp   time.Time
	CreateIndex uint64
}

func (ne *NodeEvent) String() string {
	var details []string
	for k, v := range ne.Details {
		details = append(details, fmt.Sprintf("%s: %s", k, v))
	}

	return fmt.Sprintf("Message: %s, Subsystem: %s, Details: %s, Timestamp: %s", ne.Message, ne.Subsystem, strings.Join(details, ","), ne.Timestamp.String())
}

func (ne *NodeEvent) Copy() *NodeEvent {
	c := new(NodeEvent)
	*c = *ne
	c.Details = maps.Clone(ne.Details)
	return c
}

// NewNodeEvent generates a new node event storing the current time as the
// timestamp
func NewNodeEvent() *NodeEvent {
	return &NodeEvent{Timestamp: time.Now()}
}

// SetMessage is used to set the message on the node event
func (ne *NodeEvent) SetMessage(msg string) *NodeEvent {
	ne.Message = msg
	return ne
}

// SetSubsystem is used to set the subsystem on the node event
func (ne *NodeEvent) SetSubsystem(sys string) *NodeEvent {
	ne.Subsystem = sys
	return ne
}

// SetTimestamp is used to set the timestamp on the node event
func (ne *NodeEvent) SetTimestamp(ts time.Time) *NodeEvent {
	ne.Timestamp = ts
	return ne
}

// AddDetail is used to add a detail to the node event
func (ne *NodeEvent) AddDetail(k, v string) *NodeEvent {
	if ne.Details == nil {
		ne.Details = make(map[string]string, 1)
	}
	ne.Details[k] = v
	return ne
}

const (
	NodeStatusInit         = "initializing"
	NodeStatusReady        = "ready"
	NodeStatusDown         = "down"
	NodeStatusDisconnected = "disconnected"
)

// ShouldDrainNode checks if a given node status should trigger an
// evaluation. Some states don't require any further action.
func ShouldDrainNode(status string) bool {
	switch status {
	case NodeStatusInit, NodeStatusReady, NodeStatusDisconnected:
		return false
	case NodeStatusDown:
		return true
	default:
		panic(fmt.Sprintf("unhandled node status %s", status))
	}
}

// ValidNodeStatus is used to check if a node status is valid
func ValidNodeStatus(status string) bool {
	switch status {
	case NodeStatusInit, NodeStatusReady, NodeStatusDown, NodeStatusDisconnected:
		return true
	default:
		return false
	}
}

const (
	// NodeSchedulingEligible and Ineligible marks the node as eligible or not,
	// respectively, for receiving allocations. This is orthogonal to the node
	// status being ready.
	NodeSchedulingEligible   = "eligible"
	NodeSchedulingIneligible = "ineligible"
)

// DrainSpec describes a Node's desired drain behavior.
type DrainSpec struct {
	// Deadline is the duration after StartTime when the remaining
	// allocations on a draining Node should be told to stop.
	Deadline time.Duration

	// IgnoreSystemJobs allows systems jobs to remain on the node even though it
	// has been marked for draining.
	IgnoreSystemJobs bool
}

// DrainStrategy describes a Node's drain behavior.
type DrainStrategy struct {
	// DrainSpec is the user declared drain specification
	DrainSpec

	// ForceDeadline is the deadline time for the drain after which drains will
	// be forced
	ForceDeadline time.Time

	// StartedAt is the time the drain process started
	StartedAt time.Time
}

func (d *DrainStrategy) Copy() *DrainStrategy {
	if d == nil {
		return nil
	}

	nd := new(DrainStrategy)
	*nd = *d
	return nd
}

// DeadlineTime returns a boolean whether the drain strategy allows an infinite
// duration or otherwise the deadline time. The force drain is captured by the
// deadline time being in the past.
func (d *DrainStrategy) DeadlineTime() (infinite bool, deadline time.Time) {
	// Treat the nil case as a force drain so during an upgrade where a node may
	// not have a drain strategy but has Drain set to true, it is treated as a
	// force to mimick old behavior.
	if d == nil {
		return false, time.Time{}
	}

	ns := d.Deadline.Nanoseconds()
	switch {
	case ns < 0: // Force
		return false, time.Time{}
	case ns == 0: // Infinite
		return true, time.Time{}
	default:
		return false, d.ForceDeadline
	}
}

func (d *DrainStrategy) Equal(o *DrainStrategy) bool {
	if d == nil && o == nil {
		return true
	} else if o != nil && d == nil {
		return false
	} else if d != nil && o == nil {
		return false
	}

	// Compare values
	if d.ForceDeadline != o.ForceDeadline {
		return false
	} else if d.Deadline != o.Deadline {
		return false
	} else if d.IgnoreSystemJobs != o.IgnoreSystemJobs {
		return false
	}

	return true
}

const (
	// DrainStatuses are the various states a drain can be in, as reflect in DrainMetadata
	DrainStatusDraining DrainStatus = "draining"
	DrainStatusComplete DrainStatus = "complete"
	DrainStatusCanceled DrainStatus = "canceled"
)

type DrainStatus string

// DrainMetadata contains information about the most recent drain operation for a given Node.
type DrainMetadata struct {
	// StartedAt is the time that the drain operation started. This is equal to Node.DrainStrategy.StartedAt,
	// if it exists
	StartedAt time.Time

	// UpdatedAt is the time that that this struct was most recently updated, either via API action
	// or drain completion
	UpdatedAt time.Time

	// Status reflects the status of the drain operation.
	Status DrainStatus

	// AccessorID is the accessor ID of the ACL token used in the most recent API operation against this drain
	AccessorID string

	// Meta includes the operator-submitted metadata about this drain operation
	Meta map[string]string
}

func (m *DrainMetadata) Copy() *DrainMetadata {
	if m == nil {
		return nil
	}
	c := new(DrainMetadata)
	*c = *m
	c.Meta = maps.Clone(m.Meta)
	return c
}

// Node is a representation of a schedulable client node
type Node struct {
	// ID is a unique identifier for the node. It can be constructed
	// by doing a concatenation of the Name and Datacenter as a simple
	// approach. Alternatively a UUID may be used.
	ID string

	// SecretID is an ID that is only known by the Node and the set of Servers.
	// It is not accessible via the API and is used to authenticate nodes
	// conducting privileged activities.
	SecretID string

	// Datacenter for this node
	Datacenter string

	// Node name
	Name string

	// CgroupParent for this node (linux only)
	CgroupParent string

	// HTTPAddr is the address on which the Nomad client is listening for http
	// requests
	HTTPAddr string

	// TLSEnabled indicates if the Agent has TLS enabled for the HTTP API
	TLSEnabled bool

	// Attributes is an arbitrary set of key/value
	// data that can be used for constraints. Examples
	// include "kernel.name=linux", "arch=386", "driver.docker=1",
	// "docker.runtime=1.8.3"
	Attributes map[string]string

	// NodeResources captures the available resources on the client.
	NodeResources *NodeResources

	// ReservedResources captures the set resources on the client that are
	// reserved from scheduling.
	ReservedResources *NodeReservedResources

	// Resources is the available resources on the client.
	// For example 'cpu=2' 'memory=2048'
	// COMPAT(0.10): Remove after 0.10
	Resources *Resources

	// Reserved is the set of resources that are reserved,
	// and should be subtracted from the total resources for
	// the purposes of scheduling. This may be provide certain
	// high-watermark tolerances or because of external schedulers
	// consuming resources.
	// COMPAT(0.10): Remove after 0.10
	Reserved *Resources

	// Links are used to 'link' this client to external
	// systems. For example 'consul=foo.dc1' 'aws=i-83212'
	// 'ami=ami-123'
	Links map[string]string

	// Meta is used to associate arbitrary metadata with this
	// client. This is opaque to Nomad.
	Meta map[string]string

	// NodeClass is an opaque identifier used to group nodes
	// together for the purpose of determining scheduling pressure.
	NodeClass string

	// NodePool is the node pool the node belongs to.
	NodePool string

	// ComputedClass is a unique id that identifies nodes with a common set of
	// attributes and capabilities.
	ComputedClass string

	// DrainStrategy determines the node's draining behavior.
	// Will be non-nil only while draining.
	DrainStrategy *DrainStrategy

	// SchedulingEligibility determines whether this node will receive new
	// placements.
	SchedulingEligibility string

	// Status of this node
	Status string

	// StatusDescription is meant to provide more human useful information
	StatusDescription string

	// StatusUpdatedAt is the time stamp at which the state of the node was
	// updated
	StatusUpdatedAt int64

	// Events is the most recent set of events generated for the node,
	// retaining only MaxRetainedNodeEvents number at a time
	Events []*NodeEvent

	// Drivers is a map of driver names to current driver information
	Drivers map[string]*DriverInfo

	// CSIControllerPlugins is a map of plugin names to current CSI Plugin info
	CSIControllerPlugins map[string]*CSIInfo
	// CSINodePlugins is a map of plugin names to current CSI Plugin info
	CSINodePlugins map[string]*CSIInfo

	// HostVolumes is a map of host volume names to their configuration
	HostVolumes map[string]*ClientHostVolumeConfig

	// HostNetworks is a map of host host_network names to their configuration
	HostNetworks map[string]*ClientHostNetworkConfig

	// LastDrain contains metadata about the most recent drain operation
	LastDrain *DrainMetadata

	// LastMissedHeartbeatIndex stores the Raft index when the node last missed
	// a heartbeat. It resets to zero once the node is marked as ready again.
	LastMissedHeartbeatIndex uint64

	// LastAllocUpdateIndex stores the Raft index of the last time the node
	// updatedd its allocations status.
	LastAllocUpdateIndex uint64

	// Raft Indexes
	CreateIndex uint64
	ModifyIndex uint64
}

// GetID is a helper for getting the ID when the object may be nil and is
// required for pagination.
func (n *Node) GetID() string {
	if n == nil {
		return ""
	}
	return n.ID
}

// Sanitize returns a copy of the Node omitting confidential fields
// It only returns a copy if the Node contains the confidential fields
func (n *Node) Sanitize() *Node {
	if n == nil {
		return nil
	}
	if n.SecretID == "" {
		return n
	}
	clean := n.Copy()
	clean.SecretID = ""
	return clean
}

// Ready returns true if the node is ready for running allocations
func (n *Node) Ready() bool {
	return n.Status == NodeStatusReady && n.DrainStrategy == nil && n.SchedulingEligibility == NodeSchedulingEligible
}

func (n *Node) Canonicalize() {
	if n == nil {
		return
	}

	if n.NodePool == "" {
		n.NodePool = NodePoolDefault
	}

	// Ensure SchedulingEligibility is correctly set whenever draining so the plan applier and other scheduling logic
	// only need to check SchedulingEligibility when determining whether a placement is feasible on a node.
	if n.DrainStrategy != nil {
		n.SchedulingEligibility = NodeSchedulingIneligible
	} else if n.SchedulingEligibility == "" {
		n.SchedulingEligibility = NodeSchedulingEligible
	}

	// COMPAT remove in 1.10+
	// In v1.7 we introduce Topology into the NodeResources struct which the client
	// will fingerprint. Since the upgrade path must cover servers that get upgraded
	// before clients which will send the old struct, we synthesize a pseudo topology
	// given the old struct data.
	n.NodeResources.Compatibility()

	// COMPAT remove in 1.0
	// In v0.12.0 we introduced a separate node specific network resource struct
	// so we need to covert any pre 0.12 clients to the correct struct
	if n.NodeResources != nil && n.NodeResources.NodeNetworks == nil {
		if n.NodeResources.Networks != nil {
			for _, nr := range n.NodeResources.Networks {
				nnr := &NodeNetworkResource{
					Mode:   nr.Mode,
					Speed:  nr.MBits,
					Device: nr.Device,
				}
				if nr.IP != "" {
					nnr.Addresses = []NodeNetworkAddress{
						{
							Alias:   "default",
							Address: nr.IP,
						},
					}
				}
				n.NodeResources.NodeNetworks = append(n.NodeResources.NodeNetworks, nnr)
			}
		}
	}
}

func (n *Node) Copy() *Node {
	if n == nil {
		return nil
	}
	nn := *n
	nn.Attributes = maps.Clone(nn.Attributes)
	nn.NodeResources = nn.NodeResources.Copy()
	nn.ReservedResources = nn.ReservedResources.Copy()
	nn.Resources = nn.Resources.Copy()
	nn.Reserved = nn.Reserved.Copy()
	nn.Links = maps.Clone(nn.Links)
	nn.Meta = maps.Clone(nn.Meta)
	nn.DrainStrategy = nn.DrainStrategy.Copy()
	nn.Events = helper.CopySlice(n.Events)
	nn.Drivers = helper.DeepCopyMap(n.Drivers)
	nn.CSIControllerPlugins = helper.DeepCopyMap(nn.CSIControllerPlugins)
	nn.CSINodePlugins = helper.DeepCopyMap(nn.CSINodePlugins)
	nn.HostVolumes = helper.DeepCopyMap(n.HostVolumes)
	nn.HostNetworks = helper.DeepCopyMap(n.HostNetworks)
	nn.LastDrain = nn.LastDrain.Copy()
	return &nn
}

// UnresponsiveStatus returns true if the node is a status where it is not
// communicating with the server.
func (n *Node) UnresponsiveStatus() bool {
	switch n.Status {
	case NodeStatusDown, NodeStatusDisconnected:
		return true
	default:
		return false
	}
}

// TerminalStatus returns if the current status is terminal and
// will no longer transition.
func (n *Node) TerminalStatus() bool {
	switch n.Status {
	case NodeStatusDown:
		return true
	default:
		return false
	}
}

func (n *Node) IsInAnyDC(datacenters []string) bool {
	for _, dc := range datacenters {
		if glob.Glob(dc, n.Datacenter) {
			return true
		}
	}
	return false
}

// IsInPool returns true if the node is in the pool argument or if the pool
// argument is the special "all" pool
func (n *Node) IsInPool(pool string) bool {
	return pool == NodePoolAll || n.NodePool == pool
}

// HasEvent returns true if the node has the given message in its events list.
func (n *Node) HasEvent(msg string) bool {
	for _, ev := range n.Events {
		if ev.Message == msg {
			return true
		}
	}
	return false
}

// Stub returns a summarized version of the node
func (n *Node) Stub(fields *NodeStubFields) *NodeListStub {

	addr, _, _ := net.SplitHostPort(n.HTTPAddr)

	s := &NodeListStub{
		Address:               addr,
		ID:                    n.ID,
		Datacenter:            n.Datacenter,
		Name:                  n.Name,
		NodeClass:             n.NodeClass,
		NodePool:              n.NodePool,
		Version:               n.Attributes["nomad.version"],
		Drain:                 n.DrainStrategy != nil,
		SchedulingEligibility: n.SchedulingEligibility,
		Status:                n.Status,
		StatusDescription:     n.StatusDescription,
		Drivers:               n.Drivers,
		HostVolumes:           n.HostVolumes,
		LastDrain:             n.LastDrain,
		CreateIndex:           n.CreateIndex,
		ModifyIndex:           n.ModifyIndex,
	}

	if fields != nil {
		if fields.Resources {
			s.NodeResources = n.NodeResources
			s.ReservedResources = n.ReservedResources
		}

		// Fetch key attributes from the main Attributes map.
		if fields.OS {
			m := make(map[string]string)
			m["os.name"] = n.Attributes["os.name"]
			s.Attributes = m
		}
	}

	return s
}

// NodeListStub is used to return a subset of job information
// for the job list
type NodeListStub struct {
	Address               string
	ID                    string
	Attributes            map[string]string `json:",omitempty"`
	Datacenter            string
	Name                  string
	NodePool              string
	NodeClass             string
	Version               string
	Drain                 bool
	SchedulingEligibility string
	Status                string
	StatusDescription     string
	Drivers               map[string]*DriverInfo
	HostVolumes           map[string]*ClientHostVolumeConfig
	NodeResources         *NodeResources         `json:",omitempty"`
	ReservedResources     *NodeReservedResources `json:",omitempty"`
	LastDrain             *DrainMetadata
	CreateIndex           uint64
	ModifyIndex           uint64
}

// NodeStubFields defines which fields are included in the NodeListStub.
type NodeStubFields struct {
	Resources bool
	OS        bool
}

// Resources is used to define the resources available
// on a client
type Resources struct {
	CPU         int
	Cores       int
	MemoryMB    int
	MemoryMaxMB int
	DiskMB      int
	IOPS        int // COMPAT(0.10): Only being used to issue warnings
	Networks    Networks
	Devices     ResourceDevices
	NUMA        *NUMA
	SecretsMB   int
}

const (
	BytesInMegabyte = 1024 * 1024
)

// DefaultResources is a small resources object that contains the
// default resources requests that we will provide to an object.
// ---  THIS FUNCTION IS REPLICATED IN api/resources.go and should
// be kept in sync.
func DefaultResources() *Resources {
	return &Resources{
		CPU:      100,
		Cores:    0,
		MemoryMB: 300,
	}
}

// MinResources is a small resources object that contains the
// absolute minimum resources that we will provide to an object.
// This should not be confused with the defaults which are
// provided in Canonicalize() ---  THIS FUNCTION IS REPLICATED IN
// api/resources.go and should be kept in sync.
func MinResources() *Resources {
	return &Resources{
		CPU:      1,
		Cores:    0,
		MemoryMB: 10,
	}
}

// DiskInBytes returns the amount of disk resources in bytes.
func (r *Resources) DiskInBytes() int64 {
	return int64(r.DiskMB * BytesInMegabyte)
}

const (
	// memoryNoLimit is a sentinel value indicating there is no upper hard
	// memory limit
	memoryNoLimit = -1
)

func (r *Resources) Validate() error {
	var mErr multierror.Error

	if r.Cores > 0 && r.CPU > 0 {
		mErr.Errors = append(mErr.Errors, errors.New("Task can only ask for 'cpu' or 'cores' resource, not both."))
	}

	if err := r.MeetsMinResources(); err != nil {
		mErr.Errors = append(mErr.Errors, err)
	}

	// Ensure the task isn't asking for disk resources
	if r.DiskMB > 0 {
		mErr.Errors = append(mErr.Errors, errors.New("Task can't ask for disk resources, they have to be specified at the task group level."))
	}

	// Ensure devices are valid
	devices := set.New[string](len(r.Devices))
	for i, d := range r.Devices {
		if err := d.Validate(); err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("device %d failed validation: %v", i+1, err))
		}
		devices.Insert(d.Name)
	}

	// Ensure each numa bound device matches a device requested for task
	if r.NUMA != nil {
		for _, numaDevice := range r.NUMA.Devices {
			if !devices.Contains(numaDevice) {
				mErr.Errors = append(mErr.Errors, fmt.Errorf("numa device %q not requested as task resource", numaDevice))
			}
		}
	}

	// Ensure the numa block is valid
	if err := r.NUMA.Validate(); err != nil {
		mErr.Errors = append(mErr.Errors, err)
	}

	// Ensure memory_max is greater than memory, unless it is set to 0 or -1 which
	// are both sentinel values
	if (r.MemoryMaxMB != 0 && r.MemoryMaxMB != memoryNoLimit) && r.MemoryMaxMB < r.MemoryMB {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("MemoryMaxMB value (%d) should be larger than MemoryMB value (%d)", r.MemoryMaxMB, r.MemoryMB))
	}

	if r.SecretsMB > r.MemoryMB {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("SecretsMB value (%d) cannot be larger than MemoryMB value (%d)", r.SecretsMB, r.MemoryMB))
	}
	if r.SecretsMB < 0 {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("SecretsMB value (%d) cannot be negative", r.SecretsMB))
	}

	return mErr.ErrorOrNil()
}

// Merge merges this resource with another resource.
// COMPAT(0.10): Remove in 0.10
func (r *Resources) Merge(other *Resources) {
	if other.CPU != 0 {
		r.CPU = other.CPU
	}
	if other.Cores != 0 {
		r.Cores = other.Cores
	}
	if other.MemoryMB != 0 {
		r.MemoryMB = other.MemoryMB
	}
	if other.MemoryMaxMB != 0 {
		r.MemoryMaxMB = other.MemoryMaxMB
	}
	if other.DiskMB != 0 {
		r.DiskMB = other.DiskMB
	}
	if len(other.Networks) != 0 {
		r.Networks = other.Networks
	}
	if len(other.Devices) != 0 {
		r.Devices = other.Devices
	}
	if other.SecretsMB != 0 {
		r.SecretsMB = other.SecretsMB
	}
}

// Equal Resources.
//
// COMPAT(0.10): Remove in 0.10
func (r *Resources) Equal(o *Resources) bool {
	if r == o {
		return true
	}
	if r == nil || o == nil {
		return false
	}
	return r.CPU == o.CPU &&
		r.Cores == o.Cores &&
		r.MemoryMB == o.MemoryMB &&
		r.MemoryMaxMB == o.MemoryMaxMB &&
		r.DiskMB == o.DiskMB &&
		r.IOPS == o.IOPS &&
		r.Networks.Equal(&o.Networks) &&
		r.Devices.Equal(&o.Devices) &&
		r.SecretsMB == o.SecretsMB
}

// ResourceDevices are part of Resources.
//
// COMPAT(0.10): Remove in 0.10.
type ResourceDevices []*RequestedDevice

// Copy ResourceDevices
//
// COMPAT(0.10): Remove in 0.10.
func (d ResourceDevices) Copy() ResourceDevices {
	if d == nil {
		return nil
	}
	c := make(ResourceDevices, len(d))
	for i, device := range d {
		c[i] = device.Copy()
	}
	return c
}

// Equal ResourceDevices as set keyed by Name.
//
// COMPAT(0.10): Remove in 0.10
func (d *ResourceDevices) Equal(o *ResourceDevices) bool {
	if d == o {
		return true
	}
	if d == nil || o == nil {
		return false
	}
	if len(*d) != len(*o) {
		return false
	}
	m := make(map[string]*RequestedDevice, len(*d))
	for _, e := range *d {
		m[e.Name] = e
	}
	for _, oe := range *o {
		de, ok := m[oe.Name]
		if !ok || !de.Equal(oe) {
			return false
		}
	}
	return true
}

// Canonicalize the Resources struct.
//
// COMPAT(0.10): Remove in 0.10
func (r *Resources) Canonicalize() {
	// Ensure that an empty and nil slices are treated the same to avoid scheduling
	// problems since we use reflect DeepEquals.
	if len(r.Networks) == 0 {
		r.Networks = nil
	}
	if len(r.Devices) == 0 {
		r.Devices = nil
	}

	for _, n := range r.Networks {
		n.Canonicalize()
	}

	r.NUMA.Canonicalize()
}

// MeetsMinResources returns an error if the resources specified are less than
// the minimum allowed.
// This is based on the minimums defined in the Resources type
// COMPAT(0.10): Remove in 0.10
func (r *Resources) MeetsMinResources() error {
	var mErr multierror.Error
	minResources := MinResources()
	if r.CPU < minResources.CPU && r.Cores == 0 {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("minimum CPU value is %d; got %d", minResources.CPU, r.CPU))
	}
	if r.MemoryMB < minResources.MemoryMB {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("minimum MemoryMB value is %d; got %d", minResources.MemoryMB, r.MemoryMB))
	}
	return mErr.ErrorOrNil()
}

// Copy returns a deep copy of the resources
func (r *Resources) Copy() *Resources {
	if r == nil {
		return nil
	}
	return &Resources{
		CPU:         r.CPU,
		Cores:       r.Cores,
		MemoryMB:    r.MemoryMB,
		MemoryMaxMB: r.MemoryMaxMB,
		DiskMB:      r.DiskMB,
		IOPS:        r.IOPS,
		Networks:    r.Networks.Copy(),
		Devices:     r.Devices.Copy(),
		NUMA:        r.NUMA.Copy(),
		SecretsMB:   r.SecretsMB,
	}
}

// NetIndex finds the matching net index using device name
// COMPAT(0.10): Remove in 0.10
func (r *Resources) NetIndex(n *NetworkResource) int {
	return r.Networks.NetIndex(n)
}

// Add adds the resources of the delta to this, potentially
// returning an error if not possible.
// COMPAT(0.10): Remove in 0.10
func (r *Resources) Add(delta *Resources) {
	if delta == nil {
		return
	}

	r.CPU += delta.CPU
	r.MemoryMB += delta.MemoryMB
	r.Cores += delta.Cores
	if delta.MemoryMaxMB > 0 {
		r.MemoryMaxMB += delta.MemoryMaxMB
	} else {
		r.MemoryMaxMB += delta.MemoryMB
	}
	r.DiskMB += delta.DiskMB
	r.SecretsMB += delta.SecretsMB

	for _, n := range delta.Networks {
		// Find the matching interface by IP or CIDR
		idx := r.NetIndex(n)
		if idx == -1 {
			r.Networks = append(r.Networks, n.Copy())
		} else {
			r.Networks[idx].Add(n)
		}
	}

	if r.Devices == nil && delta.Devices != nil {
		r.Devices = make(ResourceDevices, 0)
	}
	for _, dd := range delta.Devices {
		idx := slices.IndexFunc(r.Devices, func(d *RequestedDevice) bool { return d.Name == dd.Name })

		// means it's not found
		if idx < 0 {
			r.Devices = append(r.Devices, dd)
			continue
		}

		r.Devices[idx].Count += dd.Count
	}
}

// GoString returns the string representation of the Resources struct.
//
// COMPAT(0.10): Remove in 0.10
func (r *Resources) GoString() string {
	return fmt.Sprintf("*%#v", *r)
}

// NodeNetworkResource is used to describe a fingerprinted network of a node
type NodeNetworkResource struct {
	Mode string // host for physical networks, cni/<name> for cni networks

	// The following apply only to host networks
	Device     string // interface name
	MacAddress string
	Speed      int

	Addresses []NodeNetworkAddress // not valid for cni, for bridge there will only be 1 ip
}

func (n *NodeNetworkResource) Equal(o *NodeNetworkResource) bool {
	return reflect.DeepEqual(n, o)
}

func (n *NodeNetworkResource) Copy() *NodeNetworkResource {
	if n == nil {
		return nil
	}

	c := new(NodeNetworkResource)
	*c = *n

	if n.Addresses != nil {
		c.Addresses = make([]NodeNetworkAddress, len(n.Addresses))
		copy(c.Addresses, n.Addresses)
	}

	return c
}

func (n *NodeNetworkResource) HasAlias(alias string) bool {
	for _, addr := range n.Addresses {
		if addr.Alias == alias {
			return true
		}
	}
	return false
}

type NodeNetworkAF string

const (
	NodeNetworkAF_IPv4 NodeNetworkAF = "ipv4"
	NodeNetworkAF_IPv6 NodeNetworkAF = "ipv6"
)

// Validate validates that NodeNetworkAF has a legal value.
func (n NodeNetworkAF) Validate() error {
	if n == "" || n == NodeNetworkAF_IPv4 || n == NodeNetworkAF_IPv6 {
		return nil
	}
	return fmt.Errorf(`network address family must be one of: "", %q, %q`, NodeNetworkAF_IPv4, NodeNetworkAF_IPv6)
}

type NodeNetworkAddress struct {
	Family        NodeNetworkAF
	Alias         string
	Address       string
	ReservedPorts string
	Gateway       string // default route for this address
}

type AllocatedPortMapping struct {
	// msgpack omit empty fields during serialization
	_struct bool `codec:",omitempty"` // nolint: structcheck

	Label           string
	Value           int
	To              int
	HostIP          string
	IgnoreCollision bool
}

func (m *AllocatedPortMapping) Copy() *AllocatedPortMapping {
	return &AllocatedPortMapping{
		Label:           m.Label,
		Value:           m.Value,
		To:              m.To,
		HostIP:          m.HostIP,
		IgnoreCollision: m.IgnoreCollision,
	}
}

func (m *AllocatedPortMapping) Equal(o *AllocatedPortMapping) bool {
	if m == nil || o == nil {
		return m == o
	}
	switch {
	case m.Label != o.Label:
		return false
	case m.Value != o.Value:
		return false
	case m.To != o.To:
		return false
	case m.HostIP != o.HostIP:
		return false
	case m.IgnoreCollision != o.IgnoreCollision:
		return false
	}
	return true
}

type AllocatedPorts []AllocatedPortMapping

func (p AllocatedPorts) Equal(o AllocatedPorts) bool {
	return slices.EqualFunc(p, o, func(a, b AllocatedPortMapping) bool {
		return a.Equal(&b)
	})
}

func (p AllocatedPorts) Get(label string) (AllocatedPortMapping, bool) {
	for _, port := range p {
		if port.Label == label {
			return port, true
		}
	}

	return AllocatedPortMapping{}, false
}

type Port struct {
	// msgpack omit empty fields during serialization
	_struct bool `codec:",omitempty"` // nolint: structcheck

	// Label is the key for HCL port blocks: port "foo" {}
	Label string

	// Value is the static or dynamic port value. For dynamic ports this
	// will be 0 in the jobspec and set by the scheduler.
	Value int

	// To is the port inside a network namespace where this port is
	// forwarded. -1 is an internal sentinel value used by Consul Connect
	// to mean "same as the host port."
	To int

	// HostNetwork is the name of the network this port should be assigned
	// to. Jobs with a HostNetwork set can only be placed on nodes with
	// that host network available.
	HostNetwork string

	// IgnoreCollision ignores port collisions, so the port can be used more
	// than one time on a single network, for tasks that support SO_REUSEPORT
	// Should be used only with static ports.
	IgnoreCollision bool
}

type DNSConfig struct {
	Servers  []string
	Searches []string
	Options  []string
}

func (d *DNSConfig) Equal(o *DNSConfig) bool {
	if d == nil || o == nil {
		return d == o
	}

	switch {
	case !slices.Equal(d.Servers, o.Servers):
		return false
	case !slices.Equal(d.Searches, o.Searches):
		return false
	case !slices.Equal(d.Options, o.Options):
		return false
	}

	return true
}

func (d *DNSConfig) Copy() *DNSConfig {
	if d == nil {
		return nil
	}
	return &DNSConfig{
		Servers:  slices.Clone(d.Servers),
		Searches: slices.Clone(d.Searches),
		Options:  slices.Clone(d.Options),
	}
}

func (d *DNSConfig) IsZero() bool {
	if d == nil {
		return true
	}
	return len(d.Options) == 0 && len(d.Searches) == 0 && len(d.Servers) == 0
}

// NetworkResource is used to represent available network
// resources
type NetworkResource struct {
	// msgpack omit empty fields during serialization
	_struct bool `codec:",omitempty"` // nolint: structcheck

	Mode          string     // Mode of the network
	Device        string     // Name of the device
	CIDR          string     // CIDR block of addresses
	IP            string     // Host IP address
	Hostname      string     `json:",omitempty"` // Hostname of the network namespace
	MBits         int        // Throughput
	DNS           *DNSConfig // DNS Configuration
	ReservedPorts []Port     // Host Reserved ports
	DynamicPorts  []Port     // Host Dynamically assigned ports
	CNI           *CNIConfig // CNIConfig Configuration
}

func (n *NetworkResource) Hash() uint32 {
	var data []byte
	data = append(data, []byte(fmt.Sprintf("%s%s%s%s%s%d", n.Mode, n.Device, n.CIDR, n.IP, n.Hostname, n.MBits))...)

	for i, port := range n.ReservedPorts {
		data = append(data, []byte(fmt.Sprintf("r%d%s%d%d", i, port.Label, port.Value, port.To))...)
	}

	for i, port := range n.DynamicPorts {
		data = append(data, []byte(fmt.Sprintf("d%d%s%d%d", i, port.Label, port.Value, port.To))...)
	}

	return crc32.ChecksumIEEE(data)
}

func (n *NetworkResource) Equal(other *NetworkResource) bool {
	return n.Hash() == other.Hash()
}

func (n *NetworkResource) Canonicalize() {
	// Ensure that an empty and nil slices are treated the same to avoid scheduling
	// problems since we use reflect DeepEquals.
	if len(n.ReservedPorts) == 0 {
		n.ReservedPorts = nil
	}
	if len(n.DynamicPorts) == 0 {
		n.DynamicPorts = nil
	}

	for i, p := range n.DynamicPorts {
		if p.HostNetwork == "" {
			n.DynamicPorts[i].HostNetwork = "default"
		}
	}
	for i, p := range n.ReservedPorts {
		if p.HostNetwork == "" {
			n.ReservedPorts[i].HostNetwork = "default"
		}
	}
}

// Copy returns a deep copy of the network resource
func (n *NetworkResource) Copy() *NetworkResource {
	if n == nil {
		return nil
	}
	newR := new(NetworkResource)
	*newR = *n
	newR.DNS = n.DNS.Copy()
	if n.ReservedPorts != nil {
		newR.ReservedPorts = make([]Port, len(n.ReservedPorts))
		copy(newR.ReservedPorts, n.ReservedPorts)
	}
	if n.DynamicPorts != nil {
		newR.DynamicPorts = make([]Port, len(n.DynamicPorts))
		copy(newR.DynamicPorts, n.DynamicPorts)
	}
	return newR
}

// Add adds the resources of the delta to this, potentially
// returning an error if not possible.
func (n *NetworkResource) Add(delta *NetworkResource) {
	if len(delta.ReservedPorts) > 0 {
		n.ReservedPorts = append(n.ReservedPorts, delta.ReservedPorts...)
	}
	n.MBits += delta.MBits
	n.DynamicPorts = append(n.DynamicPorts, delta.DynamicPorts...)
}

func (n *NetworkResource) GoString() string {
	return fmt.Sprintf("*%#v", *n)
}

// PortLabels returns a map of port labels to their assigned host ports.
func (n *NetworkResource) PortLabels() map[string]int {
	num := len(n.ReservedPorts) + len(n.DynamicPorts)
	labelValues := make(map[string]int, num)
	for _, port := range n.ReservedPorts {
		labelValues[port.Label] = port.Value
	}
	for _, port := range n.DynamicPorts {
		labelValues[port.Label] = port.Value
	}
	return labelValues
}

func (n *NetworkResource) IsIPv6() bool {
	ip := net.ParseIP(n.IP)
	return ip != nil && ip.To4() == nil
}

// Networks defined for a task on the Resources struct.
type Networks []*NetworkResource

func (ns Networks) Copy() Networks {
	if len(ns) == 0 {
		return nil
	}

	out := make([]*NetworkResource, len(ns))
	for i := range ns {
		out[i] = ns[i].Copy()
	}
	return out
}

// Port assignment and IP for the given label or empty values.
func (ns Networks) Port(label string) AllocatedPortMapping {
	for _, n := range ns {
		for _, p := range n.ReservedPorts {
			if p.Label == label {
				return AllocatedPortMapping{
					Label:           label,
					Value:           p.Value,
					To:              p.To,
					HostIP:          n.IP,
					IgnoreCollision: p.IgnoreCollision,
				}
			}
		}
		for _, p := range n.DynamicPorts {
			if p.Label == label {
				return AllocatedPortMapping{
					Label:  label,
					Value:  p.Value,
					To:     p.To,
					HostIP: n.IP,
				}
			}
		}
	}
	return AllocatedPortMapping{}
}

func (ns Networks) NetIndex(n *NetworkResource) int {
	for idx, net := range ns {
		if net.Device == n.Device {
			return idx
		}
	}
	return -1
}

// Modes returns the set of network modes used by our NetworkResource blocks.
func (ns Networks) Modes() *set.Set[string] {
	return set.FromFunc(ns, func(nr *NetworkResource) string {
		return nr.Mode
	})
}

// RequestedDevice is used to request a device for a task.
type RequestedDevice struct {
	// Name is the request name. The possible values are as follows:
	// * <type>: A single value only specifies the type of request.
	// * <vendor>/<type>: A single slash delimiter assumes the vendor and type of device is specified.
	// * <vendor>/<type>/<name>: Two slash delimiters assume vendor, type and specific model are specified.
	//
	// Examples are as follows:
	// * "gpu"
	// * "nvidia/gpu"
	// * "nvidia/gpu/GTX2080Ti"
	Name string

	// Count is the number of requested devices
	Count uint64

	// Constraints are a set of constraints to apply when selecting the device
	// to use.
	Constraints Constraints

	// Affinities are a set of affinities to apply when selecting the device
	// to use.
	Affinities Affinities
}

func (r *RequestedDevice) String() string {
	return r.Name
}

func (r *RequestedDevice) Equal(o *RequestedDevice) bool {
	if r == o {
		return true
	}
	if r == nil || o == nil {
		return false
	}
	return r.Name == o.Name &&
		r.Count == o.Count &&
		r.Constraints.Equal(&o.Constraints) &&
		r.Affinities.Equal(&o.Affinities)
}

func (r *RequestedDevice) Copy() *RequestedDevice {
	if r == nil {
		return nil
	}

	nr := *r
	nr.Constraints = CopySliceConstraints(nr.Constraints)
	nr.Affinities = CopySliceAffinities(nr.Affinities)

	return &nr
}

func (r *RequestedDevice) ID() *DeviceIdTuple {
	if r == nil || r.Name == "" {
		return nil
	}

	parts := strings.SplitN(r.Name, "/", 3)
	switch len(parts) {
	case 1:
		return &DeviceIdTuple{
			Type: parts[0],
		}
	case 2:
		return &DeviceIdTuple{
			Vendor: parts[0],
			Type:   parts[1],
		}
	default:
		return &DeviceIdTuple{
			Vendor: parts[0],
			Type:   parts[1],
			Name:   parts[2],
		}
	}
}

func (r *RequestedDevice) Validate() error {
	if r == nil {
		return nil
	}

	var mErr multierror.Error
	if r.Name == "" {
		_ = multierror.Append(&mErr, errors.New("device name must be given as one of the following: type, vendor/type, or vendor/type/name"))
	}

	for idx, constr := range r.Constraints {
		// Ensure that the constraint doesn't use an operand we do not allow
		switch constr.Operand {
		case ConstraintDistinctHosts, ConstraintDistinctProperty:
			outer := fmt.Errorf("Constraint %d validation failed: using unsupported operand %q", idx+1, constr.Operand)
			_ = multierror.Append(&mErr, outer)
		default:
			if err := constr.Validate(); err != nil {
				outer := fmt.Errorf("Constraint %d validation failed: %s", idx+1, err)
				_ = multierror.Append(&mErr, outer)
			}
		}
	}
	for idx, affinity := range r.Affinities {
		if err := affinity.Validate(); err != nil {
			outer := fmt.Errorf("Affinity %d validation failed: %s", idx+1, err)
			_ = multierror.Append(&mErr, outer)
		}
	}

	return mErr.ErrorOrNil()
}

// NodeResources is used to define the resources available on a client node.
type NodeResources struct {
	// Do not read from this value except for compatibility (i.e. serialization).
	//
	// Deprecated; use NodeProcessorResources instead.
	Cpu LegacyNodeCpuResources

	Processors NodeProcessorResources
	Memory     NodeMemoryResources
	Disk       NodeDiskResources
	Devices    []*NodeDeviceResource

	// NodeNetworks was added in Nomad 0.12 to support multiple interfaces.
	// It is the superset of host_networks, fingerprinted networks, and the
	// node's default interface.
	NodeNetworks []*NodeNetworkResource

	// Networks is the node's bridge network and default interface. It is
	// only used when scheduling jobs with a deprecated
	// task.resources.network block.
	Networks Networks

	// MinDynamicPort and MaxDynamicPort represent the inclusive port range
	// to select dynamic ports from across all networks.
	MinDynamicPort int
	MaxDynamicPort int
}

func (n *NodeResources) Copy() *NodeResources {
	if n == nil {
		return nil
	}

	newN := new(NodeResources)
	*newN = *n
	newN.Processors = n.Processors.Copy()
	newN.Networks = n.Networks.Copy()

	if n.NodeNetworks != nil {
		newN.NodeNetworks = make([]*NodeNetworkResource, len(n.NodeNetworks))
		for i, nn := range n.NodeNetworks {
			newN.NodeNetworks[i] = nn.Copy()
		}
	}

	// Copy the devices
	if n.Devices != nil {
		devices := len(n.Devices)
		newN.Devices = make([]*NodeDeviceResource, devices)
		for i := 0; i < devices; i++ {
			newN.Devices[i] = n.Devices[i].Copy()
		}
	}

	// COMPAT remove in 1.10+
	// apply compatibility fixups covering node topology
	newN.Compatibility()

	return newN
}

// Comparable returns a comparable version of the nodes resources. This
// conversion can be lossy so care must be taken when using it.
func (n *NodeResources) Comparable() *ComparableResources {
	if n == nil {
		return nil
	}

	usableCores := n.Processors.Topology.UsableCores().Slice()
	reservableCores := helper.ConvertSlice(usableCores, func(id hw.CoreID) uint16 {
		return uint16(id)
	})

	c := &ComparableResources{
		Flattened: AllocatedTaskResources{
			Cpu: AllocatedCpuResources{
				CpuShares:     int64(n.Processors.Topology.TotalCompute()),
				ReservedCores: reservableCores,
			},
			Memory: AllocatedMemoryResources{
				MemoryMB: n.Memory.MemoryMB,
			},
			Networks: n.Networks,
		},
		Shared: AllocatedSharedResources{
			DiskMB: n.Disk.DiskMB,
		},
	}
	return c
}

func (n *NodeResources) Merge(o *NodeResources) {
	if o == nil {
		return
	}

	n.Processors.Merge(&o.Processors)
	n.Memory.Merge(&o.Memory)
	n.Disk.Merge(&o.Disk)

	if len(o.Networks) != 0 {
		n.Networks = append(n.Networks, o.Networks...)
	}

	if len(o.Devices) != 0 {
		n.Devices = o.Devices
	}

	if len(o.NodeNetworks) != 0 {
		for _, nw := range o.NodeNetworks {
			if i, nnw := lookupNetworkByDevice(n.NodeNetworks, nw.Device); nnw != nil {
				n.NodeNetworks[i] = nw
			} else {
				n.NodeNetworks = append(n.NodeNetworks, nw)
			}
		}
	}

	// COMPAT remove in 1.10+
	// apply compatibility fixups covering node topology
	n.Compatibility()
}

func lookupNetworkByDevice(nets []*NodeNetworkResource, name string) (int, *NodeNetworkResource) {
	for i, nw := range nets {
		if nw.Device == name {
			return i, nw
		}
	}
	return 0, nil
}

func (n *NodeResources) Equal(o *NodeResources) bool {
	if o == nil && n == nil {
		return true
	} else if o == nil {
		return false
	} else if n == nil {
		return false
	}

	if !n.Processors.Equal(&o.Processors) {
		return false
	}
	if !n.Memory.Equal(&o.Memory) {
		return false
	}
	if !n.Disk.Equal(&o.Disk) {
		return false
	}
	if !n.Networks.Equal(&o.Networks) {
		return false
	}

	// Check the devices
	if !DevicesEquals(n.Devices, o.Devices) {
		return false
	}

	if !NodeNetworksEquals(n.NodeNetworks, o.NodeNetworks) {
		return false
	}

	return true
}

// Equal equates Networks as a set
func (ns *Networks) Equal(o *Networks) bool {
	if ns == o {
		return true
	}
	if ns == nil || o == nil {
		return false
	}
	if len(*ns) != len(*o) {
		return false
	}
SETEQUALS:
	for _, ne := range *ns {
		for _, oe := range *o {
			if ne.Equal(oe) {
				continue SETEQUALS
			}
		}
		return false
	}
	return true
}

// DevicesEquals returns true if the two device arrays are set equal
func DevicesEquals(d1, d2 []*NodeDeviceResource) bool {
	if len(d1) != len(d2) {
		return false
	}
	idMap := make(map[DeviceIdTuple]*NodeDeviceResource, len(d1))
	for _, d := range d1 {
		idMap[*d.ID()] = d
	}
	for _, otherD := range d2 {
		if d, ok := idMap[*otherD.ID()]; !ok || !d.Equal(otherD) {
			return false
		}
	}

	return true
}

func NodeNetworksEquals(n1, n2 []*NodeNetworkResource) bool {
	if len(n1) != len(n2) {
		return false
	}

	netMap := make(map[string]*NodeNetworkResource, len(n1))
	for _, n := range n1 {
		netMap[n.Device] = n
	}
	for _, otherN := range n2 {
		if n, ok := netMap[otherN.Device]; !ok || !n.Equal(otherN) {
			return false
		}
	}

	return true

}

// NodeMemoryResources captures the memory resources of the node
type NodeMemoryResources struct {
	// MemoryMB is the total available memory on the node
	MemoryMB int64
}

func (n *NodeMemoryResources) Merge(o *NodeMemoryResources) {
	if o == nil {
		return
	}

	if o.MemoryMB != 0 {
		n.MemoryMB = o.MemoryMB
	}
}

func (n *NodeMemoryResources) Equal(o *NodeMemoryResources) bool {
	if o == nil && n == nil {
		return true
	} else if o == nil {
		return false
	} else if n == nil {
		return false
	}

	if n.MemoryMB != o.MemoryMB {
		return false
	}

	return true
}

// NodeDiskResources captures the disk resources of the node
type NodeDiskResources struct {
	// DiskMB is the total available disk space on the node
	DiskMB int64
}

func (n *NodeDiskResources) Merge(o *NodeDiskResources) {
	if o == nil {
		return
	}
	if o.DiskMB != 0 {
		n.DiskMB = o.DiskMB
	}
}

func (n *NodeDiskResources) Equal(o *NodeDiskResources) bool {
	if o == nil && n == nil {
		return true
	} else if o == nil {
		return false
	} else if n == nil {
		return false
	}

	if n.DiskMB != o.DiskMB {
		return false
	}

	return true
}

// DeviceIdTuple is the tuple that identifies a device
type DeviceIdTuple struct {
	Vendor string
	Type   string
	Name   string
}

func (id *DeviceIdTuple) String() string {
	if id == nil {
		return ""
	}

	return fmt.Sprintf("%s/%s/%s", id.Vendor, id.Type, id.Name)
}

// Matches returns if this Device ID is a superset of the passed ID.
func (id *DeviceIdTuple) Matches(other *DeviceIdTuple) bool {
	if other == nil {
		return false
	}

	if other.Name != "" && other.Name != id.Name {
		return false
	}

	if other.Vendor != "" && other.Vendor != id.Vendor {
		return false
	}

	if other.Type != "" && other.Type != id.Type {
		return false
	}

	return true
}

// Equal returns if this Device ID is the same as the passed ID.
func (id *DeviceIdTuple) Equal(o *DeviceIdTuple) bool {
	if id == nil && o == nil {
		return true
	} else if id == nil || o == nil {
		return false
	}

	return o.Vendor == id.Vendor && o.Type == id.Type && o.Name == id.Name
}

// NodeDeviceResource captures a set of devices sharing a common
// vendor/type/device_name tuple.
type NodeDeviceResource struct {
	Vendor     string
	Type       string
	Name       string
	Instances  []*NodeDevice
	Attributes map[string]*psstructs.Attribute
}

func (n *NodeDeviceResource) ID() *DeviceIdTuple {
	if n == nil {
		return nil
	}

	return &DeviceIdTuple{
		Vendor: n.Vendor,
		Type:   n.Type,
		Name:   n.Name,
	}
}

func (n *NodeDeviceResource) Copy() *NodeDeviceResource {
	if n == nil {
		return nil
	}

	// Copy the primitives
	nn := *n

	// Copy the device instances
	if l := len(nn.Instances); l != 0 {
		nn.Instances = make([]*NodeDevice, 0, l)
		for _, d := range n.Instances {
			nn.Instances = append(nn.Instances, d.Copy())
		}
	}

	// Copy the Attributes
	nn.Attributes = psstructs.CopyMapStringAttribute(nn.Attributes)

	return &nn
}

func (n *NodeDeviceResource) Equal(o *NodeDeviceResource) bool {
	if o == nil && n == nil {
		return true
	} else if o == nil {
		return false
	} else if n == nil {
		return false
	}

	if n.Vendor != o.Vendor {
		return false
	} else if n.Type != o.Type {
		return false
	} else if n.Name != o.Name {
		return false
	}

	// Check the attributes
	if len(n.Attributes) != len(o.Attributes) {
		return false
	}
	for k, v := range n.Attributes {
		if otherV, ok := o.Attributes[k]; !ok || v != otherV {
			return false
		}
	}

	// Check the instances
	if len(n.Instances) != len(o.Instances) {
		return false
	}
	idMap := make(map[string]*NodeDevice, len(n.Instances))
	for _, d := range n.Instances {
		idMap[d.ID] = d
	}
	for _, otherD := range o.Instances {
		if d, ok := idMap[otherD.ID]; !ok || !d.Equal(otherD) {
			return false
		}
	}

	return true
}

// NodeDevice is an instance of a particular device.
type NodeDevice struct {
	// ID is the ID of the device.
	ID string

	// Healthy captures whether the device is healthy.
	Healthy bool

	// HealthDescription is used to provide a human readable description of why
	// the device may be unhealthy.
	HealthDescription string

	// Locality stores HW locality information for the node to optionally be
	// used when making placement decisions.
	Locality *NodeDeviceLocality
}

func (n *NodeDevice) Equal(o *NodeDevice) bool {
	if o == nil && n == nil {
		return true
	} else if o == nil {
		return false
	} else if n == nil {
		return false
	}

	if n.ID != o.ID {
		return false
	} else if n.Healthy != o.Healthy {
		return false
	} else if n.HealthDescription != o.HealthDescription {
		return false
	} else if !n.Locality.Equal(o.Locality) {
		return false
	}

	return false
}

func (n *NodeDevice) Copy() *NodeDevice {
	if n == nil {
		return nil
	}

	// Copy the primitives
	nn := *n

	// Copy the locality
	nn.Locality = nn.Locality.Copy()

	return &nn
}

// NodeDeviceLocality stores information about the devices hardware locality on
// the node.
type NodeDeviceLocality struct {
	// PciBusID is the PCI Bus ID for the device.
	PciBusID string
}

func (n *NodeDeviceLocality) Equal(o *NodeDeviceLocality) bool {
	if o == nil && n == nil {
		return true
	} else if o == nil {
		return false
	} else if n == nil {
		return false
	}

	if n.PciBusID != o.PciBusID {
		return false
	}

	return true
}

func (n *NodeDeviceLocality) Copy() *NodeDeviceLocality {
	if n == nil {
		return nil
	}

	// Copy the primitives
	nn := *n
	return &nn
}

// NodeReservedResources is used to capture the resources on a client node that
// should be reserved and not made available to jobs.
type NodeReservedResources struct {
	Cpu      NodeReservedCpuResources
	Memory   NodeReservedMemoryResources
	Disk     NodeReservedDiskResources
	Networks NodeReservedNetworkResources
}

func (n *NodeReservedResources) Copy() *NodeReservedResources {
	if n == nil {
		return nil
	}
	newN := new(NodeReservedResources)
	*newN = *n
	return newN
}

// Comparable returns a comparable version of the node's reserved resources. The
// returned resources doesn't contain any network information. This conversion
// can be lossy so care must be taken when using it.
func (n *NodeReservedResources) Comparable() *ComparableResources {
	if n == nil {
		return nil
	}

	c := &ComparableResources{
		Flattened: AllocatedTaskResources{
			Cpu: AllocatedCpuResources{
				CpuShares:     n.Cpu.CpuShares,
				ReservedCores: n.Cpu.ReservedCpuCores,
			},
			Memory: AllocatedMemoryResources{
				MemoryMB: n.Memory.MemoryMB,
			},
		},
		Shared: AllocatedSharedResources{
			DiskMB: n.Disk.DiskMB,
		},
	}
	return c
}

// NodeReservedCpuResources captures the reserved CPU resources of the node.
type NodeReservedCpuResources struct {
	CpuShares        int64
	ReservedCpuCores []uint16
}

// NodeReservedMemoryResources captures the reserved memory resources of the node.
type NodeReservedMemoryResources struct {
	MemoryMB int64
}

// NodeReservedDiskResources captures the reserved disk resources of the node.
type NodeReservedDiskResources struct {
	DiskMB int64
}

// NodeReservedNetworkResources captures the reserved network resources of the node.
type NodeReservedNetworkResources struct {
	// ReservedHostPorts is the set of ports reserved on all host network
	// interfaces. Its format is a comma separate list of integers or integer
	// ranges. (80,443,1000-2000,2005)
	ReservedHostPorts string
}

// ParseReservedHostPorts returns the reserved host ports.
func (n *NodeReservedNetworkResources) ParseReservedHostPorts() ([]uint64, error) {
	return ParsePortRanges(n.ReservedHostPorts)
}

// AllocatedResources is the set of resources to be used by an allocation.
type AllocatedResources struct {
	// Tasks is a mapping of task name to the resources for the task.
	Tasks          map[string]*AllocatedTaskResources
	TaskLifecycles map[string]*TaskLifecycleConfig

	// Shared is the set of resource that are shared by all tasks in the group.
	Shared AllocatedSharedResources
}

// UsesCores returns true if any of the tasks in the allocation make use
// of reserved cpu cores.
func (a *AllocatedResources) UsesCores() bool {
	for _, taskRes := range a.Tasks {
		if len(taskRes.Cpu.ReservedCores) > 0 {
			return true
		}
	}
	return false
}

func (a *AllocatedResources) Copy() *AllocatedResources {
	if a == nil {
		return nil
	}

	out := AllocatedResources{
		Shared: a.Shared.Copy(),
	}

	if a.Tasks != nil {
		out.Tasks = make(map[string]*AllocatedTaskResources, len(out.Tasks))
		for task, resource := range a.Tasks {
			out.Tasks[task] = resource.Copy()
		}
	}
	if a.TaskLifecycles != nil {
		out.TaskLifecycles = make(map[string]*TaskLifecycleConfig, len(out.TaskLifecycles))
		for task, lifecycle := range a.TaskLifecycles {
			out.TaskLifecycles[task] = lifecycle.Copy()
		}

	}

	return &out
}

// Comparable returns a comparable version of the allocations allocated
// resources. This conversion can be lossy so care must be taken when using it.
func (a *AllocatedResources) Comparable() *ComparableResources {
	if a == nil {
		return nil
	}

	c := &ComparableResources{
		Shared: a.Shared,
	}

	prestartSidecarTasks := &AllocatedTaskResources{}
	prestartEphemeralTasks := &AllocatedTaskResources{}
	main := &AllocatedTaskResources{}
	poststopTasks := &AllocatedTaskResources{}

	for taskName, r := range a.Tasks {
		lc := a.TaskLifecycles[taskName]
		if lc == nil {
			main.Add(r)
		} else if lc.Hook == TaskLifecycleHookPrestart {
			if lc.Sidecar {
				prestartSidecarTasks.Add(r)
			} else {
				prestartEphemeralTasks.Add(r)
			}
		} else if lc.Hook == TaskLifecycleHookPoststop {
			poststopTasks.Add(r)
		}
	}

	// update this loop to account for lifecycle hook
	prestartEphemeralTasks.Max(main)
	prestartEphemeralTasks.Max(poststopTasks)
	prestartSidecarTasks.Add(prestartEphemeralTasks)
	c.Flattened.Add(prestartSidecarTasks)

	// Add network resources that are at the task group level
	for _, network := range a.Shared.Networks {
		c.Flattened.Add(&AllocatedTaskResources{
			Networks: []*NetworkResource{network},
		})
	}

	return c
}

// OldTaskResources returns the pre-0.9.0 map of task resources. This
// functionality is still used within the scheduling code.
func (a *AllocatedResources) OldTaskResources() map[string]*Resources {
	m := make(map[string]*Resources, len(a.Tasks))
	for name, res := range a.Tasks {
		m[name] = &Resources{
			Cores:       len(res.Cpu.ReservedCores),
			CPU:         int(res.Cpu.CpuShares),
			MemoryMB:    int(res.Memory.MemoryMB),
			MemoryMaxMB: int(res.Memory.MemoryMaxMB),
			Networks:    res.Networks,
		}
	}

	return m
}

func (a *AllocatedResources) Canonicalize() {
	a.Shared.Canonicalize()

	for _, r := range a.Tasks {
		for _, nw := range r.Networks {
			for _, port := range append(nw.DynamicPorts, nw.ReservedPorts...) {
				a.Shared.Ports = append(a.Shared.Ports, AllocatedPortMapping{
					Label:  port.Label,
					Value:  port.Value,
					To:     port.To,
					HostIP: nw.IP,
				})
			}
		}
	}
}

// AllocatedTaskResources are the set of resources allocated to a task.
type AllocatedTaskResources struct {
	Cpu      AllocatedCpuResources
	Memory   AllocatedMemoryResources
	Networks Networks
	Devices  []*AllocatedDeviceResource
}

func (a *AllocatedTaskResources) Copy() *AllocatedTaskResources {
	if a == nil {
		return nil
	}
	newA := new(AllocatedTaskResources)
	*newA = *a

	// Copy the networks
	newA.Networks = a.Networks.Copy()

	// Copy the devices
	if newA.Devices != nil {
		n := len(a.Devices)
		newA.Devices = make([]*AllocatedDeviceResource, n)
		for i := 0; i < n; i++ {
			newA.Devices[i] = a.Devices[i].Copy()
		}
	}

	return newA
}

// NetIndex finds the matching net index using device name
func (a *AllocatedTaskResources) NetIndex(n *NetworkResource) int {
	return a.Networks.NetIndex(n)
}

func (a *AllocatedTaskResources) Add(delta *AllocatedTaskResources) {
	if delta == nil {
		return
	}

	a.Cpu.Add(&delta.Cpu)
	a.Memory.Add(&delta.Memory)

	for _, n := range delta.Networks {
		// Find the matching interface by IP or CIDR
		idx := a.NetIndex(n)
		if idx == -1 {
			a.Networks = append(a.Networks, n.Copy())
		} else {
			a.Networks[idx].Add(n)
		}
	}

	for _, d := range delta.Devices {
		// Find the matching device
		idx := AllocatedDevices(a.Devices).Index(d)
		if idx == -1 {
			a.Devices = append(a.Devices, d.Copy())
		} else {
			a.Devices[idx].Add(d)
		}
	}
}

func (a *AllocatedTaskResources) Max(other *AllocatedTaskResources) {
	if other == nil {
		return
	}

	a.Cpu.Max(&other.Cpu)
	a.Memory.Max(&other.Memory)

	for _, n := range other.Networks {
		// Find the matching interface by IP or CIDR
		idx := a.NetIndex(n)
		if idx == -1 {
			a.Networks = append(a.Networks, n.Copy())
		} else {
			a.Networks[idx].Add(n)
		}
	}

	for _, d := range other.Devices {
		// Find the matching device
		idx := AllocatedDevices(a.Devices).Index(d)
		if idx == -1 {
			a.Devices = append(a.Devices, d.Copy())
		} else {
			a.Devices[idx].Add(d)
		}
	}
}

// Comparable turns AllocatedTaskResources into ComparableResources
// as a helper step in preemption
func (a *AllocatedTaskResources) Comparable() *ComparableResources {
	ret := &ComparableResources{
		Flattened: AllocatedTaskResources{
			Cpu: AllocatedCpuResources{
				CpuShares:     a.Cpu.CpuShares,
				ReservedCores: a.Cpu.ReservedCores,
			},
			Memory: AllocatedMemoryResources{
				MemoryMB:    a.Memory.MemoryMB,
				MemoryMaxMB: a.Memory.MemoryMaxMB,
			},
		},
	}
	ret.Flattened.Networks = append(ret.Flattened.Networks, a.Networks...)
	return ret
}

// Subtract only subtracts CPU and Memory resources. Network utilization
// is managed separately in NetworkIndex
func (a *AllocatedTaskResources) Subtract(delta *AllocatedTaskResources) {
	if delta == nil {
		return
	}

	a.Cpu.Subtract(&delta.Cpu)
	a.Memory.Subtract(&delta.Memory)
}

// AllocatedSharedResources are the set of resources allocated to a task group.
type AllocatedSharedResources struct {
	Networks Networks
	DiskMB   int64
	Ports    AllocatedPorts
}

func (a AllocatedSharedResources) Copy() AllocatedSharedResources {
	return AllocatedSharedResources{
		Networks: a.Networks.Copy(),
		DiskMB:   a.DiskMB,
		Ports:    a.Ports,
	}
}

func (a *AllocatedSharedResources) Add(delta *AllocatedSharedResources) {
	if delta == nil {
		return
	}
	a.Networks = append(a.Networks, delta.Networks...)
	a.DiskMB += delta.DiskMB

}

func (a *AllocatedSharedResources) Subtract(delta *AllocatedSharedResources) {
	if delta == nil {
		return
	}

	diff := map[*NetworkResource]bool{}
	for _, n := range delta.Networks {
		diff[n] = true
	}
	var nets Networks
	for _, n := range a.Networks {
		if _, ok := diff[n]; !ok {
			nets = append(nets, n)
		}
	}
	a.Networks = nets
	a.DiskMB -= delta.DiskMB
}

func (a *AllocatedSharedResources) Canonicalize() {
	if len(a.Networks) > 0 {
		if len(a.Networks[0].DynamicPorts)+len(a.Networks[0].ReservedPorts) > 0 && len(a.Ports) == 0 {
			for _, ports := range [][]Port{a.Networks[0].DynamicPorts, a.Networks[0].ReservedPorts} {
				for _, p := range ports {
					a.Ports = append(a.Ports, AllocatedPortMapping{
						Label:  p.Label,
						Value:  p.Value,
						To:     p.To,
						HostIP: a.Networks[0].IP,
					})
				}
			}
		}
	}
}

// AllocatedCpuResources captures the allocated CPU resources.
type AllocatedCpuResources struct {
	CpuShares     int64
	ReservedCores []uint16
}

func (a *AllocatedCpuResources) Add(delta *AllocatedCpuResources) {
	if delta == nil {
		return
	}

	// add cpu bandwidth
	a.CpuShares += delta.CpuShares

	// add cpu cores
	cores := idset.From[uint16](a.ReservedCores)
	deltaCores := idset.From[uint16](delta.ReservedCores)
	cores.InsertSet(deltaCores)
	a.ReservedCores = cores.Slice()
}

func (a *AllocatedCpuResources) Subtract(delta *AllocatedCpuResources) {
	if delta == nil {
		return
	}

	// remove cpu bandwidth
	a.CpuShares -= delta.CpuShares

	// remove cpu cores
	cores := idset.From[uint16](a.ReservedCores)
	deltaCores := idset.From[uint16](delta.ReservedCores)
	cores.RemoveSet(deltaCores)
	a.ReservedCores = cores.Slice()
}

func (a *AllocatedCpuResources) Max(other *AllocatedCpuResources) {
	if other == nil {
		return
	}

	if other.CpuShares > a.CpuShares {
		a.CpuShares = other.CpuShares
	}

	if len(other.ReservedCores) > len(a.ReservedCores) {
		a.ReservedCores = other.ReservedCores
	}
}

// AllocatedMemoryResources captures the allocated memory resources.
type AllocatedMemoryResources struct {
	MemoryMB    int64
	MemoryMaxMB int64
}

func (a *AllocatedMemoryResources) Add(delta *AllocatedMemoryResources) {
	if delta == nil {
		return
	}

	a.MemoryMB += delta.MemoryMB
	if delta.MemoryMaxMB != 0 {
		a.MemoryMaxMB += delta.MemoryMaxMB
	} else {
		a.MemoryMaxMB += delta.MemoryMB
	}
}

func (a *AllocatedMemoryResources) Subtract(delta *AllocatedMemoryResources) {
	if delta == nil {
		return
	}

	a.MemoryMB -= delta.MemoryMB
	if delta.MemoryMaxMB != 0 {
		a.MemoryMaxMB -= delta.MemoryMaxMB
	} else {
		a.MemoryMaxMB -= delta.MemoryMB
	}
}

func (a *AllocatedMemoryResources) Max(other *AllocatedMemoryResources) {
	if other == nil {
		return
	}

	if other.MemoryMB > a.MemoryMB {
		a.MemoryMB = other.MemoryMB
	}
	if other.MemoryMaxMB > a.MemoryMaxMB {
		a.MemoryMaxMB = other.MemoryMaxMB
	}
}

type AllocatedDevices []*AllocatedDeviceResource

// Index finds the matching index using the passed device. If not found, -1 is
// returned.
func (a AllocatedDevices) Index(d *AllocatedDeviceResource) int {
	if d == nil {
		return -1
	}

	for i, o := range a {
		if o.ID().Equal(d.ID()) {
			return i
		}
	}

	return -1
}

// AllocatedDeviceResource captures a set of allocated devices.
type AllocatedDeviceResource struct {
	// Vendor, Type, and Name are used to select the plugin to request the
	// device IDs from.
	Vendor string
	Type   string
	Name   string

	// DeviceIDs is the set of allocated devices
	DeviceIDs []string
}

func (a *AllocatedDeviceResource) ID() *DeviceIdTuple {
	if a == nil {
		return nil
	}

	return &DeviceIdTuple{
		Vendor: a.Vendor,
		Type:   a.Type,
		Name:   a.Name,
	}
}

func (a *AllocatedDeviceResource) Add(delta *AllocatedDeviceResource) {
	if delta == nil {
		return
	}

	a.DeviceIDs = append(a.DeviceIDs, delta.DeviceIDs...)
}

func (a *AllocatedDeviceResource) Copy() *AllocatedDeviceResource {
	if a == nil {
		return a
	}

	na := *a

	// Copy the devices
	na.DeviceIDs = make([]string, len(a.DeviceIDs))
	copy(na.DeviceIDs, a.DeviceIDs)
	return &na
}

// ComparableResources is the set of resources allocated to a task group but
// not keyed by Task, making it easier to compare.
type ComparableResources struct {
	Flattened AllocatedTaskResources
	Shared    AllocatedSharedResources
}

func (c *ComparableResources) Add(delta *ComparableResources) {
	if delta == nil {
		return
	}

	c.Flattened.Add(&delta.Flattened)
	c.Shared.Add(&delta.Shared)
}

func (c *ComparableResources) Subtract(delta *ComparableResources) {
	if delta == nil {
		return
	}

	c.Flattened.Subtract(&delta.Flattened)
	c.Shared.Subtract(&delta.Shared)
}

func (c *ComparableResources) Copy() *ComparableResources {
	if c == nil {
		return nil
	}
	newR := new(ComparableResources)
	*newR = *c
	return newR
}

// Superset checks if one set of resources is a superset of another. This
// ignores network resources, and the NetworkIndex should be used for that.
func (c *ComparableResources) Superset(other *ComparableResources) (bool, string) {
	if c.Flattened.Cpu.CpuShares < other.Flattened.Cpu.CpuShares {
		return false, "cpu"
	}

	cores := idset.From[uint16](c.Flattened.Cpu.ReservedCores)
	otherCores := idset.From[uint16](other.Flattened.Cpu.ReservedCores)
	if len(c.Flattened.Cpu.ReservedCores) > 0 && !cores.Superset(otherCores) {
		return false, "cores"
	}

	if c.Flattened.Memory.MemoryMB < other.Flattened.Memory.MemoryMB {
		return false, "memory"
	}

	if c.Shared.DiskMB < other.Shared.DiskMB {
		return false, "disk"
	}
	return true, ""
}

// NetIndex finds the matching net index using device name
func (c *ComparableResources) NetIndex(n *NetworkResource) int {
	return c.Flattened.Networks.NetIndex(n)
}

const (
	// JobTypeCore is reserved for internal system tasks and is
	// always handled by the CoreScheduler.
	JobTypeCore     = "_core"
	JobTypeService  = "service"
	JobTypeBatch    = "batch"
	JobTypeSystem   = "system"
	JobTypeSysBatch = "sysbatch"
)

const (
	JobStatusPending = "pending" // Pending means the job is waiting on scheduling
	JobStatusRunning = "running" // Running means the job has non-terminal allocations
	JobStatusDead    = "dead"    // Dead means all evaluation's and allocations are terminal
)

const (
	// JobMinPriority is the minimum allowed priority
	JobMinPriority = 1

	// JobDefaultPriority is the default priority if not specified.
	JobDefaultPriority = 50

	// JobDefaultMaxPriority is the default maximum allowed priority
	JobDefaultMaxPriority = 100

	// JobMaxPriority is the maximum allowed configuration value for maximum job priority
	JobMaxPriority = math.MaxInt16 - 1

	// CoreJobPriority should be higher than any user
	// specified job so that it gets priority. This is important
	// for the system to remain healthy.
	CoreJobPriority = math.MaxInt16

	// JobDefaultTrackedVersions is the number of historic job versions that are
	// kept.
	JobDefaultTrackedVersions = 6

	// JobTrackedScalingEvents is the number of scaling events that are
	// kept for a single task group.
	JobTrackedScalingEvents = 20
)

// A JobSubmission contains the original job specification, along with the Variables
// submitted with the job.
type JobSubmission struct {
	// Source contains the original job definition (may be hc1, hcl2, or json)
	Source string

	// Format indicates whether the original job was hcl1, hcl2, or json.
	// hcl1 format has been removed and can no longer be parsed.
	Format string

	// VariableFlags contain the CLI "-var" flag arguments as submitted with the
	// job (hcl2 only).
	VariableFlags map[string]string

	// Variables contains the opaque variable blob that was input from the
	// webUI (hcl2 only).
	Variables string

	// Namespace is managed internally, do not set.
	//
	// The namespace the associated job belongs to.
	Namespace string

	// JobID is managed internally, not set.
	//
	// The job.ID field.
	JobID string

	// Version is managed internally, not set.
	//
	// The version of the Job this submission is associated with.
	Version uint64

	// JobModifyIndex is managed internally, not set.
	//
	// The raft index the Job this submission is associated with.
	JobModifyIndex uint64
}

// Hash returns a value representative of the intended uniquness of a
// JobSubmission in the job_submission state store table (namespace, jobID, version).
func (js *JobSubmission) Hash() string {
	return fmt.Sprintf("%s \x00 %s \x00 %d", js.Namespace, js.JobID, js.Version)
}

// Copy creates a deep copy of js.
func (js *JobSubmission) Copy() *JobSubmission {
	if js == nil {
		return nil
	}
	return &JobSubmission{
		Source:         js.Source,
		Format:         js.Format,
		VariableFlags:  maps.Clone(js.VariableFlags),
		Variables:      js.Variables,
		Namespace:      js.Namespace,
		JobID:          js.JobID,
		Version:        js.Version,
		JobModifyIndex: js.JobModifyIndex,
	}
}

// Job is the scope of a scheduling request to Nomad. It is the largest
// scoped object, and is a named collection of task groups. Each task group
// is further composed of tasks. A task group (TG) is the unit of scheduling
// however.
type Job struct {
	// Stop marks whether the user has stopped the job. A stopped job will
	// have all created allocations stopped and acts as a way to stop a job
	// without purging it from the system. This allows existing allocs to be
	// queried and the job to be inspected as it is being killed.
	Stop bool

	// Region is the Nomad region that handles scheduling this job
	Region string

	// Namespace is the namespace the job is submitted into.
	Namespace string

	// ID is a unique identifier for the job per region. It can be
	// specified hierarchically like LineOfBiz/OrgName/Team/Project
	ID string

	// ParentID is the unique identifier of the job that spawned this job.
	ParentID string

	// Name is the logical name of the job used to refer to it. This is unique
	// per region, but not unique globally.
	Name string

	// Type is used to control various behaviors about the job. Most jobs
	// are service jobs, meaning they are expected to be long lived.
	// Some jobs are batch oriented meaning they run and then terminate.
	// This can be extended in the future to support custom schedulers.
	Type string

	// Priority is used to control scheduling importance and if this job
	// can preempt other jobs.
	Priority int

	// AllAtOnce is used to control if incremental scheduling of task groups
	// is allowed or if we must do a gang scheduling of the entire job. This
	// can slow down larger jobs if resources are not available.
	AllAtOnce bool

	// Datacenters contains all the datacenters this job is allowed to span
	Datacenters []string

	// NodePool specifies the node pool this job is allowed to run on.
	//
	// An empty value is allowed during job registration, in which case the
	// namespace default node pool is used in Enterprise and the 'default' node
	// pool in OSS. But a node pool must be set before the job is stored, so
	// that will happen in the admission mutators.
	NodePool string

	// Constraints can be specified at a job level and apply to
	// all the task groups and tasks.
	Constraints []*Constraint

	// Affinities can be specified at the job level to express
	// scheduling preferences that apply to all groups and tasks
	Affinities []*Affinity

	// Spread can be specified at the job level to express spreading
	// allocations across a desired attribute, such as datacenter
	Spreads []*Spread

	// TaskGroups are the collections of task groups that this job needs
	// to run. Each task group is an atomic unit of scheduling and placement.
	TaskGroups []*TaskGroup

	// See agent.ApiJobToStructJob
	// Update provides defaults for the TaskGroup Update blocks
	Update UpdateStrategy

	Multiregion *Multiregion

	// Periodic is used to define the interval the job is run at.
	Periodic *PeriodicConfig

	// ParameterizedJob is used to specify the job as a parameterized job
	// for dispatching.
	ParameterizedJob *ParameterizedJobConfig

	// Dispatched is used to identify if the Job has been dispatched from a
	// parameterized job.
	Dispatched bool

	// DispatchIdempotencyToken is optionally used to ensure that a dispatched job does not have any
	// non-terminal siblings which have the same token value.
	DispatchIdempotencyToken string

	// Payload is the payload supplied when the job was dispatched.
	Payload []byte

	// Meta is used to associate arbitrary metadata with this
	// job. This is opaque to Nomad.
	Meta map[string]string

	// ConsulToken is the Consul token that proves the submitter of the job has
	// access to the Service Identity policies associated with the job's
	// Consul Connect enabled services. This field is only used to transfer the
	// token and is not stored after Job submission.
	ConsulToken string

	// ConsulNamespace is the Consul namespace
	ConsulNamespace string

	// VaultToken is the Vault token that proves the submitter of the job has
	// access to the specified Vault policies. This field is only used to
	// transfer the token and is not stored after Job submission.
	VaultToken string

	// VaultNamespace is the Vault namespace
	VaultNamespace string

	// NomadTokenID is the Accessor ID of the ACL token (if any)
	// used to register this version of the job. Used by deploymentwatcher.
	NomadTokenID string

	// Job status
	Status string

	// StatusDescription is meant to provide more human useful information
	StatusDescription string

	// Stable marks a job as stable. Stability is only defined on "service" and
	// "system" jobs. The stability of a job will be set automatically as part
	// of a deployment and can be manually set via APIs. This field is updated
	// when the status of a corresponding deployment transitions to Failed
	// or Successful. This field is not meaningful for jobs that don't have an
	// update block.
	Stable bool

	// Version is a monotonically increasing version number that is incremented
	// on each job register.
	Version uint64

	// SubmitTime is the time at which the job version was submitted as
	// UnixNano in UTC
	SubmitTime int64

	// Raft Indexes
	CreateIndex uint64
	// ModifyIndex is the index at which any state of the job last changed
	ModifyIndex uint64
	// JobModifyIndex is the index at which the job *specification* last changed
	JobModifyIndex uint64

	// Links and Description fields for the Web UI
	UI *JobUIConfig

	// Metadata related to a tagged Job Version (which itself is really a Job)
	VersionTag *JobVersionTag
}

type JobVersionTag struct {
	Name        string
	Description string
	TaggedTime  int64
}

type JobApplyTagRequest struct {
	JobID   string
	Name    string
	Tag     *JobVersionTag
	Version uint64
	WriteRequest
}

type JobTagResponse struct {
	Name        string
	Description string
	TaggedTime  int64
	QueryMeta
}

func (tv *JobVersionTag) Copy() *JobVersionTag {
	if tv == nil {
		return nil
	}
	return &JobVersionTag{
		Name:        tv.Name,
		Description: tv.Description,
		TaggedTime:  tv.TaggedTime,
	}
}

type JobUIConfig struct {
	Description string
	Links       []*JobUILink
}

type JobUILink struct {
	Label string
	Url   string
}

func (j *JobUIConfig) Copy() *JobUIConfig {
	if j == nil {
		return nil
	}
	copy := new(JobUIConfig)
	copy.Description = j.Description

	if j.Links != nil {
		links := make([]*JobUILink, len(j.Links))
		for i, link := range j.Links {
			links[i] = link.Copy()
		}
		copy.Links = links
	}
	return copy
}

func (l *JobUILink) Copy() *JobUILink {
	if l == nil {
		return nil
	}
	copy := new(JobUILink)
	copy.Label = l.Label
	copy.Url = l.Url
	return copy
}

// NamespacedID returns the namespaced id useful for logging
func (j *Job) NamespacedID() NamespacedID {
	return NamespacedID{
		ID:        j.ID,
		Namespace: j.Namespace,
	}
}

// GetID implements the IDGetter interface, required for pagination.
func (j *Job) GetID() string {
	if j == nil {
		return ""
	}
	return j.ID
}

// GetNamespace implements the NamespaceGetter interface, required for
// pagination and filtering namespaces in endpoints that support glob namespace
// requests using tokens with limited access.
func (j *Job) GetNamespace() string {
	if j == nil {
		return ""
	}
	return j.Namespace
}

// GetIDforWorkloadIdentity is used when we want the job ID for identity; here we
// always want the parent ID if there is one and then fallback to the ID
func (j *Job) GetIDforWorkloadIdentity() string {
	if j.ParentID != "" {
		return j.ParentID
	}
	return j.ID
}

// GetCreateIndex implements the CreateIndexGetter interface, required for
// pagination.
func (j *Job) GetCreateIndex() uint64 {
	if j == nil {
		return 0
	}
	return j.CreateIndex
}

// GetModifyIndex implements the ModifyIndexGetter interface, required for
// pagination.
func (j *Job) GetModifyIndex() uint64 {
	if j == nil {
		return 0
	}
	return j.ModifyIndex
}

// Canonicalize is used to canonicalize fields in the Job. This should be
// called when registering a Job.
func (j *Job) Canonicalize() {
	if j == nil {
		return
	}

	// Ensure that an empty and nil map or array are treated the same to avoid scheduling
	// problems since we use reflect DeepEquals.
	if len(j.Meta) == 0 {
		j.Meta = nil
	}

	if len(j.Constraints) == 0 {
		j.Constraints = nil
	}

	if len(j.Affinities) == 0 {
		j.Affinities = nil
	}

	if len(j.Spreads) == 0 {
		j.Spreads = nil
	}

	// Ensure the job is in a namespace.
	if j.Namespace == "" {
		j.Namespace = DefaultNamespace
	}

	if len(j.Datacenters) == 0 {
		j.Datacenters = []string{"*"}
	}

	for _, tg := range j.TaskGroups {
		tg.Canonicalize(j)
	}

	if j.ParameterizedJob != nil {
		j.ParameterizedJob.Canonicalize()
	}

	if j.Multiregion != nil {
		j.Multiregion.Canonicalize()
	}

	if j.Periodic != nil {
		j.Periodic.Canonicalize()
	}
}

// Copy returns a deep copy of the Job. It is expected that callers use recover.
// This job can panic if the deep copy failed as it uses reflection.
func (j *Job) Copy() *Job {
	if j == nil {
		return nil
	}
	nj := new(Job)
	*nj = *j
	nj.Datacenters = slices.Clone(j.Datacenters)
	nj.Constraints = CopySliceConstraints(j.Constraints)
	nj.Affinities = CopySliceAffinities(j.Affinities)
	nj.Multiregion = j.Multiregion.Copy()
	nj.UI = j.UI.Copy()
	nj.VersionTag = j.VersionTag.Copy()

	if j.TaskGroups != nil {
		tgs := make([]*TaskGroup, len(j.TaskGroups))
		for i, tg := range j.TaskGroups {
			tgs[i] = tg.Copy()
		}
		nj.TaskGroups = tgs
	}

	nj.Periodic = j.Periodic.Copy()
	nj.Meta = maps.Clone(j.Meta)
	nj.ParameterizedJob = j.ParameterizedJob.Copy()
	return nj
}

// Validate is used to check a job for reasonable configuration
func (j *Job) Validate() error {
	var mErr multierror.Error

	if j.Region == "" && j.Multiregion == nil {
		mErr.Errors = append(mErr.Errors, errors.New("Missing job region"))
	}
	if j.ID == "" {
		mErr.Errors = append(mErr.Errors, errors.New("Missing job ID"))
	} else if strings.Contains(j.ID, " ") {
		mErr.Errors = append(mErr.Errors, errors.New("Job ID contains a space"))
	} else if strings.Contains(j.ID, "\000") {
		mErr.Errors = append(mErr.Errors, errors.New("Job ID contains a null character"))
	}
	if j.Name == "" {
		mErr.Errors = append(mErr.Errors, errors.New("Missing job name"))
	} else if strings.Contains(j.Name, "\000") {
		mErr.Errors = append(mErr.Errors, errors.New("Job Name contains a null character"))
	}

	if j.Namespace == "" {
		mErr.Errors = append(mErr.Errors, errors.New("Job must be in a namespace"))
	}
	switch j.Type {
	case JobTypeCore, JobTypeService, JobTypeBatch, JobTypeSystem, JobTypeSysBatch:
	case "":
		mErr.Errors = append(mErr.Errors, errors.New("Missing job type"))
	default:
		mErr.Errors = append(mErr.Errors, fmt.Errorf("Invalid job type: %q", j.Type))
	}
	if len(j.Datacenters) == 0 && !j.IsMultiregion() {
		mErr.Errors = append(mErr.Errors, errors.New("Missing job datacenters"))
	} else {
		for _, v := range j.Datacenters {
			if v == "" {
				mErr.Errors = append(mErr.Errors, errors.New("Job datacenter must be non-empty string"))
			}
		}
	}

	if len(j.TaskGroups) == 0 {
		mErr.Errors = append(mErr.Errors, errors.New("Missing job task groups"))
	}
	for idx, constr := range j.Constraints {
		if err := constr.Validate(); err != nil {
			outer := fmt.Errorf("Constraint %d validation failed: %s", idx+1, err)
			mErr.Errors = append(mErr.Errors, outer)
		}
	}
	if j.Type == JobTypeSystem {
		if j.Affinities != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("System jobs may not have an affinity block"))
		}
	} else {
		for idx, affinity := range j.Affinities {
			if err := affinity.Validate(); err != nil {
				outer := fmt.Errorf("Affinity %d validation failed: %s", idx+1, err)
				mErr.Errors = append(mErr.Errors, outer)
			}
		}
	}

	if j.Type == JobTypeSystem {
		if j.Spreads != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("System jobs may not have a spread block"))
		}
	} else {
		for idx, spread := range j.Spreads {
			if err := spread.Validate(); err != nil {
				outer := fmt.Errorf("Spread %d validation failed: %s", idx+1, err)
				mErr.Errors = append(mErr.Errors, outer)
			}
		}
	}

	const MaxDescriptionCharacters = 1000
	if j.UI != nil {
		if len(j.UI.Description) > MaxDescriptionCharacters {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("UI description must be under 1000 characters, currently %d", len(j.UI.Description)))
		}
	}

	if j.VersionTag != nil {
		if len(j.VersionTag.Description) > MaxDescriptionCharacters {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Tagged version description must be under 1000 characters, currently %d", len(j.VersionTag.Description)))
		}
	}

	// Check for duplicate task groups
	taskGroups := make(map[string]int)
	for idx, tg := range j.TaskGroups {
		if tg.Name == "" {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Job task group %d missing name", idx+1))
		} else if existing, ok := taskGroups[tg.Name]; ok {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Job task group %d redefines '%s' from group %d", idx+1, tg.Name, existing+1))
		} else {
			taskGroups[tg.Name] = idx
		}

		if tg.ShutdownDelay != nil && *tg.ShutdownDelay < 0 {
			mErr.Errors = append(mErr.Errors, errors.New("ShutdownDelay must be a positive value"))
		}

		if tg.StopAfterClientDisconnect != nil && *tg.StopAfterClientDisconnect != 0 {
			if *tg.StopAfterClientDisconnect > 0 &&
				!(j.Type == JobTypeBatch || j.Type == JobTypeService) {
				mErr.Errors = append(mErr.Errors, errors.New("stop_after_client_disconnect can only be set in batch and service jobs"))
			} else if *tg.StopAfterClientDisconnect < 0 {
				mErr.Errors = append(mErr.Errors, errors.New("stop_after_client_disconnect must be a positive value"))
			}
		}

		if j.Type == "system" && tg.Count > 1 {
			mErr.Errors = append(mErr.Errors,
				fmt.Errorf("Job task group %s has count %d. Count cannot exceed 1 with system scheduler",
					tg.Name, tg.Count))
		}

		if tg.MaxClientDisconnect != nil &&
			(tg.ReschedulePolicy != nil && tg.ReschedulePolicy.Attempts > 0) &&
			tg.PreventRescheduleOnLost {
			err := fmt.Errorf("max_client_disconnect and prevent_reschedule_on_lost cannot be enabled when rechedule.attempts > 0")
			mErr.Errors = append(mErr.Errors, err)
		}
	}

	// Validate the task group
	for _, tg := range j.TaskGroups {
		if err := tg.Validate(j); err != nil {
			outer := fmt.Errorf("Task group %s validation failed: %v", tg.Name, err)
			mErr.Errors = append(mErr.Errors, outer)
		}
	}

	// Validate periodic is only used with batch or sysbatch jobs.
	if j.IsPeriodic() && j.Periodic.Enabled {
		if j.Type != JobTypeBatch && j.Type != JobTypeSysBatch {
			mErr.Errors = append(mErr.Errors, fmt.Errorf(
				"Periodic can only be used with %q or %q scheduler", JobTypeBatch, JobTypeSysBatch,
			))
		}

		if err := j.Periodic.Validate(); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
	}

	if j.IsParameterized() {
		if j.Type != JobTypeBatch && j.Type != JobTypeSysBatch {
			mErr.Errors = append(mErr.Errors, fmt.Errorf(
				"Parameterized job can only be used with %q or %q scheduler", JobTypeBatch, JobTypeSysBatch,
			))
		}

		if err := j.ParameterizedJob.Validate(); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
	}

	if j.IsMultiregion() {
		if err := j.Multiregion.Validate(j.Type, j.Datacenters); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
	}

	return mErr.ErrorOrNil()
}

// Warnings returns a list of warnings that may be from dubious settings or
// deprecation warnings.
func (j *Job) Warnings() error {
	var mErr multierror.Error

	// Check the groups
	hasAutoPromote, allAutoPromote := false, true

	for _, tg := range j.TaskGroups {
		if err := tg.Warnings(j); err != nil {
			outer := fmt.Errorf("Group %q has warnings: %v", tg.Name, err)
			mErr.Errors = append(mErr.Errors, outer)
		}

		if u := tg.Update; u != nil {
			hasAutoPromote = hasAutoPromote || u.AutoPromote

			// Having no canaries implies auto-promotion since there are no canaries to promote.
			allAutoPromote = allAutoPromote && (u.Canary == 0 || u.AutoPromote)
		}
	}

	// Check AutoPromote, should be all or none
	if hasAutoPromote && !allAutoPromote {
		err := fmt.Errorf("auto_promote must be true for all groups to enable automatic promotion")
		mErr.Errors = append(mErr.Errors, err)
	}

	// cron -> crons
	if j.Periodic != nil && j.Periodic.Spec != "" {
		err := fmt.Errorf("cron is deprecated and may be removed in a future release. Use crons instead")
		mErr.Errors = append(mErr.Errors, err)
	}

	return mErr.ErrorOrNil()
}

// LookupTaskGroup finds a task group by name
func (j *Job) LookupTaskGroup(name string) *TaskGroup {
	if j == nil {
		return nil
	}
	for _, tg := range j.TaskGroups {
		if tg.Name == name {
			return tg
		}
	}
	return nil
}

// CombinedTaskMeta takes a TaskGroup and Task name and returns the combined
// meta data for the task. When joining Job, Group and Task Meta, the precedence
// is by deepest scope (Task > Group > Job).
func (j *Job) CombinedTaskMeta(groupName, taskName string) map[string]string {
	group := j.LookupTaskGroup(groupName)
	if group == nil {
		return j.Meta
	}

	var meta map[string]string

	task := group.LookupTask(taskName)
	if task != nil {
		meta = maps.Clone(task.Meta)
	}

	if meta == nil {
		meta = make(map[string]string, len(group.Meta)+len(j.Meta))
	}

	// Add the group specific meta
	for k, v := range group.Meta {
		if _, ok := meta[k]; !ok {
			meta[k] = v
		}
	}

	// Add the job specific meta
	for k, v := range j.Meta {
		if _, ok := meta[k]; !ok {
			meta[k] = v
		}
	}

	return meta
}

// Stopped returns if a job is stopped.
func (j *Job) Stopped() bool {
	return j == nil || j.Stop
}

// HasUpdateStrategy returns if any task group in the job has an update strategy
func (j *Job) HasUpdateStrategy() bool {
	for _, tg := range j.TaskGroups {
		if !tg.Update.IsEmpty() {
			return true
		}
	}

	return false
}

// Stub is used to return a summary of the job
func (j *Job) Stub(summary *JobSummary, fields *JobStubFields) *JobListStub {
	jobStub := &JobListStub{
		ID:                j.ID,
		Namespace:         j.Namespace,
		ParentID:          j.ParentID,
		Name:              j.Name,
		Datacenters:       j.Datacenters,
		NodePool:          j.NodePool,
		Multiregion:       j.Multiregion,
		Type:              j.Type,
		Priority:          j.Priority,
		Periodic:          j.IsPeriodic(),
		ParameterizedJob:  j.IsParameterized(),
		Stop:              j.Stop,
		Status:            j.Status,
		StatusDescription: j.StatusDescription,
		CreateIndex:       j.CreateIndex,
		ModifyIndex:       j.ModifyIndex,
		JobModifyIndex:    j.JobModifyIndex,
		SubmitTime:        j.SubmitTime,
		JobSummary:        summary,
	}

	if fields != nil {
		if fields.Meta {
			jobStub.Meta = j.Meta
		}
	}

	return jobStub
}

// IsPeriodic returns whether a job is periodic.
func (j *Job) IsPeriodic() bool {
	return j.Periodic != nil
}

// IsPeriodicActive returns whether the job is an active periodic job that will
// create child jobs
func (j *Job) IsPeriodicActive() bool {
	return j.IsPeriodic() && j.Periodic.Enabled && !j.Stopped() && !j.IsParameterized()
}

// IsParameterized returns whether a job is parameterized job.
func (j *Job) IsParameterized() bool {
	return j.ParameterizedJob != nil && !j.Dispatched
}

// IsMultiregion returns whether a job is multiregion
func (j *Job) IsMultiregion() bool {
	return j.Multiregion != nil && j.Multiregion.Regions != nil && len(j.Multiregion.Regions) > 0
}

// IsPlugin returns whether a job implements a plugin (currently just CSI)
func (j *Job) IsPlugin() bool {
	for _, tg := range j.TaskGroups {
		for _, task := range tg.Tasks {
			if task.CSIPluginConfig != nil {
				return true
			}
		}
	}
	return false
}

// HasPlugin returns whether a job implements a specific plugin ID
func (j *Job) HasPlugin(id string) bool {
	for _, tg := range j.TaskGroups {
		for _, task := range tg.Tasks {
			if task.CSIPluginConfig != nil && task.CSIPluginConfig.ID == id {
				return true
			}
		}
	}
	return false
}

// Vault returns the set of Vault blocks per task group, per task
func (j *Job) Vault() map[string]map[string]*Vault {
	blocks := make(map[string]map[string]*Vault, len(j.TaskGroups))

	for _, tg := range j.TaskGroups {
		tgBlocks := make(map[string]*Vault, len(tg.Tasks))

		for _, task := range tg.Tasks {
			if task.Vault == nil {
				continue
			}

			tgBlocks[task.Name] = task.Vault
		}

		if len(tgBlocks) != 0 {
			blocks[tg.Name] = tgBlocks
		}
	}

	return blocks
}

// ConnectTasks returns the set of Consul Connect enabled tasks defined on the
// job that will require a Service Identity token in the case that Consul ACLs
// are enabled. The TaskKind.Value is the name of the Consul service.
//
// This method is meaningful only after the Job has passed through the job
// submission Mutator functions.
func (j *Job) ConnectTasks() []TaskKind {
	var kinds []TaskKind
	for _, tg := range j.TaskGroups {
		for _, task := range tg.Tasks {
			if task.Kind.IsConnectProxy() ||
				task.Kind.IsConnectNative() ||
				task.Kind.IsAnyConnectGateway() {
				kinds = append(kinds, task.Kind)
			}
		}
	}
	return kinds
}

// RequiredSignals returns a mapping of task groups to tasks to their required
// set of signals
func (j *Job) RequiredSignals() map[string]map[string][]string {
	signals := make(map[string]map[string][]string)

	for _, tg := range j.TaskGroups {
		for _, task := range tg.Tasks {
			// Use this local one as a set
			taskSignals := make(map[string]struct{})

			// Check if the Vault change mode uses signals
			if task.Vault != nil && task.Vault.ChangeMode == VaultChangeModeSignal {
				taskSignals[task.Vault.ChangeSignal] = struct{}{}
			}

			// If a user has specified a KillSignal, add it to required signals
			if task.KillSignal != "" {
				taskSignals[task.KillSignal] = struct{}{}
			}

			// Check if any template change mode uses signals
			for _, t := range task.Templates {
				if t.ChangeMode != TemplateChangeModeSignal {
					continue
				}

				taskSignals[t.ChangeSignal] = struct{}{}
			}

			// Flatten and sort the signals
			l := len(taskSignals)
			if l == 0 {
				continue
			}

			flat := make([]string, 0, l)
			for sig := range taskSignals {
				flat = append(flat, sig)
			}

			sort.Strings(flat)
			tgSignals, ok := signals[tg.Name]
			if !ok {
				tgSignals = make(map[string][]string)
				signals[tg.Name] = tgSignals
			}
			tgSignals[task.Name] = flat
		}

	}

	return signals
}

// SpecChanged determines if the functional specification has changed between
// two job versions.
func (j *Job) SpecChanged(new *Job) bool {
	if j == nil {
		return new != nil
	}

	// Create a copy of the new job
	c := new.Copy()

	// Update the new job so we can do a reflect
	c.Status = j.Status
	c.StatusDescription = j.StatusDescription
	c.Stable = j.Stable
	c.Version = j.Version
	c.CreateIndex = j.CreateIndex
	c.ModifyIndex = j.ModifyIndex
	c.JobModifyIndex = j.JobModifyIndex
	c.SubmitTime = j.SubmitTime

	// cgbaker: FINISH: probably need some consideration of scaling policy ID here

	// Deep equals the jobs
	return !reflect.DeepEqual(j, c)
}

func (j *Job) SetSubmitTime() {
	j.SubmitTime = time.Now().UTC().UnixNano()
}

// JobListStub is used to return a subset of job information
// for the job list
type JobListStub struct {
	ID                string
	ParentID          string
	Name              string
	Namespace         string `json:",omitempty"`
	Datacenters       []string
	NodePool          string
	Multiregion       *Multiregion
	Type              string
	Priority          int
	Periodic          bool
	ParameterizedJob  bool
	Stop              bool
	Status            string
	StatusDescription string
	JobSummary        *JobSummary
	CreateIndex       uint64
	ModifyIndex       uint64
	JobModifyIndex    uint64
	SubmitTime        int64
	Meta              map[string]string `json:",omitempty"`
}

// JobSummary summarizes the state of the allocations of a job
type JobSummary struct {
	// JobID is the ID of the job the summary is for
	JobID string

	// Namespace is the namespace of the job and its summary
	Namespace string

	// Summary contains the summary per task group for the Job
	Summary map[string]TaskGroupSummary

	// Children contains a summary for the children of this job.
	Children *JobChildrenSummary

	// Raft Indexes
	CreateIndex uint64
	ModifyIndex uint64
}

// Copy returns a new copy of JobSummary
func (js *JobSummary) Copy() *JobSummary {
	newJobSummary := new(JobSummary)
	*newJobSummary = *js
	newTGSummary := make(map[string]TaskGroupSummary, len(js.Summary))
	for k, v := range js.Summary {
		newTGSummary[k] = v
	}
	newJobSummary.Summary = newTGSummary
	newJobSummary.Children = newJobSummary.Children.Copy()
	return newJobSummary
}

// JobChildrenSummary contains the summary of children job statuses
type JobChildrenSummary struct {
	Pending int64
	Running int64
	Dead    int64
}

// Copy returns a new copy of a JobChildrenSummary
func (jc *JobChildrenSummary) Copy() *JobChildrenSummary {
	if jc == nil {
		return nil
	}

	njc := new(JobChildrenSummary)
	*njc = *jc
	return njc
}

// TaskGroupSummary summarizes the state of all the allocations of a particular
// TaskGroup
type TaskGroupSummary struct {
	Queued   int
	Complete int
	Failed   int
	Running  int
	Starting int
	Lost     int
	Unknown  int
}

const (
	// Checks uses any registered health check state in combination with task
	// states to determine if an allocation is healthy.
	UpdateStrategyHealthCheck_Checks = "checks"

	// TaskStates uses the task states of an allocation to determine if the
	// allocation is healthy.
	UpdateStrategyHealthCheck_TaskStates = "task_states"

	// Manual allows the operator to manually signal to Nomad when an
	// allocations is healthy. This allows more advanced health checking that is
	// outside of the scope of Nomad.
	UpdateStrategyHealthCheck_Manual = "manual"
)

var (
	// DefaultUpdateStrategy provides a baseline that can be used to upgrade
	// jobs with the old policy or for populating field defaults.
	DefaultUpdateStrategy = &UpdateStrategy{
		Stagger:          30 * time.Second,
		MaxParallel:      1,
		HealthCheck:      UpdateStrategyHealthCheck_Checks,
		MinHealthyTime:   10 * time.Second,
		HealthyDeadline:  5 * time.Minute,
		ProgressDeadline: 10 * time.Minute,
		AutoRevert:       false,
		AutoPromote:      false,
		Canary:           0,
	}
)

// UpdateStrategy is used to modify how updates are done
type UpdateStrategy struct {
	// Stagger is used to determine the rate at which allocations are migrated
	// due to down or draining nodes.
	Stagger time.Duration

	// MaxParallel is how many updates can be done in parallel
	MaxParallel int

	// HealthCheck specifies the mechanism in which allocations are marked
	// healthy or unhealthy as part of a deployment.
	HealthCheck string

	// MinHealthyTime is the minimum time an allocation must be in the healthy
	// state before it is marked as healthy, unblocking more allocations to be
	// rolled.
	MinHealthyTime time.Duration

	// HealthyDeadline is the time in which an allocation must be marked as
	// healthy before it is automatically transitioned to unhealthy. This time
	// period doesn't count against the MinHealthyTime.
	HealthyDeadline time.Duration

	// ProgressDeadline is the time in which an allocation as part of the
	// deployment must transition to healthy. If no allocation becomes healthy
	// after the deadline, the deployment is marked as failed. If the deadline
	// is zero, the first failure causes the deployment to fail.
	ProgressDeadline time.Duration

	// AutoRevert declares that if a deployment fails because of unhealthy
	// allocations, there should be an attempt to auto-revert the job to a
	// stable version.
	AutoRevert bool

	// AutoPromote declares that the deployment should be promoted when all canaries are
	// healthy
	AutoPromote bool

	// Canary is the number of canaries to deploy when a change to the task
	// group is detected.
	Canary int
}

func (u *UpdateStrategy) Copy() *UpdateStrategy {
	if u == nil {
		return nil
	}

	c := new(UpdateStrategy)
	*c = *u
	return c
}

func (u *UpdateStrategy) Validate() error {
	if u == nil {
		return nil
	}

	var mErr multierror.Error
	switch u.HealthCheck {
	case UpdateStrategyHealthCheck_Checks, UpdateStrategyHealthCheck_TaskStates, UpdateStrategyHealthCheck_Manual:
	default:
		_ = multierror.Append(&mErr, fmt.Errorf("Invalid health check given: %q", u.HealthCheck))
	}

	if u.MaxParallel < 0 {
		_ = multierror.Append(&mErr, fmt.Errorf("Max parallel can not be less than zero: %d < 0", u.MaxParallel))
	}
	if u.Canary < 0 {
		_ = multierror.Append(&mErr, fmt.Errorf("Canary count can not be less than zero: %d < 0", u.Canary))
	}
	if u.Canary == 0 && u.AutoPromote {
		_ = multierror.Append(&mErr, fmt.Errorf("Auto Promote requires a Canary count greater than zero"))
	}
	if u.MinHealthyTime < 0 {
		_ = multierror.Append(&mErr, fmt.Errorf("Minimum healthy time may not be less than zero: %v", u.MinHealthyTime))
	}
	if u.HealthyDeadline <= 0 {
		_ = multierror.Append(&mErr, fmt.Errorf("Healthy deadline must be greater than zero: %v", u.HealthyDeadline))
	}
	if u.ProgressDeadline < 0 {
		_ = multierror.Append(&mErr, fmt.Errorf("Progress deadline must be zero or greater: %v", u.ProgressDeadline))
	}
	if u.MinHealthyTime >= u.HealthyDeadline {
		_ = multierror.Append(&mErr, fmt.Errorf("Minimum healthy time must be less than healthy deadline: %v > %v", u.MinHealthyTime, u.HealthyDeadline))
	}
	if u.ProgressDeadline != 0 && u.HealthyDeadline >= u.ProgressDeadline {
		_ = multierror.Append(&mErr, fmt.Errorf("Healthy deadline must be less than progress deadline: %v > %v", u.HealthyDeadline, u.ProgressDeadline))
	}
	if u.Stagger <= 0 {
		_ = multierror.Append(&mErr, fmt.Errorf("Stagger must be greater than zero: %v", u.Stagger))
	}

	return mErr.ErrorOrNil()
}

func (u *UpdateStrategy) IsEmpty() bool {
	if u == nil {
		return true
	}

	// When the Job is transformed from api to struct, the Update Strategy block is
	// copied into the existing task groups, the only things that are passed along
	// are MaxParallel and Stagger, because they are enforced at job level.
	// That is why checking if MaxParallel is zero is enough to know if the
	// update block is empty.

	return u.MaxParallel == 0
}

// Rolling returns if a rolling strategy should be used.
// TODO(alexdadgar): Remove once no longer used by the scheduler.
func (u *UpdateStrategy) Rolling() bool {
	return u.Stagger > 0 && u.MaxParallel > 0
}

type Multiregion struct {
	Strategy *MultiregionStrategy
	Regions  []*MultiregionRegion
}

func (m *Multiregion) Canonicalize() {
	if m.Strategy == nil {
		m.Strategy = &MultiregionStrategy{}
	}
	if m.Regions == nil {
		m.Regions = []*MultiregionRegion{}
	}
}

// Diff indicates whether the multiregion config has changed
func (m *Multiregion) Diff(m2 *Multiregion) bool {
	return !reflect.DeepEqual(m, m2)
}

func (m *Multiregion) Copy() *Multiregion {
	if m == nil {
		return nil
	}
	copy := new(Multiregion)
	if m.Strategy != nil {
		copy.Strategy = &MultiregionStrategy{
			MaxParallel: m.Strategy.MaxParallel,
			OnFailure:   m.Strategy.OnFailure,
		}
	}
	for _, region := range m.Regions {
		copyRegion := &MultiregionRegion{
			Name:        region.Name,
			Count:       region.Count,
			Datacenters: []string{},
			NodePool:    region.NodePool,
			Meta:        map[string]string{},
		}
		copyRegion.Datacenters = append(copyRegion.Datacenters, region.Datacenters...)
		for k, v := range region.Meta {
			copyRegion.Meta[k] = v
		}
		copy.Regions = append(copy.Regions, copyRegion)
	}
	return copy
}

type MultiregionStrategy struct {
	MaxParallel int
	OnFailure   string
}

type MultiregionRegion struct {
	Name        string
	Count       int
	Datacenters []string
	NodePool    string
	Meta        map[string]string
}

// Namespace allows logically grouping jobs and their associated objects.
type Namespace struct {
	// Name is the name of the namespace
	Name string

	// Description is a human readable description of the namespace
	Description string

	// Quota is the quota specification that the namespace should account
	// against.
	Quota string

	// Capabilities is the set of capabilities allowed for this namespace
	Capabilities *NamespaceCapabilities

	// NodePoolConfiguration is the namespace configuration for handling node
	// pools.
	NodePoolConfiguration *NamespaceNodePoolConfiguration

	VaultConfiguration  *NamespaceVaultConfiguration
	ConsulConfiguration *NamespaceConsulConfiguration

	// Meta is the set of metadata key/value pairs that attached to the namespace
	Meta map[string]string

	// Hash is the hash of the namespace which is used to efficiently replicate
	// cross-regions.
	Hash []byte

	// Raft Indexes
	CreateIndex uint64
	ModifyIndex uint64
}

// NamespaceCapabilities represents a set of capabilities allowed for this
// namespace, to be checked at job submission time.
type NamespaceCapabilities struct {
	EnabledTaskDrivers   []string
	DisabledTaskDrivers  []string
	EnabledNetworkModes  []string
	DisabledNetworkModes []string
}

// NamespaceNodePoolConfiguration stores configuration about node pools for a
// namespace.
type NamespaceNodePoolConfiguration struct {
	// Default is the node pool used by jobs in this namespace that don't
	// specify a node pool of their own.
	Default string

	// Allowed specifies the node pools that are allowed to be used by jobs in
	// this namespace. By default, all node pools are allowed. If an empty list
	// is provided only the namespace's default node pool is allowed. This field
	// supports wildcard globbing through the use of `*` for multi-character
	// matching. This field cannot be used with Denied.
	Allowed []string

	// Denied specifies the node pools that are not allowed to be used by jobs
	// in this namespace. This field supports wildcard globbing through the use
	// of `*` for multi-character matching. If specified, any node pool is
	// allowed to be used, except for those that match any of these patterns.
	// This field cannot be used with Allowed.
	Denied []string
}

func (n *Namespace) Validate() error {
	var mErr multierror.Error

	// Validate the name and description
	if !validNamespaceName.MatchString(n.Name) {
		err := fmt.Errorf("invalid name %q. Must match regex %s", n.Name, validNamespaceName)
		mErr.Errors = append(mErr.Errors, err)
	}
	if len(n.Description) > maxNamespaceDescriptionLength {
		err := fmt.Errorf("description longer than %d", maxNamespaceDescriptionLength)
		mErr.Errors = append(mErr.Errors, err)
	}

	err := n.NodePoolConfiguration.Validate()
	switch e := err.(type) {
	case *multierror.Error:
		for _, npErr := range e.Errors {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("invalid node pool configuration: %v", npErr))
		}
	case error:
		mErr.Errors = append(mErr.Errors, fmt.Errorf("invalid node pool configuration: %v", e))
	}

	err = n.VaultConfiguration.Validate()
	switch e := err.(type) {
	case *multierror.Error:
		for _, vErr := range e.Errors {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("invalid vault configuration: %v", vErr))
		}
	case error:
		mErr.Errors = append(mErr.Errors, fmt.Errorf("invalid vault configuration: %v", e))
	}

	err = n.ConsulConfiguration.Validate()
	switch e := err.(type) {
	case *multierror.Error:
		for _, cErr := range e.Errors {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("invalid consul configuration: %v", cErr))
		}
	case error:
		mErr.Errors = append(mErr.Errors, fmt.Errorf("invalid consul configuration: %v", e))
	}

	return mErr.ErrorOrNil()
}

// SetHash is used to compute and set the hash of the namespace
func (n *Namespace) SetHash() []byte {
	// Initialize a 256bit Blake2 hash (32 bytes)
	hash, err := blake2b.New256(nil)
	if err != nil {
		panic(err)
	}

	// Write all the user set fields
	_, _ = hash.Write([]byte(n.Name))
	_, _ = hash.Write([]byte(n.Description))
	_, _ = hash.Write([]byte(n.Quota))
	if n.Capabilities != nil {
		for _, driver := range n.Capabilities.EnabledTaskDrivers {
			_, _ = hash.Write([]byte(driver))
		}
		for _, driver := range n.Capabilities.DisabledTaskDrivers {
			_, _ = hash.Write([]byte(driver))
		}
		for _, mode := range n.Capabilities.EnabledNetworkModes {
			_, _ = hash.Write([]byte(mode))
		}
		for _, mode := range n.Capabilities.DisabledNetworkModes {
			_, _ = hash.Write([]byte(mode))
		}
	}
	if n.NodePoolConfiguration != nil {
		_, _ = hash.Write([]byte(n.NodePoolConfiguration.Default))
		for _, pool := range n.NodePoolConfiguration.Allowed {
			_, _ = hash.Write([]byte(pool))
		}
		for _, pool := range n.NodePoolConfiguration.Denied {
			_, _ = hash.Write([]byte(pool))
		}
	}

	if n.VaultConfiguration != nil {
		_, _ = hash.Write([]byte(n.VaultConfiguration.Default))
		for _, cluster := range n.VaultConfiguration.Allowed {
			_, _ = hash.Write([]byte(cluster))
		}
		for _, cluster := range n.VaultConfiguration.Denied {
			_, _ = hash.Write([]byte(cluster))
		}
	}

	if n.ConsulConfiguration != nil {
		_, _ = hash.Write([]byte(n.ConsulConfiguration.Default))
		for _, cluster := range n.ConsulConfiguration.Allowed {
			_, _ = hash.Write([]byte(cluster))
		}
		for _, cluster := range n.ConsulConfiguration.Denied {
			_, _ = hash.Write([]byte(cluster))
		}
	}

	// sort keys to ensure hash stability when meta is stored later
	var keys []string
	for k := range n.Meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		_, _ = hash.Write([]byte(k))
		_, _ = hash.Write([]byte(n.Meta[k]))
	}

	// Finalize the hash
	hashVal := hash.Sum(nil)

	// Set and return the hash
	n.Hash = hashVal
	return hashVal
}

func (n *Namespace) Copy() *Namespace {
	nc := new(Namespace)
	*nc = *n
	nc.Hash = make([]byte, len(n.Hash))
	if n.Capabilities != nil {
		c := new(NamespaceCapabilities)
		*c = *n.Capabilities
		c.EnabledTaskDrivers = slices.Clone(n.Capabilities.EnabledTaskDrivers)
		c.DisabledTaskDrivers = slices.Clone(n.Capabilities.DisabledTaskDrivers)
		c.EnabledNetworkModes = slices.Clone(n.Capabilities.EnabledNetworkModes)
		c.DisabledNetworkModes = slices.Clone(n.Capabilities.DisabledNetworkModes)
		nc.Capabilities = c
	}
	if n.NodePoolConfiguration != nil {
		np := new(NamespaceNodePoolConfiguration)
		*np = *n.NodePoolConfiguration
		np.Allowed = slices.Clone(n.NodePoolConfiguration.Allowed)
		np.Denied = slices.Clone(n.NodePoolConfiguration.Denied)
	}
	if n.VaultConfiguration != nil {
		nv := new(NamespaceVaultConfiguration)
		*nv = *n.VaultConfiguration
		nv.Allowed = slices.Clone(n.VaultConfiguration.Allowed)
		nv.Denied = slices.Clone(n.VaultConfiguration.Denied)
	}
	if n.ConsulConfiguration != nil {
		nc := new(NamespaceConsulConfiguration)
		*nc = *n.ConsulConfiguration
		nc.Allowed = slices.Clone(n.ConsulConfiguration.Allowed)
		nc.Denied = slices.Clone(n.ConsulConfiguration.Denied)
	}

	if n.Meta != nil {
		nc.Meta = make(map[string]string, len(n.Meta))
		for k, v := range n.Meta {
			nc.Meta[k] = v
		}
	}
	copy(nc.Hash, n.Hash)
	return nc
}

// NamespaceListRequest is used to request a list of namespaces
type NamespaceListRequest struct {
	QueryOptions
}

// NamespaceListResponse is used for a list request
type NamespaceListResponse struct {
	Namespaces []*Namespace
	QueryMeta
}

// NamespaceSpecificRequest is used to query a specific namespace
type NamespaceSpecificRequest struct {
	Name string
	QueryOptions
}

// SingleNamespaceResponse is used to return a single namespace
type SingleNamespaceResponse struct {
	Namespace *Namespace
	QueryMeta
}

// NamespaceSetRequest is used to query a set of namespaces
type NamespaceSetRequest struct {
	Namespaces []string
	QueryOptions
}

// NamespaceSetResponse is used to return a set of namespaces
type NamespaceSetResponse struct {
	Namespaces map[string]*Namespace // Keyed by namespace Name
	QueryMeta
}

// NamespaceDeleteRequest is used to delete a set of namespaces
type NamespaceDeleteRequest struct {
	Namespaces []string
	WriteRequest
}

// NamespaceUpsertRequest is used to upsert a set of namespaces
type NamespaceUpsertRequest struct {
	Namespaces []*Namespace
	WriteRequest
}

const (
	// PeriodicSpecCron is used for a cron spec.
	PeriodicSpecCron = "cron"

	// PeriodicSpecTest is only used by unit tests. It is a sorted, comma
	// separated list of unix timestamps at which to launch.
	PeriodicSpecTest = "_internal_test"
)

// Periodic defines the interval a job should be run at.
type PeriodicConfig struct {
	// Enabled determines if the job should be run periodically.
	Enabled bool

	// Spec specifies the interval the job should be run as. It is parsed based
	// on the SpecType.
	Spec string

	// Specs specifies the intervals the job should be run as. It is parsed based
	// on the SpecType.
	Specs []string

	// SpecType defines the format of the spec.
	SpecType string

	// ProhibitOverlap enforces that spawned jobs do not run in parallel.
	ProhibitOverlap bool

	// TimeZone is the user specified string that determines the time zone to
	// launch against. The time zones must be specified from IANA Time Zone
	// database, such as "America/New_York".
	// Reference: https://en.wikipedia.org/wiki/List_of_tz_database_time_zones
	// Reference: https://www.iana.org/time-zones
	TimeZone string

	// location is the time zone to evaluate the launch time against
	location *time.Location
}

func (p *PeriodicConfig) Copy() *PeriodicConfig {
	if p == nil {
		return nil
	}
	np := new(PeriodicConfig)
	*np = *p
	return np
}

func (p *PeriodicConfig) Validate() error {
	if !p.Enabled {
		return nil
	}

	var mErr multierror.Error
	if p.Spec != "" && len(p.Specs) != 0 {
		_ = multierror.Append(&mErr, fmt.Errorf("Only cron or crons may be used"))
	}
	if p.Spec == "" && len(p.Specs) == 0 {
		_ = multierror.Append(&mErr, fmt.Errorf("Must specify a spec"))
	}

	// Check if we got a valid time zone
	if p.TimeZone != "" {
		if _, err := time.LoadLocation(p.TimeZone); err != nil {
			_ = multierror.Append(&mErr, fmt.Errorf("Invalid time zone %q: %v", p.TimeZone, err))
		}
	}

	switch p.SpecType {
	case PeriodicSpecCron:
		// Validate the cron spec
		if p.Spec != "" {
			if _, err := cronexpr.Parse(p.Spec); err != nil {
				_ = multierror.Append(&mErr, fmt.Errorf("Invalid cron spec %q: %v", p.Spec, err))
			}
		}
		// Validate the cron specs
		for _, spec := range p.Specs {
			if _, err := cronexpr.Parse(spec); err != nil {
				_ = multierror.Append(&mErr, fmt.Errorf("Invalid cron spec %q: %v", spec, err))
			}
		}

	case PeriodicSpecTest:
		// No-op
	default:
		_ = multierror.Append(&mErr, fmt.Errorf("Unknown periodic specification type %q", p.SpecType))
	}

	return mErr.ErrorOrNil()
}

func (p *PeriodicConfig) Canonicalize() {
	// Load the location
	l, err := time.LoadLocation(p.TimeZone)
	if err != nil {
		p.location = time.UTC
	}

	p.location = l
}

// CronParseNext is a helper that parses the next time for the given expression
// but captures any panic that may occur in the underlying library.
func CronParseNext(fromTime time.Time, spec string) (t time.Time, err error) {
	defer func() {
		if recover() != nil {
			t = time.Time{}
			err = fmt.Errorf("failed parsing cron expression: %q", spec)
		}
	}()
	exp, err := cronexpr.Parse(spec)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed parsing cron expression: %s: %v", spec, err)
	}
	return exp.Next(fromTime), nil
}

// Next returns the closest time instant matching the spec that is after the
// passed time. If no matching instance exists, the zero value of time.Time is
// returned. The `time.Location` of the returned value matches that of the
// passed time.
func (p *PeriodicConfig) Next(fromTime time.Time) (time.Time, error) {
	switch p.SpecType {
	case PeriodicSpecCron:
		// Single spec parsing
		if p.Spec != "" {
			return CronParseNext(fromTime, p.Spec)
		}

		// multiple specs parsing
		var nextTime time.Time
		for _, spec := range p.Specs {
			t, err := CronParseNext(fromTime, spec)
			if err != nil {
				return time.Time{}, fmt.Errorf("failed parsing cron expression %s: %v", spec, err)
			}
			if nextTime.IsZero() || t.Before(nextTime) {
				nextTime = t
			}
		}
		return nextTime, nil

	case PeriodicSpecTest:
		split := strings.Split(p.Spec, ",")
		if len(split) == 1 && split[0] == "" {
			return time.Time{}, nil
		}

		// Parse the times
		times := make([]time.Time, len(split))
		for i, s := range split {
			unix, err := strconv.Atoi(s)
			if err != nil {
				return time.Time{}, nil
			}

			times[i] = time.Unix(int64(unix), 0)
		}

		// Find the next match
		for _, next := range times {
			if fromTime.Before(next) {
				return next, nil
			}
		}
	}

	return time.Time{}, nil
}

// GetLocation returns the location to use for determining the time zone to run
// the periodic job against.
func (p *PeriodicConfig) GetLocation() *time.Location {
	// Jobs pre 0.5.5 will not have this
	if p.location != nil {
		return p.location
	}

	return time.UTC
}

const (
	// PeriodicLaunchSuffix is the string appended to the periodic jobs ID
	// when launching derived instances of it.
	PeriodicLaunchSuffix = "/periodic-"
)

// PeriodicLaunch tracks the last launch time of a periodic job.
type PeriodicLaunch struct {
	ID        string    // ID of the periodic job.
	Namespace string    // Namespace of the periodic job
	Launch    time.Time // The last launch time.

	// Raft Indexes
	CreateIndex uint64
	ModifyIndex uint64
}

const (
	DispatchPayloadForbidden = "forbidden"
	DispatchPayloadOptional  = "optional"
	DispatchPayloadRequired  = "required"

	// DispatchLaunchSuffix is the string appended to the parameterized job's ID
	// when dispatching instances of it.
	DispatchLaunchSuffix = "/dispatch-"
)

// ParameterizedJobConfig is used to configure the parameterized job
type ParameterizedJobConfig struct {
	// Payload configure the payload requirements
	Payload string

	// MetaRequired is metadata keys that must be specified by the dispatcher
	MetaRequired []string

	// MetaOptional is metadata keys that may be specified by the dispatcher
	MetaOptional []string
}

func (d *ParameterizedJobConfig) Validate() error {
	var mErr multierror.Error
	switch d.Payload {
	case DispatchPayloadOptional, DispatchPayloadRequired, DispatchPayloadForbidden:
	default:
		_ = multierror.Append(&mErr, fmt.Errorf("Unknown payload requirement: %q", d.Payload))
	}

	// Check that the meta configurations are disjoint sets
	disjoint, offending := helper.IsDisjoint(d.MetaRequired, d.MetaOptional)
	if !disjoint {
		_ = multierror.Append(&mErr, fmt.Errorf("Required and optional meta keys should be disjoint. Following keys exist in both: %v", offending))
	}

	return mErr.ErrorOrNil()
}

func (d *ParameterizedJobConfig) Canonicalize() {
	if d.Payload == "" {
		d.Payload = DispatchPayloadOptional
	}
}

func (d *ParameterizedJobConfig) Copy() *ParameterizedJobConfig {
	if d == nil {
		return nil
	}
	nd := new(ParameterizedJobConfig)
	*nd = *d
	nd.MetaOptional = slices.Clone(nd.MetaOptional)
	nd.MetaRequired = slices.Clone(nd.MetaRequired)
	return nd
}

// DispatchedID returns an ID appropriate for a job dispatched against a
// particular parameterized job
func DispatchedID(templateID, idPrefixTemplate string, t time.Time) string {
	u := uuid.Generate()[:8]

	if idPrefixTemplate != "" {
		return fmt.Sprintf("%s%s%s-%d-%s", templateID, DispatchLaunchSuffix, idPrefixTemplate, t.Unix(), u)
	}

	return fmt.Sprintf("%s%s%d-%s", templateID, DispatchLaunchSuffix, t.Unix(), u)
}

// DispatchPayloadConfig configures how a task gets its input from a job dispatch
type DispatchPayloadConfig struct {
	// File specifies a relative path to where the input data should be written
	File string
}

func (d *DispatchPayloadConfig) Copy() *DispatchPayloadConfig {
	if d == nil {
		return nil
	}
	nd := new(DispatchPayloadConfig)
	*nd = *d
	return nd
}

func (d *DispatchPayloadConfig) Validate() error {
	// Verify the destination doesn't escape
	escaped, err := escapingfs.PathEscapesAllocViaRelative("task/local/", d.File)
	if err != nil {
		return fmt.Errorf("invalid destination path: %v", err)
	} else if escaped {
		return fmt.Errorf("destination escapes allocation directory")
	}

	return nil
}

const (
	TaskLifecycleHookPrestart  = "prestart"
	TaskLifecycleHookPoststart = "poststart"
	TaskLifecycleHookPoststop  = "poststop"
)

type TaskLifecycleConfig struct {
	Hook    string
	Sidecar bool
}

func (d *TaskLifecycleConfig) Copy() *TaskLifecycleConfig {
	if d == nil {
		return nil
	}
	nd := new(TaskLifecycleConfig)
	*nd = *d
	return nd
}

func (d *TaskLifecycleConfig) Validate() error {
	if d == nil {
		return nil
	}

	switch d.Hook {
	case TaskLifecycleHookPrestart:
	case TaskLifecycleHookPoststart:
	case TaskLifecycleHookPoststop:
	case "":
		return fmt.Errorf("no lifecycle hook provided")
	default:
		return fmt.Errorf("invalid hook: %v", d.Hook)
	}

	return nil
}

var (
	// These default restart policies needs to be in sync with
	// Canonicalize in api/tasks.go

	DefaultServiceJobRestartPolicy = RestartPolicy{
		Delay:           15 * time.Second,
		Attempts:        2,
		Interval:        30 * time.Minute,
		Mode:            RestartPolicyModeFail,
		RenderTemplates: false,
	}
	DefaultBatchJobRestartPolicy = RestartPolicy{
		Delay:           15 * time.Second,
		Attempts:        3,
		Interval:        24 * time.Hour,
		Mode:            RestartPolicyModeFail,
		RenderTemplates: false,
	}
)

var (
	// These default reschedule policies needs to be in sync with
	// NewDefaultReschedulePolicy in api/tasks.go

	DefaultServiceJobReschedulePolicy = ReschedulePolicy{
		Delay:         30 * time.Second,
		DelayFunction: "exponential",
		MaxDelay:      1 * time.Hour,
		Unlimited:     true,
	}
	DefaultBatchJobReschedulePolicy = ReschedulePolicy{
		Attempts:      1,
		Interval:      24 * time.Hour,
		Delay:         5 * time.Second,
		DelayFunction: "constant",
	}
)

const (
	// RestartPolicyModeDelay causes an artificial delay till the next interval is
	// reached when the specified attempts have been reached in the interval.
	RestartPolicyModeDelay = "delay"

	// RestartPolicyModeFail causes a job to fail if the specified number of
	// attempts are reached within an interval.
	RestartPolicyModeFail = "fail"

	// RestartPolicyMinInterval is the minimum interval that is accepted for a
	// restart policy.
	RestartPolicyMinInterval = 5 * time.Second

	// ReasonWithinPolicy describes restart events that are within policy
	ReasonWithinPolicy = "Restart within policy"
)

// JobScalingEvents contains the scaling events for a given job
type JobScalingEvents struct {
	Namespace string
	JobID     string

	// This map is indexed by target; currently, this is just task group
	// the indexed array is sorted from newest to oldest event
	// the array should have less than JobTrackedScalingEvents entries
	ScalingEvents map[string][]*ScalingEvent

	// Raft index
	ModifyIndex uint64
}

func (j *JobScalingEvents) Copy() *JobScalingEvents {
	if j == nil {
		return nil
	}
	njse := new(JobScalingEvents)
	*njse = *j

	njse.ScalingEvents = make(map[string][]*ScalingEvent, len(j.ScalingEvents))
	for taskGroup, events := range j.ScalingEvents {
		njse.ScalingEvents[taskGroup] = helper.CopySlice(events)
	}

	return njse
}

// NewScalingEvent method for ScalingEvent objects.
func NewScalingEvent(message string) *ScalingEvent {
	return &ScalingEvent{
		Time:    time.Now().Unix(),
		Message: message,
	}
}

// ScalingEvent describes a scaling event against a Job
type ScalingEvent struct {
	// Unix Nanosecond timestamp for the scaling event
	Time int64

	// Count is the new scaling count, if provided
	Count *int64

	// PreviousCount is the count at the time of the scaling event
	PreviousCount int64

	// Message is the message describing a scaling event
	Message string

	// Error indicates an error state for this scaling event
	Error bool

	// Meta is a map of metadata returned during a scaling event
	Meta map[string]interface{}

	// EvalID is the ID for an evaluation if one was created as part of a scaling event
	EvalID *string

	// Raft index
	CreateIndex uint64
}

func (e *ScalingEvent) Copy() *ScalingEvent {
	if e == nil {
		return nil
	}
	ne := new(ScalingEvent)
	*ne = *e

	ne.Count = pointer.Copy(e.Count)
	ne.Meta = maps.Clone(e.Meta)
	ne.EvalID = pointer.Copy(e.EvalID)
	return ne
}

// ScalingEventRequest is by for Job.Scale endpoint
// to register scaling events
type ScalingEventRequest struct {
	Namespace string
	JobID     string
	TaskGroup string

	ScalingEvent *ScalingEvent
}

// ScalingPolicy specifies the scaling policy for a scaling target
type ScalingPolicy struct {
	// ID is a generated UUID used for looking up the scaling policy
	ID string

	// Type is the type of scaling performed by the policy
	Type string

	// Target contains information about the target of the scaling policy, like job and group
	Target map[string]string

	// Policy is an opaque description of the scaling policy, passed to the autoscaler
	Policy map[string]interface{}

	// Min is the minimum allowable scaling count for this target
	Min int64

	// Max is the maximum allowable scaling count for this target
	Max int64

	// Enabled indicates whether this policy has been enabled/disabled
	Enabled bool

	CreateIndex uint64
	ModifyIndex uint64
}

// JobKey returns a key that is unique to a job-scoped target, useful as a map
// key. This uses the policy type, plus target (group and task).
func (p *ScalingPolicy) JobKey() string {
	return p.Type + "\000" +
		p.Target[ScalingTargetGroup] + "\000" +
		p.Target[ScalingTargetTask]
}

const (
	ScalingTargetNamespace = "Namespace"
	ScalingTargetJob       = "Job"
	ScalingTargetGroup     = "Group"
	ScalingTargetTask      = "Task"

	ScalingPolicyTypeHorizontal = "horizontal"
)

func (p *ScalingPolicy) Canonicalize(job *Job, tg *TaskGroup, task *Task) {
	if p.Type == "" {
		p.Type = ScalingPolicyTypeHorizontal
	}

	// during restore we canonicalize to update, but these values will already
	// have been populated during submit and we don't have references to the
	// job, group, and task
	if job != nil && tg != nil {
		p.Target = map[string]string{
			ScalingTargetNamespace: job.Namespace,
			ScalingTargetJob:       job.ID,
			ScalingTargetGroup:     tg.Name,
		}

		if task != nil {
			p.Target[ScalingTargetTask] = task.Name
		}
	}
}

func (p *ScalingPolicy) Copy() *ScalingPolicy {
	if p == nil {
		return nil
	}

	opaquePolicyConfig, err := copystructure.Copy(p.Policy)
	if err != nil {
		panic(err.Error())
	}

	c := ScalingPolicy{
		ID:          p.ID,
		Policy:      opaquePolicyConfig.(map[string]interface{}),
		Enabled:     p.Enabled,
		Type:        p.Type,
		Min:         p.Min,
		Max:         p.Max,
		CreateIndex: p.CreateIndex,
		ModifyIndex: p.ModifyIndex,
	}
	c.Target = make(map[string]string, len(p.Target))
	for k, v := range p.Target {
		c.Target[k] = v
	}
	return &c
}

func (p *ScalingPolicy) Validate() error {
	if p == nil {
		return nil
	}

	var mErr multierror.Error

	// Check policy type and target
	if p.Type == "" {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("missing scaling policy type"))
	} else {
		mErr.Errors = append(mErr.Errors, p.validateType().Errors...)
	}

	// Check Min and Max
	if p.Max < 0 {
		mErr.Errors = append(mErr.Errors,
			fmt.Errorf("maximum count must be specified and non-negative"))
	} else if p.Max < p.Min {
		mErr.Errors = append(mErr.Errors,
			fmt.Errorf("maximum count must not be less than minimum count"))
	}

	if p.Min < 0 {
		mErr.Errors = append(mErr.Errors,
			fmt.Errorf("minimum count must be specified and non-negative"))
	}

	return mErr.ErrorOrNil()
}

func (p *ScalingPolicy) validateTargetHorizontal() (mErr multierror.Error) {
	if len(p.Target) == 0 {
		// This is probably not a Nomad horizontal policy
		return
	}

	// Nomad horizontal policies should have Namespace, Job and TaskGroup
	if p.Target[ScalingTargetNamespace] == "" {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("missing target namespace"))
	}
	if p.Target[ScalingTargetJob] == "" {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("missing target job"))
	}
	if p.Target[ScalingTargetGroup] == "" {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("missing target group"))
	}
	return
}

// Diff indicates whether the specification for a given scaling policy has changed
func (p *ScalingPolicy) Diff(p2 *ScalingPolicy) bool {
	copy := *p2
	copy.ID = p.ID
	copy.CreateIndex = p.CreateIndex
	copy.ModifyIndex = p.ModifyIndex
	return !reflect.DeepEqual(*p, copy)
}

func (p *ScalingPolicy) Stub() *ScalingPolicyListStub {
	stub := &ScalingPolicyListStub{
		ID:          p.ID,
		Type:        p.Type,
		Target:      make(map[string]string),
		Enabled:     p.Enabled,
		CreateIndex: p.CreateIndex,
		ModifyIndex: p.ModifyIndex,
	}
	for k, v := range p.Target {
		stub.Target[k] = v
	}
	return stub
}

// GetScalingPolicies returns a slice of all scaling scaling policies for this job
func (j *Job) GetScalingPolicies() []*ScalingPolicy {
	ret := make([]*ScalingPolicy, 0)

	for _, tg := range j.TaskGroups {
		if tg.Scaling != nil {
			ret = append(ret, tg.Scaling)
		}
	}

	ret = append(ret, j.GetEntScalingPolicies()...)

	return ret
}

// UsesDeployments returns a boolean indicating whether the job configuration
// results in a deployment during scheduling.
func (j *Job) UsesDeployments() bool {
	switch j.Type {
	case JobTypeService:
		return true
	default:
		return false
	}
}

// ScalingPolicyListStub is used to return a subset of scaling policy information
// for the scaling policy list
type ScalingPolicyListStub struct {
	ID          string
	Enabled     bool
	Type        string
	Target      map[string]string
	CreateIndex uint64
	ModifyIndex uint64
}

// RestartPolicy configures how Tasks are restarted when they crash or fail.
type RestartPolicy struct {
	// Attempts is the number of restart that will occur in an interval.
	Attempts int

	// Interval is a duration in which we can limit the number of restarts
	// within.
	Interval time.Duration

	// Delay is the time between a failure and a restart.
	Delay time.Duration

	// Mode controls what happens when the task restarts more than attempt times
	// in an interval.
	Mode string

	// RenderTemplates is flag to explicitly render all templates on task restart
	RenderTemplates bool
}

func (r *RestartPolicy) Copy() *RestartPolicy {
	if r == nil {
		return nil
	}
	nrp := new(RestartPolicy)
	*nrp = *r
	return nrp
}

func (r *RestartPolicy) Validate() error {
	var mErr multierror.Error
	switch r.Mode {
	case RestartPolicyModeDelay, RestartPolicyModeFail:
	default:
		_ = multierror.Append(&mErr, fmt.Errorf("Unsupported restart mode: %q", r.Mode))
	}

	// Check for ambiguous/confusing settings
	if r.Attempts == 0 && r.Mode != RestartPolicyModeFail {
		_ = multierror.Append(&mErr, fmt.Errorf("Restart policy %q with %d attempts is ambiguous", r.Mode, r.Attempts))
	}

	if r.Interval.Nanoseconds() < RestartPolicyMinInterval.Nanoseconds() {
		_ = multierror.Append(&mErr, fmt.Errorf("Interval can not be less than %v (got %v)", RestartPolicyMinInterval, r.Interval))
	}
	if time.Duration(r.Attempts)*r.Delay > r.Interval {
		_ = multierror.Append(&mErr,
			fmt.Errorf("Nomad can't restart the TaskGroup %v times in an interval of %v with a delay of %v", r.Attempts, r.Interval, r.Delay))
	}
	return mErr.ErrorOrNil()
}

func NewRestartPolicy(jobType string) *RestartPolicy {
	switch jobType {
	case JobTypeService, JobTypeSystem:
		rp := DefaultServiceJobRestartPolicy
		return &rp
	case JobTypeBatch:
		rp := DefaultBatchJobRestartPolicy
		return &rp
	}
	return nil
}

const ReschedulePolicyMinInterval = 15 * time.Second
const ReschedulePolicyMinDelay = 5 * time.Second

var RescheduleDelayFunctions = [...]string{"constant", "exponential", "fibonacci"}

// ReschedulePolicy configures how Tasks are rescheduled  when they crash or fail.
type ReschedulePolicy struct {
	// Attempts limits the number of rescheduling attempts that can occur in an interval.
	Attempts int

	// Interval is a duration in which we can limit the number of reschedule attempts.
	Interval time.Duration

	// Delay is a minimum duration to wait between reschedule attempts.
	// The delay function determines how much subsequent reschedule attempts are delayed by.
	Delay time.Duration

	// DelayFunction determines how the delay progressively changes on subsequent reschedule
	// attempts. Valid values are "exponential", "constant", and "fibonacci".
	DelayFunction string

	// MaxDelay is an upper bound on the delay.
	MaxDelay time.Duration

	// Unlimited allows infinite rescheduling attempts. Only allowed when delay is set
	// between reschedule attempts.
	Unlimited bool
}

func (r *ReschedulePolicy) Copy() *ReschedulePolicy {
	if r == nil {
		return nil
	}
	nrp := new(ReschedulePolicy)
	*nrp = *r
	return nrp
}

func (r *ReschedulePolicy) Enabled() bool {
	enabled := r != nil && (r.Attempts > 0 || r.Unlimited)
	return enabled
}

// Validate uses different criteria to validate the reschedule policy
// Delay must be a minimum of 5 seconds
// Delay Ceiling is ignored if Delay Function is "constant"
// Number of possible attempts is validated, given the interval, delay and delay function
func (r *ReschedulePolicy) Validate() error {
	if !r.Enabled() {
		return nil
	}
	var mErr multierror.Error
	// Check for ambiguous/confusing settings
	if r.Attempts > 0 {
		if r.Interval <= 0 {
			_ = multierror.Append(&mErr, fmt.Errorf("Interval must be a non zero value if Attempts > 0"))
		}
		if r.Unlimited {
			_ = multierror.Append(&mErr, fmt.Errorf("Reschedule Policy with Attempts = %v, Interval = %v, "+
				"and Unlimited = %v is ambiguous", r.Attempts, r.Interval, r.Unlimited))
			_ = multierror.Append(&mErr, errors.New("If Attempts >0, Unlimited cannot also be set to true"))
		}
	}

	delayPreCheck := true
	// Delay should be bigger than the default
	if r.Delay.Nanoseconds() < ReschedulePolicyMinDelay.Nanoseconds() {
		_ = multierror.Append(&mErr, fmt.Errorf("Delay cannot be less than %v (got %v)", ReschedulePolicyMinDelay, r.Delay))
		delayPreCheck = false
	}

	// Must use a valid delay function
	if !isValidDelayFunction(r.DelayFunction) {
		_ = multierror.Append(&mErr, fmt.Errorf("Invalid delay function %q, must be one of %q", r.DelayFunction, RescheduleDelayFunctions))
		delayPreCheck = false
	}

	// Validate MaxDelay if not using linear delay progression
	if r.DelayFunction != "constant" {
		if r.MaxDelay.Nanoseconds() < ReschedulePolicyMinDelay.Nanoseconds() {
			_ = multierror.Append(&mErr, fmt.Errorf("Max Delay cannot be less than %v (got %v)", ReschedulePolicyMinDelay, r.Delay))
			delayPreCheck = false
		}
		if r.MaxDelay < r.Delay {
			_ = multierror.Append(&mErr, fmt.Errorf("Max Delay cannot be less than Delay %v (got %v)", r.Delay, r.MaxDelay))
			delayPreCheck = false
		}

	}

	// Validate Interval and other delay parameters if attempts are limited
	if !r.Unlimited {
		if r.Interval.Nanoseconds() < ReschedulePolicyMinInterval.Nanoseconds() {
			_ = multierror.Append(&mErr, fmt.Errorf("Interval cannot be less than %v (got %v)", ReschedulePolicyMinInterval, r.Interval))
		}
		if !delayPreCheck {
			// We can't cross validate the rest of the delay params if delayPreCheck fails, so return early
			return mErr.ErrorOrNil()
		}
		crossValidationErr := r.validateDelayParams()
		if crossValidationErr != nil {
			_ = multierror.Append(&mErr, crossValidationErr)
		}
	}
	return mErr.ErrorOrNil()
}

func isValidDelayFunction(delayFunc string) bool {
	for _, value := range RescheduleDelayFunctions {
		if value == delayFunc {
			return true
		}
	}
	return false
}

func (r *ReschedulePolicy) validateDelayParams() error {
	ok, possibleAttempts, recommendedInterval := r.viableAttempts()
	if ok {
		return nil
	}
	var mErr multierror.Error
	if r.DelayFunction == "constant" {
		_ = multierror.Append(&mErr, fmt.Errorf("Nomad can only make %v attempts in %v with initial delay %v and "+
			"delay function %q", possibleAttempts, r.Interval, r.Delay, r.DelayFunction))
	} else {
		_ = multierror.Append(&mErr, fmt.Errorf("Nomad can only make %v attempts in %v with initial delay %v, "+
			"delay function %q, and delay ceiling %v", possibleAttempts, r.Interval, r.Delay, r.DelayFunction, r.MaxDelay))
	}
	_ = multierror.Append(&mErr, fmt.Errorf("Set the interval to at least %v to accommodate %v attempts", recommendedInterval.Round(time.Second), r.Attempts))
	return mErr.ErrorOrNil()
}

func (r *ReschedulePolicy) viableAttempts() (bool, int, time.Duration) {
	var possibleAttempts int
	var recommendedInterval time.Duration
	valid := true
	switch r.DelayFunction {
	case "constant":
		recommendedInterval = time.Duration(r.Attempts) * r.Delay
		if r.Interval < recommendedInterval {
			possibleAttempts = int(r.Interval / r.Delay)
			valid = false
		}
	case "exponential":
		for i := 0; i < r.Attempts; i++ {
			nextDelay := time.Duration(math.Pow(2, float64(i))) * r.Delay
			if nextDelay > r.MaxDelay {
				nextDelay = r.MaxDelay
				recommendedInterval += nextDelay
			} else {
				recommendedInterval = nextDelay
			}
			if recommendedInterval < r.Interval {
				possibleAttempts++
			}
		}
		if possibleAttempts < r.Attempts {
			valid = false
		}
	case "fibonacci":
		var slots []time.Duration
		slots = append(slots, r.Delay)
		slots = append(slots, r.Delay)
		reachedCeiling := false
		for i := 2; i < r.Attempts; i++ {
			var nextDelay time.Duration
			if reachedCeiling {
				//switch to linear
				nextDelay = slots[i-1] + r.MaxDelay
			} else {
				nextDelay = slots[i-1] + slots[i-2]
				if nextDelay > r.MaxDelay {
					nextDelay = r.MaxDelay
					reachedCeiling = true
				}
			}
			slots = append(slots, nextDelay)
		}
		recommendedInterval = slots[len(slots)-1]
		if r.Interval < recommendedInterval {
			valid = false
			// calculate possible attempts
			for i := 0; i < len(slots); i++ {
				if slots[i] > r.Interval {
					possibleAttempts = i
					break
				}
			}
		}
	default:
		return false, 0, 0
	}
	if possibleAttempts < 0 { // can happen if delay is bigger than interval
		possibleAttempts = 0
	}
	return valid, possibleAttempts, recommendedInterval
}

func NewReschedulePolicy(jobType string) *ReschedulePolicy {
	switch jobType {
	case JobTypeService:
		rp := DefaultServiceJobReschedulePolicy
		return &rp
	case JobTypeBatch:
		rp := DefaultBatchJobReschedulePolicy
		return &rp
	}
	return nil
}

const (
	MigrateStrategyHealthChecks = "checks"
	MigrateStrategyHealthStates = "task_states"
)

type MigrateStrategy struct {
	MaxParallel     int
	HealthCheck     string
	MinHealthyTime  time.Duration
	HealthyDeadline time.Duration
}

// DefaultMigrateStrategy is used for backwards compat with pre-0.8 Allocations
// that lack an update strategy.
//
// This function should match its counterpart in api/tasks.go
func DefaultMigrateStrategy() *MigrateStrategy {
	return &MigrateStrategy{
		MaxParallel:     1,
		HealthCheck:     MigrateStrategyHealthChecks,
		MinHealthyTime:  10 * time.Second,
		HealthyDeadline: 5 * time.Minute,
	}
}

func (m *MigrateStrategy) Validate() error {
	var mErr multierror.Error

	if m.MaxParallel < 0 {
		_ = multierror.Append(&mErr, fmt.Errorf("MaxParallel must be >= 0 but found %d", m.MaxParallel))
	}

	switch m.HealthCheck {
	case MigrateStrategyHealthChecks, MigrateStrategyHealthStates:
		// ok
	case "":
		if m.MaxParallel > 0 {
			_ = multierror.Append(&mErr, fmt.Errorf("Missing HealthCheck"))
		}
	default:
		_ = multierror.Append(&mErr, fmt.Errorf("Invalid HealthCheck: %q", m.HealthCheck))
	}

	if m.MinHealthyTime < 0 {
		_ = multierror.Append(&mErr, fmt.Errorf("MinHealthyTime is %s and must be >= 0", m.MinHealthyTime))
	}

	if m.HealthyDeadline < 0 {
		_ = multierror.Append(&mErr, fmt.Errorf("HealthyDeadline is %s and must be >= 0", m.HealthyDeadline))
	}

	if m.MinHealthyTime > m.HealthyDeadline {
		_ = multierror.Append(&mErr, fmt.Errorf("MinHealthyTime must be less than HealthyDeadline"))
	}

	return mErr.ErrorOrNil()
}

// TaskGroup is an atomic unit of placement. Each task group belongs to
// a job and may contain any number of tasks. A task group support running
// in many replicas using the same configuration..
type TaskGroup struct {
	// Name of the task group
	Name string

	// Count is the number of replicas of this task group that should
	// be scheduled.
	Count int

	// Update is used to control the update strategy for this task group
	Update *UpdateStrategy

	// Migrate is used to control the migration strategy for this task group
	Migrate *MigrateStrategy

	// Constraints can be specified at a task group level and apply to
	// all the tasks contained.
	Constraints []*Constraint

	// Scaling is the list of autoscaling policies for the TaskGroup
	Scaling *ScalingPolicy

	// RestartPolicy of a TaskGroup
	RestartPolicy *RestartPolicy

	// Disconnect strategy defines how both clients and server should behave in case of
	// disconnection between them.
	Disconnect *DisconnectStrategy

	// Tasks are the collection of tasks that this task group needs to run
	Tasks []*Task

	// EphemeralDisk is the disk resources that the task group requests
	EphemeralDisk *EphemeralDisk

	// Meta is used to associate arbitrary metadata with this
	// task group. This is opaque to Nomad.
	Meta map[string]string

	// ReschedulePolicy is used to configure how the scheduler should
	// retry failed allocations.
	ReschedulePolicy *ReschedulePolicy

	// Affinities can be specified at the task group level to express
	// scheduling preferences.
	Affinities []*Affinity

	// Spread can be specified at the task group level to express spreading
	// allocations across a desired attribute, such as datacenter
	Spreads []*Spread

	// Networks are the network configuration for the task group. This can be
	// overridden in the task.
	Networks Networks

	// Consul configuration specific to this task group
	Consul *Consul

	// Services this group provides
	Services []*Service

	// Volumes is a map of volumes that have been requested by the task group.
	Volumes map[string]*VolumeRequest

	// ShutdownDelay is the amount of time to wait between deregistering
	// group services in consul and stopping tasks.
	ShutdownDelay *time.Duration

	// StopAfterClientDisconnect, if set, configures the client to stop the task group
	// after this duration since the last known good heartbeat
	// To be deprecated after 1.8.0 infavor of Disconnect.StopOnClientAfter
	StopAfterClientDisconnect *time.Duration

	// MaxClientDisconnect, if set, configures the client to allow placed
	// allocations for tasks in this group to attempt to resume running without a restart.
	// To be deprecated after 1.8.0 infavor of Disconnect.LostAfter
	MaxClientDisconnect *time.Duration

	// PreventRescheduleOnLost is used to signal that an allocation should not
	// be rescheduled if its node goes down or is disconnected.
	// To be deprecated after 1.8.0
	// To be deprecated after 1.8.0 infavor of Disconnect.Replace
	PreventRescheduleOnLost bool
}

func (tg *TaskGroup) Copy() *TaskGroup {
	if tg == nil {
		return nil
	}
	ntg := new(TaskGroup)
	*ntg = *tg
	ntg.Update = ntg.Update.Copy()
	ntg.Constraints = CopySliceConstraints(ntg.Constraints)
	ntg.RestartPolicy = ntg.RestartPolicy.Copy()
	ntg.Disconnect = ntg.Disconnect.Copy()
	ntg.ReschedulePolicy = ntg.ReschedulePolicy.Copy()
	ntg.Affinities = CopySliceAffinities(ntg.Affinities)
	ntg.Spreads = CopySliceSpreads(ntg.Spreads)
	ntg.Volumes = CopyMapVolumeRequest(ntg.Volumes)
	ntg.Scaling = ntg.Scaling.Copy()
	ntg.Consul = ntg.Consul.Copy()

	// Copy the network objects
	if tg.Networks != nil {
		n := len(tg.Networks)
		ntg.Networks = make([]*NetworkResource, n)
		for i := 0; i < n; i++ {
			ntg.Networks[i] = tg.Networks[i].Copy()
		}
	}

	if tg.Tasks != nil {
		tasks := make([]*Task, len(ntg.Tasks))
		for i, t := range ntg.Tasks {
			tasks[i] = t.Copy()
		}
		ntg.Tasks = tasks
	}

	ntg.Meta = maps.Clone(ntg.Meta)

	if tg.EphemeralDisk != nil {
		ntg.EphemeralDisk = tg.EphemeralDisk.Copy()
	}

	if tg.Services != nil {
		ntg.Services = make([]*Service, len(tg.Services))
		for i, s := range tg.Services {
			ntg.Services[i] = s.Copy()
		}
	}

	if tg.ShutdownDelay != nil {
		ntg.ShutdownDelay = tg.ShutdownDelay
	}

	if tg.StopAfterClientDisconnect != nil {
		ntg.StopAfterClientDisconnect = tg.StopAfterClientDisconnect
	}

	if tg.MaxClientDisconnect != nil {
		ntg.MaxClientDisconnect = tg.MaxClientDisconnect
	}

	return ntg
}

// Canonicalize is used to canonicalize fields in the TaskGroup.
func (tg *TaskGroup) Canonicalize(job *Job) {
	// Ensure that an empty and nil map or array are treated the same to avoid scheduling
	// problems since we use reflect DeepEquals.
	if len(tg.Meta) == 0 {
		tg.Meta = nil
	}

	if len(tg.Constraints) == 0 {
		tg.Constraints = nil
	}

	if len(tg.Affinities) == 0 {
		tg.Affinities = nil
	}

	if len(tg.Spreads) == 0 {
		tg.Spreads = nil
	}

	// Set the default restart policy.
	if tg.RestartPolicy == nil {
		tg.RestartPolicy = NewRestartPolicy(job.Type)
	}

	if tg.ReschedulePolicy == nil {
		tg.ReschedulePolicy = NewReschedulePolicy(job.Type)
	}

	if tg.Disconnect != nil {
		tg.Disconnect.Canonicalize()

		if tg.MaxClientDisconnect != nil && tg.Disconnect.LostAfter == 0 {
			tg.Disconnect.LostAfter = *tg.MaxClientDisconnect
		}

		if tg.StopAfterClientDisconnect != nil && tg.Disconnect.StopOnClientAfter == nil {
			tg.Disconnect.StopOnClientAfter = tg.StopAfterClientDisconnect
		}

		if tg.PreventRescheduleOnLost && tg.Disconnect.Replace == nil {
			tg.Disconnect.Replace = pointer.Of(false)
		}
	}

	// Canonicalize Migrate for service jobs
	if job.Type == JobTypeService && tg.Migrate == nil {
		tg.Migrate = DefaultMigrateStrategy()
	}

	// Set a default ephemeral disk object if the user has not requested for one
	if tg.EphemeralDisk == nil {
		tg.EphemeralDisk = DefaultEphemeralDisk()
	}

	if job.Type == JobTypeSystem && tg.Count == 0 {
		tg.Count = 1
	}

	if tg.Scaling != nil {
		tg.Scaling.Canonicalize(job, tg, nil)
	}

	for _, service := range tg.Services {
		service.Canonicalize(job.Name, tg.Name, "group", job.Namespace)
	}

	for _, network := range tg.Networks {
		network.Canonicalize()
	}

	for _, task := range tg.Tasks {
		task.Canonicalize(job, tg)
	}
}

// NomadServices returns a list of all group and task - level services in tg that
// are making use of the nomad service provider.
func (tg *TaskGroup) NomadServices() []*Service {
	return tg.filterServices(func(s *Service) bool {
		return s.Provider == ServiceProviderNomad
	})
}

func (tg *TaskGroup) ConsulServices() []*Service {
	return tg.filterServices(func(s *Service) bool {
		return s.Provider == ServiceProviderConsul || s.Provider == ""
	})
}

func (tg *TaskGroup) filterServices(f func(s *Service) bool) []*Service {
	var services []*Service
	for _, service := range tg.Services {
		if f(service) {
			services = append(services, service)
		}
	}
	for _, task := range tg.Tasks {
		for _, service := range task.Services {
			if f(service) {
				services = append(services, service)
			}
		}
	}
	return services
}

// Validate is used to check a task group for reasonable configuration
func (tg *TaskGroup) Validate(j *Job) error {
	var mErr *multierror.Error

	if tg.Name == "" {
		mErr = multierror.Append(mErr, errors.New("Missing task group name"))
	} else if strings.Contains(tg.Name, "\000") {
		mErr = multierror.Append(mErr, errors.New("Task group name contains null character"))
	}

	if tg.Count < 0 {
		mErr = multierror.Append(mErr, errors.New("Task group count can't be negative"))
	}

	if len(tg.Tasks) == 0 {
		// could be a lone consul gateway inserted by the connect mutator
		mErr = multierror.Append(mErr, errors.New("Missing tasks for task group"))
	}

	if tg.MaxClientDisconnect != nil && tg.StopAfterClientDisconnect != nil {
		mErr = multierror.Append(mErr, errors.New("Task group cannot be configured with both max_client_disconnect and stop_after_client_disconnect"))
	}

	if tg.MaxClientDisconnect != nil && *tg.MaxClientDisconnect < 0 {
		mErr = multierror.Append(mErr, errors.New("max_client_disconnect cannot be negative"))
	}

	if tg.Disconnect != nil {
		if tg.MaxClientDisconnect != nil && tg.Disconnect.LostAfter > 0 {
			return multierror.Append(mErr, errors.New("using both lost_after and max_client_disconnect is not allowed"))
		}

		if tg.StopAfterClientDisconnect != nil && tg.Disconnect.StopOnClientAfter != nil {
			return multierror.Append(mErr, errors.New("using both stop_after_client_disconnect and stop_on_client_after is not allowed"))
		}

		if tg.PreventRescheduleOnLost && tg.Disconnect.Replace != nil {
			return multierror.Append(mErr, errors.New("using both prevent_reschedule_on_lost and replace is not allowed"))
		}

		if err := tg.Disconnect.Validate(j); err != nil {
			mErr = multierror.Append(mErr, err)
		}
	}

	for idx, constr := range tg.Constraints {
		if err := constr.Validate(); err != nil {
			outer := fmt.Errorf("Constraint %d validation failed: %s", idx+1, err)
			mErr = multierror.Append(mErr, outer)
		}
	}
	if j.Type == JobTypeSystem {
		if tg.Affinities != nil {
			mErr = multierror.Append(mErr, fmt.Errorf("System jobs may not have an affinity block"))
		}
	} else {
		for idx, affinity := range tg.Affinities {
			if err := affinity.Validate(); err != nil {
				outer := fmt.Errorf("Affinity %d validation failed: %s", idx+1, err)
				mErr = multierror.Append(mErr, outer)
			}
		}
	}

	if tg.RestartPolicy != nil {
		if err := tg.RestartPolicy.Validate(); err != nil {
			mErr = multierror.Append(mErr, err)
		}
	} else {
		mErr = multierror.Append(mErr, fmt.Errorf("Task Group %v should have a restart policy", tg.Name))
	}

	if j.Type == JobTypeSystem {
		if tg.Spreads != nil {
			mErr = multierror.Append(mErr, fmt.Errorf("System jobs may not have a spread block"))
		}
	} else {
		for idx, spread := range tg.Spreads {
			if err := spread.Validate(); err != nil {
				outer := fmt.Errorf("Spread %d validation failed: %s", idx+1, err)
				mErr = multierror.Append(mErr, outer)
			}
		}
	}

	if j.Type == JobTypeSystem {
		if tg.ReschedulePolicy != nil {
			mErr = multierror.Append(mErr, fmt.Errorf("System jobs should not have a reschedule policy"))
		}
	} else {
		if tg.ReschedulePolicy != nil {
			if err := tg.ReschedulePolicy.Validate(); err != nil {
				mErr = multierror.Append(mErr, err)
			}
		} else {
			mErr = multierror.Append(mErr, fmt.Errorf("Task Group %v should have a reschedule policy", tg.Name))
		}
	}

	if tg.EphemeralDisk != nil {
		if err := tg.EphemeralDisk.Validate(); err != nil {
			mErr = multierror.Append(mErr, err)
		}
	} else {
		mErr = multierror.Append(mErr, fmt.Errorf("Task Group %v should have an ephemeral disk object", tg.Name))
	}

	// Validate the update strategy
	if u := tg.Update; u != nil {
		switch j.Type {
		case JobTypeService, JobTypeSystem:
		default:
			mErr = multierror.Append(mErr, fmt.Errorf("Job type %q does not allow update block", j.Type))
		}
		if err := u.Validate(); err != nil {
			mErr = multierror.Append(mErr, err)
		}
	}

	// Validate the migration strategy
	switch j.Type {
	case JobTypeService:
		if tg.Migrate != nil {
			if err := tg.Migrate.Validate(); err != nil {
				mErr = multierror.Append(mErr, err)
			}
		}
	default:
		if tg.Migrate != nil {
			mErr = multierror.Append(mErr, fmt.Errorf("Job type %q does not allow migrate block", j.Type))
		}
	}

	// Check that there is only one leader task if any
	tasks := make(map[string]int)
	leaderTasks := 0
	for idx, task := range tg.Tasks {
		if task.Name == "" {
			mErr = multierror.Append(mErr, fmt.Errorf("Task %d missing name", idx+1))
		} else if existing, ok := tasks[task.Name]; ok {
			mErr = multierror.Append(mErr, fmt.Errorf("Task %d redefines '%s' from task %d", idx+1, task.Name, existing+1))
		} else {
			tasks[task.Name] = idx
		}

		if task.Leader {
			leaderTasks++
		}
	}

	if leaderTasks > 1 {
		mErr = multierror.Append(mErr, fmt.Errorf("Only one task may be marked as leader"))
	}

	// Validate the volume requests
	var canaries int
	if tg.Update != nil {
		canaries = tg.Update.Canary
	}
	for name, volReq := range tg.Volumes {
		if err := volReq.Validate(j.Type, tg.Count, canaries); err != nil {
			mErr = multierror.Append(mErr, fmt.Errorf(
				"Task group volume validation for %s failed: %v", name, err))
		}
	}

	// Validate task group and task network resources
	if err := tg.validateNetworks(); err != nil {
		outer := fmt.Errorf("Task group network validation failed: %v", err)
		mErr = multierror.Append(mErr, outer)
	}

	// Validate task group and task services
	if err := tg.validateServices(); err != nil {
		outer := fmt.Errorf("Task group service validation failed: %v", err)
		mErr = multierror.Append(mErr, outer)
	}

	// Validate group service script-checks
	if err := tg.validateScriptChecksInGroupServices(); err != nil {
		outer := fmt.Errorf("Task group service check validation failed: %v", err)
		mErr = multierror.Append(mErr, outer)
	}

	// Validate the scaling policy
	if err := tg.validateScalingPolicy(j); err != nil {
		outer := fmt.Errorf("Task group scaling policy validation failed: %v", err)
		mErr = multierror.Append(mErr, outer)
	}

	// Validate the tasks
	for _, task := range tg.Tasks {
		if err := task.Validate(j.Type, tg); err != nil {
			outer := fmt.Errorf("Task %s validation failed: %v", task.Name, err)
			mErr = multierror.Append(mErr, outer)
		}
	}

	return mErr.ErrorOrNil()
}

func (tg *TaskGroup) validateNetworks() error {
	var mErr multierror.Error
	portLabels := make(map[string]string)
	// host_network -> static port tracking
	staticPortsIndex := make(map[string]map[int]string)
	cniArgKeys := set.New[string](len(tg.Networks))

	for _, net := range tg.Networks {
		for _, port := range append(net.ReservedPorts, net.DynamicPorts...) {
			if other, ok := portLabels[port.Label]; ok {
				mErr.Errors = append(mErr.Errors, fmt.Errorf("Port label %s already in use by %s", port.Label, other))
			} else {
				portLabels[port.Label] = "taskgroup network"
			}

			if port.Value != 0 {
				hostNetwork := port.HostNetwork
				if hostNetwork == "" {
					hostNetwork = "default"
				}
				staticPorts, ok := staticPortsIndex[hostNetwork]
				if !ok {
					staticPorts = make(map[int]string)
				}
				// static port
				if other, ok := staticPorts[port.Value]; ok {
					if !port.IgnoreCollision {
						err := fmt.Errorf("Static port %d already reserved by %s", port.Value, other)
						mErr.Errors = append(mErr.Errors, err)
					}
				} else if port.Value > math.MaxUint16 {
					err := fmt.Errorf("Port %s (%d) cannot be greater than %d", port.Label, port.Value, math.MaxUint16)
					mErr.Errors = append(mErr.Errors, err)
				} else {
					staticPorts[port.Value] = fmt.Sprintf("taskgroup network:%s", port.Label)
					staticPortsIndex[hostNetwork] = staticPorts
				}
			}

			if port.To < -1 {
				err := fmt.Errorf("Port %q cannot be mapped to negative value %d", port.Label, port.To)
				mErr.Errors = append(mErr.Errors, err)
			} else if port.To > math.MaxUint16 {
				err := fmt.Errorf("Port %q cannot be mapped to a port (%d) greater than %d", port.Label, port.To, math.MaxUint16)
				mErr.Errors = append(mErr.Errors, err)
			}

			if port.IgnoreCollision && !(net.Mode == "" || net.Mode == "host") {
				err := fmt.Errorf("Port %q collision may not be ignored on non-host network mode %q", port.Label, net.Mode)
				mErr.Errors = append(mErr.Errors, err)
			}
		}
		// Validate the cniArgs in each network resource. Make sure there are no duplicate Args in
		// different network resources or invalid characters (;) in key or value ;)
		if net.CNI != nil {
			for k, v := range net.CNI.Args {
				if cniArgKeys.Contains(k) {
					err := fmt.Errorf("duplicate CNI arg %q", k)
					mErr.Errors = append(mErr.Errors, err)
				} else {
					cniArgKeys.Insert(k)
				}
				// CNI_ARGS is a ";"-separated string of "key=val", so a ";"
				// in either key or val would confuse plugins (or libraries)
				// that parse that string.
				// Pre-validating this here protects job authors from submitting
				// a job that will most likely error later on the client anyway.
				if strings.Contains(k, ";") {
					err := fmt.Errorf("invalid ';' character in CNI arg key %q", k)
					mErr.Errors = append(mErr.Errors, err)
				}
				if strings.Contains(v, ";") {
					err := fmt.Errorf("invalid ';' character in CNI arg value %q", v)
					mErr.Errors = append(mErr.Errors, err)
				}
			}
		}

		// Validate the hostname field to be a valid DNS name. If the parameter
		// looks like it includes an interpolation value, we skip this. It
		// would be nice to validate additional parameters, but this isn't the
		// right place.
		if net.Hostname != "" && !strings.Contains(net.Hostname, "${") {
			if _, ok := dns.IsDomainName(net.Hostname); !ok {
				mErr.Errors = append(mErr.Errors, errors.New("Hostname is not a valid DNS name"))
			}
		}
	}

	// Check for duplicate tasks or port labels, and no duplicated static ports
	for _, task := range tg.Tasks {
		if task.Resources == nil {
			continue
		}

		for _, net := range task.Resources.Networks {
			for _, port := range append(net.ReservedPorts, net.DynamicPorts...) {
				if other, ok := portLabels[port.Label]; ok {
					mErr.Errors = append(mErr.Errors, fmt.Errorf("Port label %s already in use by %s", port.Label, other))
				}

				if port.Value != 0 {
					hostNetwork := port.HostNetwork
					if hostNetwork == "" {
						hostNetwork = "default"
					}
					staticPorts, ok := staticPortsIndex[hostNetwork]
					if !ok {
						staticPorts = make(map[int]string)
					}
					if other, ok := staticPorts[port.Value]; ok {
						err := fmt.Errorf("Static port %d already reserved by %s", port.Value, other)
						mErr.Errors = append(mErr.Errors, err)
					} else if port.Value > math.MaxUint16 {
						err := fmt.Errorf("Port %s (%d) cannot be greater than %d", port.Label, port.Value, math.MaxUint16)
						mErr.Errors = append(mErr.Errors, err)
					} else {
						staticPorts[port.Value] = fmt.Sprintf("%s:%s", task.Name, port.Label)
						staticPortsIndex[hostNetwork] = staticPorts
					}
				}
			}
		}
	}
	return mErr.ErrorOrNil()
}

// validateServices runs Service.Validate() on group-level services, checks
// group service checks that refer to tasks only refer to tasks that exist.
func (tg *TaskGroup) validateServices() error {
	var mErr multierror.Error

	// Accumulate task names in this group
	taskSet := set.New[string](len(tg.Tasks))

	// each service in a group must be unique (i.e. used in MakeAllocServiceID)
	type unique struct {
		name string
		task string
		port string
	}

	// Accumulate service IDs in this group
	idSet := set.New[unique](0)

	// Accumulate IDs that are duplicates
	idDuplicateSet := set.New[unique](0)

	// Accumulate the providers used for this task group. Currently, Nomad only
	// allows the use of a single service provider within a task group.
	providerSet := set.New[string](1)

	// Create a map of known tasks and their services so we can compare
	// vs the group-level services and checks
	for _, task := range tg.Tasks {
		taskSet.Insert(task.Name)

		if len(task.Services) == 0 {
			continue
		}

		for _, service := range task.Services {

			// Ensure no task-level service can only specify the task it belongs to.
			if service.TaskName != "" && service.TaskName != task.Name {
				mErr.Errors = append(mErr.Errors,
					fmt.Errorf("Service %s is invalid: may only specify task the service belongs to, got %q", service.Name, service.TaskName),
				)
			}

			// Ensure no task-level checks can only specify the task they belong to.
			for _, check := range service.Checks {
				if check.TaskName != "" && check.TaskName != task.Name {
					mErr.Errors = append(mErr.Errors,
						fmt.Errorf("Check %s is invalid: may only specify task the check belongs to, got %q", check.Name, check.TaskName),
					)
				}
			}

			// Track that we have seen this service id
			id := unique{service.Name, task.Name, service.PortLabel}
			if !idSet.Insert(id) {
				// accumulate duplicates for a single error later on
				idDuplicateSet.Insert(id)
			}

			// Track that we have seen this service provider
			providerSet.Insert(service.Provider)
		}
	}

	for i, service := range tg.Services {

		// Track that we have seen this service id
		id := unique{service.Name, "group", service.PortLabel}
		if !idSet.Insert(id) {
			// accumulate duplicates for a single error later on
			idDuplicateSet.Insert(id)
		}

		// Track that we have seen this service provider
		providerSet.Insert(service.Provider)

		if err := service.Validate(); err != nil {
			outer := fmt.Errorf("Service[%d] %s validation failed: %s", i, service.Name, err)
			mErr.Errors = append(mErr.Errors, outer)
			// we break here to avoid the risk of crashing on null-pointer
			// access in a later step, accepting that we might miss out on
			// error messages to provide the user.
			continue
		}
		if service.AddressMode == AddressModeDriver {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("service %q cannot use address_mode=\"driver\", only services defined in a \"task\" block can use this mode", service.Name))
		}

		for _, check := range service.Checks {
			if check.TaskName != "" {
				if check.AddressMode == AddressModeDriver {
					mErr.Errors = append(mErr.Errors, fmt.Errorf("Check %q invalid: cannot use address_mode=\"driver\", only checks defined in a \"task\" service block can use this mode", service.Name))
				}
				if !taskSet.Contains(check.TaskName) {
					mErr.Errors = append(mErr.Errors,
						fmt.Errorf("Check %s invalid: refers to non-existent task %s", check.Name, check.TaskName))
				}
			}
		}
	}

	// Produce an error of any services which are not unique enough in the group
	// i.e. have same <task, name, port>
	if idDuplicateSet.Size() > 0 {
		mErr.Errors = append(mErr.Errors,
			fmt.Errorf(
				"Services are not unique: %s",
				idDuplicateSet.StringFunc(
					func(u unique) string {
						s := u.task + "->" + u.name
						if u.port != "" {
							s += ":" + u.port
						}
						return s
					},
				),
			),
		)
	}

	// The initial feature release of native service discovery only allows for
	// a single service provider to be used across all services in a task
	// group.
	if providerSet.Size() > 1 {
		mErr.Errors = append(mErr.Errors,
			errors.New("Multiple service providers used: task group services must use the same provider"))
	}

	return mErr.ErrorOrNil()
}

// validateScriptChecksInGroupServices ensures group-level services with script
// checks know what task driver to use. Either the service.task or service.check.task
// parameter must be configured.
func (tg *TaskGroup) validateScriptChecksInGroupServices() error {
	var mErr multierror.Error
	for _, service := range tg.Services {
		if service.TaskName == "" {
			for _, check := range service.Checks {
				if check.Type == "script" && check.TaskName == "" {
					mErr.Errors = append(mErr.Errors,
						fmt.Errorf("Service [%s]->%s or Check %s must specify task parameter",
							tg.Name, service.Name, check.Name,
						))
				}
			}
		}
	}
	return mErr.ErrorOrNil()
}

// validateScalingPolicy ensures that the scaling policy has consistent
// min and max, not in conflict with the task group count
func (tg *TaskGroup) validateScalingPolicy(j *Job) error {
	if tg.Scaling == nil {
		return nil
	}

	var mErr multierror.Error

	err := tg.Scaling.Validate()
	if err != nil {
		// prefix scaling policy errors
		if me, ok := err.(*multierror.Error); ok {
			for _, e := range me.Errors {
				mErr.Errors = append(mErr.Errors, fmt.Errorf("Scaling policy invalid: %s", e))
			}
		}
	}

	if tg.Scaling.Max < int64(tg.Count) {
		mErr.Errors = append(mErr.Errors,
			fmt.Errorf("Scaling policy invalid: task group count must not be greater than maximum count in scaling policy"))
	}

	if int64(tg.Count) < tg.Scaling.Min && !(j.IsMultiregion() && tg.Count == 0 && j.Region == "global") {
		mErr.Errors = append(mErr.Errors,
			fmt.Errorf("Scaling policy invalid: task group count must not be less than minimum count in scaling policy"))
	}

	return mErr.ErrorOrNil()
}

// Warnings returns a list of warnings that may be from dubious settings or
// deprecation warnings.
func (tg *TaskGroup) Warnings(j *Job) error {
	var mErr multierror.Error

	// Validate the update strategy
	if u := tg.Update; u != nil {
		// Check the counts are appropriate
		if tg.Count > 1 && u.MaxParallel > tg.Count && !(j.IsMultiregion() && tg.Count == 0) {
			mErr.Errors = append(mErr.Errors,
				fmt.Errorf("Update max parallel count is greater than task group count (%d > %d). "+
					"A destructive change would result in the simultaneous replacement of all allocations.", u.MaxParallel, tg.Count))
		}
	}

	if tg.MaxClientDisconnect != nil {
		mErr.Errors = append(mErr.Errors, errors.New("MaxClientDisconnect will be deprecated favor of Disconnect.LostAfter"))
	}

	if tg.StopAfterClientDisconnect != nil {
		mErr.Errors = append(mErr.Errors, errors.New("StopAfterClientDisconnect will be deprecated favor of Disconnect.StopOnClientAfter"))
	}

	if tg.PreventRescheduleOnLost {
		mErr.Errors = append(mErr.Errors, errors.New("PreventRescheduleOnLost will be deprecated favor of Disconnect.Replace"))
	}

	// Check for mbits network field
	if len(tg.Networks) > 0 && tg.Networks[0].MBits > 0 {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("mbits has been deprecated as of Nomad 0.12.0. Please remove mbits from the network block"))
	}

	// Validate group-level services.
	for _, s := range tg.Services {
		if err := s.Warnings(); err != nil {
			err = multierror.Prefix(err, fmt.Sprintf("Service %q:", s.Name))
			mErr = *multierror.Append(&mErr, err)
		}
	}

	for _, t := range tg.Tasks {
		if err := t.Warnings(); err != nil {
			outer := fmt.Errorf("Task %q has warnings: %v", t.Name, err)
			mErr.Errors = append(mErr.Errors, outer)
		}
	}

	return mErr.ErrorOrNil()
}

// LookupTask finds a task by name
func (tg *TaskGroup) LookupTask(name string) *Task {
	for _, t := range tg.Tasks {
		if t.Name == name {
			return t
		}
	}
	return nil
}

// UsesConnect for convenience returns true if the TaskGroup contains at least
// one service that makes use of Consul Connect features.
//
// Currently used for validating that the task group contains one or more connect
// aware services before generating a service identity token.
func (tg *TaskGroup) UsesConnect() bool {
	for _, service := range tg.Services {
		if service.Connect != nil {
			if service.Connect.IsNative() || service.Connect.HasSidecar() || service.Connect.IsGateway() {
				return true
			}
		}
	}
	return false
}

// UsesConnectGateway for convenience returns true if the TaskGroup contains at
// least one service that makes use of Consul Connect Gateway features.
func (tg *TaskGroup) UsesConnectGateway() bool {
	for _, service := range tg.Services {
		if service.Connect != nil {
			if service.Connect.IsGateway() {
				return true
			}
		}
	}
	return false
}

func (tg *TaskGroup) GoString() string {
	return fmt.Sprintf("*%#v", *tg)
}

// Replace is a helper meant to simplify the future depracation of
// PreventRescheduleOnLost in favor of Disconnect.Replace
// introduced in 1.8.0.
func (tg *TaskGroup) Replace() bool {
	if tg.PreventRescheduleOnLost {
		return false
	}

	if tg.Disconnect == nil || tg.Disconnect.Replace == nil {
		return true
	}

	return *tg.Disconnect.Replace
}

// GetDisconnectLostTimeout is a helper meant to simplify the future depracation of
// MaxClientDisconnect in favor of Disconnect.LostAfter
// introduced in 1.8.0.
func (tg *TaskGroup) GetDisconnectLostTimeout() time.Duration {
	if tg.MaxClientDisconnect != nil {
		return *tg.MaxClientDisconnect
	}

	if tg.Disconnect != nil {
		return tg.Disconnect.LostAfter
	}

	return 0
}

// GetDisconnectStopTimeout is a helper meant to simplify the future depracation of
// StopAfterClientDisconnect in favor of Disconnect.StopOnClientAfter
// introduced in 1.8.0.
func (tg *TaskGroup) GetDisconnectStopTimeout() *time.Duration {
	if tg.StopAfterClientDisconnect != nil {
		return tg.StopAfterClientDisconnect
	}

	if tg.Disconnect != nil && tg.Disconnect.StopOnClientAfter != nil {
		return tg.Disconnect.StopOnClientAfter
	}

	return nil
}

func (tg *TaskGroup) GetConstraints() []*Constraint {
	return tg.Constraints
}

func (tg *TaskGroup) SetConstraints(newConstraints []*Constraint) {
	tg.Constraints = newConstraints
}

// CheckRestart describes if and when a task should be restarted based on
// failing health checks.
type CheckRestart struct {
	Limit          int           // Restart task after this many unhealthy intervals
	Grace          time.Duration // Grace time to give tasks after starting to get healthy
	IgnoreWarnings bool          // If true treat checks in `warning` as passing
}

func (c *CheckRestart) Copy() *CheckRestart {
	if c == nil {
		return nil
	}

	nc := new(CheckRestart)
	*nc = *c
	return nc
}

func (c *CheckRestart) Equal(o *CheckRestart) bool {
	if c == nil || o == nil {
		return c == o
	}

	if c.Limit != o.Limit {
		return false
	}

	if c.Grace != o.Grace {
		return false
	}

	if c.IgnoreWarnings != o.IgnoreWarnings {
		return false
	}

	return true
}

func (c *CheckRestart) Validate() error {
	if c == nil {
		return nil
	}

	var mErr multierror.Error
	if c.Limit < 0 {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("limit must be greater than or equal to 0 but found %d", c.Limit))
	}

	if c.Grace < 0 {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("grace period must be greater than or equal to 0 but found %d", c.Grace))
	}

	return mErr.ErrorOrNil()
}

const (
	// DefaultKillTimeout is the default timeout between signaling a task it
	// will be killed and killing it.
	DefaultKillTimeout = 5 * time.Second
)

// LogConfig provides configuration for log rotation
type LogConfig struct {
	MaxFiles      int
	MaxFileSizeMB int
	Disabled      bool
}

func (l *LogConfig) Equal(o *LogConfig) bool {
	if l == nil || o == nil {
		return l == o
	}

	if l.MaxFiles != o.MaxFiles {
		return false
	}

	if l.MaxFileSizeMB != o.MaxFileSizeMB {
		return false
	}

	if l.Disabled != o.Disabled {
		return false
	}

	return true
}

func (l *LogConfig) Copy() *LogConfig {
	if l == nil {
		return nil
	}
	return &LogConfig{
		MaxFiles:      l.MaxFiles,
		MaxFileSizeMB: l.MaxFileSizeMB,
		Disabled:      l.Disabled,
	}
}

// DefaultLogConfig returns the default LogConfig values.
func DefaultLogConfig() *LogConfig {
	return &LogConfig{
		MaxFiles:      10,
		MaxFileSizeMB: 10,
		Disabled:      false,
	}
}

// Validate returns an error if the log config specified are less than the
// minimum allowed. Note that because we have a non-zero default MaxFiles and
// MaxFileSizeMB, we can't validate that they're unset if Disabled=true
func (l *LogConfig) Validate(disk *EphemeralDisk) error {
	var mErr multierror.Error
	if l.MaxFiles < 1 {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("minimum number of files is 1; got %d", l.MaxFiles))
	}
	if l.MaxFileSizeMB < 1 {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("minimum file size is 1MB; got %d", l.MaxFileSizeMB))
	}
	if disk != nil {
		logUsage := (l.MaxFiles * l.MaxFileSizeMB)
		if disk.SizeMB <= logUsage {
			mErr.Errors = append(mErr.Errors,
				fmt.Errorf("log storage (%d MB) must be less than requested disk capacity (%d MB)",
					logUsage, disk.SizeMB))
		}
	}
	return mErr.ErrorOrNil()
}

// Task is a single process typically that is executed as part of a task group.
type Task struct {
	// Name of the task
	Name string

	// Driver is used to control which driver is used
	Driver string

	// User is used to determine which user will run the task. It defaults to
	// the same user the Nomad client is being run as.
	User string

	// Config is provided to the driver to initialize
	Config map[string]interface{}

	// Map of environment variables to be used by the driver
	Env map[string]string

	// List of service definitions exposed by the Task
	Services []*Service

	// Vault is used to define the set of Vault policies that this task should
	// have access to.
	Vault *Vault

	// Consul configuration specific to this task. If uset, falls back to the
	// group's Consul field.
	Consul *Consul

	// Templates are the set of templates to be rendered for the task.
	Templates []*Template

	// Constraints can be specified at a task level and apply only to
	// the particular task.
	Constraints []*Constraint

	// Affinities can be specified at the task level to express
	// scheduling preferences
	Affinities []*Affinity

	// Resources is the resources needed by this task
	Resources *Resources

	// RestartPolicy of a TaskGroup
	RestartPolicy *RestartPolicy

	// DispatchPayload configures how the task retrieves its input from a dispatch
	DispatchPayload *DispatchPayloadConfig

	Lifecycle *TaskLifecycleConfig

	// Meta is used to associate arbitrary metadata with this
	// task. This is opaque to Nomad.
	Meta map[string]string

	// KillTimeout is the time between signaling a task that it will be
	// killed and killing it.
	KillTimeout time.Duration

	// LogConfig provides configuration for log rotation
	LogConfig *LogConfig

	// Artifacts is a list of artifacts to download and extract before running
	// the task.
	Artifacts []*TaskArtifact

	// Leader marks the task as the leader within the group. When the leader
	// task exits, other tasks will be gracefully terminated.
	Leader bool

	// ShutdownDelay is the duration of the delay between de-registering a
	// task from Consul and sending it a signal to shutdown. See #2441
	ShutdownDelay time.Duration

	// VolumeMounts is a list of Volume name <-> mount configurations that will be
	// attached to this task.
	VolumeMounts []*VolumeMount

	// ScalingPolicies is a list of scaling policies scoped to this task
	ScalingPolicies []*ScalingPolicy

	// KillSignal is the kill signal to use for the task. This is an optional
	// specification and defaults to SIGINT
	KillSignal string

	// Used internally to manage tasks according to their TaskKind. Initial use case
	// is for Consul Connect
	Kind TaskKind

	// CSIPluginConfig is used to configure the plugin supervisor for the task.
	CSIPluginConfig *TaskCSIPluginConfig

	// Identity is the default Nomad Workload Identity.
	Identity *WorkloadIdentity

	// Identities are the alternate workload identities for use with 3rd party
	// endpoints.
	Identities []*WorkloadIdentity

	// Alloc-exec-like runnable commands
	Actions []*Action

	// Schedule for pausing tasks. Enterprise only.
	Schedule *TaskSchedule
}

func (t *Task) UsesCores() bool {
	return t.Resources.Cores > 0
}

// UsesConnect is for conveniently detecting if the Task is able to make use
// of Consul Connect features. This will be indicated in the TaskKind of the
// Task, which exports known types of Tasks. UsesConnect will be true if the
// task is a connect proxy, connect native, or is a connect gateway.
func (t *Task) UsesConnect() bool {
	return t.Kind.IsConnectNative() || t.UsesConnectSidecar()
}

func (t *Task) UsesConnectSidecar() bool {
	return t.Kind.IsConnectProxy() || t.Kind.IsAnyConnectGateway()
}

func (t *Task) IsPrestart() bool {
	return t != nil && t.Lifecycle != nil &&
		t.Lifecycle.Hook == TaskLifecycleHookPrestart
}

func (t *Task) IsMain() bool {
	return t != nil && (t.Lifecycle == nil || t.Lifecycle.Hook == "")
}

func (t *Task) IsPoststart() bool {
	return t != nil && t.Lifecycle != nil &&
		t.Lifecycle.Hook == TaskLifecycleHookPoststart
}

func (t *Task) IsPoststop() bool {
	return t != nil && t.Lifecycle != nil &&
		t.Lifecycle.Hook == TaskLifecycleHookPoststop
}

func (t *Task) GetIdentity(name string) *WorkloadIdentity {
	for _, wid := range t.Identities {
		if wid.Name == name {
			return wid
		}
	}
	return nil
}

func (t *Task) GetAction(name string) *Action {
	for _, a := range t.Actions {
		if a.Name == name {
			return a
		}
	}
	return nil
}

// IdentityHandle returns a WorkloadIdentityHandle which is a pair of unique WI
// name and task name.
func (t *Task) IdentityHandle(identity *WorkloadIdentity) *WIHandle {
	return &WIHandle{
		IdentityName:       identity.Name,
		WorkloadIdentifier: t.Name,
		WorkloadType:       WorkloadTypeTask,
	}
}

func (t *Task) Copy() *Task {
	if t == nil {
		return nil
	}
	nt := new(Task)
	*nt = *t
	nt.Env = maps.Clone(nt.Env)

	if t.Services != nil {
		services := make([]*Service, len(nt.Services))
		for i, s := range nt.Services {
			services[i] = s.Copy()
		}
		nt.Services = services
	}

	nt.Constraints = CopySliceConstraints(nt.Constraints)
	nt.Affinities = CopySliceAffinities(nt.Affinities)
	nt.VolumeMounts = CopySliceVolumeMount(nt.VolumeMounts)
	nt.CSIPluginConfig = nt.CSIPluginConfig.Copy()

	nt.Vault = nt.Vault.Copy()
	nt.Consul = nt.Consul.Copy()
	nt.Resources = nt.Resources.Copy()
	nt.LogConfig = nt.LogConfig.Copy()
	nt.Meta = maps.Clone(nt.Meta)
	nt.DispatchPayload = nt.DispatchPayload.Copy()
	nt.Lifecycle = nt.Lifecycle.Copy()
	nt.Identity = nt.Identity.Copy()
	nt.Identities = helper.CopySlice(nt.Identities)
	nt.Actions = helper.CopySlice(nt.Actions)

	if t.Artifacts != nil {
		artifacts := make([]*TaskArtifact, 0, len(t.Artifacts))
		for _, a := range nt.Artifacts {
			artifacts = append(artifacts, a.Copy())
		}
		nt.Artifacts = artifacts
	}

	if i, err := copystructure.Copy(nt.Config); err != nil {
		panic(err.Error())
	} else {
		nt.Config = i.(map[string]interface{})
	}

	if t.Templates != nil {
		templates := make([]*Template, len(t.Templates))
		for i, tmpl := range nt.Templates {
			templates[i] = tmpl.Copy()
		}
		nt.Templates = templates
	}

	return nt
}

// Canonicalize canonicalizes fields in the task.
func (t *Task) Canonicalize(job *Job, tg *TaskGroup) {
	// Ensure that an empty and nil map or array are treated the same to avoid scheduling
	// problems since we use reflect DeepEquals.
	if len(t.Meta) == 0 {
		t.Meta = nil
	}
	if len(t.Config) == 0 {
		t.Config = nil
	}
	if len(t.Env) == 0 {
		t.Env = nil
	}
	if len(t.Constraints) == 0 {
		t.Constraints = nil
	}

	if len(t.Affinities) == 0 {
		t.Affinities = nil
	}

	if len(t.VolumeMounts) == 0 {
		t.VolumeMounts = nil
	}

	for _, service := range t.Services {
		service.Canonicalize(job.Name, tg.Name, t.Name, job.Namespace)
	}

	// If Resources are nil initialize them to defaults, otherwise canonicalize
	if t.Resources == nil {
		t.Resources = DefaultResources()
	} else {
		t.Resources.Canonicalize()
	}

	if t.RestartPolicy == nil {
		t.RestartPolicy = tg.RestartPolicy
	}

	// Set the default timeout if it is not specified.
	if t.KillTimeout == 0 {
		t.KillTimeout = DefaultKillTimeout
	}

	for _, policy := range t.ScalingPolicies {
		policy.Canonicalize(job, tg, t)
	}

	if t.Vault != nil {
		t.Vault.Canonicalize()
	}

	for _, template := range t.Templates {
		template.Canonicalize()
	}

	// Initialize default Nomad workload identity
	defaultIdx := -1
	for i, wid := range t.Identities {
		wid.Canonicalize()

		// For backward compatibility put the default identity in Task.Identity.
		if wid.Name == WorkloadIdentityDefaultName {
			t.Identity = wid
			defaultIdx = i
		}
	}

	// If the default identity was found in Identities above, remove it from the
	// slice.
	if defaultIdx >= 0 {
		t.Identities = slices.Delete(t.Identities, defaultIdx, defaultIdx+1)
	}

	// If there was no default identity, always create one.
	if t.Identity == nil {
		t.Identity = DefaultWorkloadIdentity()
	} else {
		t.Identity.Canonicalize()
	}
}

func (t *Task) GoString() string {
	return fmt.Sprintf("*%#v", *t)
}

// Validate is used to check a task for reasonable configuration
func (t *Task) Validate(jobType string, tg *TaskGroup) error {
	var mErr multierror.Error
	if t.Name == "" {
		mErr.Errors = append(mErr.Errors, errors.New("Missing task name"))
	}
	if strings.ContainsAny(t.Name, `/\`) {
		// We enforce this so that when creating the directory on disk it will
		// not have any slashes.
		mErr.Errors = append(mErr.Errors, errors.New("Task name cannot include slashes"))
	} else if strings.Contains(t.Name, "\000") {
		mErr.Errors = append(mErr.Errors, errors.New("Task name cannot include null characters"))
	}
	if t.Driver == "" {
		mErr.Errors = append(mErr.Errors, errors.New("Missing task driver"))
	}
	if t.KillTimeout < 0 {
		mErr.Errors = append(mErr.Errors, errors.New("KillTimeout must be a positive value"))
	} else {
		// Validate the group's update strategy does not conflict with the
		// task's kill_timeout for service jobs.
		//
		// progress_deadline = 0 has a special meaning so it should not be
		// validated against the task's kill_timeout.
		conflictsWithProgressDeadline := jobType == JobTypeService &&
			tg.Update != nil &&
			tg.Update.ProgressDeadline > 0 &&
			t.KillTimeout > tg.Update.ProgressDeadline
		if conflictsWithProgressDeadline {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("KillTimout (%s) longer than the group's ProgressDeadline (%s)",
				t.KillTimeout, tg.Update.ProgressDeadline))
		}
	}
	if t.ShutdownDelay < 0 {
		mErr.Errors = append(mErr.Errors, errors.New("ShutdownDelay must be a positive value"))
	}

	// Validate the resources.
	if t.Resources == nil {
		mErr.Errors = append(mErr.Errors, errors.New("Missing task resources"))
	} else if err := t.Resources.Validate(); err != nil {
		mErr.Errors = append(mErr.Errors, err)
	}

	// Validate the log config
	if t.LogConfig == nil {
		mErr.Errors = append(mErr.Errors, errors.New("Missing Log Config"))
	} else if err := t.LogConfig.Validate(tg.EphemeralDisk); err != nil {
		mErr.Errors = append(mErr.Errors, err)
	}

	// Validate constraints and affinities.
	for idx, constr := range t.Constraints {
		if err := constr.Validate(); err != nil {
			outer := fmt.Errorf("Constraint %d validation failed: %s", idx+1, err)
			mErr.Errors = append(mErr.Errors, outer)
		}

		switch constr.Operand {
		case ConstraintDistinctHosts, ConstraintDistinctProperty:
			outer := fmt.Errorf("Constraint %d has disallowed Operand at task level: %s", idx+1, constr.Operand)
			mErr.Errors = append(mErr.Errors, outer)
		}
	}

	if jobType == JobTypeSystem {
		if t.Affinities != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("System jobs may not have an affinity block"))
		}
	} else {
		for idx, affinity := range t.Affinities {
			if err := affinity.Validate(); err != nil {
				outer := fmt.Errorf("Affinity %d validation failed: %s", idx+1, err)
				mErr.Errors = append(mErr.Errors, outer)
			}
		}
	}

	// Validate Services
	if err := validateServices(t, tg.Networks); err != nil {
		mErr.Errors = append(mErr.Errors, err)
	}

	// Validate artifacts.
	for idx, artifact := range t.Artifacts {
		if err := artifact.Validate(); err != nil {
			outer := fmt.Errorf("Artifact %d validation failed: %v", idx+1, err)
			mErr.Errors = append(mErr.Errors, outer)
		}
	}

	// Validate Vault.
	if t.Vault != nil {
		if err := t.Vault.Validate(); err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Vault validation failed: %v", err))
		}
	}

	// Validate templates.
	destinations := make(map[string]int, len(t.Templates))
	for idx, tmpl := range t.Templates {
		if err := tmpl.Validate(); err != nil {
			outer := fmt.Errorf("Template %d validation failed: %s", idx+1, err)
			mErr.Errors = append(mErr.Errors, outer)
		}

		if other, ok := destinations[tmpl.DestPath]; ok {
			outer := fmt.Errorf("Template %d has same destination as %d", idx+1, other)
			mErr.Errors = append(mErr.Errors, outer)
		} else {
			destinations[tmpl.DestPath] = idx + 1
		}
	}

	// Validate actions.
	actions := make(map[string]bool)
	for _, action := range t.Actions {
		if err := action.Validate(); err != nil {
			outer := fmt.Errorf("Action %s validation failed: %s", action.Name, err)
			mErr.Errors = append(mErr.Errors, outer)
		}

		if handled, seen := actions[action.Name]; seen && !handled {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Action %s defined multiple times", action.Name))
			actions[action.Name] = true
			continue
		}
		actions[action.Name] = false
	}

	// Validate the dispatch payload block if there
	if t.DispatchPayload != nil {
		if err := t.DispatchPayload.Validate(); err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Dispatch Payload validation failed: %v", err))
		}
	}

	// Validate the Lifecycle block if there
	if t.Lifecycle != nil {
		if err := t.Lifecycle.Validate(); err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Lifecycle validation failed: %v", err))
		}

	}

	// Validation for TaskKind field which is used for Consul Connect integration
	if t.Kind.IsConnectProxy() {
		// This task is a Connect proxy so it should not have service blocks
		if len(t.Services) > 0 {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Connect proxy task must not have a service block"))
		}
		if t.Leader {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Connect proxy task must not have leader set"))
		}

		// Ensure the proxy task has a corresponding service entry
		serviceErr := ValidateConnectProxyService(t.Kind.Value(), tg.Services)
		if serviceErr != nil {
			mErr.Errors = append(mErr.Errors, serviceErr)
		}
	}

	// Validation for volumes
	for idx, vm := range t.VolumeMounts {
		if _, ok := tg.Volumes[vm.Volume]; !ok {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Volume Mount (%d) references undefined volume %s", idx, vm.Volume))
		}

		if err := vm.Validate(); err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Volume Mount (%d) is invalid: \"%w\"", idx, err))
		}
	}

	// Validate CSI Plugin Config
	if t.CSIPluginConfig != nil {
		if t.CSIPluginConfig.ID == "" {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("CSIPluginConfig must have a non-empty PluginID"))
		}

		if !CSIPluginTypeIsValid(t.CSIPluginConfig.Type) {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("CSIPluginConfig PluginType must be one of 'node', 'controller', or 'monolith', got: \"%s\"", t.CSIPluginConfig.Type))
		}

		if t.CSIPluginConfig.StagePublishBaseDir != "" && t.CSIPluginConfig.MountDir != "" &&
			strings.HasPrefix(t.CSIPluginConfig.StagePublishBaseDir, t.CSIPluginConfig.MountDir) {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("CSIPluginConfig StagePublishBaseDir must not be a subdirectory of MountDir, got: StagePublishBaseDir=\"%s\" MountDir=\"%s\"", t.CSIPluginConfig.StagePublishBaseDir, t.CSIPluginConfig.MountDir))
		}

		// TODO: Investigate validation of the PluginMountDir. Not much we can do apart from check IsAbs until after we understand its execution environment though :(
	}

	// Validate default Identity
	if t.Identity != nil {
		if err := t.Identity.Validate(); err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Identity %q is invalid: %w", t.Identity.Name, err))
		}
	}

	// Validate Identities
	for _, wid := range t.Identities {
		// Task.Canonicalize should move the default identity out of the Identities
		// slice, so if one is found that means it is a duplicate.
		if wid.Name == WorkloadIdentityDefaultName {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Duplicate default identities found"))
		}

		if err := wid.Validate(); err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Identity %q is invalid: %w", wid.Name, err))
		}
	}

	return mErr.ErrorOrNil()
}

// validateServices takes a task and validates the services within it are valid
// and reference ports that exist.
func validateServices(t *Task, tgNetworks Networks) error {
	var mErr multierror.Error

	// Ensure that services don't ask for nonexistent ports and their names are
	// unique.
	servicePorts := make(map[string]map[string]struct{})
	addServicePort := func(label, service string) {
		if _, ok := servicePorts[label]; !ok {
			servicePorts[label] = map[string]struct{}{}
		}
		servicePorts[label][service] = struct{}{}
	}
	knownServices := make(map[string]struct{})
	for i, service := range t.Services {
		if err := service.Validate(); err != nil {
			outer := fmt.Errorf("service[%d] %+q validation failed: %s", i, service.Name, err)
			mErr.Errors = append(mErr.Errors, outer)
		}

		if service.AddressMode == AddressModeAlloc {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("service %q cannot use address_mode=\"alloc\", only services defined in a \"group\" block can use this mode", service.Name))
		}

		// Ensure that services with the same name are not being registered for
		// the same port
		if _, ok := knownServices[service.Name+service.PortLabel]; ok {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("service %q is duplicate", service.Name))
		}
		knownServices[service.Name+service.PortLabel] = struct{}{}

		if service.PortLabel != "" {
			if service.AddressMode == "driver" {
				// Numeric port labels are valid for address_mode=driver
				_, err := strconv.Atoi(service.PortLabel)
				if err != nil {
					// Not a numeric port label, add it to list to check
					addServicePort(service.PortLabel, service.Name)
				}
			} else {
				addServicePort(service.PortLabel, service.Name)
			}
		}

		// connect block is only allowed on group level
		if service.Connect != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("service %q cannot have \"connect\" block, only services defined in a \"group\" block can", service.Name))
		}

		// Ensure that check names are unique and have valid ports
		knownChecks := make(map[string]struct{})
		for _, check := range service.Checks {
			if _, ok := knownChecks[check.Name]; ok {
				mErr.Errors = append(mErr.Errors, fmt.Errorf("check %q is duplicate", check.Name))
			}
			knownChecks[check.Name] = struct{}{}

			if check.AddressMode == AddressModeAlloc {
				mErr.Errors = append(mErr.Errors, fmt.Errorf("check %q cannot use address_mode=\"alloc\", only checks defined in a \"group\" service block can use this mode", service.Name))
			}

			if !check.RequiresPort() {
				// No need to continue validating check if it doesn't need a port
				continue
			}

			effectivePort := check.PortLabel
			if effectivePort == "" {
				// Inherits from service
				effectivePort = service.PortLabel
			}

			if effectivePort == "" {
				mErr.Errors = append(mErr.Errors, fmt.Errorf("check %q is missing a port", check.Name))
				continue
			}

			isNumeric := false
			portNumber, err := strconv.Atoi(effectivePort)
			if err == nil {
				isNumeric = true
			}

			// Numeric ports are fine for address_mode = "driver"
			if check.AddressMode == "driver" && isNumeric {
				if portNumber <= 0 {
					mErr.Errors = append(mErr.Errors, fmt.Errorf("check %q has invalid numeric port %d", check.Name, portNumber))
				}
				continue
			}

			if isNumeric {
				mErr.Errors = append(mErr.Errors, fmt.Errorf(`check %q cannot use a numeric port %d without setting address_mode="driver"`, check.Name, portNumber))
				continue
			}

			// PortLabel must exist, report errors by its parent service
			addServicePort(effectivePort, service.Name)
		}
	}

	// Get the set of group port labels.
	portLabels := make(map[string]struct{})
	if len(tgNetworks) > 0 {
		ports := tgNetworks[0].PortLabels()
		for portLabel := range ports {
			portLabels[portLabel] = struct{}{}
		}
	}

	// COMPAT(0.13)
	// Append the set of task port labels. (Note that network resources on the
	// task resources are deprecated, but we must let them continue working; a
	// warning will be emitted on job submission).
	if t.Resources != nil {
		for _, network := range t.Resources.Networks {
			for portLabel := range network.PortLabels() {
				portLabels[portLabel] = struct{}{}
			}
		}
	}

	// Iterate over a sorted list of keys to make error listings stable
	keys := make([]string, 0, len(servicePorts))
	for p := range servicePorts {
		keys = append(keys, p)
	}
	sort.Strings(keys)

	// Ensure all ports referenced in services exist.
	for _, servicePort := range keys {
		services := servicePorts[servicePort]
		_, ok := portLabels[servicePort]
		if !ok {
			names := make([]string, 0, len(services))
			for name := range services {
				names = append(names, name)
			}

			// Keep order deterministic
			sort.Strings(names)
			joined := strings.Join(names, ", ")
			err := fmt.Errorf("port label %q referenced by services %v does not exist", servicePort, joined)
			mErr.Errors = append(mErr.Errors, err)
		}
	}

	// Ensure address mode is valid
	return mErr.ErrorOrNil()
}

func (t *Task) Warnings() error {
	var mErr multierror.Error

	// Validate the resources
	if t.Resources != nil && t.Resources.IOPS != 0 {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("IOPS has been deprecated as of Nomad 0.9.0. Please remove IOPS from resource block."))
	}

	if t.Resources != nil && len(t.Resources.Networks) != 0 {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("task network resources have been deprecated as of Nomad 0.12.0. Please configure networking via group network block."))
	}

	for idx, tmpl := range t.Templates {
		if err := tmpl.Warnings(); err != nil {
			err = multierror.Prefix(err, fmt.Sprintf("Template[%d]", idx))
			mErr = *multierror.Append(&mErr, err)
		}
	}

	// Validate task-level services.
	for _, s := range t.Services {
		if err := s.Warnings(); err != nil {
			err = multierror.Prefix(err, fmt.Sprintf("Service %q:", s.Name))
			mErr = *multierror.Append(&mErr, err)
		}
	}

	for _, wid := range t.Identities {
		if err := wid.Warnings(); err != nil {
			err = multierror.Prefix(err, fmt.Sprintf("Identity[%s]", wid.Name))
			mErr.Errors = append(mErr.Errors, err)
		}
	}

	return mErr.ErrorOrNil()
}

func (t *Task) GetConstraints() []*Constraint {
	return t.Constraints
}

func (t *Task) SetConstraints(newConstraints []*Constraint) {
	t.Constraints = newConstraints
}

// TaskKind identifies the special kinds of tasks using the following format:
// '<kind_name>(:<identifier>)`. The TaskKind can optionally include an identifier that
// is opaque to the Task. This identifier can be used to relate the task to some
// other entity based on the kind.
//
// For example, a task may have the TaskKind of `connect-proxy:service` where
// 'connect-proxy' is the kind name and 'service' is the identifier that relates the
// task to the service name of which it is a connect proxy for.
type TaskKind string

func NewTaskKind(name, identifier string) TaskKind {
	return TaskKind(fmt.Sprintf("%s:%s", name, identifier))
}

// Name returns the kind name portion of the TaskKind
func (k TaskKind) Name() string {
	return strings.Split(string(k), ":")[0]
}

// Value returns the identifier of the TaskKind or an empty string if it doesn't
// include one.
func (k TaskKind) Value() string {
	if s := strings.SplitN(string(k), ":", 2); len(s) > 1 {
		return s[1]
	}
	return ""
}

func (k TaskKind) hasPrefix(prefix string) bool {
	return strings.HasPrefix(string(k), prefix+":") && len(k) > len(prefix)+1
}

// IsConnectProxy returns true if the TaskKind is connect-proxy.
func (k TaskKind) IsConnectProxy() bool {
	return k.hasPrefix(ConnectProxyPrefix)
}

// IsConnectNative returns true if the TaskKind is connect-native.
func (k TaskKind) IsConnectNative() bool {
	return k.hasPrefix(ConnectNativePrefix)
}

// IsConnectIngress returns true if the TaskKind is connect-ingress.
func (k TaskKind) IsConnectIngress() bool {
	return k.hasPrefix(ConnectIngressPrefix)
}

// IsConnectTerminating returns true if the TaskKind is connect-terminating.
func (k TaskKind) IsConnectTerminating() bool {
	return k.hasPrefix(ConnectTerminatingPrefix)
}

// IsConnectMesh returns true if the TaskKind is connect-mesh.
func (k TaskKind) IsConnectMesh() bool {
	return k.hasPrefix(ConnectMeshPrefix)
}

// IsAnyConnectGateway returns true if the TaskKind represents any one of the
// supported connect gateway types.
func (k TaskKind) IsAnyConnectGateway() bool {
	switch {
	case k.IsConnectIngress():
		return true
	case k.IsConnectTerminating():
		return true
	case k.IsConnectMesh():
		return true
	default:
		return false
	}
}

const (
	// ConnectProxyPrefix is the prefix used for fields referencing a Consul Connect
	// Proxy
	ConnectProxyPrefix = "connect-proxy"

	// ConnectNativePrefix is the prefix used for fields referencing a Connect
	// Native Task
	ConnectNativePrefix = "connect-native"

	// ConnectIngressPrefix is the prefix used for fields referencing a Consul
	// Connect Ingress Gateway Proxy.
	ConnectIngressPrefix = "connect-ingress"

	// ConnectTerminatingPrefix is the prefix used for fields referencing a Consul
	// Connect Terminating Gateway Proxy.
	ConnectTerminatingPrefix = "connect-terminating"

	// ConnectMeshPrefix is the prefix used for fields referencing a Consul Connect
	// Mesh Gateway Proxy.
	ConnectMeshPrefix = "connect-mesh"
)

// ValidateConnectProxyService checks that the service that is being
// proxied by this task exists in the task group and contains
// valid Connect config.
func ValidateConnectProxyService(serviceName string, tgServices []*Service) error {
	found := false
	names := make([]string, 0, len(tgServices))
	for _, svc := range tgServices {
		if svc.Connect == nil || svc.Connect.SidecarService == nil {
			continue
		}

		if svc.Name == serviceName {
			found = true
			break
		}

		// Build up list of mismatched Connect service names for error
		// reporting.
		names = append(names, svc.Name)
	}

	if !found {
		if len(names) == 0 {
			return fmt.Errorf("No Connect services in task group with Connect proxy (%q)", serviceName)
		} else {
			return fmt.Errorf("Connect proxy service name (%q) not found in Connect services from task group: %s", serviceName, names)
		}
	}

	return nil
}

const (
	// TemplateChangeModeNoop marks that no action should be taken if the
	// template is re-rendered
	TemplateChangeModeNoop = "noop"

	// TemplateChangeModeSignal marks that the task should be signaled if the
	// template is re-rendered
	TemplateChangeModeSignal = "signal"

	// TemplateChangeModeRestart marks that the task should be restarted if the
	// template is re-rendered
	TemplateChangeModeRestart = "restart"

	// TemplateChangeModeScript marks that the task should trigger a script if
	// the template is re-rendered
	TemplateChangeModeScript = "script"
)

var (
	// TemplateChangeModeInvalidError is the error for when an invalid change
	// mode is given
	TemplateChangeModeInvalidError = errors.New("Invalid change mode. Must be one of the following: noop, signal, script, restart")
)

// Template represents a template configuration to be rendered for a given task
type Template struct {
	// SourcePath is the path to the template to be rendered
	SourcePath string

	// DestPath is the path to where the template should be rendered
	DestPath string

	// EmbeddedTmpl store the raw template. This is useful for smaller templates
	// where they are embedded in the job file rather than sent as an artifact
	EmbeddedTmpl string

	// ChangeMode indicates what should be done if the template is re-rendered
	ChangeMode string

	// ChangeSignal is the signal that should be sent if the change mode
	// requires it.
	ChangeSignal string

	// ChangeScript is the configuration of the script. It's required if
	// ChangeMode is set to script.
	ChangeScript *ChangeScript

	// Splay is used to avoid coordinated restarts of processes by applying a
	// random wait between 0 and the given splay value before signalling the
	// application of a change
	Splay time.Duration

	// Perms is the permission the file should be written out with.
	Perms string
	// User and group that should own the file.
	Uid *int
	Gid *int

	// LeftDelim and RightDelim are optional configurations to control what
	// delimiter is utilized when parsing the template.
	LeftDelim  string
	RightDelim string

	// Envvars enables exposing the template as environment variables
	// instead of as a file. The template must be of the form:
	//
	//	VAR_NAME_1={{ key service/my-key }}
	//	VAR_NAME_2=raw string and {{ env "attr.kernel.name" }}
	//
	// Lines will be split on the initial "=" with the first part being the
	// key name and the second part the value.
	// Empty lines and lines starting with # will be ignored, but to avoid
	// escaping issues #s within lines will not be treated as comments.
	Envvars bool

	// VaultGrace is the grace duration between lease renewal and reacquiring a
	// secret. If the lease of a secret is less than the grace, a new secret is
	// acquired.
	// COMPAT(0.12) VaultGrace has been ignored by Vault since Vault v0.5.
	VaultGrace time.Duration

	// WaitConfig is used to override the global WaitConfig on a per-template basis
	Wait *WaitConfig

	// ErrMissingKey is used to control how the template behaves when attempting
	// to index a struct or map key that does not exist.
	ErrMissingKey bool
}

// DefaultTemplate returns a default template.
func DefaultTemplate() *Template {
	return &Template{
		ChangeMode: TemplateChangeModeRestart,
		Splay:      5 * time.Second,
		Perms:      "0644",
	}
}

func (t *Template) Equal(o *Template) bool {
	if t == nil || o == nil {
		return t == o
	}
	switch {
	case t.SourcePath != o.SourcePath:
		return false
	case t.DestPath != o.DestPath:
		return false
	case t.EmbeddedTmpl != o.EmbeddedTmpl:
		return false
	case t.ChangeMode != o.ChangeMode:
		return false
	case t.ChangeSignal != o.ChangeSignal:
		return false
	case !t.ChangeScript.Equal(o.ChangeScript):
		return false
	case t.Splay != o.Splay:
		return false
	case t.Perms != o.Perms:
		return false
	case !pointer.Eq(t.Uid, o.Uid):
		return false
	case !pointer.Eq(t.Gid, o.Gid):
		return false
	case t.LeftDelim != o.LeftDelim:
		return false
	case t.RightDelim != o.RightDelim:
		return false
	case t.Envvars != o.Envvars:
		return false
	case t.VaultGrace != o.VaultGrace:
		return false
	case !t.Wait.Equal(o.Wait):
		return false
	case t.ErrMissingKey != o.ErrMissingKey:
		return false
	}
	return true
}

func (t *Template) Copy() *Template {
	if t == nil {
		return nil
	}
	nt := new(Template)
	*nt = *t

	nt.ChangeScript = t.ChangeScript.Copy()
	nt.Wait = t.Wait.Copy()

	return nt
}

func (t *Template) Canonicalize() {
	if t.ChangeSignal != "" {
		t.ChangeSignal = strings.ToUpper(t.ChangeSignal)
	}
}

func (t *Template) Validate() error {
	var mErr multierror.Error

	// Verify we have something to render
	if t.SourcePath == "" && t.EmbeddedTmpl == "" {
		_ = multierror.Append(&mErr, fmt.Errorf("Must specify a source path or have an embedded template"))
	}

	// Verify we can render somewhere
	if t.DestPath == "" {
		_ = multierror.Append(&mErr, fmt.Errorf("Must specify a destination for the template"))
	}

	// Verify the destination doesn't escape
	escaped, err := escapingfs.PathEscapesAllocViaRelative("task", t.DestPath)
	if err != nil {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("invalid destination path: %v", err))
	} else if escaped {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("destination escapes allocation directory"))
	}

	// Verify a proper change mode
	switch t.ChangeMode {
	case TemplateChangeModeNoop, TemplateChangeModeRestart:
	case TemplateChangeModeSignal:
		if t.ChangeSignal == "" {
			_ = multierror.Append(&mErr, fmt.Errorf("Must specify signal value when change mode is signal"))
		}
		if t.Envvars {
			_ = multierror.Append(&mErr, fmt.Errorf("cannot use signals with env var templates"))
		}
	case TemplateChangeModeScript:
		if t.ChangeScript == nil {
			_ = multierror.Append(&mErr, fmt.Errorf("must specify change script configuration value when change mode is script"))
		}

		if err = t.ChangeScript.Validate(); err != nil {
			_ = multierror.Append(&mErr, err)
		}
	default:
		_ = multierror.Append(&mErr, TemplateChangeModeInvalidError)
	}

	// Verify the splay is positive
	if t.Splay < 0 {
		_ = multierror.Append(&mErr, fmt.Errorf("Must specify positive splay value"))
	}

	// Verify the permissions
	if t.Perms != "" {
		if _, err := strconv.ParseUint(t.Perms, 8, 12); err != nil {
			_ = multierror.Append(&mErr, fmt.Errorf("Failed to parse %q as octal: %v", t.Perms, err))
		}
	}

	if err = t.Wait.Validate(); err != nil {
		_ = multierror.Append(&mErr, err)
	}

	return mErr.ErrorOrNil()
}

func (t *Template) Warnings() error {
	var mErr multierror.Error

	// Deprecation notice for vault_grace
	if t.VaultGrace != 0 {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("VaultGrace has been deprecated as of Nomad 0.11 and ignored since Vault 0.5. Please remove VaultGrace / vault_grace from template block."))
	}

	return mErr.ErrorOrNil()
}

// DiffID fulfills the DiffableWithID interface.
func (t *Template) DiffID() string {
	return t.DestPath
}

// ChangeScript holds the configuration for the script that is executed if
// change mode is set to script
type ChangeScript struct {
	// Command is the full path to the script
	Command string
	// Args is a slice of arguments passed to the script
	Args []string
	// Timeout is the amount of seconds we wait for the script to finish
	Timeout time.Duration
	// FailOnError indicates whether a task should fail in case script execution
	// fails or log script failure and don't interrupt the task
	FailOnError bool
}

func (cs *ChangeScript) Equal(o *ChangeScript) bool {
	if cs == nil || o == nil {
		return cs == o
	}
	switch {
	case cs.Command != o.Command:
		return false
	case !slices.Equal(cs.Args, o.Args):
		return false
	case cs.Timeout != o.Timeout:
		return false
	case cs.FailOnError != o.FailOnError:
		return false
	}
	return true
}

func (cs *ChangeScript) Copy() *ChangeScript {
	if cs == nil {
		return nil
	}
	return &ChangeScript{
		Command:     cs.Command,
		Args:        slices.Clone(cs.Args),
		Timeout:     cs.Timeout,
		FailOnError: cs.FailOnError,
	}
}

// Validate makes sure all the required fields of ChangeScript are present
func (cs *ChangeScript) Validate() error {
	if cs == nil {
		return nil
	}

	if cs.Command == "" {
		return fmt.Errorf("must specify script path value when change mode is script")
	}

	return nil
}

// WaitConfig is the Min/Max duration used by the Consul Template Watcher. Consul
// Template relies on pointer based business logic. This struct uses pointers so
// that we tell the different between zero values and unset values.
type WaitConfig struct {
	Min *time.Duration
	Max *time.Duration
}

// Copy returns a deep copy of this configuration.
func (wc *WaitConfig) Copy() *WaitConfig {
	if wc == nil {
		return nil
	}

	nwc := new(WaitConfig)

	if wc.Min != nil {
		nwc.Min = wc.Min
	}

	if wc.Max != nil {
		nwc.Max = wc.Max
	}

	return nwc
}

func (wc *WaitConfig) Equal(o *WaitConfig) bool {
	if wc == nil || o == nil {
		return wc == o
	}
	switch {
	case !pointer.Eq(wc.Min, o.Min):
		return false
	case !pointer.Eq(wc.Max, o.Max):
		return false
	}
	return true
}

// Validate that the min is not greater than the max
func (wc *WaitConfig) Validate() error {
	if wc == nil {
		return nil
	}

	// If either one is nil, they aren't comparable, so they can't be invalid.
	if wc.Min == nil || wc.Max == nil {
		return nil
	}

	if *wc.Min > *wc.Max {
		return fmt.Errorf("wait min %s is greater than max %s", wc.Min, wc.Max)
	}

	return nil
}

// AllocStateField records a single event that changes the state of the whole allocation
type AllocStateField uint8

const (
	AllocStateFieldClientStatus AllocStateField = iota
)

type AllocState struct {
	Field AllocStateField
	Value string
	Time  time.Time
}

// TaskHandle is  optional handle to a task propogated to the servers for use
// by remote tasks. Since remote tasks are not implicitly lost when the node
// they are assigned to is down, their state is migrated to the replacement
// allocation.
//
// Minimal set of fields from plugins/drivers/task_handle.go:TaskHandle
type TaskHandle struct {
	// Version of driver state. Used by the driver to gracefully handle
	// plugin upgrades.
	Version int

	// Driver-specific state containing a handle to the remote task.
	DriverState []byte
}

func (h *TaskHandle) Copy() *TaskHandle {
	if h == nil {
		return nil
	}

	newTH := TaskHandle{
		Version:     h.Version,
		DriverState: make([]byte, len(h.DriverState)),
	}
	copy(newTH.DriverState, h.DriverState)
	return &newTH
}

func (h *TaskHandle) Equal(o *TaskHandle) bool {
	if h == nil || o == nil {
		return h == o
	}
	if h.Version != o.Version {
		return false
	}
	return bytes.Equal(h.DriverState, o.DriverState)
}

// Set of possible states for a task.
const (
	TaskStatePending = "pending" // The task is waiting to be run.
	TaskStateRunning = "running" // The task is currently running.
	TaskStateDead    = "dead"    // Terminal state of task.
)

// TaskState tracks the current state of a task and events that caused state
// transitions.
type TaskState struct {
	// The current state of the task.
	State string

	// Failed marks a task as having failed
	Failed bool

	// Restarts is the number of times the task has restarted
	Restarts uint64

	// LastRestart is the time the task last restarted. It is updated each time the
	// task restarts
	LastRestart time.Time

	// StartedAt is the time the task is started. It is updated each time the
	// task starts
	StartedAt time.Time

	// FinishedAt is the time at which the task transitioned to dead and will
	// not be started again.
	FinishedAt time.Time

	// Series of task events that transition the state of the task.
	Events []*TaskEvent

	// Experimental -  TaskHandle is based on drivers.TaskHandle and used
	// by remote task drivers to migrate task handles between allocations.
	TaskHandle *TaskHandle

	// Enterprise Only - Paused is set to the paused state of the task. See
	// task_sched.go
	Paused TaskScheduleState
}

// NewTaskState returns a TaskState initialized in the Pending state.
func NewTaskState() *TaskState {
	return &TaskState{
		State: TaskStatePending,
	}
}

// Canonicalize ensures the TaskState has a State set. It should default to
// Pending.
func (ts *TaskState) Canonicalize() {
	if ts.State == "" {
		ts.State = TaskStatePending
	}
}

func (ts *TaskState) Copy() *TaskState {
	if ts == nil {
		return nil
	}
	newTS := new(TaskState)
	*newTS = *ts

	if ts.Events != nil {
		newTS.Events = make([]*TaskEvent, len(ts.Events))
		for i, e := range ts.Events {
			newTS.Events[i] = e.Copy()
		}
	}

	newTS.TaskHandle = ts.TaskHandle.Copy()
	return newTS
}

// Successful returns whether a task finished successfully. Only meaningful for
// for batch allocations or ephemeral (non-sidecar) lifecycle tasks part of a
// service or system allocation.
func (ts *TaskState) Successful() bool {
	return ts.State == TaskStateDead && !ts.Failed
}

func (ts *TaskState) Equal(o *TaskState) bool {
	if ts.State != o.State {
		return false
	}
	if ts.Failed != o.Failed {
		return false
	}
	if ts.Restarts != o.Restarts {
		return false
	}
	if ts.LastRestart != o.LastRestart {
		return false
	}
	if ts.StartedAt != o.StartedAt {
		return false
	}
	if ts.FinishedAt != o.FinishedAt {
		return false
	}
	if !slices.EqualFunc(ts.Events, o.Events, func(ts, o *TaskEvent) bool {
		return ts.Equal(o)
	}) {
		return false
	}
	if !ts.TaskHandle.Equal(o.TaskHandle) {
		return false
	}

	return true
}

const (
	// TaskSetupFailure indicates that the task could not be started due to a
	// a setup failure.
	TaskSetupFailure = "Setup Failure"

	// TaskDriveFailure indicates that the task could not be started due to a
	// failure in the driver. TaskDriverFailure is considered Recoverable.
	TaskDriverFailure = "Driver Failure"

	// TaskReceived signals that the task has been pulled by the client at the
	// given timestamp.
	TaskReceived = "Received"

	// TaskFailedValidation indicates the task was invalid and as such was not run.
	// TaskFailedValidation is not considered Recoverable.
	TaskFailedValidation = "Failed Validation"

	// TaskStarted signals that the task was started and its timestamp can be
	// used to determine the running length of the task.
	TaskStarted = "Started"

	// TaskPausing indicates the task is being killed, but will be
	// started again to await the next start of its task schedule (Enterprise).
	TaskPausing = "Pausing"

	// TaskTerminated indicates that the task was started and exited.
	TaskTerminated = "Terminated"

	// TaskKilling indicates a kill signal has been sent to the task.
	TaskKilling = "Killing"

	// TaskKilled indicates a user has killed the task.
	TaskKilled = "Killed"

	// TaskRestarting indicates that task terminated and is being restarted.
	TaskRestarting = "Restarting"

	// TaskNotRestarting indicates that the task has failed and is not being
	// restarted because it has exceeded its restart policy.
	TaskNotRestarting = "Not Restarting"

	// TaskRestartSignal indicates that the task has been signaled to be
	// restarted
	TaskRestartSignal = "Restart Signaled"

	// TaskSignaling indicates that the task is being signalled.
	TaskSignaling = "Signaling"

	// TaskDownloadingArtifacts means the task is downloading the artifacts
	// specified in the task.
	TaskDownloadingArtifacts = "Downloading Artifacts"

	// TaskArtifactDownloadFailed indicates that downloading the artifacts
	// failed.
	TaskArtifactDownloadFailed = "Failed Artifact Download"

	// TaskBuildingTaskDir indicates that the task directory/chroot is being
	// built.
	TaskBuildingTaskDir = "Building Task Directory"

	// TaskSetup indicates the task runner is setting up the task environment
	TaskSetup = "Task Setup"

	// TaskDiskExceeded indicates that one of the tasks in a taskgroup has
	// exceeded the requested disk resources.
	TaskDiskExceeded = "Disk Resources Exceeded"

	// TaskSiblingFailed indicates that a sibling task in the task group has
	// failed.
	TaskSiblingFailed = "Sibling Task Failed"

	// TaskDriverMessage is an informational event message emitted by
	// drivers such as when they're performing a long running action like
	// downloading an image.
	TaskDriverMessage = "Driver"

	// TaskLeaderDead indicates that the leader task within the has finished.
	TaskLeaderDead = "Leader Task Dead"

	// TaskMainDead indicates that the main tasks have dead
	TaskMainDead = "Main Tasks Dead"

	// TaskHookFailed indicates that one of the hooks for a task failed.
	TaskHookFailed = "Task hook failed"

	// TaskHookMessage indicates that one of the hooks for a task emitted a
	// message.
	TaskHookMessage = "Task hook message"

	// TaskRestoreFailed indicates Nomad was unable to reattach to a
	// restored task.
	TaskRestoreFailed = "Failed Restoring Task"

	// TaskPluginUnhealthy indicates that a plugin managed by Nomad became unhealthy
	TaskPluginUnhealthy = "Plugin became unhealthy"

	// TaskPluginHealthy indicates that a plugin managed by Nomad became healthy
	TaskPluginHealthy = "Plugin became healthy"

	// TaskClientReconnected indicates that the client running the task reconnected.
	TaskClientReconnected = "Reconnected"

	// TaskWaitingShuttingDownDelay indicates that the task is waiting for
	// shutdown delay before being TaskKilled
	TaskWaitingShuttingDownDelay = "Waiting for shutdown delay"

	// TaskSkippingShutdownDelay indicates that the task operation was
	// configured to ignore the shutdown delay value set for the tas.
	TaskSkippingShutdownDelay = "Skipping shutdown delay"

	// TaskRunning indicates a task is running due to a schedule or schedule
	// override. (Enterprise)
	TaskRunning = "Running"
)

// TaskEvent is an event that effects the state of a task and contains meta-data
// appropriate to the events type.
type TaskEvent struct {
	Type string
	Time int64 // Unix Nanosecond timestamp

	Message string // A possible message explaining the termination of the task.

	// DisplayMessage is a human friendly message about the event
	DisplayMessage string

	// Details is a map with annotated info about the event
	Details map[string]string

	// DEPRECATION NOTICE: The following fields are deprecated and will be removed
	// in a future release. Field values are available in the Details map.

	// FailsTask marks whether this event fails the task.
	// Deprecated, use Details["fails_task"] to access this.
	FailsTask bool

	// Restart fields.
	// Deprecated, use Details["restart_reason"] to access this.
	RestartReason string

	// Setup Failure fields.
	// Deprecated, use Details["setup_error"] to access this.
	SetupError string

	// Driver Failure fields.
	// Deprecated, use Details["driver_error"] to access this.
	DriverError string // A driver error occurred while starting the task.

	// Task Terminated Fields.

	// Deprecated, use Details["exit_code"] to access this.
	ExitCode int // The exit code of the task.

	// Deprecated, use Details["signal"] to access this.
	Signal int // The signal that terminated the task.

	// Killing fields
	// Deprecated, use Details["kill_timeout"] to access this.
	KillTimeout time.Duration

	// Task Killed Fields.
	// Deprecated, use Details["kill_error"] to access this.
	KillError string // Error killing the task.

	// KillReason is the reason the task was killed
	// Deprecated, use Details["kill_reason"] to access this.
	KillReason string

	// TaskRestarting fields.
	// Deprecated, use Details["start_delay"] to access this.
	StartDelay int64 // The sleep period before restarting the task in unix nanoseconds.

	// Artifact Download fields
	// Deprecated, use Details["download_error"] to access this.
	DownloadError string // Error downloading artifacts

	// Validation fields
	// Deprecated, use Details["validation_error"] to access this.
	ValidationError string // Validation error

	// The maximum allowed task disk size.
	// Deprecated, use Details["disk_limit"] to access this.
	DiskLimit int64

	// Name of the sibling task that caused termination of the task that
	// the TaskEvent refers to.
	// Deprecated, use Details["failed_sibling"] to access this.
	FailedSibling string

	// VaultError is the error from token renewal
	// Deprecated, use Details["vault_renewal_error"] to access this.
	VaultError string

	// TaskSignalReason indicates the reason the task is being signalled.
	// Deprecated, use Details["task_signal_reason"] to access this.
	TaskSignalReason string

	// TaskSignal is the signal that was sent to the task
	// Deprecated, use Details["task_signal"] to access this.
	TaskSignal string

	// DriverMessage indicates a driver action being taken.
	// Deprecated, use Details["driver_message"] to access this.
	DriverMessage string

	// GenericSource is the source of a message.
	// Deprecated, is redundant with event type.
	GenericSource string
}

func (e *TaskEvent) PopulateEventDisplayMessage() {
	// Build up the description based on the event type.
	if e == nil { //TODO(preetha) needs investigation alloc_runner's Run method sends a nil event when sigterming nomad. Why?
		return
	}

	if e.DisplayMessage != "" {
		return
	}

	var desc string
	switch e.Type {
	case TaskSetup:
		desc = e.Message
	case TaskStarted:
		desc = "Task started by client"
	case TaskReceived:
		desc = "Task received by client"
	case TaskFailedValidation:
		if e.ValidationError != "" {
			desc = e.ValidationError
		} else {
			desc = "Validation of task failed"
		}
	case TaskSetupFailure:
		if e.SetupError != "" {
			desc = e.SetupError
		} else {
			desc = "Task setup failed"
		}
	case TaskDriverFailure:
		if e.DriverError != "" {
			desc = e.DriverError
		} else {
			desc = "Failed to start task"
		}
	case TaskDownloadingArtifacts:
		desc = "Client is downloading artifacts"
	case TaskArtifactDownloadFailed:
		if e.DownloadError != "" {
			desc = e.DownloadError
		} else {
			desc = "Failed to download artifacts"
		}
	case TaskKilling:
		if e.KillReason != "" {
			desc = e.KillReason
		} else if e.KillTimeout != 0 {
			desc = fmt.Sprintf("Sent interrupt. Waiting %v before force killing", e.KillTimeout)
		} else {
			desc = "Sent interrupt"
		}
	case TaskKilled:
		if e.KillError != "" {
			desc = e.KillError
		} else {
			desc = "Task successfully killed"
		}
	case TaskTerminated:
		var parts []string
		parts = append(parts, fmt.Sprintf("Exit Code: %d", e.ExitCode))

		if e.Signal != 0 {
			parts = append(parts, fmt.Sprintf("Signal: %d", e.Signal))
		}

		if e.Message != "" {
			parts = append(parts, fmt.Sprintf("Exit Message: %q", e.Message))
		}
		desc = strings.Join(parts, ", ")
	case TaskRestarting:
		in := fmt.Sprintf("Task restarting in %v", time.Duration(e.StartDelay))
		if e.RestartReason != "" && e.RestartReason != ReasonWithinPolicy {
			desc = fmt.Sprintf("%s - %s", e.RestartReason, in)
		} else {
			desc = in
		}
	case TaskNotRestarting:
		if e.RestartReason != "" {
			desc = e.RestartReason
		} else {
			desc = "Task exceeded restart policy"
		}
	case TaskSiblingFailed:
		if e.FailedSibling != "" {
			desc = fmt.Sprintf("Task's sibling %q failed", e.FailedSibling)
		} else {
			desc = "Task's sibling failed"
		}
	case TaskSignaling:
		sig := e.TaskSignal
		reason := e.TaskSignalReason

		if sig == "" && reason == "" {
			desc = "Task being sent a signal"
		} else if sig == "" {
			desc = reason
		} else if reason == "" {
			desc = fmt.Sprintf("Task being sent signal %v", sig)
		} else {
			desc = fmt.Sprintf("Task being sent signal %v: %v", sig, reason)
		}
	case TaskRestartSignal:
		if e.RestartReason != "" {
			desc = e.RestartReason
		} else {
			desc = "Task signaled to restart"
		}
	case TaskDriverMessage:
		desc = e.DriverMessage
	case TaskLeaderDead:
		desc = "Leader Task in Group dead"
	case TaskMainDead:
		desc = "Main tasks in the group died"
	case TaskClientReconnected:
		desc = "Client reconnected"
	default:
		desc = e.Message
	}

	e.DisplayMessage = desc
}

func (e *TaskEvent) GoString() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%v - %v", e.Time, e.Type)
}

// Equal on TaskEvent ignores the deprecated fields
func (e *TaskEvent) Equal(o *TaskEvent) bool {
	if e == nil || o == nil {
		return e == o
	}

	if e.Type != o.Type {
		return false
	}
	if e.Time != o.Time {
		return false
	}
	if e.Message != o.Message {
		return false
	}
	if e.DisplayMessage != o.DisplayMessage {
		return false
	}
	if !maps.Equal(e.Details, o.Details) {
		return false
	}

	return true
}

// SetDisplayMessage sets the display message of TaskEvent
func (e *TaskEvent) SetDisplayMessage(msg string) *TaskEvent {
	e.DisplayMessage = msg
	return e
}

// SetMessage sets the message of TaskEvent
func (e *TaskEvent) SetMessage(msg string) *TaskEvent {
	e.Message = msg
	e.Details["message"] = msg
	return e
}

func (e *TaskEvent) Copy() *TaskEvent {
	if e == nil {
		return nil
	}
	copy := new(TaskEvent)
	*copy = *e
	return copy
}

func NewTaskEvent(event string) *TaskEvent {
	return &TaskEvent{
		Type:    event,
		Time:    time.Now().UnixNano(),
		Details: make(map[string]string),
	}
}

// SetSetupError is used to store an error that occurred while setting up the
// task
func (e *TaskEvent) SetSetupError(err error) *TaskEvent {
	if err != nil {
		e.SetupError = err.Error()
		e.Details["setup_error"] = err.Error()
	}
	return e
}

func (e *TaskEvent) SetFailsTask() *TaskEvent {
	e.FailsTask = true
	e.Details["fails_task"] = "true"
	return e
}

func (e *TaskEvent) SetDriverError(err error) *TaskEvent {
	if err != nil {
		e.DriverError = err.Error()
		e.Details["driver_error"] = err.Error()
	}
	return e
}

func (e *TaskEvent) SetExitCode(c int) *TaskEvent {
	e.ExitCode = c
	e.Details["exit_code"] = fmt.Sprintf("%d", c)
	return e
}

func (e *TaskEvent) SetSignal(s int) *TaskEvent {
	e.Signal = s
	e.Details["signal"] = fmt.Sprintf("%d", s)
	return e
}

func (e *TaskEvent) SetSignalText(s string) *TaskEvent {
	e.Details["signal"] = s
	return e
}

func (e *TaskEvent) SetExitMessage(err error) *TaskEvent {
	if err != nil {
		e.Message = err.Error()
		e.Details["exit_message"] = err.Error()
	}
	return e
}

func (e *TaskEvent) SetKillError(err error) *TaskEvent {
	if err != nil {
		e.KillError = err.Error()
		e.Details["kill_error"] = err.Error()
	}
	return e
}

func (e *TaskEvent) SetKillReason(r string) *TaskEvent {
	e.KillReason = r
	e.Details["kill_reason"] = r
	return e
}

func (e *TaskEvent) SetRestartDelay(delay time.Duration) *TaskEvent {
	e.StartDelay = int64(delay)
	e.Details["start_delay"] = fmt.Sprintf("%d", delay)
	return e
}

func (e *TaskEvent) SetRestartReason(reason string) *TaskEvent {
	e.RestartReason = reason
	e.Details["restart_reason"] = reason
	return e
}

func (e *TaskEvent) SetTaskSignalReason(r string) *TaskEvent {
	e.TaskSignalReason = r
	e.Details["task_signal_reason"] = r
	return e
}

func (e *TaskEvent) SetTaskSignal(s os.Signal) *TaskEvent {
	e.TaskSignal = s.String()
	e.Details["task_signal"] = s.String()
	return e
}

func (e *TaskEvent) SetDownloadError(err error) *TaskEvent {
	if err != nil {
		e.DownloadError = err.Error()
		e.Details["download_error"] = err.Error()
	}
	return e
}

func (e *TaskEvent) SetValidationError(err error) *TaskEvent {
	if err != nil {
		e.ValidationError = err.Error()
		e.Details["validation_error"] = err.Error()
	}
	return e
}

func (e *TaskEvent) SetKillTimeout(timeout, maxTimeout time.Duration) *TaskEvent {
	actual := min(timeout, maxTimeout)
	e.KillTimeout = actual
	e.Details["kill_timeout"] = actual.String()
	return e
}

func (e *TaskEvent) SetDiskLimit(limit int64) *TaskEvent {
	e.DiskLimit = limit
	e.Details["disk_limit"] = fmt.Sprintf("%d", limit)
	return e
}

func (e *TaskEvent) SetFailedSibling(sibling string) *TaskEvent {
	e.FailedSibling = sibling
	e.Details["failed_sibling"] = sibling
	return e
}

func (e *TaskEvent) SetVaultRenewalError(err error) *TaskEvent {
	if err != nil {
		e.VaultError = err.Error()
		e.Details["vault_renewal_error"] = err.Error()
	}
	return e
}

func (e *TaskEvent) SetDriverMessage(m string) *TaskEvent {
	e.DriverMessage = m
	e.Details["driver_message"] = m
	return e
}

func (e *TaskEvent) SetOOMKilled(oom bool) *TaskEvent {
	e.Details["oom_killed"] = strconv.FormatBool(oom)
	return e
}

// TaskArtifact is an artifact to download before running the task.
type TaskArtifact struct {
	// GetterSource is the source to download an artifact using go-getter
	GetterSource string

	// GetterOptions are options to use when downloading the artifact using
	// go-getter.
	GetterOptions map[string]string

	// GetterHeaders are headers to use when downloading the artifact using
	// go-getter.
	GetterHeaders map[string]string

	// GetterMode is the go-getter.ClientMode for fetching resources.
	// Defaults to "any" but can be set to "file" or "dir".
	GetterMode string

	// GetterInsecure is a flag to disable SSL certificate verification when
	// downloading the artifact using go-getter.
	GetterInsecure bool

	// RelativeDest is the download destination given relative to the task's
	// directory.
	RelativeDest string

	// Chown the resulting files and directories to the user of the task.
	//
	// Defaults to false.
	Chown bool
}

func (ta *TaskArtifact) Equal(o *TaskArtifact) bool {
	if ta == nil || o == nil {
		return ta == o
	}
	switch {
	case ta.GetterSource != o.GetterSource:
		return false
	case !maps.Equal(ta.GetterOptions, o.GetterOptions):
		return false
	case !maps.Equal(ta.GetterHeaders, o.GetterHeaders):
		return false
	case ta.GetterMode != o.GetterMode:
		return false
	case ta.GetterInsecure != o.GetterInsecure:
		return false
	case ta.RelativeDest != o.RelativeDest:
		return false
	case ta.Chown != o.Chown:
		return false
	}
	return true
}

func (ta *TaskArtifact) Copy() *TaskArtifact {
	if ta == nil {
		return nil
	}
	return &TaskArtifact{
		GetterSource:   ta.GetterSource,
		GetterOptions:  maps.Clone(ta.GetterOptions),
		GetterHeaders:  maps.Clone(ta.GetterHeaders),
		GetterMode:     ta.GetterMode,
		GetterInsecure: ta.GetterInsecure,
		RelativeDest:   ta.RelativeDest,
		Chown:          ta.Chown,
	}
}

func (ta *TaskArtifact) GoString() string {
	return fmt.Sprintf("%+v", ta)
}

// DiffID fulfills the DiffableWithID interface.
func (ta *TaskArtifact) DiffID() string {
	return ta.RelativeDest
}

// hashStringMap appends a deterministic hash of m onto h.
func hashStringMap(h hash.Hash, m map[string]string) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		_, _ = h.Write([]byte(k))
		_, _ = h.Write([]byte(m[k]))
	}
}

// Hash creates a unique identifier for a TaskArtifact as the same GetterSource
// may be specified multiple times with different destinations.
func (ta *TaskArtifact) Hash() string {
	h, err := blake2b.New256(nil)
	if err != nil {
		panic(err)
	}

	_, _ = h.Write([]byte(ta.GetterSource))

	hashStringMap(h, ta.GetterOptions)
	hashStringMap(h, ta.GetterHeaders)

	_, _ = h.Write([]byte(ta.GetterMode))
	_, _ = h.Write([]byte(strconv.FormatBool(ta.GetterInsecure)))
	_, _ = h.Write([]byte(ta.RelativeDest))
	_, _ = h.Write([]byte(strconv.FormatBool(ta.Chown)))
	return base64.RawStdEncoding.EncodeToString(h.Sum(nil))
}

func (ta *TaskArtifact) Validate() error {
	// Verify the source
	var mErr multierror.Error
	if ta.GetterSource == "" {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("source must be specified"))
	}

	switch ta.GetterMode {
	case "":
		// Default to any
		ta.GetterMode = GetterModeAny
	case GetterModeAny, GetterModeFile, GetterModeDir:
		// Ok
	default:
		mErr.Errors = append(mErr.Errors, fmt.Errorf("invalid artifact mode %q; must be one of: %s, %s, %s",
			ta.GetterMode, GetterModeAny, GetterModeFile, GetterModeDir))
	}

	escaped, err := escapingfs.PathEscapesAllocViaRelative("task", ta.RelativeDest)
	if err != nil {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("invalid destination path: %v", err))
	} else if escaped {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("destination escapes allocation directory"))
	}

	if err := ta.validateChecksum(); err != nil {
		mErr.Errors = append(mErr.Errors, err)
	}

	return mErr.ErrorOrNil()
}

func (ta *TaskArtifact) validateChecksum() error {
	check, ok := ta.GetterOptions["checksum"]
	if !ok {
		return nil
	}

	// Job struct validation occurs before interpolation resolution can be effective.
	// Skip checking if checksum contain variable reference, and artifacts fetching will
	// eventually fail, if checksum is indeed invalid.
	if args.ContainsEnv(check) {
		return nil
	}

	check = strings.TrimSpace(check)
	if check == "" {
		return fmt.Errorf("checksum value cannot be empty")
	}

	parts := strings.Split(check, ":")
	if l := len(parts); l != 2 {
		return fmt.Errorf(`checksum must be given as "type:value"; got %q`, check)
	}

	checksumVal := parts[1]
	checksumBytes, err := hex.DecodeString(checksumVal)
	if err != nil {
		return fmt.Errorf("invalid checksum: %v", err)
	}

	checksumType := parts[0]
	expectedLength := 0
	switch checksumType {
	case "md5":
		expectedLength = md5.Size
	case "sha1":
		expectedLength = sha1.Size
	case "sha256":
		expectedLength = sha256.Size
	case "sha512":
		expectedLength = sha512.Size
	default:
		return fmt.Errorf("unsupported checksum type: %s", checksumType)
	}

	if len(checksumBytes) != expectedLength {
		return fmt.Errorf("invalid %s checksum: %v", checksumType, checksumVal)
	}

	return nil
}

const (
	ConstraintDistinctProperty  = "distinct_property"
	ConstraintDistinctHosts     = "distinct_hosts"
	ConstraintRegex             = "regexp"
	ConstraintVersion           = "version"
	ConstraintSemver            = "semver"
	ConstraintSetContains       = "set_contains"
	ConstraintSetContainsAll    = "set_contains_all"
	ConstraintSetContainsAny    = "set_contains_any"
	ConstraintAttributeIsSet    = "is_set"
	ConstraintAttributeIsNotSet = "is_not_set"
)

// A Constraint is used to restrict placement options.
type Constraint struct {
	LTarget string // Left-hand target
	RTarget string // Right-hand target
	Operand string // Constraint operand (<=, <, =, !=, >, >=), contains, near
}

// Equal checks if two constraints are equal.
func (c *Constraint) Equal(o *Constraint) bool {
	return c == o ||
		c.LTarget == o.LTarget &&
			c.RTarget == o.RTarget &&
			c.Operand == o.Operand
}

func (c *Constraint) Copy() *Constraint {
	if c == nil {
		return nil
	}
	return &Constraint{
		LTarget: c.LTarget,
		RTarget: c.RTarget,
		Operand: c.Operand,
	}
}

func (c *Constraint) String() string {
	return fmt.Sprintf("%s %s %s", c.LTarget, c.Operand, c.RTarget)
}

func (c *Constraint) Validate() error {
	var mErr multierror.Error
	if c.Operand == "" {
		mErr.Errors = append(mErr.Errors, errors.New("Missing constraint operand"))
	}

	// requireLtarget specifies whether the constraint requires an LTarget to be
	// provided.
	requireLtarget := true

	// Perform additional validation based on operand
	switch c.Operand {
	case ConstraintDistinctHosts:
		requireLtarget = false
	case ConstraintSetContainsAll, ConstraintSetContainsAny, ConstraintSetContains:
		if c.RTarget == "" {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Set contains constraint requires an RTarget"))
		}
	case ConstraintRegex:
		if _, err := regexp.Compile(c.RTarget); err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Regular expression failed to compile: %v", err))
		}
	case ConstraintVersion:
		if _, err := version.NewConstraint(c.RTarget); err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Version constraint is invalid: %v", err))
		}
	case ConstraintSemver:
		if _, err := semver.NewConstraint(c.RTarget); err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Semver constraint is invalid: %v", err))
		}
	case ConstraintDistinctProperty:
		// If a count is set, make sure it is convertible to a uint64
		if c.RTarget != "" {
			count, err := strconv.ParseUint(c.RTarget, 10, 64)
			if err != nil {
				mErr.Errors = append(mErr.Errors, fmt.Errorf("Failed to convert RTarget %q to uint64: %v", c.RTarget, err))
			} else if count < 1 {
				mErr.Errors = append(mErr.Errors, fmt.Errorf("Distinct Property must have an allowed count of 1 or greater: %d < 1", count))
			}
		}
	case ConstraintAttributeIsSet, ConstraintAttributeIsNotSet:
		if c.RTarget != "" {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Operator %q does not support an RTarget", c.Operand))
		}
	case "=", "==", "is", "!=", "not", "<", "<=", ">", ">=":
		if c.RTarget == "" {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Operator %q requires an RTarget", c.Operand))
		}
	default:
		mErr.Errors = append(mErr.Errors, fmt.Errorf("Unknown constraint type %q", c.Operand))
	}

	// Ensure we have an LTarget for the constraints that need one
	if requireLtarget && c.LTarget == "" {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("No LTarget provided but is required by constraint"))
	}

	return mErr.ErrorOrNil()
}

type Constraints []*Constraint

// Equal compares Constraints as a set
func (xs *Constraints) Equal(ys *Constraints) bool {
	if xs == ys {
		return true
	}
	if xs == nil || ys == nil {
		return false
	}
	if len(*xs) != len(*ys) {
		return false
	}
SETEQUALS:
	for _, x := range *xs {
		for _, y := range *ys {
			if x.Equal(y) {
				continue SETEQUALS
			}
		}
		return false
	}
	return true
}

// Affinity is used to score placement options based on a weight
type Affinity struct {
	LTarget string // Left-hand target
	RTarget string // Right-hand target
	Operand string // Affinity operand (<=, <, =, !=, >, >=), set_contains_all, set_contains_any
	Weight  int8   // Weight applied to nodes that match the affinity. Can be negative
}

// Equal checks if two affinities are equal.
func (a *Affinity) Equal(o *Affinity) bool {
	if a == nil || o == nil {
		return a == o
	}
	switch {
	case a.LTarget != o.LTarget:
		return false
	case a.RTarget != o.RTarget:
		return false
	case a.Operand != o.Operand:
		return false
	case a.Weight != o.Weight:
		return false
	}
	return true
}

func (a *Affinity) Copy() *Affinity {
	if a == nil {
		return nil
	}
	return &Affinity{
		LTarget: a.LTarget,
		RTarget: a.RTarget,
		Operand: a.Operand,
		Weight:  a.Weight,
	}
}

func (a *Affinity) String() string {
	return fmt.Sprintf("%s %s %s %v", a.LTarget, a.Operand, a.RTarget, a.Weight)
}

func (a *Affinity) Validate() error {
	var mErr multierror.Error
	if a.Operand == "" {
		mErr.Errors = append(mErr.Errors, errors.New("Missing affinity operand"))
	}

	// Perform additional validation based on operand
	switch a.Operand {
	case ConstraintSetContainsAll, ConstraintSetContainsAny, ConstraintSetContains:
		if a.RTarget == "" {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Set contains operators require an RTarget"))
		}
	case ConstraintRegex:
		if _, err := regexp.Compile(a.RTarget); err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Regular expression failed to compile: %v", err))
		}
	case ConstraintVersion:
		if _, err := version.NewConstraint(a.RTarget); err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Version affinity is invalid: %v", err))
		}
	case ConstraintSemver:
		if _, err := semver.NewConstraint(a.RTarget); err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Semver affinity is invalid: %v", err))
		}
	case "=", "==", "is", "!=", "not", "<", "<=", ">", ">=":
		if a.RTarget == "" {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Operator %q requires an RTarget", a.Operand))
		}
	default:
		mErr.Errors = append(mErr.Errors, fmt.Errorf("Unknown affinity operator %q", a.Operand))
	}

	// Ensure we have an LTarget
	if a.LTarget == "" {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("No LTarget provided but is required"))
	}

	// Ensure that weight is between -100 and 100, and not zero
	if a.Weight == 0 {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("Affinity weight cannot be zero"))
	}

	if a.Weight > 100 || a.Weight < -100 {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("Affinity weight must be within the range [-100,100]"))
	}

	return mErr.ErrorOrNil()
}

// Spread is used to specify desired distribution of allocations according to weight
type Spread struct {
	// Attribute is the node attribute used as the spread criteria
	Attribute string

	// Weight is the relative weight of this spread, useful when there are multiple
	// spread and affinities
	Weight int8

	// SpreadTarget is used to describe desired percentages for each attribute value
	SpreadTarget []*SpreadTarget

	// Memoized string representation
	str string
}

func (s *Spread) Equal(o *Spread) bool {
	if s == nil || o == nil {
		return s == o
	}
	switch {
	case s.Attribute != o.Attribute:
		return false
	case s.Weight != o.Weight:
		return false
	case !slices.EqualFunc(s.SpreadTarget, o.SpreadTarget, func(a, b *SpreadTarget) bool { return a.Equal(b) }):
		return false
	}
	return true
}

type Affinities []*Affinity

// Equal compares Affinities as a set
func (xs *Affinities) Equal(ys *Affinities) bool {
	if xs == ys {
		return true
	}
	if xs == nil || ys == nil {
		return false
	}
	if len(*xs) != len(*ys) {
		return false
	}
SETEQUALS:
	for _, x := range *xs {
		for _, y := range *ys {
			if x.Equal(y) {
				continue SETEQUALS
			}
		}
		return false
	}
	return true
}

func (s *Spread) Copy() *Spread {
	if s == nil {
		return nil
	}
	ns := new(Spread)
	*ns = *s

	ns.SpreadTarget = CopySliceSpreadTarget(s.SpreadTarget)
	return ns
}

func (s *Spread) String() string {
	if s.str != "" {
		return s.str
	}
	s.str = fmt.Sprintf("%s %s %v", s.Attribute, s.SpreadTarget, s.Weight)
	return s.str
}

func (s *Spread) Validate() error {
	var mErr multierror.Error
	if s.Attribute == "" {
		mErr.Errors = append(mErr.Errors, errors.New("Missing spread attribute"))
	}
	if s.Weight <= 0 || s.Weight > 100 {
		mErr.Errors = append(mErr.Errors, errors.New("Spread block must have a positive weight from 0 to 100"))
	}
	seen := make(map[string]struct{})
	sumPercent := uint32(0)

	for _, target := range s.SpreadTarget {
		// Make sure there are no duplicates
		_, ok := seen[target.Value]
		if !ok {
			seen[target.Value] = struct{}{}
		} else {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Spread target value %q already defined", target.Value))
		}
		if target.Percent > 100 {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Spread target percentage for value %q must be between 0 and 100", target.Value))
		}
		sumPercent += uint32(target.Percent)
	}
	if sumPercent > 100 {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("Sum of spread target percentages must not be greater than 100%%; got %d%%", sumPercent))
	}
	return mErr.ErrorOrNil()
}

// SpreadTarget is used to specify desired percentages for each attribute value
type SpreadTarget struct {
	// Value is a single attribute value, like "dc1"
	Value string

	// Percent is the desired percentage of allocs
	Percent uint8

	// Memoized string representation
	str string
}

func (s *SpreadTarget) Copy() *SpreadTarget {
	if s == nil {
		return nil
	}

	ns := new(SpreadTarget)
	*ns = *s
	return ns
}

func (s *SpreadTarget) String() string {
	if s.str != "" {
		return s.str
	}
	s.str = fmt.Sprintf("%q %v%%", s.Value, s.Percent)
	return s.str
}

func (s *SpreadTarget) Equal(o *SpreadTarget) bool {
	if s == nil || o == nil {
		return s == o
	}
	switch {
	case s.Value != o.Value:
		return false
	case s.Percent != o.Percent:
		return false
	}
	return true
}

// EphemeralDisk is an ephemeral disk object
type EphemeralDisk struct {
	// Sticky indicates whether the allocation is sticky to a node
	Sticky bool

	// SizeMB is the size of the local disk
	SizeMB int

	// Migrate determines if Nomad client should migrate the allocation dir for
	// sticky allocations
	Migrate bool
}

// DefaultEphemeralDisk returns a EphemeralDisk with default configurations
func DefaultEphemeralDisk() *EphemeralDisk {
	return &EphemeralDisk{
		SizeMB: 300,
	}
}

func (d *EphemeralDisk) Equal(o *EphemeralDisk) bool {
	if d == nil || o == nil {
		return d == o
	}
	switch {
	case d.Sticky != o.Sticky:
		return false
	case d.SizeMB != o.SizeMB:
		return false
	case d.Migrate != o.Migrate:
		return false
	}
	return true
}

// Validate validates EphemeralDisk
func (d *EphemeralDisk) Validate() error {
	if d.SizeMB < 10 {
		return fmt.Errorf("minimum DiskMB value is 10; got %d", d.SizeMB)
	}
	return nil
}

// Copy copies the EphemeralDisk struct and returns a new one
func (d *EphemeralDisk) Copy() *EphemeralDisk {
	ld := new(EphemeralDisk)
	*ld = *d
	return ld
}

var (
	// VaultUnrecoverableError matches unrecoverable errors returned by a Vault
	// server
	VaultUnrecoverableError = regexp.MustCompile(`Code:\s+40(0|3|4)`)
)

const (
	// VaultChangeModeNoop takes no action when a new token is retrieved.
	VaultChangeModeNoop = "noop"

	// VaultChangeModeSignal signals the task when a new token is retrieved.
	VaultChangeModeSignal = "signal"

	// VaultChangeModeRestart restarts the task when a new token is retrieved.
	VaultChangeModeRestart = "restart"
)

// Vault stores the set of permissions a task needs access to from Vault.
type Vault struct {
	// Role is the Vault role used to login to Vault using a JWT.
	//
	// If empty, defaults to the server's create_from_role value or the Vault
	// cluster default role.
	Role string

	// Policies is the set of policies that the task needs access to
	Policies []string

	// Namespace is the vault namespace that should be used.
	Namespace string

	// Cluster (by name) to send API requests to
	Cluster string

	// Env marks whether the Vault Token should be exposed as an environment
	// variable
	Env bool

	// DisableFile marks whether the Vault Token should be exposed in the file
	// vault_token in the task's secrets directory.
	DisableFile bool

	// ChangeMode is used to configure the task's behavior when the Vault
	// token changes because the original token could not be renewed in time.
	ChangeMode string

	// ChangeSignal is the signal sent to the task when a new token is
	// retrieved. This is only valid when using the signal change mode.
	ChangeSignal string

	// AllowTokenExpiration disables the Vault token refresh loop on the client
	AllowTokenExpiration bool
}

// IdentityName returns the name of the workload identity to be used to access
// this Vault cluster.
func (v *Vault) IdentityName() string {
	return fmt.Sprintf("%s%s", WorkloadIdentityVaultPrefix, v.Cluster)
}

func (v *Vault) Equal(o *Vault) bool {
	if v == nil || o == nil {
		return v == o
	}
	switch {
	case v.Role != o.Role:
		return false
	case !slices.Equal(v.Policies, o.Policies):
		return false
	case v.Namespace != o.Namespace:
		return false
	case v.Cluster != o.Cluster:
		return false
	case v.Env != o.Env:
		return false
	case v.DisableFile != o.DisableFile:
		return false
	case v.ChangeMode != o.ChangeMode:
		return false
	case v.ChangeSignal != o.ChangeSignal:
		return false
	case v.AllowTokenExpiration != o.AllowTokenExpiration:
		return false
	}
	return true
}

// Copy returns a copy of this Vault block.
func (v *Vault) Copy() *Vault {
	if v == nil {
		return nil
	}

	nv := new(Vault)
	*nv = *v
	return nv
}

func (v *Vault) Canonicalize() {
	// The Vault cluster name is canonicalized in the jobVaultHook during job
	// registration because the value may be read from the server config.

	if v.ChangeSignal != "" {
		v.ChangeSignal = strings.ToUpper(v.ChangeSignal)
	}

	if v.ChangeMode == "" {
		v.ChangeMode = VaultChangeModeRestart
	}
}

// Validate returns if the Vault block is valid.
func (v *Vault) Validate() error {
	if v == nil {
		return nil
	}

	var mErr multierror.Error
	for _, p := range v.Policies {
		if p == "root" {
			_ = multierror.Append(&mErr, fmt.Errorf("Can not specify \"root\" policy"))
		}
	}

	switch v.ChangeMode {
	case VaultChangeModeSignal:
		if v.ChangeSignal == "" {
			_ = multierror.Append(&mErr, fmt.Errorf("Signal must be specified when using change mode %q", VaultChangeModeSignal))
		}
	case VaultChangeModeNoop, VaultChangeModeRestart:
	default:
		_ = multierror.Append(&mErr, fmt.Errorf("Unknown change mode %q", v.ChangeMode))
	}

	return mErr.ErrorOrNil()
}

const (
	// DeploymentStatuses are the various states a deployment can be be in
	DeploymentStatusRunning      = "running"
	DeploymentStatusPaused       = "paused"
	DeploymentStatusFailed       = "failed"
	DeploymentStatusSuccessful   = "successful"
	DeploymentStatusCancelled    = "cancelled"
	DeploymentStatusInitializing = "initializing"
	DeploymentStatusPending      = "pending"
	DeploymentStatusBlocked      = "blocked"
	DeploymentStatusUnblocking   = "unblocking"

	// TODO Statuses and Descriptions do not match 1:1 and we sometimes use the Description as a status flag

	// DeploymentStatusDescriptions are the various descriptions of the states a
	// deployment can be in.
	DeploymentStatusDescriptionRunning               = "Deployment is running"
	DeploymentStatusDescriptionRunningNeedsPromotion = "Deployment is running but requires manual promotion"
	DeploymentStatusDescriptionRunningAutoPromotion  = "Deployment is running pending automatic promotion"
	DeploymentStatusDescriptionPaused                = "Deployment is paused"
	DeploymentStatusDescriptionSuccessful            = "Deployment completed successfully"
	DeploymentStatusDescriptionStoppedJob            = "Cancelled because job is stopped"
	DeploymentStatusDescriptionNewerJob              = "Cancelled due to newer version of job"
	DeploymentStatusDescriptionFailedAllocations     = "Failed due to unhealthy allocations"
	DeploymentStatusDescriptionProgressDeadline      = "Failed due to progress deadline"
	DeploymentStatusDescriptionFailedByUser          = "Deployment marked as failed"

	// used only in multiregion deployments
	DeploymentStatusDescriptionFailedByPeer   = "Failed because of an error in peer region"
	DeploymentStatusDescriptionBlocked        = "Deployment is complete but waiting for peer region"
	DeploymentStatusDescriptionUnblocking     = "Deployment is unblocking remaining regions"
	DeploymentStatusDescriptionPendingForPeer = "Deployment is pending, waiting for peer region"
)

// DeploymentStatusDescriptionRollback is used to get the status description of
// a deployment when rolling back to an older job.
func DeploymentStatusDescriptionRollback(baseDescription string, jobVersion uint64) string {
	return fmt.Sprintf("%s - rolling back to job version %d", baseDescription, jobVersion)
}

// DeploymentStatusDescriptionRollbackNoop is used to get the status description of
// a deployment when rolling back is not possible because it has the same specification
func DeploymentStatusDescriptionRollbackNoop(baseDescription string, jobVersion uint64) string {
	return fmt.Sprintf("%s - not rolling back to stable job version %d as current job has same specification", baseDescription, jobVersion)
}

// DeploymentStatusDescriptionNoRollbackTarget is used to get the status description of
// a deployment when there is no target to rollback to but autorevert is desired.
func DeploymentStatusDescriptionNoRollbackTarget(baseDescription string) string {
	return fmt.Sprintf("%s - no stable job version to auto revert to", baseDescription)
}

// Deployment is the object that represents a job deployment which is used to
// transition a job between versions.
type Deployment struct {
	// ID is a generated UUID for the deployment
	ID string

	// Namespace is the namespace the deployment is created in
	Namespace string

	// JobID is the job the deployment is created for
	JobID string

	// JobVersion is the version of the job at which the deployment is tracking
	JobVersion uint64

	// JobModifyIndex is the ModifyIndex of the job which the deployment is
	// tracking.
	JobModifyIndex uint64

	// JobSpecModifyIndex is the JobModifyIndex of the job which the
	// deployment is tracking.
	JobSpecModifyIndex uint64

	// JobCreateIndex is the create index of the job which the deployment is
	// tracking. It is needed so that if the job gets stopped and reran we can
	// present the correct list of deployments for the job and not old ones.
	JobCreateIndex uint64

	// Multiregion specifies if deployment is part of multiregion deployment
	IsMultiregion bool

	// TaskGroups is the set of task groups effected by the deployment and their
	// current deployment status.
	TaskGroups map[string]*DeploymentState

	// The status of the deployment
	Status string

	// StatusDescription allows a human readable description of the deployment
	// status.
	StatusDescription string

	// EvalPriority tracks the priority of the evaluation which lead to the
	// creation of this Deployment object. Any additional evaluations created
	// as a result of this deployment can therefore inherit this value, which
	// is not guaranteed to be that of the job priority parameter.
	EvalPriority int

	CreateIndex uint64
	ModifyIndex uint64
}

// NewDeployment creates a new deployment given the job.
func NewDeployment(job *Job, evalPriority int) *Deployment {
	return &Deployment{
		ID:                 uuid.Generate(),
		Namespace:          job.Namespace,
		JobID:              job.ID,
		JobVersion:         job.Version,
		JobModifyIndex:     job.ModifyIndex,
		JobSpecModifyIndex: job.JobModifyIndex,
		JobCreateIndex:     job.CreateIndex,
		IsMultiregion:      job.IsMultiregion(),
		Status:             DeploymentStatusRunning,
		StatusDescription:  DeploymentStatusDescriptionRunning,
		TaskGroups:         make(map[string]*DeploymentState, len(job.TaskGroups)),
		EvalPriority:       evalPriority,
	}
}

func (d *Deployment) Copy() *Deployment {
	if d == nil {
		return nil
	}

	c := &Deployment{}
	*c = *d

	c.TaskGroups = nil
	if l := len(d.TaskGroups); d.TaskGroups != nil {
		c.TaskGroups = make(map[string]*DeploymentState, l)
		for tg, s := range d.TaskGroups {
			c.TaskGroups[tg] = s.Copy()
		}
	}

	return c
}

// Active returns whether the deployment is active or terminal.
func (d *Deployment) Active() bool {
	switch d.Status {
	case DeploymentStatusRunning, DeploymentStatusPaused, DeploymentStatusBlocked,
		DeploymentStatusUnblocking, DeploymentStatusInitializing, DeploymentStatusPending:
		return true
	default:
		return false
	}
}

// GetID is a helper for getting the ID when the object may be nil
func (d *Deployment) GetID() string {
	if d == nil {
		return ""
	}
	return d.ID
}

// GetCreateIndex implements the CreateIndexGetter interface, required for
// pagination.
func (d *Deployment) GetCreateIndex() uint64 {
	if d == nil {
		return 0
	}
	return d.CreateIndex
}

// HasPlacedCanaries returns whether the deployment has placed canaries
func (d *Deployment) HasPlacedCanaries() bool {
	if d == nil || len(d.TaskGroups) == 0 {
		return false
	}
	for _, group := range d.TaskGroups {
		if len(group.PlacedCanaries) != 0 {
			return true
		}
	}
	return false
}

// RequiresPromotion returns whether the deployment requires promotion to
// continue
func (d *Deployment) RequiresPromotion() bool {
	if d == nil || len(d.TaskGroups) == 0 || d.Status != DeploymentStatusRunning {
		return false
	}
	for _, group := range d.TaskGroups {
		if group.DesiredCanaries > 0 && !group.Promoted {
			return true
		}
	}
	return false
}

// HasAutoPromote determines if all taskgroups are marked auto_promote
func (d *Deployment) HasAutoPromote() bool {
	if d == nil || len(d.TaskGroups) == 0 || d.Status != DeploymentStatusRunning {
		return false
	}
	for _, group := range d.TaskGroups {
		if group.DesiredCanaries > 0 && !group.AutoPromote {
			return false
		}
	}
	return true
}

func (d *Deployment) GoString() string {
	base := fmt.Sprintf("Deployment ID %q for job %q has status %q (%v):", d.ID, d.JobID, d.Status, d.StatusDescription)
	for group, state := range d.TaskGroups {
		base += fmt.Sprintf("\nTask Group %q has state:\n%#v", group, state)
	}
	return base
}

// GetNamespace implements the NamespaceGetter interface, required for pagination.
func (d *Deployment) GetNamespace() string {
	if d == nil {
		return ""
	}
	return d.Namespace
}

// DeploymentState tracks the state of a deployment for a given task group.
type DeploymentState struct {
	// AutoRevert marks whether the task group has indicated the job should be
	// reverted on failure
	AutoRevert bool

	// AutoPromote marks promotion triggered automatically by healthy canaries
	// copied from TaskGroup UpdateStrategy in scheduler.reconcile
	AutoPromote bool

	// ProgressDeadline is the deadline by which an allocation must transition
	// to healthy before the deployment is considered failed. This value is set
	// by the jobspec `update.progress_deadline` field.
	ProgressDeadline time.Duration

	// RequireProgressBy is the time by which an allocation must transition to
	// healthy before the deployment is considered failed. This value is reset
	// to "now" + ProgressDeadline when an allocation updates the deployment.
	RequireProgressBy time.Time

	// Promoted marks whether the canaries have been promoted
	Promoted bool

	// PlacedCanaries is the set of placed canary allocations
	PlacedCanaries []string

	// DesiredCanaries is the number of canaries that should be created.
	DesiredCanaries int

	// DesiredTotal is the total number of allocations that should be created as
	// part of the deployment.
	DesiredTotal int

	// PlacedAllocs is the number of allocations that have been placed
	PlacedAllocs int

	// HealthyAllocs is the number of allocations that have been marked healthy.
	HealthyAllocs int

	// UnhealthyAllocs are allocations that have been marked as unhealthy.
	UnhealthyAllocs int
}

func (d *DeploymentState) GoString() string {
	base := fmt.Sprintf("\tDesired Total: %d", d.DesiredTotal)
	base += fmt.Sprintf("\n\tDesired Canaries: %d", d.DesiredCanaries)
	base += fmt.Sprintf("\n\tPlaced Canaries: %#v", d.PlacedCanaries)
	base += fmt.Sprintf("\n\tPromoted: %v", d.Promoted)
	base += fmt.Sprintf("\n\tPlaced: %d", d.PlacedAllocs)
	base += fmt.Sprintf("\n\tHealthy: %d", d.HealthyAllocs)
	base += fmt.Sprintf("\n\tUnhealthy: %d", d.UnhealthyAllocs)
	base += fmt.Sprintf("\n\tAutoRevert: %v", d.AutoRevert)
	base += fmt.Sprintf("\n\tAutoPromote: %v", d.AutoPromote)
	return base
}

func (d *DeploymentState) Copy() *DeploymentState {
	c := &DeploymentState{}
	*c = *d
	c.PlacedCanaries = slices.Clone(d.PlacedCanaries)
	return c
}

// DeploymentStatusUpdate is used to update the status of a given deployment
type DeploymentStatusUpdate struct {
	// DeploymentID is the ID of the deployment to update
	DeploymentID string

	// Status is the new status of the deployment.
	Status string

	// StatusDescription is the new status description of the deployment.
	StatusDescription string
}

// RescheduleTracker encapsulates previous reschedule events
type RescheduleTracker struct {
	Events []*RescheduleEvent

	// LastReschedule represents whether the most recent attempt to reschedule
	// the allocation (if any) was successful
	LastReschedule RescheduleTrackerAnnotation
}

type RescheduleTrackerAnnotation string

const (
	LastRescheduleSuccess       RescheduleTrackerAnnotation = "ok"
	LastRescheduleFailedToPlace RescheduleTrackerAnnotation = "no placement"
)

func (rt *RescheduleTracker) Copy() *RescheduleTracker {
	if rt == nil {
		return nil
	}
	nt := &RescheduleTracker{}
	*nt = *rt
	rescheduleEvents := make([]*RescheduleEvent, 0, len(rt.Events))
	for _, tracker := range rt.Events {
		rescheduleEvents = append(rescheduleEvents, tracker.Copy())
	}
	nt.Events = rescheduleEvents
	return nt
}

func (rt *RescheduleTracker) RescheduleEligible(reschedulePolicy *ReschedulePolicy, failTime time.Time) bool {
	if reschedulePolicy == nil {
		return false
	}
	attempts := reschedulePolicy.Attempts
	enabled := attempts > 0 || reschedulePolicy.Unlimited
	if !enabled {
		return false
	}
	if reschedulePolicy.Unlimited {
		return true
	}
	// Early return true if there are no attempts yet and the number of allowed attempts is > 0
	if (rt == nil || len(rt.Events) == 0) && attempts > 0 {
		return true
	}
	attempted, _ := rt.rescheduleInfo(reschedulePolicy, failTime)
	return attempted < attempts
}

func (rt *RescheduleTracker) rescheduleInfo(reschedulePolicy *ReschedulePolicy, failTime time.Time) (int, int) {
	if reschedulePolicy == nil {
		return 0, 0
	}
	attempts := reschedulePolicy.Attempts
	interval := reschedulePolicy.Interval

	attempted := 0
	if rt != nil && attempts > 0 {
		for j := len(rt.Events) - 1; j >= 0; j-- {
			lastAttempt := rt.Events[j].RescheduleTime
			timeDiff := failTime.UTC().UnixNano() - lastAttempt
			if timeDiff < interval.Nanoseconds() {
				attempted += 1
			}
		}
	}
	return attempted, attempts
}

// RescheduleEvent is used to keep track of previous attempts at rescheduling an allocation
type RescheduleEvent struct {
	// RescheduleTime is the timestamp of a reschedule attempt
	RescheduleTime int64

	// PrevAllocID is the ID of the previous allocation being restarted
	PrevAllocID string

	// PrevNodeID is the node ID of the previous allocation
	PrevNodeID string

	// Delay is the reschedule delay associated with the attempt
	Delay time.Duration
}

func NewRescheduleEvent(rescheduleTime int64, prevAllocID string, prevNodeID string, delay time.Duration) *RescheduleEvent {
	return &RescheduleEvent{RescheduleTime: rescheduleTime,
		PrevAllocID: prevAllocID,
		PrevNodeID:  prevNodeID,
		Delay:       delay}
}

func (re *RescheduleEvent) Copy() *RescheduleEvent {
	if re == nil {
		return nil
	}
	copy := new(RescheduleEvent)
	*copy = *re
	return copy
}

// DesiredTransition is used to mark an allocation as having a desired state
// transition. This information can be used by the scheduler to make the
// correct decision.
type DesiredTransition struct {
	// Migrate is used to indicate that this allocation should be stopped and
	// migrated to another node.
	Migrate *bool

	// Reschedule is used to indicate that this allocation is eligible to be
	// rescheduled. Most allocations are automatically eligible for
	// rescheduling, so this field is only required when an allocation is not
	// automatically eligible. An example is an allocation that is part of a
	// deployment.
	Reschedule *bool

	// ForceReschedule is used to indicate that this allocation must be rescheduled.
	// This field is only used when operators want to force a placement even if
	// a failed allocation is not eligible to be rescheduled
	ForceReschedule *bool

	// NoShutdownDelay, if set to true, will override the group and
	// task shutdown_delay configuration and ignore the delay for any
	// allocations stopped as a result of this Deregister call.
	NoShutdownDelay *bool
}

// Merge merges the two desired transitions, preferring the values from the
// passed in object.
func (d *DesiredTransition) Merge(o *DesiredTransition) {
	if o.Migrate != nil {
		d.Migrate = o.Migrate
	}

	if o.Reschedule != nil {
		d.Reschedule = o.Reschedule
	}

	if o.ForceReschedule != nil {
		d.ForceReschedule = o.ForceReschedule
	}

	if o.NoShutdownDelay != nil {
		d.NoShutdownDelay = o.NoShutdownDelay
	}
}

// ShouldMigrate returns whether the transition object dictates a migration.
func (d *DesiredTransition) ShouldMigrate() bool {
	return d.Migrate != nil && *d.Migrate
}

// ShouldReschedule returns whether the transition object dictates a
// rescheduling.
func (d *DesiredTransition) ShouldReschedule() bool {
	return d.Reschedule != nil && *d.Reschedule
}

// ShouldForceReschedule returns whether the transition object dictates a
// forced rescheduling.
func (d *DesiredTransition) ShouldForceReschedule() bool {
	if d == nil {
		return false
	}
	return d.ForceReschedule != nil && *d.ForceReschedule
}

// ShouldIgnoreShutdownDelay returns whether the transition object dictates
// that shutdown skip any shutdown delays.
func (d *DesiredTransition) ShouldIgnoreShutdownDelay() bool {
	if d == nil {
		return false
	}
	return d.NoShutdownDelay != nil && *d.NoShutdownDelay
}

const (
	AllocDesiredStatusRun   = "run"   // Allocation should run
	AllocDesiredStatusStop  = "stop"  // Allocation should stop
	AllocDesiredStatusEvict = "evict" // Allocation should stop, and was evicted
)

const (
	AllocClientStatusPending  = "pending"
	AllocClientStatusRunning  = "running"
	AllocClientStatusComplete = "complete"
	AllocClientStatusFailed   = "failed"
	AllocClientStatusLost     = "lost"
	AllocClientStatusUnknown  = "unknown"
)

// terminalAllocationStatuses lists allocation statutes that we consider
// terminal
var terminalAllocationStatuses = []string{
	AllocClientStatusComplete,
	AllocClientStatusFailed,
	AllocClientStatusLost,
}

// Allocation is used to allocate the placement of a task group to a node.
type Allocation struct {
	// msgpack omit empty fields during serialization
	_struct bool `codec:",omitempty"` // nolint: structcheck

	// ID of the allocation (UUID)
	ID string

	// Namespace is the namespace the allocation is created in
	Namespace string

	// ID of the evaluation that generated this allocation
	EvalID string

	// Name is a logical name of the allocation.
	Name string

	// NodeID is the node this is being placed on
	NodeID string

	// NodeName is the name of the node this is being placed on.
	NodeName string

	// Job is the parent job of the task group being allocated.
	// This is copied at allocation time to avoid issues if the job
	// definition is updated.
	JobID string
	Job   *Job

	// TaskGroup is the name of the task group that should be run
	TaskGroup string

	// COMPAT(0.11): Remove in 0.11
	// Resources is the total set of resources allocated as part
	// of this allocation of the task group. Dynamic ports will be set by
	// the scheduler.
	Resources *Resources

	// SharedResources are the resources that are shared by all the tasks in an
	// allocation
	// Deprecated: use AllocatedResources.Shared instead.
	// Keep field to allow us to handle upgrade paths from old versions
	SharedResources *Resources

	// TaskResources is the set of resources allocated to each
	// task. These should sum to the total Resources. Dynamic ports will be
	// set by the scheduler.
	// Deprecated: use AllocatedResources.Tasks instead.
	// Keep field to allow us to handle upgrade paths from old versions
	TaskResources map[string]*Resources

	// AllocatedResources is the total resources allocated for the task group.
	AllocatedResources *AllocatedResources

	// Metrics associated with this allocation
	Metrics *AllocMetric

	// Desired Status of the allocation on the client
	DesiredStatus string

	// DesiredStatusDescription is meant to provide more human useful information
	DesiredDescription string

	// DesiredTransition is used to indicate that a state transition
	// is desired for a given reason.
	DesiredTransition DesiredTransition

	// Status of the allocation on the client
	ClientStatus string

	// ClientStatusDescription is meant to provide more human useful information
	ClientDescription string

	// TaskStates stores the state of each task,
	TaskStates map[string]*TaskState

	// AllocStates track meta data associated with changes to the state of the whole allocation, like becoming lost
	AllocStates []*AllocState

	// PreviousAllocation is the allocation that this allocation is replacing
	PreviousAllocation string

	// NextAllocation is the allocation that this allocation is being replaced by
	NextAllocation string

	// DeploymentID identifies an allocation as being created from a
	// particular deployment
	DeploymentID string

	// DeploymentStatus captures the status of the allocation as part of the
	// given deployment
	DeploymentStatus *AllocDeploymentStatus

	// RescheduleTrackers captures details of previous reschedule attempts of the allocation
	RescheduleTracker *RescheduleTracker

	// NetworkStatus captures networking details of an allocation known at runtime
	NetworkStatus *AllocNetworkStatus

	// FollowupEvalID captures a follow up evaluation created to handle a failed allocation
	// that can be rescheduled in the future
	FollowupEvalID string

	// PreemptedAllocations captures IDs of any allocations that were preempted
	// in order to place this allocation
	PreemptedAllocations []string

	// PreemptedByAllocation tracks the alloc ID of the allocation that caused this allocation
	// to stop running because it got preempted
	PreemptedByAllocation string

	// SignedIdentities is a map of task names to signed identity/capability
	// claim tokens for those tasks. If needed, it is populated in the plan
	// applier.
	SignedIdentities map[string]string `json:"-"`

	// SigningKeyID is the key used to sign the SignedIdentities field.
	SigningKeyID string

	// Raft Indexes
	CreateIndex uint64
	ModifyIndex uint64

	// AllocModifyIndex is not updated when the client updates allocations. This
	// lets the client pull only the allocs updated by the server.
	AllocModifyIndex uint64

	// CreateTime is the time the allocation has finished scheduling and been
	// verified by the plan applier.
	CreateTime int64

	// ModifyTime is the time the allocation was last updated.
	ModifyTime int64
}

// GetID implements the IDGetter interface, required for pagination.
func (a *Allocation) GetID() string {
	if a == nil {
		return ""
	}
	return a.ID
}

// GetNamespace implements the NamespaceGetter interface, required for
// pagination and filtering namespaces in endpoints that support glob namespace
// requests using tokens with limited access.
func (a *Allocation) GetNamespace() string {
	if a == nil {
		return ""
	}
	return a.Namespace
}

// GetCreateIndex implements the CreateIndexGetter interface, required for
// pagination.
func (a *Allocation) GetCreateIndex() uint64 {
	if a == nil {
		return 0
	}
	return a.CreateIndex
}

// ReservedCores returns the union of reserved cores across tasks in this alloc.
func (a *Allocation) ReservedCores() *idset.Set[hw.CoreID] {
	s := idset.Empty[hw.CoreID]()
	if a == nil || a.AllocatedResources == nil {
		return s
	}
	for _, taskResources := range a.AllocatedResources.Tasks {
		if len(taskResources.Cpu.ReservedCores) > 0 {
			for _, core := range taskResources.Cpu.ReservedCores {
				s.Insert(hw.CoreID(core))
			}
		}
	}
	return s
}

// ConsulNamespace returns the Consul namespace of the task group associated
// with this allocation.
func (a *Allocation) ConsulNamespace() string {
	return a.Job.LookupTaskGroup(a.TaskGroup).Consul.GetNamespace()
}

func (a *Allocation) ConsulNamespaceForTask(taskName string) string {
	tg := a.Job.LookupTaskGroup(a.TaskGroup)
	task := tg.LookupTask(taskName)
	if task.Consul != nil {
		return task.Consul.GetNamespace()
	}

	return tg.Consul.GetNamespace()
}

func (a *Allocation) JobNamespacedID() NamespacedID {
	return NewNamespacedID(a.JobID, a.Namespace)
}

// Index returns the index of the allocation. If the allocation is from a task
// group with count greater than 1, there will be multiple allocations for it.
func (a *Allocation) Index() uint {
	return AllocIndexFromName(a.Name, a.JobID, a.TaskGroup)
}

// AllocIndexFromName returns the index of an allocation given its name, the
// jobID and the task group name.
func AllocIndexFromName(allocName, jobID, taskGroup string) uint {
	l := len(allocName)
	prefix := len(jobID) + len(taskGroup) + 2
	if l <= 3 || l <= prefix {
		return uint(0)
	}

	strNum := allocName[prefix : len(allocName)-1]
	num, _ := strconv.Atoi(strNum)
	return uint(num)
}

// Copy provides a copy of the allocation and deep copies the job
func (a *Allocation) Copy() *Allocation {
	return a.copyImpl(true)
}

// CopySkipJob provides a copy of the allocation but doesn't deep copy the job
func (a *Allocation) CopySkipJob() *Allocation {
	return a.copyImpl(false)
}

// Canonicalize Allocation to ensure fields are initialized to the expectations
// of this version of Nomad. Should be called when restoring persisted
// Allocations or receiving Allocations from Nomad agents potentially on an
// older version of Nomad.
func (a *Allocation) Canonicalize() {
	if a.AllocatedResources == nil && a.TaskResources != nil {
		ar := AllocatedResources{}

		tasks := make(map[string]*AllocatedTaskResources, len(a.TaskResources))
		for name, tr := range a.TaskResources {
			atr := AllocatedTaskResources{}
			atr.Cpu.CpuShares = int64(tr.CPU)
			atr.Memory.MemoryMB = int64(tr.MemoryMB)
			atr.Networks = tr.Networks.Copy()

			tasks[name] = &atr
		}
		ar.Tasks = tasks

		if a.SharedResources != nil {
			ar.Shared.DiskMB = int64(a.SharedResources.DiskMB)
			ar.Shared.Networks = a.SharedResources.Networks.Copy()
		}

		a.AllocatedResources = &ar
	}

	a.Job.Canonicalize()
}

func (a *Allocation) copyImpl(job bool) *Allocation {
	if a == nil {
		return nil
	}
	na := new(Allocation)
	*na = *a

	if job {
		na.Job = na.Job.Copy()
	}

	na.AllocatedResources = na.AllocatedResources.Copy()
	na.Resources = na.Resources.Copy()
	na.SharedResources = na.SharedResources.Copy()

	if a.TaskResources != nil {
		tr := make(map[string]*Resources, len(na.TaskResources))
		for task, resource := range na.TaskResources {
			tr[task] = resource.Copy()
		}
		na.TaskResources = tr
	}

	na.Metrics = na.Metrics.Copy()
	na.DeploymentStatus = na.DeploymentStatus.Copy()

	if a.TaskStates != nil {
		ts := make(map[string]*TaskState, len(na.TaskStates))
		for task, state := range na.TaskStates {
			ts[task] = state.Copy()
		}
		na.TaskStates = ts
	}

	na.RescheduleTracker = a.RescheduleTracker.Copy()
	na.PreemptedAllocations = slices.Clone(a.PreemptedAllocations)
	return na
}

// TerminalStatus returns if the desired or actual status is terminal and
// will no longer transition.
func (a *Allocation) TerminalStatus() bool {
	// First check the desired state and if that isn't terminal, check client
	// state.
	return a.ServerTerminalStatus() || a.ClientTerminalStatus()
}

// ServerTerminalStatus returns true if the desired state of the allocation is terminal
func (a *Allocation) ServerTerminalStatus() bool {
	switch a.DesiredStatus {
	case AllocDesiredStatusStop, AllocDesiredStatusEvict:
		return true
	default:
		return false
	}
}

// ClientTerminalStatus returns if the client status is terminal and will no longer transition
func (a *Allocation) ClientTerminalStatus() bool {
	return slices.Contains(terminalAllocationStatuses, a.ClientStatus)
}

// ShouldReschedule returns if the allocation is eligible to be rescheduled according
// to its status and ReschedulePolicy given its failure time
func (a *Allocation) ShouldReschedule(reschedulePolicy *ReschedulePolicy, failTime time.Time) bool {
	// First check the desired state
	switch a.DesiredStatus {
	case AllocDesiredStatusStop, AllocDesiredStatusEvict:
		return false
	default:
	}
	switch a.ClientStatus {
	case AllocClientStatusFailed:
		return a.RescheduleEligible(reschedulePolicy, failTime)
	default:
		return false
	}
}

// RescheduleEligible returns if the allocation is eligible to be rescheduled according
// to its ReschedulePolicy and the current state of its reschedule trackers
func (a *Allocation) RescheduleEligible(reschedulePolicy *ReschedulePolicy, failTime time.Time) bool {
	return a.RescheduleTracker.RescheduleEligible(reschedulePolicy, failTime)
}

func (a *Allocation) RescheduleInfo() (int, int) {
	return a.RescheduleTracker.rescheduleInfo(a.ReschedulePolicy(), a.LastEventTime())
}

// LastEventTime is the time of the last task event in the allocation.
// It is used to determine allocation failure time. If the FinishedAt field
// is not set, the alloc's modify time is used
func (a *Allocation) LastEventTime() time.Time {
	var lastEventTime time.Time
	if a.TaskStates != nil {
		for _, s := range a.TaskStates {
			if lastEventTime.IsZero() || s.FinishedAt.After(lastEventTime) {
				lastEventTime = s.FinishedAt
			}
		}
	}

	if lastEventTime.IsZero() {
		return time.Unix(0, a.ModifyTime).UTC()
	}
	return lastEventTime
}

// ReschedulePolicy returns the reschedule policy based on the task group
func (a *Allocation) ReschedulePolicy() *ReschedulePolicy {
	tg := a.Job.LookupTaskGroup(a.TaskGroup)
	if tg == nil {
		return nil
	}
	return tg.ReschedulePolicy
}

// MigrateStrategy returns the migrate strategy based on the task group
func (a *Allocation) MigrateStrategy() *MigrateStrategy {
	tg := a.Job.LookupTaskGroup(a.TaskGroup)
	if tg == nil {
		return nil
	}
	return tg.Migrate
}

// NextRescheduleTime returns a time on or after which the allocation is eligible to be rescheduled,
// and whether the next reschedule time is within policy's interval if the policy doesn't allow unlimited reschedules
func (a *Allocation) NextRescheduleTime() (time.Time, bool) {
	failTime := a.LastEventTime()
	reschedulePolicy := a.ReschedulePolicy()

	//If reschedule is disabled, return early
	if reschedulePolicy.Attempts == 0 && !reschedulePolicy.Unlimited {
		return time.Time{}, false
	}

	if (a.DesiredStatus == AllocDesiredStatusStop && !a.LastRescheduleFailed()) ||
		(a.ClientStatus != AllocClientStatusFailed && a.ClientStatus != AllocClientStatusLost) ||
		failTime.IsZero() || reschedulePolicy == nil {
		return time.Time{}, false
	}

	return a.nextRescheduleTime(failTime, reschedulePolicy)
}

func (a *Allocation) nextRescheduleTime(failTime time.Time, reschedulePolicy *ReschedulePolicy) (time.Time, bool) {
	nextDelay := a.NextDelay()
	nextRescheduleTime := failTime.Add(nextDelay)
	rescheduleEligible := reschedulePolicy.Unlimited || (reschedulePolicy.Attempts > 0 && a.RescheduleTracker == nil)
	if reschedulePolicy.Attempts > 0 && a.RescheduleTracker != nil && a.RescheduleTracker.Events != nil {
		// Check for eligibility based on the interval if max attempts is set
		attempted, attempts := a.RescheduleTracker.rescheduleInfo(reschedulePolicy, failTime)
		rescheduleEligible = attempted < attempts && nextDelay < reschedulePolicy.Interval
	}
	return nextRescheduleTime, rescheduleEligible
}

// NextRescheduleTimeByTime works like NextRescheduleTime but allows callers
// specify a failure time. Useful for things like determining whether to reschedule
// an alloc on a disconnected node.
func (a *Allocation) NextRescheduleTimeByTime(t time.Time) (time.Time, bool) {
	reschedulePolicy := a.ReschedulePolicy()
	if reschedulePolicy == nil {
		return time.Time{}, false
	}

	return a.nextRescheduleTime(t, reschedulePolicy)
}

func (a *Allocation) RescheduleTimeOnDisconnect(now time.Time) (time.Time, bool) {
	tg := a.Job.LookupTaskGroup(a.TaskGroup)
	if tg == nil || tg.Disconnect == nil || tg.Disconnect.Replace == nil {
		// Kept to maintain backwards compatibility with behavior prior to 1.8.0
		return a.NextRescheduleTimeByTime(now)
	}

	return now, *tg.Disconnect.Replace
}

// ShouldClientStop tests an alloc for StopAfterClient on the Disconnect configuration
func (a *Allocation) ShouldClientStop() bool {
	tg := a.Job.LookupTaskGroup(a.TaskGroup)
	timeout := tg.GetDisconnectStopTimeout()

	if tg == nil ||
		timeout == nil ||
		*timeout == 0*time.Nanosecond {
		return false
	}
	return true
}

// WaitClientStop uses the reschedule delay mechanism to block rescheduling until
// StopAfterClientDisconnect's block interval passes
func (a *Allocation) WaitClientStop() time.Time {
	tg := a.Job.LookupTaskGroup(a.TaskGroup)

	// An alloc can only be marked lost once, so use the first lost transition
	var t time.Time
	for _, s := range a.AllocStates {
		if s.Field == AllocStateFieldClientStatus &&
			s.Value == AllocClientStatusLost {
			t = s.Time
			break
		}
	}

	// On the first pass, the alloc hasn't been marked lost yet, and so we start
	// counting from now
	if t.IsZero() {
		t = time.Now().UTC()
	}

	// Find the max kill timeout
	kill := DefaultKillTimeout
	for _, t := range tg.Tasks {
		if t.KillTimeout > kill {
			kill = t.KillTimeout
		}
	}

	return t.Add(*tg.GetDisconnectStopTimeout() + kill)
}

// DisconnectTimeout uses the MaxClientDisconnect to compute when the allocation
// should transition to lost.
func (a *Allocation) DisconnectTimeout(now time.Time) time.Time {
	if a == nil || a.Job == nil {
		return now
	}

	tg := a.Job.LookupTaskGroup(a.TaskGroup)

	timeout := tg.GetDisconnectLostTimeout()
	if timeout == 0 {
		return now
	}

	return now.Add(timeout)
}

// SupportsDisconnectedClients determines whether both the server and the task group
// are configured to allow the allocation to reconnect after network connectivity
// has been lost and then restored.
func (a *Allocation) SupportsDisconnectedClients(serverSupportsDisconnectedClients bool) bool {
	if !serverSupportsDisconnectedClients {
		return false
	}

	if a.Job != nil {
		tg := a.Job.LookupTaskGroup(a.TaskGroup)
		if tg != nil {
			return tg.GetDisconnectLostTimeout() != 0
		}
	}

	return false
}

// PreventRescheduleOnLost determines if an alloc allows to have a replacement
// when Disconnected.
func (a *Allocation) PreventRescheduleOnDisconnect() bool {
	if a.Job != nil {
		tg := a.Job.LookupTaskGroup(a.TaskGroup)
		if tg != nil {
			return (tg.Disconnect != nil && tg.Disconnect.Replace != nil &&
				!*tg.Disconnect.Replace) ||
				tg.PreventRescheduleOnLost
		}
	}

	return false
}

// NextDelay returns a duration after which the allocation can be rescheduled.
// It is calculated according to the delay function and previous reschedule attempts.
func (a *Allocation) NextDelay() time.Duration {
	policy := a.ReschedulePolicy()
	// Can be nil if the task group was updated to remove its reschedule policy
	if policy == nil {
		return 0
	}
	delayDur := policy.Delay
	if a.RescheduleTracker == nil || a.RescheduleTracker.Events == nil || len(a.RescheduleTracker.Events) == 0 {
		return delayDur
	}
	events := a.RescheduleTracker.Events
	switch policy.DelayFunction {
	case "exponential":
		delayDur = a.RescheduleTracker.Events[len(a.RescheduleTracker.Events)-1].Delay * 2
	case "fibonacci":
		if len(events) >= 2 {
			fibN1Delay := events[len(events)-1].Delay
			fibN2Delay := events[len(events)-2].Delay
			// Handle reset of delay ceiling which should cause
			// a new series to start
			if fibN2Delay == policy.MaxDelay && fibN1Delay == policy.Delay {
				delayDur = fibN1Delay
			} else {
				delayDur = fibN1Delay + fibN2Delay
			}
		}
	default:
		return delayDur
	}
	if policy.MaxDelay > 0 && delayDur > policy.MaxDelay {
		delayDur = policy.MaxDelay
		// check if delay needs to be reset

		lastRescheduleEvent := a.RescheduleTracker.Events[len(a.RescheduleTracker.Events)-1]
		timeDiff := a.LastEventTime().UTC().UnixNano() - lastRescheduleEvent.RescheduleTime
		if timeDiff > delayDur.Nanoseconds() {
			delayDur = policy.Delay
		}

	}

	return delayDur
}

// Terminated returns if the allocation is in a terminal state on a client.
func (a *Allocation) Terminated() bool {
	if a.ClientStatus == AllocClientStatusFailed ||
		a.ClientStatus == AllocClientStatusComplete ||
		a.ClientStatus == AllocClientStatusLost {
		return true
	}
	return false
}

// SetStop updates the allocation in place to a DesiredStatus stop, with the ClientStatus
func (a *Allocation) SetStop(clientStatus, clientDesc string) {
	a.DesiredStatus = AllocDesiredStatusStop
	a.ClientStatus = clientStatus
	a.ClientDescription = clientDesc
	a.AppendState(AllocStateFieldClientStatus, clientStatus)
}

// AppendState creates and appends an AllocState entry recording the time of the state
// transition. Used to mark the transition to lost
func (a *Allocation) AppendState(field AllocStateField, value string) {
	a.AllocStates = append(a.AllocStates, &AllocState{
		Field: field,
		Value: value,
		Time:  time.Now().UTC(),
	})
}

// RanSuccessfully returns whether the client has ran the allocation and all
// tasks finished successfully. Critically this function returns whether the
// allocation has ran to completion and not just that the alloc has converged to
// its desired state. That is to say that a batch allocation must have finished
// with exit code 0 on all task groups. This doesn't really have meaning on a
// non-batch allocation because a service and system allocation should not
// finish.
func (a *Allocation) RanSuccessfully() bool {
	// Handle the case the client hasn't started the allocation.
	if len(a.TaskStates) == 0 {
		return false
	}

	// Check to see if all the tasks finished successfully in the allocation
	allSuccess := true
	for _, state := range a.TaskStates {
		allSuccess = allSuccess && state.Successful()
	}

	return allSuccess
}

// ShouldMigrate returns if the allocation needs data migration
func (a *Allocation) ShouldMigrate() bool {
	if a.PreviousAllocation == "" {
		return false
	}

	if a.DesiredStatus == AllocDesiredStatusStop || a.DesiredStatus == AllocDesiredStatusEvict {
		return false
	}

	tg := a.Job.LookupTaskGroup(a.TaskGroup)

	// if the task group is nil or the ephemeral disk block isn't present then
	// we won't migrate
	if tg == nil || tg.EphemeralDisk == nil {
		return false
	}

	// We won't migrate any data if the user hasn't enabled migration
	return tg.EphemeralDisk.Migrate
}

// SetEventDisplayMessages populates the display message if its not already set,
// a temporary fix to handle old allocations that don't have it.
// This method will be removed in a future release.
func (a *Allocation) SetEventDisplayMessages() {
	setDisplayMsg(a.TaskStates)
}

// LookupTask by name from the Allocation. Returns nil if the Job is not set, the
// TaskGroup does not exist, or the task name cannot be found.
func (a *Allocation) LookupTask(name string) *Task {
	if a.Job == nil {
		return nil
	}

	tg := a.Job.LookupTaskGroup(a.TaskGroup)
	if tg == nil {
		return nil
	}

	return tg.LookupTask(name)
}

// Stub returns a list stub for the allocation
func (a *Allocation) Stub(fields *AllocStubFields) *AllocListStub {
	s := &AllocListStub{
		ID:                    a.ID,
		EvalID:                a.EvalID,
		Name:                  a.Name,
		Namespace:             a.Namespace,
		NodeID:                a.NodeID,
		NodeName:              a.NodeName,
		JobID:                 a.JobID,
		JobType:               a.Job.Type,
		JobVersion:            a.Job.Version,
		TaskGroup:             a.TaskGroup,
		DesiredStatus:         a.DesiredStatus,
		DesiredDescription:    a.DesiredDescription,
		ClientStatus:          a.ClientStatus,
		ClientDescription:     a.ClientDescription,
		DesiredTransition:     a.DesiredTransition,
		TaskStates:            a.TaskStates,
		DeploymentStatus:      a.DeploymentStatus,
		FollowupEvalID:        a.FollowupEvalID,
		NextAllocation:        a.NextAllocation,
		RescheduleTracker:     a.RescheduleTracker,
		PreemptedAllocations:  a.PreemptedAllocations,
		PreemptedByAllocation: a.PreemptedByAllocation,
		CreateIndex:           a.CreateIndex,
		ModifyIndex:           a.ModifyIndex,
		CreateTime:            a.CreateTime,
		ModifyTime:            a.ModifyTime,
	}

	if fields != nil {
		if fields.Resources {
			s.AllocatedResources = a.AllocatedResources
		}
		if !fields.TaskStates {
			s.TaskStates = nil
		}
	}

	return s
}

// AllocationDiff converts an Allocation type to an AllocationDiff type
// If at any time, modification are made to AllocationDiff so that an
// Allocation can no longer be safely converted to AllocationDiff,
// this method should be changed accordingly.
func (a *Allocation) AllocationDiff() *AllocationDiff {
	return (*AllocationDiff)(a)
}

// Expired determines whether an allocation has exceeded its Disconnect.LostAfter
// duration relative to the passed time stamp.
func (a *Allocation) Expired(now time.Time) bool {
	if a == nil || a.Job == nil {
		return false
	}

	// If alloc is not Unknown it cannot be expired.
	if a.ClientStatus != AllocClientStatusUnknown {
		return false
	}

	lastUnknown := a.LastUnknown()
	if lastUnknown.IsZero() {
		return false
	}

	tg := a.Job.LookupTaskGroup(a.TaskGroup)
	if tg == nil {
		return false
	}

	timeout := tg.GetDisconnectLostTimeout()
	if timeout == 0 && tg.Replace() {
		return false
	}

	expiry := lastUnknown.Add(timeout)
	return expiry.Sub(now) <= 0
}

// LastUnknown returns the timestamp for the last time the allocation
// transitioned into the unknown client status.
func (a *Allocation) LastUnknown() time.Time {
	var lastUnknown time.Time

	for _, s := range a.AllocStates {
		if s.Field == AllocStateFieldClientStatus &&
			s.Value == AllocClientStatusUnknown {
			if lastUnknown.IsZero() || lastUnknown.Before(s.Time) {
				lastUnknown = s.Time
			}
		}
	}

	return lastUnknown.UTC()
}

// NeedsToReconnect returns true if the last known ClientStatus value is
// "unknown" and so the allocation did not reconnect yet.
func (a *Allocation) NeedsToReconnect() bool {
	disconnected := false

	// AllocStates are appended to the list and we only need the latest
	// ClientStatus transition, so traverse from the end until we find one.
	for i := len(a.AllocStates) - 1; i >= 0; i-- {
		s := a.AllocStates[i]
		if s.Field != AllocStateFieldClientStatus {
			continue
		}

		disconnected = s.Value == AllocClientStatusUnknown
		break
	}

	return disconnected
}

// LastStartOfTask returns the time of the last start event for the given task
// using the allocations TaskStates. If the task has not started, the zero time
// will be returned.
func (a *Allocation) LastStartOfTask(taskName string) time.Time {
	task := a.TaskStates[taskName]
	if task == nil {
		return time.Time{}
	}

	if task.Restarts > 0 {
		return task.LastRestart
	}

	return task.StartedAt
}

// HasAnyPausedTasks returns true if any of the TaskStates on the alloc
// are Paused (Enterprise feature) either due to a schedule or being forced.
func (a *Allocation) HasAnyPausedTasks() bool {
	if a == nil {
		return false
	}
	for _, ts := range a.TaskStates {
		if ts == nil {
			continue
		}
		if ts.Paused.Stop() {
			return true
		}
	}
	return false
}

// LastRescheduleFailed returns whether the scheduler previously attempted to
// reschedule this allocation but failed to find a placement
func (a *Allocation) LastRescheduleFailed() bool {
	if a.RescheduleTracker == nil {
		return false
	}
	return a.RescheduleTracker.LastReschedule != "" &&
		a.RescheduleTracker.LastReschedule != LastRescheduleSuccess
}

// AllocationDiff is another named type for Allocation (to use the same fields),
// which is used to represent the delta for an Allocation. If you need a method
// defined on the al
type AllocationDiff Allocation

// AllocListStub is used to return a subset of alloc information
type AllocListStub struct {
	ID                    string
	EvalID                string
	Name                  string
	Namespace             string
	NodeID                string
	NodeName              string
	JobID                 string
	JobType               string
	JobVersion            uint64
	TaskGroup             string
	AllocatedResources    *AllocatedResources `json:",omitempty"`
	DesiredStatus         string
	DesiredDescription    string
	ClientStatus          string
	ClientDescription     string
	DesiredTransition     DesiredTransition
	TaskStates            map[string]*TaskState
	DeploymentStatus      *AllocDeploymentStatus
	FollowupEvalID        string
	NextAllocation        string
	RescheduleTracker     *RescheduleTracker
	PreemptedAllocations  []string
	PreemptedByAllocation string
	CreateIndex           uint64
	ModifyIndex           uint64
	CreateTime            int64
	ModifyTime            int64
}

// SetEventDisplayMessages populates the display message if its not already
// set, a temporary fix to handle old allocations that don't have it. This
// method will be removed in a future release.
func (a *AllocListStub) SetEventDisplayMessages() {
	setDisplayMsg(a.TaskStates)
}

// RescheduleEligible returns if the allocation is eligible to be rescheduled according
// to its ReschedulePolicy and the current state of its reschedule trackers
func (a *AllocListStub) RescheduleEligible(reschedulePolicy *ReschedulePolicy, failTime time.Time) bool {
	return a.RescheduleTracker.RescheduleEligible(reschedulePolicy, failTime)
}

// ClientTerminalStatus returns if the client status is terminal and will no longer transition
func (a *AllocListStub) ClientTerminalStatus() bool {
	return slices.Contains(terminalAllocationStatuses, a.ClientStatus)
}

func setDisplayMsg(taskStates map[string]*TaskState) {
	for _, taskState := range taskStates {
		for _, event := range taskState.Events {
			event.PopulateEventDisplayMessage()
		}
	}
}

// AllocStubFields defines which fields are included in the AllocListStub.
type AllocStubFields struct {
	// Resources includes resource-related fields if true.
	Resources bool

	// TaskStates removes the TaskStates field if false (default is to
	// include TaskStates).
	TaskStates bool
}

func NewAllocStubFields() *AllocStubFields {
	return &AllocStubFields{
		// Maintain backward compatibility by retaining task states by
		// default.
		TaskStates: true,
	}
}

// AllocMetric is used to track various metrics while attempting
// to make an allocation. These are used to debug a job, or to better
// understand the pressure within the system.
type AllocMetric struct {
	// NodesEvaluated is the number of nodes that were evaluated
	NodesEvaluated int

	// NodesFiltered is the number of nodes filtered due to a constraint
	NodesFiltered int

	// NodesInPool is the number of nodes in the node pool used by the job.
	NodesInPool int

	// NodesAvailable is the number of nodes available for evaluation per DC.
	NodesAvailable map[string]int

	// ClassFiltered is the number of nodes filtered by class
	ClassFiltered map[string]int

	// ConstraintFiltered is the number of failures caused by constraint
	ConstraintFiltered map[string]int

	// NodesExhausted is the number of nodes skipped due to being
	// exhausted of at least one resource
	NodesExhausted int

	// ClassExhausted is the number of nodes exhausted by class
	ClassExhausted map[string]int

	// DimensionExhausted provides the count by dimension or reason
	DimensionExhausted map[string]int

	// QuotaExhausted provides the exhausted dimensions
	QuotaExhausted []string

	// ResourcesExhausted provides the amount of resources exhausted by task
	// during the allocation placement
	ResourcesExhausted map[string]*Resources

	// Scores is the scores of the final few nodes remaining
	// for placement. The top score is typically selected.
	// Deprecated: Replaced by ScoreMetaData in Nomad 0.9
	Scores map[string]float64

	// ScoreMetaData is a slice of top scoring nodes displayed in the CLI
	ScoreMetaData []*NodeScoreMeta

	// nodeScoreMeta is used to keep scores for a single node id. It is cleared out after
	// we receive normalized score during the last step of the scoring stack.
	nodeScoreMeta *NodeScoreMeta

	// topScores is used to maintain a heap of the top K nodes with
	// the highest normalized score
	topScores *kheap.ScoreHeap

	// AllocationTime is a measure of how long the allocation
	// attempt took. This can affect performance and SLAs.
	AllocationTime time.Duration

	// CoalescedFailures indicates the number of other
	// allocations that were coalesced into this failed allocation.
	// This is to prevent creating many failed allocations for a
	// single task group.
	CoalescedFailures int
}

func (a *AllocMetric) Copy() *AllocMetric {
	if a == nil {
		return nil
	}
	na := new(AllocMetric)
	*na = *a
	na.NodesAvailable = maps.Clone(na.NodesAvailable)
	na.ClassFiltered = maps.Clone(na.ClassFiltered)
	na.ConstraintFiltered = maps.Clone(na.ConstraintFiltered)
	na.ClassExhausted = maps.Clone(na.ClassExhausted)
	na.DimensionExhausted = maps.Clone(na.DimensionExhausted)
	na.QuotaExhausted = slices.Clone(na.QuotaExhausted)
	na.Scores = maps.Clone(na.Scores)
	na.ScoreMetaData = CopySliceNodeScoreMeta(na.ScoreMetaData)
	return na
}

func (a *AllocMetric) EvaluateNode() {
	a.NodesEvaluated += 1
}

func (a *AllocMetric) FilterNode(node *Node, constraint string) {
	a.NodesFiltered += 1
	if node != nil && node.NodeClass != "" {
		if a.ClassFiltered == nil {
			a.ClassFiltered = make(map[string]int)
		}
		a.ClassFiltered[node.NodeClass] += 1
	}
	if constraint != "" {
		if a.ConstraintFiltered == nil {
			a.ConstraintFiltered = make(map[string]int)
		}
		a.ConstraintFiltered[constraint] += 1
	}
}

func (a *AllocMetric) ExhaustedNode(node *Node, dimension string) {
	a.NodesExhausted += 1
	if node != nil && node.NodeClass != "" {
		if a.ClassExhausted == nil {
			a.ClassExhausted = make(map[string]int)
		}
		a.ClassExhausted[node.NodeClass] += 1
	}
	if dimension != "" {
		if a.DimensionExhausted == nil {
			a.DimensionExhausted = make(map[string]int)
		}
		a.DimensionExhausted[dimension] += 1
	}
}

func (a *AllocMetric) ExhaustQuota(dimensions []string) {
	if a.QuotaExhausted == nil {
		a.QuotaExhausted = make([]string, 0, len(dimensions))
	}

	a.QuotaExhausted = append(a.QuotaExhausted, dimensions...)
}

// ExhaustResources updates the amount of resources exhausted for the
// allocation because of the given task group.
func (a *AllocMetric) ExhaustResources(tg *TaskGroup) {
	if a.DimensionExhausted == nil {
		return
	}

	if a.ResourcesExhausted == nil {
		a.ResourcesExhausted = make(map[string]*Resources)
	}

	for _, t := range tg.Tasks {
		exhaustedResources := a.ResourcesExhausted[t.Name]
		if exhaustedResources == nil {
			exhaustedResources = &Resources{}
		}

		if a.DimensionExhausted["memory"] > 0 {
			exhaustedResources.MemoryMB += t.Resources.MemoryMB
		}

		if a.DimensionExhausted["cpu"] > 0 {
			exhaustedResources.CPU += t.Resources.CPU
		}

		a.ResourcesExhausted[t.Name] = exhaustedResources
	}
}

// ScoreNode is used to gather top K scoring nodes in a heap
func (a *AllocMetric) ScoreNode(node *Node, name string, score float64) {
	// Create nodeScoreMeta lazily if its the first time or if its a new node
	if a.nodeScoreMeta == nil || a.nodeScoreMeta.NodeID != node.ID {
		a.nodeScoreMeta = &NodeScoreMeta{
			NodeID: node.ID,
			Scores: make(map[string]float64),
		}
	}
	if name == NormScorerName {
		a.nodeScoreMeta.NormScore = score
		// Once we have the normalized score we can push to the heap
		// that tracks top K by normalized score

		// Create the heap if its not there already
		if a.topScores == nil {
			a.topScores = kheap.NewScoreHeap(MaxRetainedNodeScores)
		}
		heap.Push(a.topScores, a.nodeScoreMeta)

		// Clear out this entry because its now in the heap
		a.nodeScoreMeta = nil
	} else {
		a.nodeScoreMeta.Scores[name] = score
	}
}

// PopulateScoreMetaData populates a map of scorer to scoring metadata
// The map is populated by popping elements from a heap of top K scores
// maintained per scorer
func (a *AllocMetric) PopulateScoreMetaData() {
	if a.topScores == nil {
		return
	}

	if a.ScoreMetaData == nil {
		a.ScoreMetaData = make([]*NodeScoreMeta, a.topScores.Len())
	}
	heapItems := a.topScores.GetItemsReverse()
	for i, item := range heapItems {
		a.ScoreMetaData[i] = item.(*NodeScoreMeta)
	}
}

// MaxNormScore returns the ScoreMetaData entry with the highest normalized
// score.
func (a *AllocMetric) MaxNormScore() *NodeScoreMeta {
	if a == nil || len(a.ScoreMetaData) == 0 {
		return nil
	}
	return a.ScoreMetaData[0]
}

// NodeScoreMeta captures scoring meta data derived from
// different scoring factors.
type NodeScoreMeta struct {
	NodeID    string
	Scores    map[string]float64
	NormScore float64
}

func (s *NodeScoreMeta) Copy() *NodeScoreMeta {
	if s == nil {
		return nil
	}
	ns := new(NodeScoreMeta)
	*ns = *s
	return ns
}

func (s *NodeScoreMeta) String() string {
	return fmt.Sprintf("%s %f %v", s.NodeID, s.NormScore, s.Scores)
}

func (s *NodeScoreMeta) Score() float64 {
	return s.NormScore
}

func (s *NodeScoreMeta) Data() interface{} {
	return s
}

// AllocNetworkStatus captures the status of an allocation's network during runtime.
// Depending on the network mode, an allocation's address may need to be known to other
// systems in Nomad such as service registration.
type AllocNetworkStatus struct {
	InterfaceName string
	Address       string
	AddressIPv6   string
	DNS           *DNSConfig
}

func (a *AllocNetworkStatus) Copy() *AllocNetworkStatus {
	if a == nil {
		return nil
	}
	return &AllocNetworkStatus{
		InterfaceName: a.InterfaceName,
		Address:       a.Address,
		AddressIPv6:   a.AddressIPv6,
		DNS:           a.DNS.Copy(),
	}
}

func (a *AllocNetworkStatus) Equal(o *AllocNetworkStatus) bool {
	// note: this accounts for when DNSConfig is non-nil but empty
	switch {
	case a == nil && o.IsZero():
		return true
	case o == nil && a.IsZero():
		return true
	case a == nil || o == nil:
		return a == o
	}

	switch {
	case a.InterfaceName != o.InterfaceName:
		return false
	case a.Address != o.Address:
		return false
	case a.AddressIPv6 != o.AddressIPv6:
		return false
	case !a.DNS.Equal(o.DNS):
		return false
	}
	return true
}

func (a *AllocNetworkStatus) IsZero() bool {
	if a == nil {
		return true
	}
	if a.InterfaceName != "" || a.Address != "" {
		return false
	}
	if !a.DNS.IsZero() {
		return false
	}
	return true
}

// NetworkStatus is an interface satisfied by alloc runner, for acquiring the
// network status of an allocation.
type NetworkStatus interface {
	NetworkStatus() *AllocNetworkStatus
}

// AllocDeploymentStatus captures the status of the allocation as part of the
// deployment. This can include things like if the allocation has been marked as
// healthy.
type AllocDeploymentStatus struct {
	// Healthy marks whether the allocation has been marked healthy or unhealthy
	// as part of a deployment. It can be unset if it has neither been marked
	// healthy or unhealthy.
	Healthy *bool

	// Timestamp is the time at which the health status was set.
	Timestamp time.Time

	// Canary marks whether the allocation is a canary or not. A canary that has
	// been promoted will have this field set to false.
	Canary bool

	// ModifyIndex is the raft index in which the deployment status was last
	// changed.
	ModifyIndex uint64
}

// HasHealth returns true if the allocation has its health set.
func (a *AllocDeploymentStatus) HasHealth() bool {
	return a != nil && a.Healthy != nil
}

// IsHealthy returns if the allocation is marked as healthy as part of a
// deployment
func (a *AllocDeploymentStatus) IsHealthy() bool {
	if a == nil {
		return false
	}

	return a.Healthy != nil && *a.Healthy
}

// IsUnhealthy returns if the allocation is marked as unhealthy as part of a
// deployment
func (a *AllocDeploymentStatus) IsUnhealthy() bool {
	if a == nil {
		return false
	}

	return a.Healthy != nil && !*a.Healthy
}

// IsCanary returns if the allocation is marked as a canary
func (a *AllocDeploymentStatus) IsCanary() bool {
	if a == nil {
		return false
	}

	return a.Canary
}

func (a *AllocDeploymentStatus) Copy() *AllocDeploymentStatus {
	if a == nil {
		return nil
	}

	c := new(AllocDeploymentStatus)
	*c = *a

	if a.Healthy != nil {
		c.Healthy = pointer.Of(*a.Healthy)
	}

	return c
}

func (a *AllocDeploymentStatus) Equal(o *AllocDeploymentStatus) bool {
	if a == nil || o == nil {
		return a == o
	}

	switch {
	case !pointer.Eq(a.Healthy, o.Healthy):
		return false
	case a.Timestamp != o.Timestamp:
		return false
	case a.Canary != o.Canary:
		return false
	case a.ModifyIndex != o.ModifyIndex:
		return false
	}
	return true
}

const (
	EvalStatusBlocked   = "blocked"
	EvalStatusPending   = "pending"
	EvalStatusComplete  = "complete"
	EvalStatusFailed    = "failed"
	EvalStatusCancelled = "canceled"
)

const (
	EvalTriggerJobRegister          = "job-register"
	EvalTriggerJobDeregister        = "job-deregister"
	EvalTriggerPeriodicJob          = "periodic-job"
	EvalTriggerNodeDrain            = "node-drain"
	EvalTriggerNodeUpdate           = "node-update"
	EvalTriggerAllocStop            = "alloc-stop"
	EvalTriggerScheduled            = "scheduled"
	EvalTriggerRollingUpdate        = "rolling-update"
	EvalTriggerDeploymentWatcher    = "deployment-watcher"
	EvalTriggerFailedFollowUp       = "failed-follow-up"
	EvalTriggerMaxPlans             = "max-plan-attempts"
	EvalTriggerRetryFailedAlloc     = "alloc-failure"
	EvalTriggerQueuedAllocs         = "queued-allocs"
	EvalTriggerPreemption           = "preemption"
	EvalTriggerScaling              = "job-scaling"
	EvalTriggerMaxDisconnectTimeout = "max-disconnect-timeout"
	EvalTriggerReconnect            = "reconnect"
)

const (
	// CoreJobEvalGC is used for the garbage collection of evaluations
	// and allocations. We periodically scan evaluations in a terminal state,
	// in which all the corresponding allocations are also terminal. We
	// delete these out of the system to bound the state.
	CoreJobEvalGC = "eval-gc"

	// CoreJobNodeGC is used for the garbage collection of failed nodes.
	// We periodically scan nodes in a terminal state, and if they have no
	// corresponding allocations we delete these out of the system.
	CoreJobNodeGC = "node-gc"

	// CoreJobJobGC is used for the garbage collection of eligible jobs. We
	// periodically scan garbage collectible jobs and check if both their
	// evaluations and allocations are terminal. If so, we delete these out of
	// the system.
	CoreJobJobGC = "job-gc"

	// CoreJobDeploymentGC is used for the garbage collection of eligible
	// deployments. We periodically scan garbage collectible deployments and
	// check if they are terminal. If so, we delete these out of the system.
	CoreJobDeploymentGC = "deployment-gc"

	// CoreJobCSIVolumeClaimGC is use for the garbage collection of CSI
	// volume claims. We periodically scan volumes to see if no allocs are
	// claiming them. If so, we unclaim the volume.
	CoreJobCSIVolumeClaimGC = "csi-volume-claim-gc"

	// CoreJobCSIPluginGC is use for the garbage collection of CSI plugins.
	// We periodically scan plugins to see if they have no associated volumes
	// or allocs running them. If so, we delete the plugin.
	CoreJobCSIPluginGC = "csi-plugin-gc"

	// CoreJobOneTimeTokenGC is use for the garbage collection of one-time
	// tokens. We periodically scan for expired tokens and delete them.
	CoreJobOneTimeTokenGC = "one-time-token-gc"

	// CoreJobLocalTokenExpiredGC is used for the garbage collection of
	// expired local ACL tokens. We periodically scan for expired tokens and
	// delete them.
	CoreJobLocalTokenExpiredGC = "local-token-expired-gc"

	// CoreJobGlobalTokenExpiredGC is used for the garbage collection of
	// expired global ACL tokens. We periodically scan for expired tokens and
	// delete them.
	CoreJobGlobalTokenExpiredGC = "global-token-expired-gc"

	// CoreJobRootKeyRotateGC is used for periodic key rotation and
	// garbage collection of unused encryption keys.
	CoreJobRootKeyRotateOrGC = "root-key-rotate-gc"

	// CoreJobVariablesRekey is used to fully rotate the encryption keys for
	// variables by decrypting all variables and re-encrypting them with the
	// active key
	CoreJobVariablesRekey = "variables-rekey"

	// CoreJobForceGC is used to force garbage collection of all GCable objects.
	CoreJobForceGC = "force-gc"
)

// Evaluation is used anytime we need to apply business logic as a result
// of a change to our desired state (job specification) or the emergent state
// (registered nodes). When the inputs change, we need to "evaluate" them,
// potentially taking action (allocation of work) or doing nothing if the state
// of the world does not require it.
type Evaluation struct {
	// msgpack omit empty fields during serialization
	_struct bool `codec:",omitempty"` // nolint: structcheck

	// ID is a randomly generated UUID used for this evaluation. This
	// is assigned upon the creation of the evaluation.
	ID string

	// Namespace is the namespace the evaluation is created in
	Namespace string

	// Priority is used to control scheduling importance and if this job
	// can preempt other jobs.
	Priority int

	// Type is used to control which schedulers are available to handle
	// this evaluation.
	Type string

	// TriggeredBy is used to give some insight into why this Eval
	// was created. (Job change, node failure, alloc failure, etc).
	TriggeredBy string

	// JobID is the job this evaluation is scoped to. Evaluations cannot
	// be run in parallel for a given JobID, so we serialize on this.
	JobID string

	// JobModifyIndex is the modify index of the job at the time
	// the evaluation was created
	JobModifyIndex uint64

	// NodeID is the node that was affected triggering the evaluation.
	NodeID string

	// NodeModifyIndex is the modify index of the node at the time
	// the evaluation was created
	NodeModifyIndex uint64

	// DeploymentID is the ID of the deployment that triggered the evaluation.
	DeploymentID string

	// Status of the evaluation
	Status string

	// StatusDescription is meant to provide more human useful information
	StatusDescription string

	// Wait is a minimum wait time for running the eval. This is used to
	// support a rolling upgrade in versions prior to 0.7.0
	// Deprecated
	Wait time.Duration

	// WaitUntil is the time when this eval should be run. This is used to
	// supported delayed rescheduling of failed allocations, and delayed
	// stopping of allocations that are configured with max_client_disconnect.
	WaitUntil time.Time

	// NextEval is the evaluation ID for the eval created to do a followup.
	// This is used to support rolling upgrades and failed-follow-up evals, where
	// we need a chain of evaluations.
	NextEval string

	// PreviousEval is the evaluation ID for the eval creating this one to do a followup.
	// This is used to support rolling upgrades and failed-follow-up evals, where
	// we need a chain of evaluations.
	PreviousEval string

	// BlockedEval is the evaluation ID for a created blocked eval. A
	// blocked eval will be created if all allocations could not be placed due
	// to constraints or lacking resources.
	BlockedEval string

	// RelatedEvals is a list of all the evaluations that are related (next,
	// previous, or blocked) to this one. It may be nil if not requested.
	RelatedEvals []*EvaluationStub

	// FailedTGAllocs are task groups which have allocations that could not be
	// made, but the metrics are persisted so that the user can use the feedback
	// to determine the cause.
	FailedTGAllocs map[string]*AllocMetric

	// ClassEligibility tracks computed node classes that have been explicitly
	// marked as eligible or ineligible.
	ClassEligibility map[string]bool

	// QuotaLimitReached marks whether a quota limit was reached for the
	// evaluation.
	QuotaLimitReached string

	// EscapedComputedClass marks whether the job has constraints that are not
	// captured by computed node classes.
	EscapedComputedClass bool

	// AnnotatePlan triggers the scheduler to provide additional annotations
	// during the evaluation. This should not be set during normal operations.
	AnnotatePlan bool

	// QueuedAllocations is the number of unplaced allocations at the time the
	// evaluation was processed. The map is keyed by Task Group names.
	QueuedAllocations map[string]int

	// LeaderACL provides the ACL token to when issuing RPCs back to the
	// leader. This will be a valid management token as long as the leader is
	// active. This should not ever be exposed via the API.
	LeaderACL string

	// SnapshotIndex is the Raft index of the snapshot used to process the
	// evaluation. The index will either be set when it has gone through the
	// scheduler or if a blocked evaluation is being created. The index is set
	// in this case so we can determine if an early unblocking is required since
	// capacity has changed since the evaluation was created. This can result in
	// the SnapshotIndex being less than the CreateIndex.
	SnapshotIndex uint64

	// Raft Indexes
	CreateIndex uint64
	ModifyIndex uint64

	CreateTime int64
	ModifyTime int64
}

type EvaluationStub struct {
	ID                string
	Namespace         string
	Priority          int
	Type              string
	TriggeredBy       string
	JobID             string
	NodeID            string
	DeploymentID      string
	Status            string
	StatusDescription string
	WaitUntil         time.Time
	NextEval          string
	PreviousEval      string
	BlockedEval       string
	CreateIndex       uint64
	ModifyIndex       uint64
	CreateTime        int64
	ModifyTime        int64
}

// GetID implements the IDGetter interface, required for pagination.
func (e *Evaluation) GetID() string {
	if e == nil {
		return ""
	}
	return e.ID
}

// GetNamespace implements the NamespaceGetter interface, required for pagination.
func (e *Evaluation) GetNamespace() string {
	if e == nil {
		return ""
	}
	return e.Namespace
}

// GetCreateIndex implements the CreateIndexGetter interface, required for
// pagination.
func (e *Evaluation) GetCreateIndex() uint64 {
	if e == nil {
		return 0
	}
	return e.CreateIndex
}

// TerminalStatus returns if the current status is terminal and
// will no longer transition.
func (e *Evaluation) TerminalStatus() bool {
	switch e.Status {
	case EvalStatusComplete, EvalStatusFailed, EvalStatusCancelled:
		return true
	default:
		return false
	}
}

func (e *Evaluation) GoString() string {
	return fmt.Sprintf("<Eval %q JobID: %q Namespace: %q>", e.ID, e.JobID, e.Namespace)
}

func (e *Evaluation) RelatedIDs() []string {
	if e == nil {
		return nil
	}

	ids := []string{e.NextEval, e.PreviousEval, e.BlockedEval}
	related := make([]string, 0, len(ids))

	for _, id := range ids {
		if id != "" {
			related = append(related, id)
		}
	}

	return related
}

func (e *Evaluation) Stub() *EvaluationStub {
	if e == nil {
		return nil
	}

	return &EvaluationStub{
		ID:                e.ID,
		Namespace:         e.Namespace,
		Priority:          e.Priority,
		Type:              e.Type,
		TriggeredBy:       e.TriggeredBy,
		JobID:             e.JobID,
		NodeID:            e.NodeID,
		DeploymentID:      e.DeploymentID,
		Status:            e.Status,
		StatusDescription: e.StatusDescription,
		WaitUntil:         e.WaitUntil,
		NextEval:          e.NextEval,
		PreviousEval:      e.PreviousEval,
		BlockedEval:       e.BlockedEval,
		CreateIndex:       e.CreateIndex,
		ModifyIndex:       e.ModifyIndex,
		CreateTime:        e.CreateTime,
		ModifyTime:        e.ModifyTime,
	}
}

func (e *Evaluation) Copy() *Evaluation {
	if e == nil {
		return nil
	}
	ne := new(Evaluation)
	*ne = *e

	// Copy ClassEligibility
	if e.ClassEligibility != nil {
		classes := make(map[string]bool, len(e.ClassEligibility))
		for class, elig := range e.ClassEligibility {
			classes[class] = elig
		}
		ne.ClassEligibility = classes
	}

	// Copy FailedTGAllocs
	if e.FailedTGAllocs != nil {
		failedTGs := make(map[string]*AllocMetric, len(e.FailedTGAllocs))
		for tg, metric := range e.FailedTGAllocs {
			failedTGs[tg] = metric.Copy()
		}
		ne.FailedTGAllocs = failedTGs
	}

	// Copy queued allocations
	if e.QueuedAllocations != nil {
		queuedAllocations := make(map[string]int, len(e.QueuedAllocations))
		for tg, num := range e.QueuedAllocations {
			queuedAllocations[tg] = num
		}
		ne.QueuedAllocations = queuedAllocations
	}

	return ne
}

// ShouldEnqueue checks if a given evaluation should be enqueued into the
// eval_broker
func (e *Evaluation) ShouldEnqueue() bool {
	switch e.Status {
	case EvalStatusPending:
		return true
	case EvalStatusComplete, EvalStatusFailed, EvalStatusBlocked, EvalStatusCancelled:
		return false
	default:
		panic(fmt.Sprintf("unhandled evaluation (%s) status %s", e.ID, e.Status))
	}
}

// ShouldBlock checks if a given evaluation should be entered into the blocked
// eval tracker.
func (e *Evaluation) ShouldBlock() bool {
	switch e.Status {
	case EvalStatusBlocked:
		return true
	case EvalStatusComplete, EvalStatusFailed, EvalStatusPending, EvalStatusCancelled:
		return false
	default:
		panic(fmt.Sprintf("unhandled evaluation (%s) status %s", e.ID, e.Status))
	}
}

// MakePlan is used to make a plan from the given evaluation
// for a given Job
func (e *Evaluation) MakePlan(j *Job) *Plan {
	p := &Plan{
		EvalID:          e.ID,
		Priority:        e.Priority,
		Job:             j,
		NodeUpdate:      make(map[string][]*Allocation),
		NodeAllocation:  make(map[string][]*Allocation),
		NodePreemptions: make(map[string][]*Allocation),
	}
	if j != nil {
		p.AllAtOnce = j.AllAtOnce
	}
	return p
}

// NextRollingEval creates an evaluation to followup this eval for rolling updates
func (e *Evaluation) NextRollingEval(wait time.Duration) *Evaluation {
	now := time.Now().UTC().UnixNano()
	return &Evaluation{
		ID:             uuid.Generate(),
		Namespace:      e.Namespace,
		Priority:       e.Priority,
		Type:           e.Type,
		TriggeredBy:    EvalTriggerRollingUpdate,
		JobID:          e.JobID,
		JobModifyIndex: e.JobModifyIndex,
		Status:         EvalStatusPending,
		Wait:           wait,
		PreviousEval:   e.ID,
		CreateTime:     now,
		ModifyTime:     now,
	}
}

// CreateBlockedEval creates a blocked evaluation to followup this eval to place any
// failed allocations. It takes the classes marked explicitly eligible or
// ineligible, whether the job has escaped computed node classes and whether the
// quota limit was reached.
func (e *Evaluation) CreateBlockedEval(classEligibility map[string]bool,
	escaped bool, quotaReached string, failedTGAllocs map[string]*AllocMetric) *Evaluation {
	now := time.Now().UTC().UnixNano()
	return &Evaluation{
		ID:                   uuid.Generate(),
		Namespace:            e.Namespace,
		Priority:             e.Priority,
		Type:                 e.Type,
		TriggeredBy:          EvalTriggerQueuedAllocs,
		JobID:                e.JobID,
		JobModifyIndex:       e.JobModifyIndex,
		Status:               EvalStatusBlocked,
		PreviousEval:         e.ID,
		FailedTGAllocs:       failedTGAllocs,
		ClassEligibility:     classEligibility,
		EscapedComputedClass: escaped,
		QuotaLimitReached:    quotaReached,
		CreateTime:           now,
		ModifyTime:           now,
	}
}

// CreateFailedFollowUpEval creates a follow up evaluation when the current one
// has been marked as failed because it has hit the delivery limit and will not
// be retried by the eval_broker. Callers should copy the created eval's ID to
// into the old eval's NextEval field.
func (e *Evaluation) CreateFailedFollowUpEval(wait time.Duration) *Evaluation {
	now := time.Now().UTC().UnixNano()
	return &Evaluation{
		ID:             uuid.Generate(),
		Namespace:      e.Namespace,
		Priority:       e.Priority,
		Type:           e.Type,
		TriggeredBy:    EvalTriggerFailedFollowUp,
		JobID:          e.JobID,
		JobModifyIndex: e.JobModifyIndex,
		Status:         EvalStatusPending,
		Wait:           wait,
		PreviousEval:   e.ID,
		CreateTime:     now,
		ModifyTime:     now,
	}
}

// UpdateModifyTime takes into account that clocks on different servers may be
// slightly out of sync. Even in case of a leader change, this method will
// guarantee that ModifyTime will always be after CreateTime.
func (e *Evaluation) UpdateModifyTime() {
	now := time.Now().UTC().UnixNano()
	if now <= e.CreateTime {
		e.ModifyTime = e.CreateTime + 1
	} else {
		e.ModifyTime = now
	}
}

// Plan is used to submit a commit plan for task allocations. These
// are submitted to the leader which verifies that resources have
// not been overcommitted before admitting the plan.
type Plan struct {
	// msgpack omit empty fields during serialization
	_struct bool `codec:",omitempty"` // nolint: structcheck

	// EvalID is the evaluation ID this plan is associated with
	EvalID string

	// EvalToken is used to prevent a split-brain processing of
	// an evaluation. There should only be a single scheduler running
	// an Eval at a time, but this could be violated after a leadership
	// transition. This unique token is used to reject plans that are
	// being submitted from a different leader.
	EvalToken string

	// Priority is the priority of the upstream job
	Priority int

	// AllAtOnce is used to control if incremental scheduling of task groups
	// is allowed or if we must do a gang scheduling of the entire job.
	// If this is false, a plan may be partially applied. Otherwise, the
	// entire plan must be able to make progress.
	AllAtOnce bool

	// Job is the parent job of all the allocations in the Plan.
	// Since a Plan only involves a single Job, we can reduce the size
	// of the plan by only including it once.
	Job *Job

	// NodeUpdate contains all the allocations to be stopped or evicted for
	// each node.
	NodeUpdate map[string][]*Allocation

	// NodeAllocation contains all the allocations for each node.
	// The evicts must be considered prior to the allocations.
	NodeAllocation map[string][]*Allocation

	// Annotations contains annotations by the scheduler to be used by operators
	// to understand the decisions made by the scheduler.
	Annotations *PlanAnnotations

	// Deployment is the deployment created or updated by the scheduler that
	// should be applied by the planner.
	Deployment *Deployment

	// DeploymentUpdates is a set of status updates to apply to the given
	// deployments. This allows the scheduler to cancel any unneeded deployment
	// because the job is stopped or the update block is removed.
	DeploymentUpdates []*DeploymentStatusUpdate

	// NodePreemptions is a map from node id to a set of allocations from other
	// lower priority jobs that are preempted. Preempted allocations are marked
	// as evicted.
	NodePreemptions map[string][]*Allocation

	// SnapshotIndex is the Raft index of the snapshot used to create the
	// Plan. The leader will wait to evaluate the plan until its StateStore
	// has reached at least this index.
	SnapshotIndex uint64
}

func (p *Plan) GoString() string {
	out := fmt.Sprintf("(eval %s", p.EvalID[:8])
	if p.Job != nil {
		out += fmt.Sprintf(", job %s", p.Job.ID)
	}
	if p.Deployment != nil {
		out += fmt.Sprintf(", deploy %s", p.Deployment.ID[:8])
	}
	if len(p.NodeUpdate) > 0 {
		out += ", NodeUpdates: "
		for node, allocs := range p.NodeUpdate {
			out += fmt.Sprintf("(node[%s]", node[:8])
			for _, alloc := range allocs {
				out += fmt.Sprintf(" (%s stop/evict)", alloc.ID[:8])
			}
			out += ")"
		}
	}
	if len(p.NodeAllocation) > 0 {
		out += ", NodeAllocations: "
		for node, allocs := range p.NodeAllocation {
			out += fmt.Sprintf("(node[%s]", node[:8])
			for _, alloc := range allocs {
				out += fmt.Sprintf(" (%s %s %s)",
					alloc.ID[:8], alloc.Name, alloc.DesiredStatus,
				)
			}
			out += ")"
		}
	}
	if len(p.NodePreemptions) > 0 {
		out += ", NodePreemptions: "
		for node, allocs := range p.NodePreemptions {
			out += fmt.Sprintf("(node[%s]", node[:8])
			for _, alloc := range allocs {
				out += fmt.Sprintf(" (%s %s %s)",
					alloc.ID[:8], alloc.Name, alloc.DesiredStatus,
				)
			}
			out += ")"
		}
	}
	if len(p.DeploymentUpdates) > 0 {
		out += ", DeploymentUpdates: "
		for _, dupdate := range p.DeploymentUpdates {
			out += fmt.Sprintf("(%s %s)",
				dupdate.DeploymentID[:8], dupdate.Status)
		}
	}
	if p.Annotations != nil {
		out += ", Annotations: "
		for tg, updates := range p.Annotations.DesiredTGUpdates {
			out += fmt.Sprintf("(update[%s] %v)", tg, updates)
		}
		for _, preempted := range p.Annotations.PreemptedAllocs {
			out += fmt.Sprintf("(preempt %s)", preempted.ID[:8])
		}
	}

	out += ")"
	return out
}

// AppendStoppedAlloc marks an allocation to be stopped. The clientStatus of the
// allocation may be optionally set by passing in a non-empty value.
func (p *Plan) AppendStoppedAlloc(alloc *Allocation, desiredDesc, clientStatus, followupEvalID string) {
	newAlloc := new(Allocation)
	*newAlloc = *alloc

	// If the job is not set in the plan we are deregistering a job so we
	// extract the job from the allocation.
	if p.Job == nil && newAlloc.Job != nil {
		p.Job = newAlloc.Job
	}

	// Normalize the job
	newAlloc.Job = nil

	// Strip the resources as it can be rebuilt.
	newAlloc.Resources = nil

	newAlloc.DesiredStatus = AllocDesiredStatusStop
	newAlloc.DesiredDescription = desiredDesc

	if clientStatus != "" {
		newAlloc.ClientStatus = clientStatus
	}

	newAlloc.AppendState(AllocStateFieldClientStatus, clientStatus)

	if followupEvalID != "" {
		newAlloc.FollowupEvalID = followupEvalID
	}

	node := alloc.NodeID
	existing := p.NodeUpdate[node]
	p.NodeUpdate[node] = append(existing, newAlloc)
}

// AppendPreemptedAlloc is used to append an allocation that's being preempted to the plan.
// To minimize the size of the plan, this only sets a minimal set of fields in the allocation
func (p *Plan) AppendPreemptedAlloc(alloc *Allocation, preemptingAllocID string) {
	newAlloc := &Allocation{}
	newAlloc.ID = alloc.ID
	newAlloc.JobID = alloc.JobID
	newAlloc.Namespace = alloc.Namespace
	newAlloc.DesiredStatus = AllocDesiredStatusEvict
	newAlloc.PreemptedByAllocation = preemptingAllocID

	desiredDesc := fmt.Sprintf("Preempted by alloc ID %v", preemptingAllocID)
	newAlloc.DesiredDescription = desiredDesc

	// TaskResources are needed by the plan applier to check if allocations fit
	// after removing preempted allocations
	if alloc.AllocatedResources != nil {
		newAlloc.AllocatedResources = alloc.AllocatedResources
	} else {
		// COMPAT Remove in version 0.11
		newAlloc.TaskResources = alloc.TaskResources
		newAlloc.SharedResources = alloc.SharedResources
	}

	// Append this alloc to slice for this node
	node := alloc.NodeID
	existing := p.NodePreemptions[node]
	p.NodePreemptions[node] = append(existing, newAlloc)
}

// AppendUnknownAlloc marks an allocation as unknown.
func (p *Plan) AppendUnknownAlloc(alloc *Allocation) {
	// Strip the resources as they can be rebuilt.
	alloc.Resources = nil

	existing := p.NodeAllocation[alloc.NodeID]
	p.NodeAllocation[alloc.NodeID] = append(existing, alloc)
}

func (p *Plan) PopUpdate(alloc *Allocation) {
	existing := p.NodeUpdate[alloc.NodeID]
	n := len(existing)
	if n > 0 && existing[n-1].ID == alloc.ID {
		existing = existing[:n-1]
		if len(existing) > 0 {
			p.NodeUpdate[alloc.NodeID] = existing
		} else {
			delete(p.NodeUpdate, alloc.NodeID)
		}
	}
}

// AppendAlloc appends the alloc to the plan allocations.
// Uses the passed job if explicitly passed, otherwise
// it is assumed the alloc will use the plan Job version.
func (p *Plan) AppendAlloc(alloc *Allocation, job *Job) {
	node := alloc.NodeID
	existing := p.NodeAllocation[node]

	alloc.Job = job

	p.NodeAllocation[node] = append(existing, alloc)
}

// IsNoOp checks if this plan would do nothing
func (p *Plan) IsNoOp() bool {
	return len(p.NodeUpdate) == 0 &&
		len(p.NodeAllocation) == 0 &&
		p.Deployment == nil &&
		len(p.DeploymentUpdates) == 0
}

// NormalizeAllocations normalizes allocations to remove fields that can
// be fetched from the MemDB instead of sending over the wire
func (p *Plan) NormalizeAllocations() {
	for _, allocs := range p.NodeUpdate {
		for i, alloc := range allocs {
			allocs[i] = &Allocation{
				ID:                 alloc.ID,
				DesiredDescription: alloc.DesiredDescription,
				ClientStatus:       alloc.ClientStatus,
				FollowupEvalID:     alloc.FollowupEvalID,
			}
		}
	}

	for _, allocs := range p.NodePreemptions {
		for i, alloc := range allocs {
			allocs[i] = &Allocation{
				ID:                    alloc.ID,
				PreemptedByAllocation: alloc.PreemptedByAllocation,
			}
		}
	}
}

// PlanResult is the result of a plan submitted to the leader.
type PlanResult struct {
	// NodeUpdate contains all the evictions and stops that were committed.
	NodeUpdate map[string][]*Allocation

	// NodeAllocation contains all the allocations that were committed.
	NodeAllocation map[string][]*Allocation

	// Deployment is the deployment that was committed.
	Deployment *Deployment

	// DeploymentUpdates is the set of deployment updates that were committed.
	DeploymentUpdates []*DeploymentStatusUpdate

	// NodePreemptions is a map from node id to a set of allocations from other
	// lower priority jobs that are preempted. Preempted allocations are marked
	// as stopped.
	NodePreemptions map[string][]*Allocation

	// RejectedNodes are nodes the scheduler worker has rejected placements for
	// and should be considered for ineligibility by the plan applier to avoid
	// retrying them repeatedly.
	RejectedNodes []string

	// IneligibleNodes are nodes the plan applier has repeatedly rejected
	// placements for and should therefore be considered ineligible by workers
	// to avoid retrying them repeatedly.
	IneligibleNodes []string

	// RefreshIndex is the index the worker should refresh state up to.
	// This allows all evictions and allocations to be materialized.
	// If any allocations were rejected due to stale data (node state,
	// over committed) this can be used to force a worker refresh.
	RefreshIndex uint64

	// AllocIndex is the Raft index in which the evictions and
	// allocations took place. This is used for the write index.
	AllocIndex uint64
}

// IsNoOp checks if this plan result would do nothing
func (p *PlanResult) IsNoOp() bool {
	return len(p.IneligibleNodes) == 0 && len(p.NodeUpdate) == 0 &&
		len(p.NodeAllocation) == 0 && len(p.DeploymentUpdates) == 0 &&
		p.Deployment == nil
}

// FullCommit is used to check if all the allocations in a plan
// were committed as part of the result. Returns if there was
// a match, and the number of expected and actual allocations.
func (p *PlanResult) FullCommit(plan *Plan) (bool, int, int) {
	expected := 0
	actual := 0
	for name, allocList := range plan.NodeAllocation {
		didAlloc := p.NodeAllocation[name]
		expected += len(allocList)
		actual += len(didAlloc)
	}
	return actual == expected, expected, actual
}

// PlanAnnotations holds annotations made by the scheduler to give further debug
// information to operators.
type PlanAnnotations struct {
	// DesiredTGUpdates is the set of desired updates per task group.
	DesiredTGUpdates map[string]*DesiredUpdates

	// PreemptedAllocs is the set of allocations to be preempted to make the placement successful.
	PreemptedAllocs []*AllocListStub
}

// DesiredUpdates is the set of changes the scheduler would like to make given
// sufficient resources and cluster capacity.
type DesiredUpdates struct {
	Ignore            uint64
	Place             uint64
	Migrate           uint64
	Stop              uint64
	InPlaceUpdate     uint64
	DestructiveUpdate uint64
	Canary            uint64
	Preemptions       uint64
}

func (d *DesiredUpdates) GoString() string {
	return fmt.Sprintf("(place %d) (inplace %d) (destructive %d) (stop %d) (migrate %d) (ignore %d) (canary %d)",
		d.Place, d.InPlaceUpdate, d.DestructiveUpdate, d.Stop, d.Migrate, d.Ignore, d.Canary)
}

// msgpackHandle is a shared handle for encoding/decoding of structs
var MsgpackHandle = func() *codec.MsgpackHandle {
	h := &codec.MsgpackHandle{}
	h.RawToString = true

	// maintain binary format from time prior to upgrading latest ugorji
	h.BasicHandle.TimeNotBuiltin = true

	// Sets the default type for decoding a map into a nil interface{}.
	// This is necessary in particular because we store the driver configs as a
	// nil interface{}.
	h.MapType = reflect.TypeOf(map[string]interface{}(nil))

	// only review struct codec tags
	h.TypeInfos = codec.NewTypeInfos([]string{"codec"})

	return h
}()

// Decode is used to decode a MsgPack encoded object
func Decode(buf []byte, out interface{}) error {
	return codec.NewDecoder(bytes.NewReader(buf), MsgpackHandle).Decode(out)
}

// Encode is used to encode a MsgPack object with type prefix
func Encode(t MessageType, msg interface{}) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte(uint8(t))
	err := codec.NewEncoder(&buf, MsgpackHandle).Encode(msg)
	return buf.Bytes(), err
}

// KeyringResponse is a unified key response and can be used for install,
// remove, use, as well as listing key queries.
type KeyringResponse struct {
	Messages map[string]string
	Keys     map[string]int
	NumNodes int
}

// KeyringRequest is request objects for serf key operations.
type KeyringRequest struct {
	Key string
}

// RecoverableError wraps an error and marks whether it is recoverable and could
// be retried or it is fatal.
type RecoverableError struct {
	Err         string
	Recoverable bool
	wrapped     error
}

// NewRecoverableError is used to wrap an error and mark it as recoverable or
// not.
func NewRecoverableError(e error, recoverable bool) error {
	if e == nil {
		return nil
	}

	return &RecoverableError{
		Err:         e.Error(),
		Recoverable: recoverable,
		wrapped:     e,
	}
}

// WrapRecoverable wraps an existing error in a new RecoverableError with a new
// message. If the error was recoverable before the returned error is as well;
// otherwise it is unrecoverable.
func WrapRecoverable(msg string, err error) error {
	return &RecoverableError{Err: msg, Recoverable: IsRecoverable(err)}
}

func (r *RecoverableError) Error() string {
	return r.Err
}

func (r *RecoverableError) IsRecoverable() bool {
	return r.Recoverable
}

func (r *RecoverableError) IsUnrecoverable() bool {
	return !r.Recoverable
}

func (r *RecoverableError) Unwrap() error {
	return r.wrapped
}

// Recoverable is an interface for errors to implement to indicate whether or
// not they are fatal or recoverable.
type Recoverable interface {
	error
	IsRecoverable() bool
}

// IsRecoverable returns true if error is a RecoverableError with
// Recoverable=true. Otherwise false is returned.
func IsRecoverable(e error) bool {
	if re, ok := e.(Recoverable); ok {
		return re.IsRecoverable()
	}
	return false
}

// WrappedServerError wraps an error and satisfies
// both the Recoverable and the ServerSideError interfaces
type WrappedServerError struct {
	Err error
}

// NewWrappedServerError is used to create a wrapped server side error
func NewWrappedServerError(e error) error {
	return &WrappedServerError{
		Err: e,
	}
}

func (r *WrappedServerError) IsRecoverable() bool {
	return IsRecoverable(r.Err)
}

func (r *WrappedServerError) Error() string {
	return r.Err.Error()
}

func (r *WrappedServerError) IsServerSide() bool {
	return true
}

// ServerSideError is an interface for errors to implement to indicate
// errors occurring after the request makes it to a server
type ServerSideError interface {
	error
	IsServerSide() bool
}

// IsServerSide returns true if error is a wrapped
// server side error
func IsServerSide(e error) bool {
	if se, ok := e.(ServerSideError); ok {
		return se.IsServerSide()
	}
	return false
}

// ACLPolicy is used to represent an ACL policy
type ACLPolicy struct {
	Name        string      // Unique name
	Description string      // Human readable
	Rules       string      // HCL or JSON format
	RulesJSON   *acl.Policy // Generated from Rules on read
	JobACL      *JobACL
	Hash        []byte

	CreateIndex uint64
	ModifyIndex uint64
}

// JobACL represents an ACL policy's attachment to a job, group, or task.
type JobACL struct {
	Namespace string // namespace of the job
	JobID     string // ID of the job
	Group     string // ID of the group
	Task      string // ID of the task
}

// SetHash is used to compute and set the hash of the ACL policy
func (a *ACLPolicy) SetHash() []byte {
	// Initialize a 256bit Blake2 hash (32 bytes)
	hash, err := blake2b.New256(nil)
	if err != nil {
		panic(err)
	}

	// Write all the user set fields
	_, _ = hash.Write([]byte(a.Name))
	_, _ = hash.Write([]byte(a.Description))
	_, _ = hash.Write([]byte(a.Rules))

	if a.JobACL != nil {
		_, _ = hash.Write([]byte(a.JobACL.Namespace))
		_, _ = hash.Write([]byte(a.JobACL.JobID))
		_, _ = hash.Write([]byte(a.JobACL.Group))
		_, _ = hash.Write([]byte(a.JobACL.Task))
	}

	// Finalize the hash
	hashVal := hash.Sum(nil)

	// Set and return the hash
	a.Hash = hashVal
	return hashVal
}

func (a *ACLPolicy) Stub() *ACLPolicyListStub {
	return &ACLPolicyListStub{
		Name:        a.Name,
		Description: a.Description,
		Hash:        a.Hash,
		CreateIndex: a.CreateIndex,
		ModifyIndex: a.ModifyIndex,
	}
}

func (a *ACLPolicy) Validate() error {
	var mErr multierror.Error
	if !ValidPolicyName.MatchString(a.Name) {
		err := fmt.Errorf("invalid name '%s'", a.Name)
		mErr.Errors = append(mErr.Errors, err)
	}
	if _, err := acl.Parse(a.Rules); err != nil {
		err = fmt.Errorf("failed to parse rules: %v", err)
		mErr.Errors = append(mErr.Errors, err)
	}
	if len(a.Description) > maxPolicyDescriptionLength {
		err := fmt.Errorf("description longer than %d", maxPolicyDescriptionLength)
		mErr.Errors = append(mErr.Errors, err)
	}
	if a.JobACL != nil {
		if a.JobACL.JobID != "" && a.JobACL.Namespace == "" {
			err := fmt.Errorf("namespace must be set to set job ID")
			mErr.Errors = append(mErr.Errors, err)
		}
		if a.JobACL.Group != "" && a.JobACL.JobID == "" {
			err := fmt.Errorf("job ID must be set to set group")
			mErr.Errors = append(mErr.Errors, err)
		}
		if a.JobACL.Task != "" && a.JobACL.Group == "" {
			err := fmt.Errorf("group must be set to set task")
			mErr.Errors = append(mErr.Errors, err)
		}
	}

	return mErr.ErrorOrNil()
}

// ACLPolicyListStub is used to for listing ACL policies
type ACLPolicyListStub struct {
	Name        string
	Description string
	Hash        []byte
	CreateIndex uint64
	ModifyIndex uint64
}

// ACLPolicyListRequest is used to request a list of policies
type ACLPolicyListRequest struct {
	QueryOptions
}

// ACLPolicySpecificRequest is used to query a specific policy
type ACLPolicySpecificRequest struct {
	Name string
	QueryOptions
}

// ACLPolicySetRequest is used to query a set of policies
type ACLPolicySetRequest struct {
	Names []string
	QueryOptions
}

// ACLPolicyListResponse is used for a list request
type ACLPolicyListResponse struct {
	Policies []*ACLPolicyListStub
	QueryMeta
}

// SingleACLPolicyResponse is used to return a single policy
type SingleACLPolicyResponse struct {
	Policy *ACLPolicy
	QueryMeta
}

// ACLPolicySetResponse is used to return a set of policies
type ACLPolicySetResponse struct {
	Policies map[string]*ACLPolicy
	QueryMeta
}

// ACLPolicyDeleteRequest is used to delete a set of policies
type ACLPolicyDeleteRequest struct {
	Names []string
	WriteRequest
}

// ACLPolicyUpsertRequest is used to upsert a set of policies
type ACLPolicyUpsertRequest struct {
	Policies []*ACLPolicy
	WriteRequest
}

// ACLToken represents a client token which is used to Authenticate
type ACLToken struct {
	AccessorID string   // Public Accessor ID (UUID)
	SecretID   string   // Secret ID, private (UUID)
	Name       string   // Human friendly name
	Type       string   // Client or Management
	Policies   []string // Policies this token ties to

	// Roles represents the ACL roles that this token is tied to. The token
	// will inherit the permissions of all policies detailed within the role.
	Roles []*ACLTokenRoleLink

	Global     bool // Global or Region local
	Hash       []byte
	CreateTime time.Time // Time of creation

	// ExpirationTime represents the point after which a token should be
	// considered revoked and is eligible for destruction. This time should
	// always use UTC to account for multi-region global tokens. It is a
	// pointer, so we can store nil, rather than the zero value of time.Time.
	ExpirationTime *time.Time

	// ExpirationTTL is a convenience field for helping set ExpirationTime to a
	// value of CreateTime+ExpirationTTL. This can only be set during token
	// creation. This is a string version of a time.Duration like "2m".
	ExpirationTTL time.Duration

	CreateIndex uint64
	ModifyIndex uint64
}

// GetID implements the IDGetter interface, required for pagination.
func (a *ACLToken) GetID() string {
	if a == nil {
		return ""
	}
	return a.AccessorID
}

// GetCreateIndex implements the CreateIndexGetter interface, required for
// pagination.
func (a *ACLToken) GetCreateIndex() uint64 {
	if a == nil {
		return 0
	}
	return a.CreateIndex
}

func (a *ACLToken) Copy() *ACLToken {
	c := new(ACLToken)
	*c = *a

	c.Policies = make([]string, len(a.Policies))
	copy(c.Policies, a.Policies)

	c.Hash = make([]byte, len(a.Hash))
	copy(c.Hash, a.Hash)

	c.Roles = make([]*ACLTokenRoleLink, len(a.Roles))
	copy(c.Roles, a.Roles)

	return c
}

var (
	// AnonymousACLToken is used when no SecretID is provided, and the request
	// is made anonymously.
	AnonymousACLToken = &ACLToken{
		AccessorID: "anonymous",
		Name:       "Anonymous Token",
		Type:       ACLClientToken,
		Policies:   []string{"anonymous"},
		Global:     false,
	}

	// LeaderACLToken is used to represent a leader's own token; this object
	// never gets used except on the leader
	LeaderACLToken = &ACLToken{
		AccessorID: "leader",
		Name:       "Leader Token",
		Type:       ACLManagementToken,
	}

	// ACLsDisabledToken is used when ACLs are disabled.
	ACLsDisabledToken = &ACLToken{
		AccessorID: "acls-disabled",
		Name:       "ACLs disabled token",
		Type:       ACLClientToken,
		Global:     false,
	}
)

type ACLTokenListStub struct {
	AccessorID     string
	Name           string
	Type           string
	Policies       []string
	Roles          []*ACLTokenRoleLink
	Global         bool
	Hash           []byte
	CreateTime     time.Time
	ExpirationTime *time.Time
	CreateIndex    uint64
	ModifyIndex    uint64
}

// SetHash is used to compute and set the hash of the ACL token. It only hashes
// fields which can be updated, and as such, does not hash fields such as
// ExpirationTime.
func (a *ACLToken) SetHash() []byte {
	// Initialize a 256bit Blake2 hash (32 bytes)
	hash, err := blake2b.New256(nil)
	if err != nil {
		panic(err)
	}

	// Write all the user set fields
	_, _ = hash.Write([]byte(a.Name))
	_, _ = hash.Write([]byte(a.Type))
	for _, policyName := range a.Policies {
		_, _ = hash.Write([]byte(policyName))
	}
	if a.Global {
		_, _ = hash.Write([]byte("global"))
	} else {
		_, _ = hash.Write([]byte("local"))
	}

	// Iterate the ACL role links and hash the ID. The ID is immutable and the
	// canonical way to reference a role. The name can be modified by
	// operators, but won't impact the ACL token resolution.
	for _, roleLink := range a.Roles {
		_, _ = hash.Write([]byte(roleLink.ID))
	}

	// Finalize the hash
	hashVal := hash.Sum(nil)

	// Set and return the hash
	a.Hash = hashVal
	return hashVal
}

func (a *ACLToken) Stub() *ACLTokenListStub {
	return &ACLTokenListStub{
		AccessorID:     a.AccessorID,
		Name:           a.Name,
		Type:           a.Type,
		Policies:       a.Policies,
		Roles:          a.Roles,
		Global:         a.Global,
		Hash:           a.Hash,
		CreateTime:     a.CreateTime,
		ExpirationTime: a.ExpirationTime,
		CreateIndex:    a.CreateIndex,
		ModifyIndex:    a.ModifyIndex,
	}
}

// ACLTokenListRequest is used to request a list of tokens
type ACLTokenListRequest struct {
	GlobalOnly bool
	QueryOptions
}

// ACLTokenSpecificRequest is used to query a specific token
type ACLTokenSpecificRequest struct {
	AccessorID string
	QueryOptions
}

// ACLTokenSetRequest is used to query a set of tokens
type ACLTokenSetRequest struct {
	AccessorIDS []string
	QueryOptions
}

// ACLTokenListResponse is used for a list request
type ACLTokenListResponse struct {
	Tokens []*ACLTokenListStub
	QueryMeta
}

// SingleACLTokenResponse is used to return a single token
type SingleACLTokenResponse struct {
	Token *ACLToken
	QueryMeta
}

// ACLTokenSetResponse is used to return a set of token
type ACLTokenSetResponse struct {
	Tokens map[string]*ACLToken // Keyed by Accessor ID
	QueryMeta
}

// ResolveACLTokenRequest is used to resolve a specific token
type ResolveACLTokenRequest struct {
	SecretID string
	QueryOptions
}

// ResolveACLTokenResponse is used to resolve a single token
type ResolveACLTokenResponse struct {
	Token *ACLToken
	QueryMeta
}

// ACLTokenDeleteRequest is used to delete a set of tokens
type ACLTokenDeleteRequest struct {
	AccessorIDs []string
	WriteRequest
}

// ACLTokenBootstrapRequest is used to bootstrap ACLs
type ACLTokenBootstrapRequest struct {
	Token           *ACLToken // Not client specifiable
	ResetIndex      uint64    // Reset index is used to clear the bootstrap token
	BootstrapSecret string
	WriteRequest
}

// ACLTokenUpsertRequest is used to upsert a set of tokens
type ACLTokenUpsertRequest struct {
	Tokens []*ACLToken
	WriteRequest
}

// ACLTokenUpsertResponse is used to return from an ACLTokenUpsertRequest
type ACLTokenUpsertResponse struct {
	Tokens []*ACLToken
	WriteMeta
}

// OneTimeToken is used to log into the web UI using a token provided by the
// command line.
type OneTimeToken struct {
	OneTimeSecretID string
	AccessorID      string
	ExpiresAt       time.Time
	CreateIndex     uint64
	ModifyIndex     uint64
}

// OneTimeTokenUpsertRequest is the request for a UpsertOneTimeToken RPC
type OneTimeTokenUpsertRequest struct {
	WriteRequest
}

// OneTimeTokenUpsertResponse is the response to a UpsertOneTimeToken RPC.
type OneTimeTokenUpsertResponse struct {
	OneTimeToken *OneTimeToken
	WriteMeta
}

// OneTimeTokenExchangeRequest is a request to swap the one-time token with
// the backing ACL token
type OneTimeTokenExchangeRequest struct {
	OneTimeSecretID string
	WriteRequest
}

// OneTimeTokenExchangeResponse is the response to swapping the one-time token
// with the backing ACL token
type OneTimeTokenExchangeResponse struct {
	Token *ACLToken
	WriteMeta
}

// OneTimeTokenDeleteRequest is a request to delete a group of one-time tokens
type OneTimeTokenDeleteRequest struct {
	AccessorIDs []string
	WriteRequest
}

// OneTimeTokenExpireRequest is a request to delete all expired one-time tokens
type OneTimeTokenExpireRequest struct {
	Timestamp time.Time
	WriteRequest
}

// RpcError is used for serializing errors with a potential error code
type RpcError struct {
	Message string
	Code    *int64
}

func NewRpcError(err error, code *int64) *RpcError {
	return &RpcError{
		Message: err.Error(),
		Code:    code,
	}
}

func (r *RpcError) Error() string {
	return r.Message
}
