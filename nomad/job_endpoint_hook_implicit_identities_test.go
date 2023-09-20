// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/shoenig/test/must"
)

func Test_jobImplicitIndentitiesHook_Mutate_consul_service(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name              string
		inputJob          *structs.Job
		inputConfig       *Config
		expectedOutputJob *structs.Job
	}{
		{
			name: "no mutation when identity is disabled",
			inputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Services: []*structs.Service{{
						Provider: "consul",
					}},
				}},
			},
			inputConfig: &Config{
				ConsulConfig: &config.ConsulConfig{
					UseIdentity: pointer.Of(false),
				},
			},
			expectedOutputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Services: []*structs.Service{{
						Provider: "consul",
					}},
				}},
			},
		},
		{
			name: "no mutation when identity is enabled but no service identity is configured",
			inputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Services: []*structs.Service{{
						Provider: "consul",
					}},
				}},
			},
			inputConfig: &Config{
				ConsulConfig: &config.ConsulConfig{
					UseIdentity: pointer.Of(true),
				},
			},
			expectedOutputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Services: []*structs.Service{{
						Provider: "consul",
					}},
				}},
			},
		},
		{
			name: "no mutation when nomad service",
			inputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Services: []*structs.Service{{
						Provider: "nomad",
					}},
				}},
			},
			inputConfig: &Config{
				ConsulConfig: &config.ConsulConfig{
					UseIdentity: pointer.Of(true),
					ServiceIdentity: &config.WorkloadIdentityConfig{
						Audience: []string{"consul.io"},
					},
				},
			},
			expectedOutputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Services: []*structs.Service{{
						Provider: "nomad",
					}},
				}},
			},
		},
		{
			name: "mutate identity name and service name when custom identity is provided",
			inputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Services: []*structs.Service{
						{
							Provider: "consul",
							Name:     "web",
							Identity: &structs.WorkloadIdentity{
								Audience: []string{"consul.io", "nomad.dev"},
								File:     true,
								Env:      false,
							},
						},
						{
							Name: "web",
							Identity: &structs.WorkloadIdentity{
								Audience: []string{"consul.io", "nomad.dev"},
								File:     true,
								Env:      false,
							},
						},
					},
					Tasks: []*structs.Task{{
						Services: []*structs.Service{{
							Provider: "consul",
							Name:     "web-task",
							Identity: &structs.WorkloadIdentity{
								Audience: []string{"consul.io", "nomad.dev"},
								File:     true,
								Env:      false,
							},
						}},
					}},
				}},
			},
			inputConfig: &Config{
				ConsulConfig: &config.ConsulConfig{
					UseIdentity: pointer.Of(true),
					ServiceIdentity: &config.WorkloadIdentityConfig{
						Audience: []string{"consul.io"},
					},
				},
			},
			expectedOutputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Services: []*structs.Service{
						{
							Provider: "consul",
							Name:     "web",
							Identity: &structs.WorkloadIdentity{
								Name:        "consul-service/web",
								Audience:    []string{"consul.io", "nomad.dev"},
								File:        true,
								Env:         false,
								ServiceName: "web",
							},
						},
						{
							Name: "web",
							Identity: &structs.WorkloadIdentity{
								Name:        "consul-service/web",
								Audience:    []string{"consul.io", "nomad.dev"},
								File:        true,
								Env:         false,
								ServiceName: "web",
							},
						},
					},
					Tasks: []*structs.Task{{
						Services: []*structs.Service{{
							Provider: "consul",
							Name:     "web-task",
							Identity: &structs.WorkloadIdentity{
								Name:        "consul-service/web-task",
								Audience:    []string{"consul.io", "nomad.dev"},
								File:        true,
								Env:         false,
								ServiceName: "web-task",
							},
						}},
					}},
				}},
			},
		},
		{
			name: "mutate service to inject identity",
			inputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Services: []*structs.Service{{
						Provider: "consul",
						Name:     "web",
					}},
					Tasks: []*structs.Task{{
						Services: []*structs.Service{{
							Provider: "consul",
							Name:     "web-task",
						}},
					}},
				}},
			},
			inputConfig: &Config{
				ConsulConfig: &config.ConsulConfig{
					UseIdentity: pointer.Of(true),
					ServiceIdentity: &config.WorkloadIdentityConfig{
						Audience: []string{"consul.io"},
					},
				},
			},
			expectedOutputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Services: []*structs.Service{{
						Provider: "consul",
						Name:     "web",
						Identity: &structs.WorkloadIdentity{
							Name:        "consul-service/web",
							Audience:    []string{"consul.io"},
							ServiceName: "web",
						},
					}},
					Tasks: []*structs.Task{{
						Services: []*structs.Service{{
							Provider: "consul",
							Name:     "web-task",
							Identity: &structs.WorkloadIdentity{
								Name:        "consul-service/web-task",
								Audience:    []string{"consul.io"},
								ServiceName: "web-task",
							},
						}},
					}},
				}},
			},
		},
		{
			name: "mutate task to inject identity for templates",
			inputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Tasks: []*structs.Task{{
						Name:      "web-task",
						Templates: []*structs.Template{{}},
					}},
				}},
			},
			inputConfig: &Config{
				ConsulConfig: &config.ConsulConfig{
					UseIdentity: pointer.Of(true),
					TaskIdentity: &config.WorkloadIdentityConfig{
						Audience: []string{"consul.io"},
					},
				},
			},
			expectedOutputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Tasks: []*structs.Task{{
						Name:      "web-task",
						Templates: []*structs.Template{{}},
						Identities: []*structs.WorkloadIdentity{{
							Name:     "consul/web-task",
							Audience: []string{"consul.io"},
						}},
					}},
				}},
			},
		},
		{
			name: "no mutation for templates when identity is enabled but no task identity is configured",
			inputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Tasks: []*structs.Task{{
						Name:      "web-task",
						Templates: []*structs.Template{{}},
					}},
				}},
			},
			inputConfig: &Config{
				ConsulConfig: &config.ConsulConfig{
					UseIdentity: pointer.Of(true),
				},
			},
			expectedOutputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Tasks: []*structs.Task{{
						Name:      "web-task",
						Templates: []*structs.Template{{}},
					}},
				}},
			},
		},
		{
			name: "no task mutation for templates when identity is disabled",
			inputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Tasks: []*structs.Task{{
						Name:      "web-task",
						Templates: []*structs.Template{{}},
					}},
				}},
			},
			inputConfig: &Config{
				ConsulConfig: &config.ConsulConfig{
					UseIdentity: pointer.Of(false),
					TaskIdentity: &config.WorkloadIdentityConfig{
						Audience: []string{"consul.io"},
					},
				},
			},
			expectedOutputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Tasks: []*structs.Task{{
						Name:      "web-task",
						Templates: []*structs.Template{{}},
					}},
				}},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			impl := jobImplicitIdentitiesHook{srv: &Server{
				config: tc.inputConfig,
			}}
			actualJob, actualWarnings, actualError := impl.Mutate(tc.inputJob)
			must.Eq(t, tc.expectedOutputJob, actualJob)
			must.NoError(t, actualError)
			must.Nil(t, actualWarnings)
		})
	}
}

