// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package acl

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
)

const (
	// The following levels are the only valid values for the `policy = "read"` block.
	// When policies are merged together, the most privilege is granted, except for deny
	// which always takes precedence and supersedes.
	PolicyDeny  = "deny"
	PolicyRead  = "read"
	PolicyList  = "list"
	PolicyWrite = "write"
	PolicyScale = "scale"
)

const (
	// The following are the fine-grained capabilities that can be granted within a namespace.
	// The Policy field is a short hand for granting several of these. When capabilities are
	// combined we take the union of all capabilities. If the deny capability is present, it
	// takes precedence and overwrites all other capabilities.

	NamespaceCapabilityDeny                 = "deny"
	NamespaceCapabilityListJobs             = "list-jobs"
	NamespaceCapabilityParseJob             = "parse-job"
	NamespaceCapabilityReadJob              = "read-job"
	NamespaceCapabilitySubmitJob            = "submit-job"
	NamespaceCapabilityDispatchJob          = "dispatch-job"
	NamespaceCapabilityReadLogs             = "read-logs"
	NamespaceCapabilityReadFS               = "read-fs"
	NamespaceCapabilityAllocExec            = "alloc-exec"
	NamespaceCapabilityAllocNodeExec        = "alloc-node-exec"
	NamespaceCapabilityAllocLifecycle       = "alloc-lifecycle"
	NamespaceCapabilitySentinelOverride     = "sentinel-override"
	NamespaceCapabilityCSIRegisterPlugin    = "csi-register-plugin"
	NamespaceCapabilityCSIWriteVolume       = "csi-write-volume"
	NamespaceCapabilityCSIReadVolume        = "csi-read-volume"
	NamespaceCapabilityCSIListVolume        = "csi-list-volume"
	NamespaceCapabilityCSIMountVolume       = "csi-mount-volume"
	NamespaceCapabilityHostVolumeCreate     = "host-volume-create"
	NamespaceCapabilityHostVolumeRegister   = "host-volume-register"
	NamespaceCapabilityHostVolumeRead       = "host-volume-read"
	NamespaceCapabilityHostVolumeWrite      = "host-volume-write"
	NamespaceCapabilityHostVolumeDelete     = "host-volume-delete"
	NamespaceCapabilityListScalingPolicies  = "list-scaling-policies"
	NamespaceCapabilityReadScalingPolicy    = "read-scaling-policy"
	NamespaceCapabilityReadJobScaling       = "read-job-scaling"
	NamespaceCapabilityScaleJob             = "scale-job"
	NamespaceCapabilitySubmitRecommendation = "submit-recommendation"

	// Fine-grained job capabilities separated from submit-job
	NamespaceCapabilityRegisterJob   = "register-job"
	NamespaceCapabilityRevertJob     = "revert-job"
	NamespaceCapabilityDeregisterJob = "deregister-job"
	NamespaceCapabilityPurgeJob      = "purge-job"
	NamespaceCapabilityEvaluateJob   = "evaluate-job"
	NamespaceCapabilityPlanJob       = "plan-job"
	NamespaceCapabilityTagJobVersion = "tag-job-version"
	NamespaceCapabilityStableJob     = "stable-job"

	NamespaceCapabilityFailDeployment           = "fail-deployment"
	NamespaceCapabilityPauseDeployment          = "pause-deployment"
	NamespaceCapabilityPromoteDeployment        = "promote-deployment"
	NamespaceCapabilityUnblockDeployment        = "unblock-deployment"
	NamespaceCapabilityCancelDeployment         = "cancel-deployment"
	NamespaceCapabilitySetAllocHealthDeployment = "set-alloc-health-deployment"

	NamespaceCapabilityGCAllocation    = "gc-allocation"
	NamespaceCapabilityPauseAllocation = "pause-allocation"

	NamespaceCapabilityForcePeriodicJob          = "force-periodic-job"
	NamespaceCapabilityDeleteServiceRegistration = "delete-service-registration"
)

