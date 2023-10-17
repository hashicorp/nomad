// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/shoenig/test/must"
)

func Test_jobValidate_Validate_consul_service(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name          string
		inputService  *structs.Service
		inputConfig   *Config
		expectedWarns []string
		expectedErr   string
	}{
		{
			name: "no error when consul identity is not enabled and service does not have an identity",
			inputService: &structs.Service{
				Provider: "consul",
				Name:     "web",
			},
			inputConfig: &Config{
				ConsulConfig: &config.ConsulConfig{
					UseIdentity: pointer.Of(false),
				},
			},
		},
		{
			name: "no error when consul identity is enabled and identity is provided via server config",
			inputService: &structs.Service{
				Provider: "consul",
				Name:     "web",
			},
			inputConfig: &Config{
				ConsulConfig: &config.ConsulConfig{
					UseIdentity: pointer.Of(true),
					ServiceIdentity: &config.WorkloadIdentityConfig{
						Audience: []string{"consul.io"},
						TTL:      pointer.Of(time.Hour),
					},
				},
			},
		},
		{
			name: "no error when consul identity is enabled and identity is provided via service",
			inputService: &structs.Service{
				Provider: "consul",
				Name:     "web",
				Identity: &structs.WorkloadIdentity{
					Name:        "consul-service_web",
					Audience:    []string{"consul.io"},
					File:        true,
					Env:         false,
					ServiceName: "web",
					TTL:         time.Hour,
				},
			},
			inputConfig: &Config{
				ConsulConfig: &config.ConsulConfig{
					UseIdentity: pointer.Of(true),
				},
			},
		},
		{
			name: "warn when service identity has no TTL",
			inputService: &structs.Service{
				Provider: "consul",
				Name:     "web",
				Identity: &structs.WorkloadIdentity{
					Name:        "consul-service_web",
					Audience:    []string{"consul.io"},
					File:        true,
					Env:         false,
					ServiceName: "web",
				},
			},
			inputConfig: &Config{
				ConsulConfig: &config.ConsulConfig{
					UseIdentity: pointer.Of(true),
				},
			},
			expectedWarns: []string{
				"identities without an expiration are insecure",
			},
		},
		{
			name: "error when consul identity is disabled and service has identity",
			inputService: &structs.Service{
				Provider: "consul",
				Name:     "web",
				Identity: &structs.WorkloadIdentity{
					Name:     fmt.Sprintf("%s_web", structs.ConsulServiceIdentityNamePrefix),
					Audience: []string{"consul.io"},
					File:     true,
					Env:      false,
					TTL:      time.Hour,
				},
			},
			inputConfig: &Config{
				ConsulConfig: &config.ConsulConfig{
					UseIdentity: pointer.Of(false),
				},
			},
			expectedErr: "defines an identity but server is not configured to use Consul identities",
		},
		{
			name: "error when consul identity is enabled but no service identity is provided",
			inputService: &structs.Service{
				Provider: "consul",
				Name:     "web",
			},
			inputConfig: &Config{
				ConsulConfig: &config.ConsulConfig{
					UseIdentity: pointer.Of(true),
				},
			},
			expectedErr: "expected to have an identity",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.inputConfig.JobMaxPriority = 100
			impl := jobValidate{srv: &Server{
				config: tc.inputConfig,
			}}

			job := mock.Job()
			job.TaskGroups[0].Services = []*structs.Service{tc.inputService}
			job.TaskGroups[0].Tasks[0].Services = []*structs.Service{tc.inputService}

			warns, err := impl.Validate(job)

			if len(tc.expectedErr) == 0 {
				must.NoError(t, err)
			} else {
				must.Error(t, err)
				must.ErrorContains(t, err, tc.expectedErr)
			}

			must.Len(t, len(tc.expectedWarns), warns, must.Sprintf("got warnings: %v", warns))
			for _, exp := range tc.expectedWarns {
				hasWarn := false
				for _, w := range warns {
					if strings.Contains(w.Error(), exp) {
						hasWarn = true
						break
					}
				}
				must.True(t, hasWarn, must.Sprintf("expected %v to have warning with %q", warns, exp))
			}
		})
	}
}