func Test_jobImplicitIndentitiesHook_Mutate_vault(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name              string
		inputJob          *structs.Job
		inputConfig       *Config
		expectedOutputJob *structs.Job
	}{
		{
			name: "no mutation when task does not have a vault block",
			inputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Tasks: []*structs.Task{{}},
				}},
			},
			inputConfig: &Config{
				VaultConfig: &config.VaultConfig{
					UseIdentity: pointer.Of(true),
					DefaultIdentity: &config.WorkloadIdentityConfig{
						Audience: []string{"vault.io"},
					},
				},
			},
			expectedOutputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Tasks: []*structs.Task{{}},
				}},
			},
		},
		{
			name: "no mutation when vault identity is disabled",
			inputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Tasks: []*structs.Task{{
						Vault: &structs.Vault{},
					}},
				}},
			},
			inputConfig: &Config{
				VaultConfig: &config.VaultConfig{
					UseIdentity: pointer.Of(false),
					DefaultIdentity: &config.WorkloadIdentityConfig{
						Audience: []string{"vault.io"},
					},
				},
			},
			expectedOutputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Tasks: []*structs.Task{{
						Vault: &structs.Vault{},
					}},
				}},
			},
		},
		{
			name: "no mutation when vault identity is enabled but no default identity is configured",
			inputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Tasks: []*structs.Task{{
						Vault: &structs.Vault{},
					}},
				}},
			},
			inputConfig: &Config{
				VaultConfig: &config.VaultConfig{
					UseIdentity: pointer.Of(true),
				},
			},
			expectedOutputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Tasks: []*structs.Task{{
						Vault: &structs.Vault{},
					}},
				}},
			},
		},
		{
			name: "no mutation when task has vault identity",
			inputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Tasks: []*structs.Task{{
						Identities: []*structs.WorkloadIdentity{{
							Name:     "vault",
							Audience: []string{"vault.io"},
						}},
						Vault: &structs.Vault{},
					}},
				}},
			},
			inputConfig: &Config{
				VaultConfig: &config.VaultConfig{
					UseIdentity: pointer.Of(true),
					DefaultIdentity: &config.WorkloadIdentityConfig{
						Audience: []string{"vault.io"},
					},
				},
			},
			expectedOutputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Tasks: []*structs.Task{{
						Identities: []*structs.WorkloadIdentity{{
							Name:     "vault",
							Audience: []string{"vault.io"},
						}},
						Vault: &structs.Vault{},
					}},
				}},
			},
		},
		{
			name: "mutate when task does not have a vault idenity",
			inputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Tasks: []*structs.Task{{
						Vault: &structs.Vault{},
					}},
				}},
			},
			inputConfig: &Config{
				VaultConfig: &config.VaultConfig{
					UseIdentity: pointer.Of(true),
					DefaultIdentity: &config.WorkloadIdentityConfig{
						Audience: []string{"vault.io"},
					},
				},
			},
			expectedOutputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Tasks: []*structs.Task{{
						Identities: []*structs.WorkloadIdentity{{
							Name:     "vault",
							Audience: []string{"vault.io"},
						}},
						Vault: &structs.Vault{},
					}},
				}},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			impl := jobImplicitIdentitiesHook{srv: &Server{
				config: tc.inputConfig,
			}}
			actualJob, actualWarnings, actualError := impl.Mutate(tc.inputJob)
			must.Eq(t, tc.expectedOutputJob, actualJob)
			must.NoError(t, actualError)
			must.Nil(t, actualWarnings)
		})
	}
}