var (
	validNamespace = regexp.MustCompile("^[a-zA-Z0-9-*]{1,128}$")
)

const (
	// The following are the fine-grained capabilities that can be granted for
	// node volume management.
	//
	// The Policy field is a short hand for granting several of these. When
	// capabilities are combined we take the union of all capabilities. If the
	// deny capability is present, it takes precedence and overwrites all other
	// capabilities.

	NodePoolCapabilityDelete = "delete"
	NodePoolCapabilityDeny   = "deny"
	NodePoolCapabilityRead   = "read"
	NodePoolCapabilityWrite  = "write"
)

var (
	validNodePool = regexp.MustCompile("^[a-zA-Z0-9-_*]{1,128}$")
)

const (
	// The following are the fine-grained capabilities that can be granted for a volume set.
	// The Policy field is a short hand for granting several of these. When capabilities are
	// combined we take the union of all capabilities. If the deny capability is present, it
	// takes precedence and overwrites all other capabilities.

	HostVolumeCapabilityDeny           = "deny"
	HostVolumeCapabilityMountReadOnly  = "mount-readonly"
	HostVolumeCapabilityMountReadWrite = "mount-readwrite"
)

var (
	validVolume = regexp.MustCompile("^[a-zA-Z0-9-*]{1,128}$")
)

const (
	// The following are the fine-grained capabilities that can be
	// granted for a variables path. When capabilities are
	// combined we take the union of all capabilities.
	VariablesCapabilityList    = "list"
	VariablesCapabilityRead    = "read"
	VariablesCapabilityWrite   = "write"
	VariablesCapabilityDestroy = "destroy"
	VariablesCapabilityDeny    = "deny"
)

const (
	// The following are the fine-grained capabilities that can be granted for
	// operator-level operations. Deny takes precedence and overwrites all other
	// capabilities.
	OperatorCapabilityDeny         = "deny"
	OperatorCapabilitySnapshotSave = "snapshot-save"
	OperatorCapabilityLicenseRead  = "license-read"
)

// Policy represents a parsed HCL or JSON policy.
type Policy struct {
	Namespaces  []*NamespacePolicy  `hcl:"namespace,expand"`
	NodePools   []*NodePoolPolicy   `hcl:"node_pool,expand"`
	HostVolumes []*HostVolumePolicy `hcl:"host_volume,expand"`
	Agent       *AgentPolicy        `hcl:"agent"`
	Node        *NodePolicy         `hcl:"node"`
	Operator    *OperatorPolicy     `hcl:"operator"`
	Quota       *QuotaPolicy        `hcl:"quota"`
	Plugin      *PluginPolicy       `hcl:"plugin"`
	Raw         string              `hcl:"-"`

	// ExtraKeysHCL is used to capture any extra keys in the HCL input, so we
	// can return an error if the user specified something unknown.
	//
	// Unfortunately, due to our current HCL use, keys from known blocks
	// (namespace, node pools, and host volumes) will appear here, so we need to
	// remove those as we process them. If the policy contains multiple blocks
	// of the same type (e.g. multiple namespace blocks), the extra keys will
	// also include "namespace" for all but the first block, so we need to
	// remove those as we process them too.
	ExtraKeysHCL []string `hcl:",unusedKeys"`
}

// IsEmpty checks to make sure that at least one policy has been set and is not
// comprised of only a raw policy.
func (p *Policy) IsEmpty() bool {
	return len(p.Namespaces) == 0 &&
		len(p.NodePools) == 0 &&
		len(p.HostVolumes) == 0 &&
		p.Agent == nil &&
		p.Node == nil &&
		p.Operator == nil &&
		p.Quota == nil &&
		p.Plugin == nil
}

// removeExtraKey removes a single occurrence of the passed key from the
// ExtraKeysHCL slice. If the key is not found, this is a no-op.
func (p *Policy) removeExtraKey(key string) {
	if idx := slices.Index(p.ExtraKeysHCL, key); idx > -1 {
		p.ExtraKeysHCL = append(p.ExtraKeysHCL[:idx], p.ExtraKeysHCL[idx+1:]...)
	}
}

