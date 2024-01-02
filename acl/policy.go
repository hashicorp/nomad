// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package acl

import (
	"errors"
	"fmt"
	"regexp"

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
	NamespaceCapabilityListScalingPolicies  = "list-scaling-policies"
	NamespaceCapabilityReadScalingPolicy    = "read-scaling-policy"
	NamespaceCapabilityReadJobScaling       = "read-job-scaling"
	NamespaceCapabilityScaleJob             = "scale-job"
	NamespaceCapabilitySubmitRecommendation = "submit-recommendation"
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
	Policy string
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
	case PolicyDeny, PolicyRead, PolicyList:
		return true
	default:
		return false
	}
}

// isNamespaceCapabilityValid ensures the given capability is valid for a namespace policy
func isNamespaceCapabilityValid(cap string) bool {
	switch cap {
	case NamespaceCapabilityDeny, NamespaceCapabilityParseJob, NamespaceCapabilityListJobs, NamespaceCapabilityReadJob,
		NamespaceCapabilitySubmitJob, NamespaceCapabilityDispatchJob, NamespaceCapabilityReadLogs,
		NamespaceCapabilityReadFS, NamespaceCapabilityAllocLifecycle,
		NamespaceCapabilityAllocExec, NamespaceCapabilityAllocNodeExec,
		NamespaceCapabilityCSIReadVolume, NamespaceCapabilityCSIWriteVolume, NamespaceCapabilityCSIListVolume, NamespaceCapabilityCSIMountVolume, NamespaceCapabilityCSIRegisterPlugin,
		NamespaceCapabilityListScalingPolicies, NamespaceCapabilityReadScalingPolicy, NamespaceCapabilityReadJobScaling, NamespaceCapabilityScaleJob:
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
		}
	default:
		return nil
	}
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

// Parse is used to parse the specified ACL rules into an
// intermediary set of policies, before being compiled into
// the ACL
func Parse(rules string) (*Policy, error) {
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

		if ns.Variables != nil {
			if len(ns.Variables.Paths) == 0 {
				return nil, fmt.Errorf("Invalid variable policy: no variable paths in namespace %s", ns.Name)
			}
			for _, pathPolicy := range ns.Variables.Paths {
				if pathPolicy.PathSpec == "" {
					return nil, fmt.Errorf("Invalid missing variable path in namespace %s", ns.Name)
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
	}

	if p.Agent != nil && !isPolicyValid(p.Agent.Policy) {
		return nil, fmt.Errorf("Invalid agent policy: %#v", p.Agent)
	}

	if p.Node != nil && !isPolicyValid(p.Node.Policy) {
		return nil, fmt.Errorf("Invalid node policy: %#v", p.Node)
	}

	if p.Operator != nil && !isPolicyValid(p.Operator.Policy) {
		return nil, fmt.Errorf("Invalid operator policy: %#v", p.Operator)
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
	}

	hvList := list.Filter("host_volume")
	for i, hvObj := range hvList.Items {
		// Fix missing host volume key.
		if len(hvObj.Keys) == 0 {
			p.HostVolumes[i].Name = ""
		}
	}

	return nil
}