func Test_jobValidate_Validate_vault(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                string
		inputTaskVault      *structs.Vault
		inputTaskIdentities []*structs.WorkloadIdentity
		inputConfig         map[string]*config.VaultConfig
		expectedWarns       []string
		expectedErr         string
	}{
		{
			name: "no error when vault identity is provided via config",
			inputTaskVault: &structs.Vault{
				Cluster: structs.VaultDefaultCluster,
			},
			inputTaskIdentities: nil,
			inputConfig: map[string]*config.VaultConfig{
				structs.VaultDefaultCluster: {
					DefaultIdentity: &config.WorkloadIdentityConfig{
						Audience: []string{"vault.io"},
						TTL:      pointer.Of(time.Hour),
					},
				},
			},
		},
		{
			name:           "no error when vault identity is provided via task",
			inputTaskVault: &structs.Vault{},
			inputTaskIdentities: []*structs.WorkloadIdentity{{
				Name:     "vault_default",
				Audience: []string{"vault.io"},
				TTL:      time.Hour,
			}},
		},
		{
			name:                "error when not using vault identity and vault block is missing policies",
			inputTaskVault:      &structs.Vault{},
			inputTaskIdentities: nil,
			expectedErr:         "Vault block with an empty list of policies",
		},
		{
			name: "warn when using default vault identity but task has vault policies",
			inputTaskVault: &structs.Vault{
				Cluster:  structs.VaultDefaultCluster,
				Policies: []string{"nomad-workload"},
			},
			inputTaskIdentities: nil,
			inputConfig: map[string]*config.VaultConfig{
				structs.VaultDefaultCluster: {
					DefaultIdentity: &config.WorkloadIdentityConfig{
						Audience: []string{"vault.io"},
						TTL:      pointer.Of(time.Hour),
					},
				},
			},
			expectedWarns: []string{"policies will be ignored"},
		},
		{
			name: "warn when using task vault identity but task has vault policies",
			inputTaskVault: &structs.Vault{
				Policies: []string{"nomad-workload"},
			},
			inputTaskIdentities: []*structs.WorkloadIdentity{{
				Name:     "vault_default",
				Audience: []string{"vault.io"},
				TTL:      time.Hour,
			}},
			expectedWarns: []string{"policies will be ignored"},
		},
		{
			name:           "warn when vault identity is provided but task does not have vault block",
			inputTaskVault: nil,
			inputTaskIdentities: []*structs.WorkloadIdentity{{
				Name:     "vault_default",
				Audience: []string{"vault.io"},
				TTL:      time.Hour,
			}},
			expectedWarns: []string{
				"has an identity called vault_default but no vault block",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			impl := jobValidate{srv: &Server{
				config: &Config{
					JobMaxPriority: 100,
					VaultConfigs:   tc.inputConfig,
				},
			}}

			job := mock.Job()
			task := job.TaskGroups[0].Tasks[0]

			task.Identities = tc.inputTaskIdentities
			task.Vault = tc.inputTaskVault
			if task.Vault != nil {
				task.Vault.ChangeMode = structs.VaultChangeModeRestart
			}

			warns, err := impl.Validate(job)

			if len(tc.expectedErr) == 0 {
				must.NoError(t, err)
			} else {
				must.Error(t, err)
				must.ErrorContains(t, err, tc.expectedErr)
			}

			must.Len(t, len(tc.expectedWarns), warns, must.Sprintf("got warnings: %v", warns))
			for _, exp := range tc.expectedWarns {
				hasWarn := false
				for _, w := range warns {
					if strings.Contains(w.Error(), exp) {
						hasWarn = true
						break
					}
				}
				must.True(t, hasWarn, must.Sprintf("expected %v to have warning with %q", warns, exp))
			}
		})
	}
}