// NamespacePolicy is the policy for a specific namespace
type NamespacePolicy struct {
	Name         string `hcl:",key"`
	Policy       string
	Capabilities []string
	Variables    *VariablesPolicy `hcl:"variables"`
}

// NodePoolPolicy is the policfy for a specific node pool.
type NodePoolPolicy struct {
	Name         string `hcl:",key"`
	Policy       string
	Capabilities []string
}

type VariablesPolicy struct {
	Paths []*VariablesPathPolicy `hcl:"path"`
}

type VariablesPathPolicy struct {
	PathSpec     string `hcl:",key"`
	Capabilities []string
}

// HostVolumePolicy is the policy for a specific named host volume
type HostVolumePolicy struct {
	Name         string `hcl:",key"`
	Policy       string
	Capabilities []string
}

type AgentPolicy struct {
	Policy string
}

type NodePolicy struct {
	Policy string
}

type OperatorPolicy struct {
	Policy       string
	Capabilities []string
}

type QuotaPolicy struct {
	Policy string
}

type PluginPolicy struct {
	Policy string
}

// isPolicyValid makes sure the given string matches one of the valid policies.
func isPolicyValid(policy string) bool {
	switch policy {
	case PolicyDeny, PolicyRead, PolicyWrite, PolicyScale:
		return true
	default:
		return false
	}
}

func (p *PluginPolicy) isValid() bool {
	switch p.Policy {
	case PolicyDeny, PolicyRead, PolicyList, PolicyWrite:
		return true
	default:
		return false
	}
}

// isNamespaceCapabilityValid ensures the given capability is valid for a namespace policy
func isNamespaceCapabilityValid(cap string) bool {
	switch cap {
	case NamespaceCapabilityDeny, NamespaceCapabilityListJobs, NamespaceCapabilityParseJob,
		NamespaceCapabilityReadJob, NamespaceCapabilitySubmitJob, NamespaceCapabilityDispatchJob,
		NamespaceCapabilityReadLogs, NamespaceCapabilityReadFS, NamespaceCapabilityAllocExec,
		NamespaceCapabilityAllocNodeExec, NamespaceCapabilityAllocLifecycle,
		NamespaceCapabilityCSIRegisterPlugin, NamespaceCapabilityCSIWriteVolume,
		NamespaceCapabilityCSIReadVolume, NamespaceCapabilityCSIListVolume,
		NamespaceCapabilityCSIMountVolume, NamespaceCapabilityHostVolumeCreate,
		NamespaceCapabilityHostVolumeRegister, NamespaceCapabilityHostVolumeRead,
		NamespaceCapabilityHostVolumeWrite, NamespaceCapabilityHostVolumeDelete,
		NamespaceCapabilityListScalingPolicies, NamespaceCapabilityReadScalingPolicy,
		NamespaceCapabilityReadJobScaling, NamespaceCapabilityScaleJob,
		NamespaceCapabilityRegisterJob, NamespaceCapabilityRevertJob,
		NamespaceCapabilityDeregisterJob, NamespaceCapabilityPurgeJob,
		NamespaceCapabilityEvaluateJob, NamespaceCapabilityPlanJob,
		NamespaceCapabilityTagJobVersion, NamespaceCapabilityStableJob,
		NamespaceCapabilityFailDeployment, NamespaceCapabilityPauseDeployment,
		NamespaceCapabilityPromoteDeployment, NamespaceCapabilityUnblockDeployment,
		NamespaceCapabilityCancelDeployment, NamespaceCapabilitySetAllocHealthDeployment,
		NamespaceCapabilityGCAllocation, NamespaceCapabilityPauseAllocation,
		NamespaceCapabilityForcePeriodicJob, NamespaceCapabilityDeleteServiceRegistration:
		return true
	// Separate the enterprise-only capabilities
	case NamespaceCapabilitySentinelOverride, NamespaceCapabilitySubmitRecommendation:
		return true
	default:
		return false
	}
}

