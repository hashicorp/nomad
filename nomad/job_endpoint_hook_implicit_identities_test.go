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
