package nomad

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func Test_jobImplicitIdentityHook_Mutate(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name              string
		inputJob          *structs.Job
		expectedOutputJob *structs.Job
	}{
		{
			name: "no mutation",
			inputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Services: []*structs.Service{{
						Provider: "nomad",
					}},
				}},
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
			name: "custom set identity, mutate",
			inputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Services: []*structs.Service{{
						Provider: "consul",
						Name:     "web",
						Identity: &structs.WorkloadIdentity{
							Name:     "test",
							Audience: []string{"consul.io"},
						},
					}},
				}},
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
				}},
			},
		},
		{
			name: "mutate",
			inputJob: &structs.Job{
				TaskGroups: []*structs.TaskGroup{{
					Services: []*structs.Service{{
						Provider: "consul",
						Name:     "web",
					}},
				}},
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
				}},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := DefaultConfig()
			impl := jobImplicitIdentityHook{
				srv: &Server{config: config},
			}

			actualJob, actualWarnings, actualError := impl.Mutate(tc.inputJob)
			must.Eq(t, tc.expectedOutputJob, actualJob)
			must.NoError(t, actualError)
			must.Nil(t, actualWarnings)
		})
	}
}
