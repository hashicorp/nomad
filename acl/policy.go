// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package acl

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/json"
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
	// The Policy block is a short hand for granting several of these. When capabilities are
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
	// The following are the fine-grained capabilities that can be granted for a volume set.
	// The Policy block is a short hand for granting several of these. When capabilities are
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
	Namespaces  []*NamespacePolicy  `hcl:"namespace,block"`
	HostVolumes []*HostVolumePolicy `hcl:"host_volume,block"`
	Agent       *AgentPolicy        `hcl:"agent,block"`
	Node        *NodePolicy         `hcl:"node,block"`
	Operator    *OperatorPolicy     `hcl:"operator,block"`
	Quota       *QuotaPolicy        `hcl:"quota,block"`
	Plugin      *PluginPolicy       `hcl:"plugin,block"`
	Raw         string
}

// IsEmpty checks to make sure that at least one policy has been set and is not
// comprised of only a raw policy.
func (p *Policy) IsEmpty() bool {
	return len(p.Namespaces) == 0 &&
		len(p.HostVolumes) == 0 &&
		p.Agent == nil &&
		p.Node == nil &&
		p.Operator == nil &&
		p.Quota == nil &&
		p.Plugin == nil
}

// NamespacePolicy is the policy for a specific namespace
type NamespacePolicy struct {
	Name         string           `hcl:"name,label"`
	Policy       string           `hcl:"policy,optional"`
	Capabilities []string         `hcl:"capabilities,optional"`
	Variables    *VariablesPolicy `hcl:"variables,block"`
}

type VariablesPolicy struct {
	Paths []*VariablesPathPolicy `hcl:"path,block"`
}

type VariablesPathPolicy struct {
	PathSpec     string   `hcl:"name,label"`
	Capabilities []string `hcl:"capabilities,optional"`
}

// HostVolumePolicy is the policy for a specific named host volume
type HostVolumePolicy struct {
	Name         string   `hcl:"name,label"`
	Policy       string   `hcl:"policy,optional"`
	Capabilities []string `hcl:"capabilities,optional"`
}

type AgentPolicy struct {
	Policy string `hcl:"policy"`
}

type NodePolicy struct {
	Policy string `hcl:"policy"`
}

type OperatorPolicy struct {
	Policy string `hcl:"policy"`
}

type QuotaPolicy struct {
	Policy string `hcl:"policy"`
}

type PluginPolicy struct {
	Policy string `hcl:"policy"`
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
func Parse(name string, rules string) (*Policy, error) {
	// Decode the rules
	p := &Policy{Raw: rules}
	if rules == "" {
		// Hot path for empty rules
		return p, nil
	}

	// Attempt to parse
	if err := hclDecode(name, p, rules); err != nil {
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
func hclDecode(name string, p *Policy, rules string) (err error) {
	defer func() {
		if rerr := recover(); rerr != nil {
			err = fmt.Errorf("invalid acl policy: %v", rerr)
		}
	}()

	var file *hcl.File
	var diags hcl.Diagnostics

	trimmed := strings.TrimSpace(rules)
	if strings.HasPrefix(trimmed, "{") {
		file, diags = json.Parse([]byte(rules), name)
	} else {
		file, diags = hclsyntax.ParseConfig([]byte(rules), name, hcl.Pos{Line: 1, Column: 1})
	}
	if diags.HasErrors() {
		return diags
	}

	diags = gohcl.DecodeBody(file.Body, nil, p)
	if diags.HasErrors() {
		return diags
	}
	return nil
}