// isPathCapabilityValid ensures the given capability is valid for a
// variables path policy
func isPathCapabilityValid(cap string) bool {
	switch cap {
	case VariablesCapabilityWrite, VariablesCapabilityRead,
		VariablesCapabilityList, VariablesCapabilityDestroy, VariablesCapabilityDeny:
		return true
	default:
		return false
	}
}

// expandNamespacePolicy provides the equivalent set of capabilities for
// a namespace policy
func expandNamespacePolicy(policy string) []string {
	read := []string{
		NamespaceCapabilityListJobs,
		NamespaceCapabilityParseJob,
		NamespaceCapabilityReadJob,
		NamespaceCapabilityCSIListVolume,
		NamespaceCapabilityCSIReadVolume,
		NamespaceCapabilityReadJobScaling,
		NamespaceCapabilityListScalingPolicies,
		NamespaceCapabilityReadScalingPolicy,
		NamespaceCapabilityHostVolumeRead,
	}

	write := make([]string, len(read))
	copy(write, read)

	write = append(write, []string{
		NamespaceCapabilityScaleJob,
		NamespaceCapabilitySubmitJob,
		NamespaceCapabilityDispatchJob,
		NamespaceCapabilityReadLogs,
		NamespaceCapabilityReadFS,
		NamespaceCapabilityAllocExec,
		NamespaceCapabilityAllocLifecycle,
		NamespaceCapabilityCSIMountVolume,
		NamespaceCapabilityCSIWriteVolume,
		NamespaceCapabilitySubmitRecommendation,
		NamespaceCapabilityHostVolumeCreate,
	}...)

	switch policy {
	case PolicyDeny:
		return []string{NamespaceCapabilityDeny}
	case PolicyRead:
		return read
	case PolicyWrite:
		return write
	case PolicyScale:
		return []string{
			NamespaceCapabilityListScalingPolicies,
			NamespaceCapabilityReadScalingPolicy,
			NamespaceCapabilityReadJobScaling,
			NamespaceCapabilityScaleJob,
			NamespaceCapabilityReadJob,
			NamespaceCapabilitySubmitRecommendation,
		}
	default:
		return nil
	}
}

// expandNamespaceCapabilities adds extra capabilities implied by fine-grained
// capabilities.
func expandNamespaceCapabilities(ns *NamespacePolicy) {
	extraCaps := []string{}
	for _, cap := range ns.Capabilities {
		switch cap {
		case NamespaceCapabilityHostVolumeWrite:
			extraCaps = append(extraCaps,
				NamespaceCapabilityHostVolumeRegister,
				NamespaceCapabilityHostVolumeCreate,
				NamespaceCapabilityHostVolumeDelete,
				NamespaceCapabilityHostVolumeRead)
		case NamespaceCapabilityHostVolumeRegister:
			extraCaps = append(extraCaps,
				NamespaceCapabilityHostVolumeCreate,
				NamespaceCapabilityHostVolumeRead)
		case NamespaceCapabilityHostVolumeCreate:
			extraCaps = append(extraCaps, NamespaceCapabilityHostVolumeRead)
		}
	}

	// These may end up being duplicated, but they'll get deduplicated in NewACL
	// when inserted into the radix tree.
	ns.Capabilities = append(ns.Capabilities, extraCaps...)
}

func isNodePoolCapabilityValid(cap string) bool {
	switch cap {
	case NodePoolCapabilityDelete, NodePoolCapabilityRead, NodePoolCapabilityWrite,
		NodePoolCapabilityDeny:
		return true
	default:
		return false
	}
}

// isOperatorCapabilityValid ensures the given capability is valid for an operator policy
func isOperatorCapabilityValid(cap string) bool {
	switch cap {
	case OperatorCapabilityDeny, OperatorCapabilitySnapshotSave, OperatorCapabilityLicenseRead:
		return true
	default:
		return false
	}
}

