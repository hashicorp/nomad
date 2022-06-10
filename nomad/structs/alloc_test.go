package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAllocServiceRegistrationsRequest_StaleReadSupport(t *testing.T) {
	req := &AllocServiceRegistrationsRequest{}
	require.True(t, req.IsRead())
}

func Test_Allocation_ServiceProviderNamespace(t *testing.T) {
	testCases := []struct {
		inputAllocation *Allocation
		expectedOutput  string
		name            string
	}{
		{
			inputAllocation: &Allocation{
				Job: &Job{
					TaskGroups: []*TaskGroup{
						{
							Name: "test-group",
							Services: []*Service{
								{
									Provider: ServiceProviderConsul,
								},
							},
						},
					},
				},
				TaskGroup: "test-group",
			},
			expectedOutput: "",
			name:           "consul task group service",
		},
		{
			inputAllocation: &Allocation{
				Job: &Job{
					TaskGroups: []*TaskGroup{
						{
							Name: "test-group",
							Tasks: []*Task{
								{
									Services: []*Service{
										{
											Provider: ServiceProviderConsul,
										},
									},
								},
							},
						},
					},
				},
				TaskGroup: "test-group",
			},
			expectedOutput: "",
			name:           "consul task service",
		},
		{
			inputAllocation: &Allocation{
				Job: &Job{
					Namespace: "platform",
					TaskGroups: []*TaskGroup{
						{
							Name: "test-group",
							Services: []*Service{
								{
									Provider: ServiceProviderNomad,
								},
							},
						},
					},
				},
				TaskGroup: "test-group",
			},
			expectedOutput: "platform",
			name:           "nomad task group service",
		},
		{
			inputAllocation: &Allocation{
				Job: &Job{
					Namespace: "platform",
					TaskGroups: []*TaskGroup{
						{
							Name: "test-group",
							Tasks: []*Task{
								{
									Services: []*Service{
										{
											Provider: ServiceProviderNomad,
										},
									},
								},
							},
						},
					},
				},
				TaskGroup: "test-group",
			},
			expectedOutput: "platform",
			name:           "nomad task service",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputAllocation.ServiceProviderNamespace()
			require.Equal(t, tc.expectedOutput, actualOutput)
		})
	}
}