func Test_jobImpliedConstraints_Mutate(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                   string
		inputJob               *structs.Job
		expectedOutputJob      *structs.Job
		expectedOutputWarnings []error
		expectedOutputError    error
	}{
		{
			name: "no needed constraints",
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
		},
		{
			name: "task with vault",
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
		},
		{
			name: "group with multiple tasks with vault",
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
		},
		{
			name: "group with multiple vault clusters",
			inputJob: &structs.Job{
				Name: "example",
				TaskGroups: []*structs.TaskGroup{
					{
						Name: "group1",
						Tasks: []*structs.Task{
							{
								Vault: &structs.Vault{Cluster: "infra"},
								Name:  "group1-task1",
							},
							{
								Vault: &structs.Vault{Cluster: "infra"},
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
								Vault: &structs.Vault{Cluster: "infra"},
								Name:  "group1-task1",
							},
							{
								Vault: &structs.Vault{Cluster: "infra"},
								Name:  "group1-task2",
							},
							{
								Vault: &structs.Vault{},
								Name:  "group1-task3",
							},
						},
						Constraints: []*structs.Constraint{
							&structs.Constraint{
								LTarget: "${attr.vault.infra.version}",
								RTarget: ">= 0.6.1",
								Operand: structs.ConstraintSemver,
							},
							vaultConstraint,
						},
					},
				},
			},
			expectedOutputWarnings: nil,
			expectedOutputError:    nil,
		},
		{
			name: "multiple groups only one with vault",
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
		},
		{
			name: "existing vault version constraint",
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
		},
		{
			name: "vault with other constraints",
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
		},
		{
			name: "task with vault signal change",
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
		},
		{
			name: "task with kill signal",
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
		},
		{
			name: "multiple tasks with template signal change",
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
		},
		{
			name: "task group nomad discovery",
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
		},
		{
			name: "task group nomad discovery constraint found",
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
		},
		{
			name: "task group nomad discovery other constraints",
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
		},
		{
			name: "task group Consul discovery",
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
		},
		{
			name: "task group Consul discovery constraint found",
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
		},
		{
			name: "task group Consul discovery with multiple clusters",
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
							{
								Name:     "example-group-service-2",
								Provider: structs.ServiceProviderConsul,
								Cluster:  "infra",
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
							{
								Name:     "example-group-service-2",
								Provider: structs.ServiceProviderConsul,
								Cluster:  "infra",
							},
						},
						Constraints: []*structs.Constraint{
							consulServiceDiscoveryConstraint,
							&structs.Constraint{
								LTarget: "${attr.consul.infra.version}",
								RTarget: ">= 1.7.0",
								Operand: structs.ConstraintSemver,
							},
						},
					},
				},
			},
			expectedOutputWarnings: nil,
			expectedOutputError:    nil,
		},

		{
			name: "task group Consul discovery other constraints",
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
		},
		{
			name: "task group with empty provider",
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
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			impl := jobImpliedConstraints{}
			actualJob, actualWarnings, actualError := impl.Mutate(tc.inputJob)
			must.Eq(t, tc.expectedOutputJob, actualJob)
			must.SliceContainsAll(t, actualWarnings, tc.expectedOutputWarnings)
			must.Eq(t, tc.expectedOutputError, actualError)
		})
	}
}

func Test_jobCanonicalizer_Mutate(t *testing.T) {
	ci.Parallel(t)

	serverJobDefaultPriority := 100

	testCases := []struct {
		name              string
		inputJob          *structs.Job
		expectedOutputJob *structs.Job
	}{
		{
			name: "no mutation",
			inputJob: &structs.Job{
				Namespace:   "default",
				Datacenters: []string{"*"},
				Priority:    123,
			},
			expectedOutputJob: &structs.Job{
				Namespace:   "default",
				Datacenters: []string{"*"},
				Priority:    123,
			},
		},
		{
			name: "when priority is 0 mutate using the value present in the server config",
			inputJob: &structs.Job{
				Namespace:   "default",
				Datacenters: []string{"*"},
				Priority:    0,
			},
			expectedOutputJob: &structs.Job{
				Namespace:   "default",
				Datacenters: []string{"*"},
				Priority:    serverJobDefaultPriority,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			impl := jobCanonicalizer{srv: &Server{config: &Config{JobDefaultPriority: serverJobDefaultPriority}}}
			actualJob, actualWarnings, actualError := impl.Mutate(tc.inputJob)
			must.Eq(t, tc.expectedOutputJob, actualJob)
			must.NoError(t, actualError)
			must.Nil(t, actualWarnings)
		})
	}
}

func TestJob_submissionController(t *testing.T) {
	ci.Parallel(t)
	args := &structs.JobRegisterRequest{
		Submission: &structs.JobSubmission{
			Source:    "this is some hcl content",
			Format:    "hcl2",
			Variables: "variables",
		},
	}
	t.Run("nil", func(t *testing.T) {
		j := &Job{srv: &Server{
			config: &Config{JobMaxSourceSize: 1024},
		}}
		err := j.submissionController(&structs.JobRegisterRequest{
			Submission: nil,
		})
		must.NoError(t, err)
	})
	t.Run("under max size", func(t *testing.T) {
		j := &Job{srv: &Server{
			config: &Config{JobMaxSourceSize: 1024},
		}}
		err := j.submissionController(args)
		must.NoError(t, err)
		must.NotNil(t, args.Submission)
	})

	t.Run("over max size", func(t *testing.T) {
		j := &Job{srv: &Server{
			config: &Config{JobMaxSourceSize: 1},
		}}
		err := j.submissionController(args)
		must.ErrorContains(t, err, "job source size of 33 B exceeds maximum of 1 B and will be discarded")
		must.Nil(t, args.Submission)
	})
}