func expandNodePoolPolicy(policy string) []string {
	switch policy {
	case PolicyDeny:
		return []string{NodePoolCapabilityDeny}
	case PolicyRead:
		return []string{NodePoolCapabilityRead}
	case PolicyWrite:
		return []string{
			NodePoolCapabilityDelete,
			NodePoolCapabilityRead,
			NodePoolCapabilityWrite,
		}
	default:
		return nil
	}
}

// expandOperatorPolicy provides the equivalent set of capabilities for
// an operator policy
func expandOperatorPolicy(policy string) []string {
	switch policy {
	case PolicyDeny:
		return []string{OperatorCapabilityDeny}
	case PolicyRead:
		return []string{OperatorCapabilityLicenseRead}
	case PolicyWrite:
		return []string{OperatorCapabilitySnapshotSave, OperatorCapabilityLicenseRead}
	default:
		return nil
	}
}

func isHostVolumeCapabilityValid(cap string) bool {
	switch cap {
	case HostVolumeCapabilityDeny, HostVolumeCapabilityMountReadOnly, HostVolumeCapabilityMountReadWrite:
		return true
	default:
		return false
	}
}

func expandHostVolumePolicy(policy string) []string {
	switch policy {
	case PolicyDeny:
		return []string{HostVolumeCapabilityDeny}
	case PolicyRead:
		return []string{HostVolumeCapabilityMountReadOnly}
	case PolicyWrite:
		return []string{HostVolumeCapabilityMountReadOnly, HostVolumeCapabilityMountReadWrite}
	default:
		return nil
	}
}

func expandVariablesCapabilities(caps []string) []string {
	var foundRead, foundList bool
	for _, cap := range caps {
		switch cap {
		case VariablesCapabilityDeny:
			return []string{VariablesCapabilityDeny}
		case VariablesCapabilityRead:
			foundRead = true
		case VariablesCapabilityList:
			foundList = true
		}
	}
	if foundRead && !foundList {
		caps = append(caps, PolicyList)
	}
	return caps
}

const (
	// PolicyParseStrict can be used to indicate that the policy should be
	// parsed in strict mode, returning an error if there are any unknown keys.
	// This should be used when creating or updating policies.
	PolicyParseStrict = true

	// PolicyParseLenient can be used to indicate that the policy should be
	// parsed in lenient mode, ignoring any unknown keys. This should be used
	// when evaluating policies, so we gracefully handle policies that were
	// created before we added stricter validation.
	PolicyParseLenient = false
)

