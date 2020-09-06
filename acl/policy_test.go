package acl

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	type tcase struct {
		Raw    string
		ErrStr string
		Expect *Policy
	}
	tcases := []tcase{
		{
			`
			namespace "default" {
				policy = "read"
			}
			`,
			"",
			&Policy{
				Namespaces: []*NamespacePolicy{
					{
						Name:   "default",
						Policy: PolicyRead,
						Capabilities: []string{
							NamespaceCapabilityListJobs,
							NamespaceCapabilityReadJob,
							NamespaceCapabilityCSIListVolume,
							NamespaceCapabilityCSIReadVolume,
							NamespaceCapabilityReadJobScaling,
							NamespaceCapabilityListScalingPolicies,
							NamespaceCapabilityReadScalingPolicy,
						},
					},
				},
			},
		},
		{
			`
			namespace "default" {
				policy = "read"
			}
			namespace "other" {
				policy = "write"
			}
			namespace "secret" {
				capabilities = ["deny", "read-logs"]
			}
			namespace "autoscaler" {
				policy = "scale"
			}
			agent {
				policy = "read"
			}
			node {
				policy = "write"
			}
			operator {
				policy = "deny"
			}
			quota {
				policy = "read"
			}
			plugin {
				policy = "read"
			}
			`,
			"",
			&Policy{
				Namespaces: []*NamespacePolicy{
					{
						Name:   "default",
						Policy: PolicyRead,
						Capabilities: []string{
							NamespaceCapabilityListJobs,
							NamespaceCapabilityReadJob,
							NamespaceCapabilityCSIListVolume,
							NamespaceCapabilityCSIReadVolume,
							NamespaceCapabilityReadJobScaling,
							NamespaceCapabilityListScalingPolicies,
							NamespaceCapabilityReadScalingPolicy,
						},
					},
					{
						Name:   "other",
						Policy: PolicyWrite,
						Capabilities: []string{
							NamespaceCapabilityListJobs,
							NamespaceCapabilityReadJob,
							NamespaceCapabilityCSIListVolume,
							NamespaceCapabilityCSIReadVolume,
							NamespaceCapabilityReadJobScaling,
							NamespaceCapabilityListScalingPolicies,
							NamespaceCapabilityReadScalingPolicy,
							NamespaceCapabilityScaleJob,
							NamespaceCapabilitySubmitJob,
							NamespaceCapabilityDispatchJob,
							NamespaceCapabilityReadLogs,
							NamespaceCapabilityReadFS,
							NamespaceCapabilityAllocExec,
							NamespaceCapabilityAllocLifecycle,
							NamespaceCapabilityCSIMountVolume,
							NamespaceCapabilityCSIWriteVolume,
						},
					},
					{
						Name: "secret",
						Capabilities: []string{
							NamespaceCapabilityDeny,
							NamespaceCapabilityReadLogs,
						},
					},
					{
						Name:   "autoscaler",
						Policy: PolicyScale,
						Capabilities: []string{
							NamespaceCapabilityListScalingPolicies,
							NamespaceCapabilityReadScalingPolicy,
							NamespaceCapabilityReadJobScaling,
							NamespaceCapabilityScaleJob,
						},
					},
				},
				Agent: &AgentPolicy{
					Policy: PolicyRead,
				},
				Node: &NodePolicy{
					Policy: PolicyWrite,
				},
				Operator: &OperatorPolicy{
					Policy: PolicyDeny,
				},
				Quota: &QuotaPolicy{
					Policy: PolicyRead,
				},
				Plugin: &PluginPolicy{
					Policy: PolicyRead,
				},
			},
		},
		{
			`
			namespace "default" {
				policy = "foo"
			}
			`,
			"Invalid namespace policy",
			nil,
		},
		{
			`
			namespace "default" {
				capabilities = ["deny", "foo"]
			}
			`,
			"Invalid namespace capability",
			nil,
		},
		{
			`
			agent {
				policy = "foo"
			}
			`,
			"Invalid agent policy",
			nil,
		},
		{
			`
			node {
				policy = "foo"
			}
			`,
			"Invalid node policy",
			nil,
		},
		{
			`
			operator {
				policy = "foo"
			}
			`,
			"Invalid operator policy",
			nil,
		},
		{
			`
			quota {
				policy = "foo"
			}
			`,
			"Invalid quota policy",
			nil,
		},
		{
			`
			{
				"Name": "my-policy",
				"Description": "This is a great policy",
				"Rules": "anything"
			}
			`,
			"Invalid policy",
			nil,
		},
		{
			`
			namespace "has a space"{
				policy = "read"
			}
			`,
			"Invalid namespace name",
			nil,
		},
		{
			`
			namespace "default" {
				capabilities = ["sentinel-override"]
			}
			`,
			"",
			&Policy{
				Namespaces: []*NamespacePolicy{
					{
						Name:   "default",
						Policy: "",
						Capabilities: []string{
							NamespaceCapabilitySentinelOverride,
						},
					},
				},
			},
		},
		{
			`
			host_volume "production-tls-*" {
				capabilities = ["mount-readonly"]
			}
			`,
			"",
			&Policy{
				HostVolumes: []*HostVolumePolicy{
					{
						Name:   "production-tls-*",
						Policy: "",
						Capabilities: []string{
							HostVolumeCapabilityMountReadOnly,
						},
					},
				},
			},
		},
		{
			`
			host_volume "production-tls-*" {
				capabilities = ["mount-readwrite"]
			}
			`,
			"",
			&Policy{
				HostVolumes: []*HostVolumePolicy{
					{
						Name:   "production-tls-*",
						Policy: "",
						Capabilities: []string{
							HostVolumeCapabilityMountReadWrite,
						},
					},
				},
			},
		},
		{
			`
			host_volume "volume has a space" {
				capabilities = ["mount-readwrite"]
			}
			`,
			"Invalid host volume name",
			nil,
		},
		{
			`
			plugin {
				policy = "list"
			}
			`,
			"",
			&Policy{
				Plugin: &PluginPolicy{
					Policy: PolicyList,
				},
			},
		},
		{
			`
			plugin {
				policy = "reader"
			}
			`,
			"Invalid plugin policy",
			nil,
		},
	}

	for idx, tc := range tcases {
		t.Run(fmt.Sprintf("%d", idx), func(t *testing.T) {
			p, err := Parse(tc.Raw)
			if err != nil {
				if tc.ErrStr == "" {
					t.Fatalf("Unexpected err: %v", err)
				}
				if !strings.Contains(err.Error(), tc.ErrStr) {
					t.Fatalf("Unexpected err: %v", err)
				}
				return
			}
			if err == nil && tc.ErrStr != "" {
				t.Fatalf("Missing expected err")
			}
			tc.Expect.Raw = tc.Raw
			assert.EqualValues(t, tc.Expect, p)
		})
	}
}

func TestParse_BadInput(t *testing.T) {
	inputs := []string{
		`namespace "\500" {}`,
	}

	for i, c := range inputs {
		t.Run(fmt.Sprintf("%d: %v", i, c), func(t *testing.T) {
			_, err := Parse(c)
			assert.Error(t, err)
		})
	}
}
