package nomad

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func Test_jobImpliedConstraints_Mutate(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		inputJob               *structs.Job
		expectedOutputJob      *structs.Job
		expectedOutputWarnings []error
		expectedOutputError    error
		name                   string
	}{
		{
			inputJob: &structs.Job{
				Name: "example",
				TaskGroups: []*structs.TaskGroup{
					{
						Name: "example-group-1",
					},
				},
			},
			expectedOutputJob: &structs.Job{
				Name: "example",
				TaskGroups: []*structs.TaskGroup{
					{
						Name: "example-group-1",
					},
				},
			},
			expectedOutputWarnings: nil,
			expectedOutputError:    nil,
			name:                   "no needed constraints",
		},
		{
			inputJob: &structs.Job{
				Name: "example",
				TaskGroups: []*structs.TaskGroup{
					{
						Name: "example-group-1",
						Services: []*structs.Service{
							{
								Name:     "example-group-service-1",
								Provider: structs.ServiceProviderNomad,
							},
						},
					},
				},
			},
			expectedOutputJob: &structs.Job{
				Name: "example",
				TaskGroups: []*structs.TaskGroup{
					{
						Name: "example-group-1",
						Services: []*structs.Service{
							{
								Name:     "example-group-service-1",
								Provider: structs.ServiceProviderNomad,
							},
						},
						Constraints: []*structs.Constraint{nativeServiceDiscoveryConstraint},
					},
				},
			},
			expectedOutputWarnings: nil,
			expectedOutputError:    nil,
			name:                   "task group nomad discovery",
		},
		{
			inputJob: &structs.Job{
				Name: "example",
				TaskGroups: []*structs.TaskGroup{
					{
						Name: "example-group-1",
						Services: []*structs.Service{
							{
								Name:     "example-group-service-1",
								Provider: structs.ServiceProviderNomad,
							},
						},
						Constraints: []*structs.Constraint{nativeServiceDiscoveryConstraint},
					},
				},
			},
			expectedOutputJob: &structs.Job{
				Name: "example",
				TaskGroups: []*structs.TaskGroup{
					{
						Name: "example-group-1",
						Services: []*structs.Service{
							{
								Name:     "example-group-service-1",
								Provider: structs.ServiceProviderNomad,
							},
						},
						Constraints: []*structs.Constraint{nativeServiceDiscoveryConstraint},
					},
				},
			},
			expectedOutputWarnings: nil,
			expectedOutputError:    nil,
			name:                   "task group nomad discovery constraint found",
		},
		{
			inputJob: &structs.Job{
				Name: "example",
				TaskGroups: []*structs.TaskGroup{
					{
						Name: "example-group-1",
						Services: []*structs.Service{
							{
								Name:     "example-group-service-1",
								Provider: structs.ServiceProviderNomad,
							},
						},
						Constraints: []*structs.Constraint{
							{
								LTarget: "${node.class}",
								RTarget: "high-memory",
								Operand: "=",
							},
						},
					},
				},
			},
			expectedOutputJob: &structs.Job{
				Name: "example",
				TaskGroups: []*structs.TaskGroup{
					{
						Name: "example-group-1",
						Services: []*structs.Service{
							{
								Name:     "example-group-service-1",
								Provider: structs.ServiceProviderNomad,
							},
						},
						Constraints: []*structs.Constraint{
							{
								LTarget: "${node.class}",
								RTarget: "high-memory",
								Operand: "=",
							},
							nativeServiceDiscoveryConstraint,
						},
					},
				},
			},
			expectedOutputWarnings: nil,
			expectedOutputError:    nil,
			name:                   "task group nomad discovery other constraints",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			impl := jobImpliedConstraints{}
			actualJob, actualWarnings, actualError := impl.Mutate(tc.inputJob)
			require.Equal(t, tc.expectedOutputJob, actualJob)
			require.ElementsMatch(t, tc.expectedOutputWarnings, actualWarnings)
			require.Equal(t, tc.expectedOutputError, actualError)
		})
	}
}