// Parse is used to parse the specified ACL rules into an intermediary set of
// policies, before being compiled into the ACL.
//
// The "strict" parameter should be set to true if the policy is being created
// or updated, and false if it is being used for evaluation. This allowed us to
// tighten restrictions around unknown keys when writing policies, while not
// breaking existing policies that may have unknown keys when evaluating them,
// since they may have been written before the restrictions were added. The
// constants PolicyParseStrict and PolicyParseLenient can be used to make the
// intent clear at the call site.
func Parse(rules string, strict bool) (*Policy, error) {
	// Decode the rules
	p := &Policy{Raw: rules}
	if rules == "" {
		// Hot path for empty rules
		return p, nil
	}

	// Attempt to parse
	if err := hclDecode(p, rules); err != nil {
		return nil, fmt.Errorf("Failed to parse ACL Policy: %v", err)
	}

	// At least one valid policy must be specified, we don't want to store only
	// raw data
	if p.IsEmpty() {
		return nil, fmt.Errorf("Invalid policy: %s", p.Raw)
	}

	// Validate the policy
	for _, ns := range p.Namespaces {
		if !validNamespace.MatchString(ns.Name) {
			return nil, fmt.Errorf("Invalid namespace name: %#v", ns)
		}
		if ns.Policy != "" && !isPolicyValid(ns.Policy) {
			return nil, fmt.Errorf("Invalid namespace policy: %#v", ns)
		}
		for _, cap := range ns.Capabilities {
			if !isNamespaceCapabilityValid(cap) {
				return nil, fmt.Errorf("Invalid namespace capability '%s': %#v", cap, ns)
			}
		}

		// Expand the short hand policy to the capabilities and
		// add to any existing capabilities
		if ns.Policy != "" {
			extraCap := expandNamespacePolicy(ns.Policy)
			ns.Capabilities = append(ns.Capabilities, extraCap...)
		}

		// Expand implicit capabilities
		expandNamespaceCapabilities(ns)

		if ns.Variables != nil {
			if len(ns.Variables.Paths) == 0 {
				return nil, fmt.Errorf("Invalid variable policy: no variable paths in namespace %s", ns.Name)
			}
			for _, pathPolicy := range ns.Variables.Paths {
				if pathPolicy.PathSpec == "" {
					return nil, fmt.Errorf("Invalid missing variable path in namespace %s", ns.Name)
				}
				if strings.HasPrefix(pathPolicy.PathSpec, "/") {
					return nil, fmt.Errorf(
						"Invalid variable path %q in namespace %s: cannot start with a leading '/'`",
						pathPolicy.PathSpec, ns.Name)
				}
				for _, cap := range pathPolicy.Capabilities {
					if !isPathCapabilityValid(cap) {
						return nil, fmt.Errorf(
							"Invalid variable capability '%s' in namespace %s", cap, ns.Name)
					}
				}
				pathPolicy.Capabilities = expandVariablesCapabilities(pathPolicy.Capabilities)

			}
		}

		// Remove the namespace name from the extra key list.
		p.removeExtraKey(ns.Name)
	}

	for _, np := range p.NodePools {
		if !validNodePool.MatchString(np.Name) {
			return nil, fmt.Errorf("Invalid node pool name '%s'", np.Name)
		}
		if np.Policy != "" && !isPolicyValid(np.Policy) {
			return nil, fmt.Errorf("Invalid node pool policy '%s' for '%s'", np.Policy, np.Name)
		}
		for _, cap := range np.Capabilities {
			if !isNodePoolCapabilityValid(cap) {
				return nil, fmt.Errorf("Invalid node pool capability '%s' for '%s'", cap, np.Name)
			}
		}

		if np.Policy != "" {
			extraCap := expandNodePoolPolicy(np.Policy)
			np.Capabilities = append(np.Capabilities, extraCap...)
		}

		// Remove the node-pool name from the extra key list.
		p.removeExtraKey(np.Name)
	}

	for _, hv := range p.HostVolumes {
		if !validVolume.MatchString(hv.Name) {
			return nil, fmt.Errorf("Invalid host volume name: %#v", hv)
		}
		if hv.Policy != "" && !isPolicyValid(hv.Policy) {
			return nil, fmt.Errorf("Invalid host volume policy: %#v", hv)
		}
		for _, cap := range hv.Capabilities {
			if !isHostVolumeCapabilityValid(cap) {
				return nil, fmt.Errorf("Invalid host volume capability '%s': %#v", cap, hv)
			}
		}

		// Expand the short hand policy to the capabilities and
		// add to any existing capabilities
		if hv.Policy != "" {
			extraCap := expandHostVolumePolicy(hv.Policy)
			hv.Capabilities = append(hv.Capabilities, extraCap...)
		}

		// Remove the host-volume name from the extra key list.
		p.removeExtraKey(hv.Name)
	}

	// Now that we have processed all known keys, return an error if the
	// operator wrote a policy with unknown keys if we are being strict. While
	// these do not grant any extra privileges, it can be misleaing to allow
	// these and cause problems later if we add new capabilities that collide
	// with the unknown keys.
	if len(p.ExtraKeysHCL) > 0 && strict {
		return nil, fmt.Errorf("Invalid or duplicate policy keys: %v",
			strings.Join(p.ExtraKeysHCL, ", "))
	}

	p.ExtraKeysHCL = nil

	if p.Agent != nil && !isPolicyValid(p.Agent.Policy) {
		return nil, fmt.Errorf("Invalid agent policy: %#v", p.Agent)
	}

	if p.Node != nil && !isPolicyValid(p.Node.Policy) {
		return nil, fmt.Errorf("Invalid node policy: %#v", p.Node)
	}

	if p.Operator != nil {
		if p.Operator.Policy != "" && !isPolicyValid(p.Operator.Policy) {
			return nil, fmt.Errorf("Invalid operator policy: %#v", p.Operator)
		}
		for _, cap := range p.Operator.Capabilities {
			if !isOperatorCapabilityValid(cap) {
				return nil, fmt.Errorf("Invalid operator capability '%s'", cap)
			}
		}

		// Expand the short hand policy to the capabilities and
		// add to any existing capabilities
		if p.Operator.Policy != "" {
			extraCap := expandOperatorPolicy(p.Operator.Policy)
			p.Operator.Capabilities = append(p.Operator.Capabilities, extraCap...)
		}
	}

	if p.Quota != nil && !isPolicyValid(p.Quota.Policy) {
		return nil, fmt.Errorf("Invalid quota policy: %#v", p.Quota)
	}

	if p.Plugin != nil && !p.Plugin.isValid() {
		return nil, fmt.Errorf("Invalid plugin policy: %#v", p.Plugin)
	}
	return p, nil
}

