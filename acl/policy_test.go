// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package acl

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	ci.Parallel(t)

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
							NamespaceCapabilityParseJob,
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
			namespace "apps" {
				variables {
					path "jobs/write-does-not-imply-read-or-delete" {
						capabilities = ["write"]
					}
					path "project/read-implies-list" {
						capabilities = ["read"]
					}
					path "project/explicit" {
						capabilities = ["read", "list", "destroy"]
					}
				}
			}
			namespace "autoscaler" {
				policy = "scale"
			}
			host_volume "production-tls-*" {
				capabilities = ["mount-readonly"]
			}
			host_volume "staging-tls-*" {
				policy = "write"
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
							NamespaceCapabilityParseJob,
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
							NamespaceCapabilityParseJob,
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
							NamespaceCapabilitySubmitRecommendation,
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
						Name: "apps",
						Variables: &VariablesPolicy{
							Paths: []*VariablesPathPolicy{
								{
									PathSpec:     "jobs/write-does-not-imply-read-or-delete",
									Capabilities: []string{VariablesCapabilityWrite},
								},
								{
									PathSpec: "project/read-implies-list",
									Capabilities: []string{
										VariablesCapabilityRead,
										VariablesCapabilityList,
									},
								},
								{
									PathSpec: "project/explicit",
									Capabilities: []string{
										VariablesCapabilityRead,
										VariablesCapabilityList,
										VariablesCapabilityDestroy,
									},
								},
							},
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
				HostVolumes: []*HostVolumePolicy{
					{
						Name:         "production-tls-*",
						Capabilities: []string{"mount-readonly"},
					},
					{
						Name:   "staging-tls-*",
						Policy: "write",
						Capabilities: []string{
							"mount-readonly",
							"mount-readwrite",
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
			{
				"namespace": [
					{
						"default": {
							"policy": "read"
						},
					},
					{
						"other": {
							"policy": "write"
						},
					},
					{
						"secret": {
							"capabilities": [
								"deny",
								"read-logs"
							]
						}
					},
					{
						"apps": {
							"variables": [
								{
									"path": [
										{
											"jobs/write-does-not-imply-read-or-delete": {
												"capabilities": ["write"],
											},
										},
										{
											"project/read-implies-list": {
												"capabilities": ["read"],
											},
										},
										{
											"project/explicit": {
												"capabilities": ["read", "list", "destroy"],
											},
										},
									],
								},
							],
						},
					},
					{
						"autoscaler": {
							"policy": "scale"
						},
					},
				],
				"host_volume": [
					{
						"production-tls-*": {
							"capabilities": ["mount-readonly"]
						}
					},
					{
						"staging-tls-*": {
							"policy": "write"
						}
					}
				],
				"agent": {
					"policy": "read"
				},
				"node": {
					"policy": "write"
				},
				"operator": {
					"policy": "deny"
				},
				"quota": {
					"policy": "read"
				},
				"plugin": {
					"policy": "read"
				}
			}`,
			"",
			&Policy{
				Namespaces: []*NamespacePolicy{
					{
						Name:   "default",
						Policy: PolicyRead,
						Capabilities: []string{
							NamespaceCapabilityListJobs,
							NamespaceCapabilityParseJob,
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
							NamespaceCapabilityParseJob,
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
							NamespaceCapabilitySubmitRecommendation,
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
						Name: "apps",
						Variables: &VariablesPolicy{
							Paths: []*VariablesPathPolicy{
								{
									PathSpec:     "jobs/write-does-not-imply-read-or-delete",
									Capabilities: []string{VariablesCapabilityWrite},
								},
								{
									PathSpec: "project/read-implies-list",
									Capabilities: []string{
										VariablesCapabilityRead,
										VariablesCapabilityList,
									},
								},
								{
									PathSpec: "project/explicit",
									Capabilities: []string{
										VariablesCapabilityRead,
										VariablesCapabilityList,
										VariablesCapabilityDestroy,
									},
								},
							},
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
				HostVolumes: []*HostVolumePolicy{
					{
						Name:         "production-tls-*",
						Capabilities: []string{"mount-readonly"},
					},
					{
						Name:   "staging-tls-*",
						Policy: "write",
						Capabilities: []string{
							"mount-readonly",
							"mount-readwrite",
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
			namespace {
				policy = "read"
			}
			`,
			"Invalid namespace name",
			nil,
		},
		{
			`
			{
				"namespace": [
					{
						"": {
							"policy": "read"
						}
					}
				]
			}
			`,
			"Invalid namespace name",
			nil,
		},
		{
			`
			namespace "dev" {
			  variables "*" {
			      capabilities = ["read", "write"]
			  }
			}
			`,
			"Invalid variable policy: no variable paths in namespace dev",
			nil,
		},
		{
			`
			namespace "dev" {
				policy = "read"

				variables {
					path {}
					path "nomad/jobs/example" {
						capabilities = ["read"]
					}
				}
			}
			`,
			"Invalid missing variable path in namespace",
			nil,
		},
		{
			`
			{
				"namespace": [
					{
						"dev": {
							"policy": "read",
							"variables": [
								{
									"paths": [
										{
											"": {
												"capabilities": ["read"]
											}
										}
									]
								]
							]
						}
					}
				]
			}
			`,
			"no variable paths in namespace dev",
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
			`namespace {}`,
			"invalid acl policy",
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
			host_volume {
				policy = "read"
			}
			`,
			"Invalid host volume name",
			nil,
		},
		{
			`
			{
				"host_volume": [
					{
						"": {
							"policy": "read"
						}
					}
				]
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
	ci.Parallel(t)

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
