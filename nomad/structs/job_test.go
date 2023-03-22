package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServiceRegistrationsRequest_StaleReadSupport(t *testing.T) {
	req := &AllocServiceRegistrationsRequest{}
	require.True(t, req.IsRead())
}

func TestJob_RequiresNativeServiceDiscovery(t *testing.T) {
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
			expectedOutput: map[string]bool{"group1": true, "group2": true},
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
			expectedOutput: map[string]bool{"group1": true, "group2": true},
			name:           "multiple task services with Nomad provider",
		},
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
			expectedOutput: map[string]bool{},
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
			expectedOutput: map[string]bool{},
			name:           "multiple task services with Consul provider",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputJob.RequiredNativeServiceDiscovery()
			require.Equal(t, tc.expectedOutput, actualOutput)
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