// hclDecode wraps hcl.Decode function but handles any unexpected panics
func hclDecode(p *Policy, rules string) (err error) {
	defer func() {
		if rerr := recover(); rerr != nil {
			err = fmt.Errorf("invalid acl policy: %v", rerr)
		}
	}()

	if err = hcl.Decode(p, rules); err != nil {
		return err
	}

	// Manually parse the policy to fix blocks without labels.
	//
	// Due to a bug in the way HCL decodes files, a block without a label may
	// return an incorrect key value and make it impossible to determine if the
	// key was set by the user or incorrectly set by the decoder.
	//
	// By manually parsing the file we are able to determine if the label is
	// missing in the file and set them to an empty string so the policy
	// validation can return the appropriate errors.
	root, err := hcl.Parse(rules)
	if err != nil {
		return fmt.Errorf("failed to parse policy: %w", err)
	}

	list, ok := root.Node.(*ast.ObjectList)
	if !ok {
		return errors.New("error parsing: root should be an object")
	}

	nsList := list.Filter("namespace")
	for i, nsObj := range nsList.Items {
		// Fix missing namespace key.
		if len(nsObj.Keys) == 0 {
			p.Namespaces[i].Name = ""
		}
		if i > 0 {
			p.removeExtraKey("namespace")
		}

		// Fix missing variable paths.
		nsOT, ok := nsObj.Val.(*ast.ObjectType)
		if !ok {
			continue
		}
		varsList := nsOT.List.Filter("variables")
		if varsList == nil || len(varsList.Items) == 0 {
			continue
		}

		varsObj, ok := varsList.Items[0].Val.(*ast.ObjectType)
		if !ok {
			continue
		}
		paths := varsObj.List.Filter("path")
		for j, path := range paths.Items {
			if len(path.Keys) == 0 {
				p.Namespaces[i].Variables.Paths[j].PathSpec = ""
			}
		}
	}

	npList := list.Filter("node_pool")
	for i, npObj := range npList.Items {
		// Fix missing node pool key.
		if len(npObj.Keys) == 0 {
			p.NodePools[i].Name = ""
		}
		if i > 0 {
			p.removeExtraKey("node_pool")
		}
	}

	hvList := list.Filter("host_volume")
	for i, hvObj := range hvList.Items {
		// Fix missing host volume key.
		if len(hvObj.Keys) == 0 {
			p.HostVolumes[i].Name = ""
		}
		if i > 0 {
			p.removeExtraKey("host_volume")
		}
	}

	return nil
}
