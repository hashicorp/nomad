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
						Name: "group1",
						Tasks: []*structs.Task{
							{
								Name:  "group1-task1",
								Vault: &structs.Vault{},
							},
						},
					},
				},
			},
			expectedOutputJob: &structs.Job{
				Name: "example",
				TaskGroups: []*structs.TaskGroup{
					{
						Name: "group1",
						Tasks: []*structs.Task{
							{
								Vault: &structs.Vault{},
								Name:  "group1-task1",
							},
						},
						Constraints: []*structs.Constraint{vaultConstraint},
					},
				},
			},
			expectedOutputWarnings: nil,
			expectedOutputError:    nil,
			name:                   "task with vault",
		},
		{
			inputJob: &structs.Job{
				Name: "example",
				TaskGroups: []*structs.TaskGroup{
					{
						Name: "group1",
						Tasks: []*structs.Task{
							{
								Vault: &structs.Vault{},
								Name:  "group1-task1",
							},
							{
								Vault: &structs.Vault{},
								Name:  "group1-task2",
							},
							{
								Vault: &structs.Vault{},
								Name:  "group1-task3",
							},
						},
					},
				},
			},
			expectedOutputJob: &structs.Job{
				Name: "example",
				TaskGroups: []*structs.TaskGroup{
					{
						Name: "group1",
						Tasks: []*structs.Task{
							{
								Vault: &structs.Vault{},
								Name:  "group1-task1",
							},
							{
								Vault: &structs.Vault{},
								Name:  "group1-task2",
							},
							{
								Vault: &structs.Vault{},
								Name:  "group1-task3",
							},
						},
						Constraints: []*structs.Constraint{vaultConstraint},
					},
				},
			},
			expectedOutputWarnings: nil,
			expectedOutputError:    nil,
			name:                   "group with multiple tasks with vault",
		},
		{
			inputJob: &structs.Job{
				Name: "example",
				TaskGroups: []*structs.TaskGroup{
					{
						Name: "group1",
						Tasks: []*structs.Task{
							{
								Name: "group1-task1",
							},
						},
					},
					{
						Name: "group2",
						Tasks: []*structs.Task{
							{
								Name:  "group2-task1",
								Vault: &structs.Vault{},
							},
						},
					},
					{
						Name: "group3",
						Tasks: []*structs.Task{
							{
								Name: "group3-task1",
							},
						},
					},
				},
			},
			expectedOutputJob: &structs.Job{
				Name: "example",
				TaskGroups: []*structs.TaskGroup{
					{
						Name: "group1",
						Tasks: []*structs.Task{
							{
								Name: "group1-task1",
							},
						},
					},
					{
						Name: "group2",
						Tasks: []*structs.Task{
							{
								Name:  "group2-task1",
								Vault: &structs.Vault{},
							},
						},
						Constraints: []*structs.Constraint{vaultConstraint},
					},
					{
						Name: "group3",
						Tasks: []*structs.Task{
							{
								Name: "group3-task1",
							},
						},
					},
				},
			},
			expectedOutputWarnings: nil,
			expectedOutputError:    nil,
			name:                   "multiple groups only one with vault",
		},
		{
			inputJob: &structs.Job{
				Name: "example",
				TaskGroups: []*structs.TaskGroup{
					{
						Name: "group1",
						Tasks: []*structs.Task{
							{
								Name:  "group1-task1",
								Vault: &structs.Vault{},
							},
						},
						Constraints: []*structs.Constraint{
							{
								LTarget: attrVaultVersion,
								RTarget: ">= 1.0.0",
								Operand: structs.ConstraintSemver,
							},
						},
					},
				},
			},
			expectedOutputJob: &structs.Job{
				Name: "example",
				TaskGroups: []*structs.TaskGroup{
					{
						Name: "group1",
						Tasks: []*structs.Task{
							{
								Name:  "group1-task1",
								Vault: &structs.Vault{},
							},
						},
						Constraints: []*structs.Constraint{
							{
								LTarget: attrVaultVersion,
								RTarget: ">= 1.0.0",
								Operand: structs.ConstraintSemver,
							},
						},
					},
				},
			},
			expectedOutputWarnings: nil,
			expectedOutputError:    nil,
			name:                   "existing vault version constraint",
		},
		{
			inputJob: &structs.Job{
				Name: "example",
				TaskGroups: []*structs.TaskGroup{
					{
						Name: "group1",
						Tasks: []*structs.Task{
							{
								Name:  "group1-task1",
								Vault: &structs.Vault{},
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
						Name: "group1",
						Tasks: []*structs.Task{
							{
								Name:  "group1-task1",
								Vault: &structs.Vault{},
							},
						},
						Constraints: []*structs.Constraint{
							{
								LTarget: "${node.class}",
								RTarget: "high-memory",
								Operand: "=",
							},
							vaultConstraint,
						},
					},
				},
			},
			expectedOutputWarnings: nil,
			expectedOutputError:    nil,
			name:                   "vault with other constraints",
		},
		{
			inputJob: &structs.Job{
				Name: "example",
				TaskGroups: []*structs.TaskGroup{
					{
						Name: "example-group-1",
						Tasks: []*structs.Task{
							{
								Name: "group1-task1",
								Vault: &structs.Vault{
									ChangeSignal: "SIGINT",
									ChangeMode:   structs.VaultChangeModeSignal,
								},
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
						Tasks: []*structs.Task{
							{
								Name: "group1-task1",
								Vault: &structs.Vault{
									ChangeSignal: "SIGINT",
									ChangeMode:   structs.VaultChangeModeSignal,
								},
							},
						},
						Constraints: []*structs.Constraint{
							vaultConstraint,
							{
								LTarget: "${attr.os.signals}",
								RTarget: "SIGINT",
								Operand: "set_contains",
							},
						},
					},
				},
			},
			expectedOutputWarnings: nil,
			expectedOutputError:    nil,
			name:                   "task with vault signal change",
		},
		{
			inputJob: &structs.Job{
				Name: "example",
				TaskGroups: []*structs.TaskGroup{
					{
						Name: "example-group-1",
						Tasks: []*structs.Task{
							{
								Name:       "group1-task1",
								KillSignal: "SIGINT",
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
						Tasks: []*structs.Task{
							{
								Name:       "group1-task1",
								KillSignal: "SIGINT",
							},
						},
						Constraints: []*structs.Constraint{
							{
								LTarget: "${attr.os.signals}",
								RTarget: "SIGINT",
								Operand: "set_contains",
							},
						},
					},
				},
			},
			expectedOutputWarnings: nil,
			expectedOutputError:    nil,
			name:                   "task with kill signal",
		},
		{
			inputJob: &structs.Job{
				Name: "example",
				TaskGroups: []*structs.TaskGroup{
					{
						Name: "example-group-1",
						Tasks: []*structs.Task{
							{
								Name: "group1-task1",
								Templates: []*structs.Template{
									{
										ChangeMode:   "signal",
										ChangeSignal: "SIGINT",
									},
								},
							},
							{
								Name: "group1-task2",
								Templates: []*structs.Template{
									{
										ChangeMode:   "signal",
										ChangeSignal: "SIGHUP",
									},
								},
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
						Tasks: []*structs.Task{
							{
								Name: "group1-task1",
								Templates: []*structs.Template{
									{
										ChangeMode:   "signal",
										ChangeSignal: "SIGINT",
									},
								},
							},
							{
								Name: "group1-task2",
								Templates: []*structs.Template{
									{
										ChangeMode:   "signal",
										ChangeSignal: "SIGHUP",
									},
								},
							},
						},
						Constraints: []*structs.Constraint{
							{
								LTarget: "${attr.os.signals}",
								RTarget: "SIGHUP,SIGINT",
								Operand: "set_contains",
							},
						},
					},
				},
			},
			expectedOutputWarnings: nil,
			expectedOutputError:    nil,
			name:                   "multiple tasks with template signal change",
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
		{
			inputJob: &structs.Job{
				Name: "example",
				TaskGroups: []*structs.TaskGroup{
					{
						Name: "example-group-1",
						Services: []*structs.Service{
							{
								Name:     "example-group-service-1",
								Provider: structs.ServiceProviderConsul,
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
								Provider: structs.ServiceProviderConsul,
							},
						},
						Constraints: []*structs.Constraint{consulServiceDiscoveryConstraint},
					},
				},
			},
			expectedOutputWarnings: nil,
			expectedOutputError:    nil,
			name:                   "task group Consul discovery",
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
								Provider: structs.ServiceProviderConsul,
							},
						},
						Constraints: []*structs.Constraint{consulServiceDiscoveryConstraint},
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
								Provider: structs.ServiceProviderConsul,
							},
						},
						Constraints: []*structs.Constraint{consulServiceDiscoveryConstraint},
					},
				},
			},
			expectedOutputWarnings: nil,
			expectedOutputError:    nil,
			name:                   "task group Consul discovery constraint found",
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
								Provider: structs.ServiceProviderConsul,
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
								Provider: structs.ServiceProviderConsul,
							},
						},
						Constraints: []*structs.Constraint{
							{
								LTarget: "${node.class}",
								RTarget: "high-memory",
								Operand: "=",
							},
							consulServiceDiscoveryConstraint,
						},
					},
				},
			},
			expectedOutputWarnings: nil,
			expectedOutputError:    nil,
			name:                   "task group Consul discovery other constraints",
		},
		{
			inputJob: &structs.Job{
				Name: "example",
				TaskGroups: []*structs.TaskGroup{
					{
						Name: "example-group-1",
						Services: []*structs.Service{
							{
								Name: "example-group-service-1",
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
								Name: "example-group-service-1",
							},
						},
						Constraints: []*structs.Constraint{consulServiceDiscoveryConstraint},
					},
				},
			},
			expectedOutputWarnings: nil,
			expectedOutputError:    nil,
			name:                   "task group with empty provider",
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
