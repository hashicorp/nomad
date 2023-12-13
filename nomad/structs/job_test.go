// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"

	"github.com/hashicorp/go-set"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func TestServiceRegistrationsRequest_StaleReadSupport(t *testing.T) {
	req := &AllocServiceRegistrationsRequest{}
	require.True(t, req.IsRead())
}

func TestJob_RequiresNativeServiceDiscovery(t *testing.T) {
	testCases := []struct {
		name      string
		inputJob  *Job
		expBasic  []string
		expChecks []string
	}{
		{
			name: "multiple group services with Nomad provider",
			inputJob: &Job{
				TaskGroups: []*TaskGroup{
					{
						Name: "group1",
						Services: []*Service{
							{Provider: "nomad"},
							{Provider: "nomad"},
						},
					},
					{
						Name: "group2",
						Services: []*Service{
							{Provider: "nomad"},
							{Provider: "nomad"},
						},
					},
				},
			},
			expBasic:  []string{"group1", "group2"},
			expChecks: nil,
		},
		{
			name: "multiple group services with Nomad provider with checks",
			inputJob: &Job{
				TaskGroups: []*TaskGroup{
					{
						Name: "group1",
						Services: []*Service{
							{Provider: "nomad", Checks: []*ServiceCheck{{Name: "c1"}}},
							{Provider: "nomad"},
						},
					},
					{
						Name: "group2",
						Services: []*Service{
							{Provider: "nomad"},
						},
					},
					{
						Name: "group3",
						Services: []*Service{
							{Provider: "nomad"},
							{Provider: "nomad", Checks: []*ServiceCheck{{Name: "c2"}}},
						},
					},
				},
			},
			expBasic:  []string{"group1", "group2", "group3"},
			expChecks: []string{"group1", "group3"},
		},
		{
			name: "multiple task services with Nomad provider",
			inputJob: &Job{
				TaskGroups: []*TaskGroup{
					{
						Name: "group1",
						Tasks: []*Task{
							{
								Services: []*Service{
									{Provider: "nomad"},
									{Provider: "nomad"},
								},
							},
							{
								Services: []*Service{
									{Provider: "nomad"},
									{Provider: "nomad"},
								},
							},
						},
					},
					{
						Name: "group2",
						Tasks: []*Task{
							{
								Services: []*Service{
									{Provider: "nomad"},
									{Provider: "nomad"},
								},
							},
							{
								Services: []*Service{
									{Provider: "nomad"},
									{Provider: "nomad", Checks: []*ServiceCheck{{Name: "c1"}}},
								},
							},
						},
					},
				},
			},
			expBasic:  []string{"group1", "group2"},
			expChecks: []string{"group2"},
		},
		{
			name: "multiple group services with Consul provider",
			inputJob: &Job{
				TaskGroups: []*TaskGroup{
					{
						Name: "group1",
						Services: []*Service{
							{Provider: "consul"},
							{Provider: "consul"},
						},
					},
					{
						Name: "group2",
						Services: []*Service{
							{Provider: "consul"},
							{Provider: "consul"},
						},
					},
				},
			},
			expBasic:  nil,
			expChecks: nil,
		},
		{
			name: "multiple task services with Consul provider",
			inputJob: &Job{
				TaskGroups: []*TaskGroup{
					{
						Name: "group1",
						Tasks: []*Task{
							{
								Services: []*Service{
									{Provider: "consul"},
									{Provider: "consul"},
								},
							},
							{
								Services: []*Service{
									{Provider: "consul"},
									{Provider: "consul"},
								},
							},
						},
					},
					{
						Name: "group2",
						Tasks: []*Task{
							{
								Services: []*Service{
									{Provider: "consul"},
									{Provider: "consul"},
								},
							},
							{
								Services: []*Service{
									{Provider: "consul"},
									{Provider: "consul"},
								},
							},
						},
					},
				},
			},
			expBasic:  nil,
			expChecks: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nsdUsage := tc.inputJob.RequiredNativeServiceDiscovery()
			must.Equal(t, set.From(tc.expBasic), nsdUsage.Basic)
			must.Equal(t, set.From(tc.expChecks), nsdUsage.Checks)
		})
	}
}

func TestJob_RequiredConsulServiceDiscovery(t *testing.T) {
	testCases := []struct {
		inputJob       *Job
		expectedOutput map[string]bool
		name           string
	}{
		{
			inputJob: &Job{
				TaskGroups: []*TaskGroup{
					{
						Name: "group1",
						Services: []*Service{
							{Provider: "consul"},
							{Provider: "consul"},
						},
					},
					{
						Name: "group2",
						Services: []*Service{
							{Provider: "consul"},
							{Provider: "consul"},
						},
					},
				},
			},
			expectedOutput: map[string]bool{"group1": true, "group2": true},
			name:           "multiple group services with Consul provider",
		},
		{
			inputJob: &Job{
				TaskGroups: []*TaskGroup{
					{
						Name: "group1",
						Tasks: []*Task{
							{
								Services: []*Service{
									{Provider: "consul"},
									{Provider: "consul"},
								},
							},
							{
								Services: []*Service{
									{Provider: "consul"},
									{Provider: "consul"},
								},
							},
						},
					},
					{
						Name: "group2",
						Tasks: []*Task{
							{
								Services: []*Service{
									{Provider: "consul"},
									{Provider: "consul"},
								},
							},
							{
								Services: []*Service{
									{Provider: "consul"},
									{Provider: "consul"},
								},
							},
						},
					},
				},
			},
			expectedOutput: map[string]bool{"group1": true, "group2": true},
			name:           "multiple task services with Consul provider",
		},
		{
			inputJob: &Job{
				TaskGroups: []*TaskGroup{
					{
						Name: "group1",
						Services: []*Service{
							{Provider: "nomad"},
							{Provider: "nomad"},
						},
					},
					{
						Name: "group2",
						Services: []*Service{
							{Provider: "nomad"},
							{Provider: "nomad"},
						},
					},
				},
			},
			expectedOutput: map[string]bool{},
			name:           "multiple group services with Nomad provider",
		},
		{
			inputJob: &Job{
				TaskGroups: []*TaskGroup{
					{
						Name: "group1",
						Tasks: []*Task{
							{
								Services: []*Service{
									{Provider: "nomad"},
									{Provider: "nomad"},
								},
							},
							{
								Services: []*Service{
									{Provider: "nomad"},
									{Provider: "nomad"},
								},
							},
						},
					},
					{
						Name: "group2",
						Tasks: []*Task{
							{
								Services: []*Service{
									{Provider: "nomad"},
									{Provider: "nomad"},
								},
							},
							{
								Services: []*Service{
									{Provider: "nomad"},
									{Provider: "nomad"},
								},
							},
						},
					},
				},
			},
			expectedOutput: map[string]bool{},
			name:           "multiple task services with Nomad provider",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputJob.RequiredConsulServiceDiscovery()
			require.Equal(t, tc.expectedOutput, actualOutput)
		})
	}
}
