package api

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestServiceRegistrations_List(t *testing.T) {
	// TODO(jrasell) add tests once registration process is in place.
}

func TestServiceRegistrations_Get(t *testing.T) {
	// TODO(jrasell) add tests once registration process is in place.
}

func TestServiceRegistrations_Delete(t *testing.T) {
	// TODO(jrasell) add tests once registration process is in place.
}

func TestService_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	j := &Job{Name: pointerOf("job")}
	tg := &TaskGroup{Name: pointerOf("group")}
	task := &Task{Name: "task"}
	s := &Service{
		TaggedAddresses: make(map[string]string),
	}

	s.Canonicalize(task, tg, j)

	must.Eq(t, fmt.Sprintf("%s-%s-%s", *j.Name, *tg.Name, task.Name), s.Name)
	must.Eq(t, "auto", s.AddressMode)
	must.Eq(t, OnUpdateRequireHealthy, s.OnUpdate)
	must.Eq(t, ServiceProviderConsul, s.Provider)
	must.Nil(t, s.Meta)
	must.Nil(t, s.CanaryMeta)
	must.Nil(t, s.TaggedAddresses)
}

func TestServiceCheck_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	j := &Job{Name: pointerOf("job")}
	tg := &TaskGroup{Name: pointerOf("group")}
	task := &Task{Name: "task"}
	s := &Service{
		Checks: []ServiceCheck{
			{
				Name: "check",
			},
		},
	}

	s.Canonicalize(task, tg, j)
	must.Eq(t, OnUpdateRequireHealthy, s.Checks[0].OnUpdate)
}

func TestService_Check_PassFail(t *testing.T) {
	testutil.Parallel(t)

	job := &Job{Name: pointerOf("job")}
	tg := &TaskGroup{Name: pointerOf("group")}
	task := &Task{Name: "task"}

	t.Run("enforce minimums", func(t *testing.T) {
		s := &Service{
			Checks: []ServiceCheck{{
				SuccessBeforePassing:   -1,
				FailuresBeforeCritical: -2,
			}},
		}

		s.Canonicalize(task, tg, job)
		must.Zero(t, s.Checks[0].SuccessBeforePassing)
		must.Zero(t, s.Checks[0].FailuresBeforeCritical)
	})

	t.Run("normal", func(t *testing.T) {
		s := &Service{
			Checks: []ServiceCheck{{
				SuccessBeforePassing:   3,
				FailuresBeforeCritical: 4,
			}},
		}

		s.Canonicalize(task, tg, job)
		must.Eq(t, 3, s.Checks[0].SuccessBeforePassing)
		must.Eq(t, 4, s.Checks[0].FailuresBeforeCritical)
	})
}

// TestService_CheckRestart asserts Service.CheckRestart settings are properly
// inherited by Checks.
func TestService_CheckRestart(t *testing.T) {
	testutil.Parallel(t)

	job := &Job{Name: pointerOf("job")}
	tg := &TaskGroup{Name: pointerOf("group")}
	task := &Task{Name: "task"}
	service := &Service{
		CheckRestart: &CheckRestart{
			Limit:          11,
			Grace:          pointerOf(11 * time.Second),
			IgnoreWarnings: true,
		},
		Checks: []ServiceCheck{
			{
				Name: "all-set",
				CheckRestart: &CheckRestart{
					Limit:          22,
					Grace:          pointerOf(22 * time.Second),
					IgnoreWarnings: true,
				},
			},
			{
				Name: "some-set",
				CheckRestart: &CheckRestart{
					Limit: 33,
					Grace: pointerOf(33 * time.Second),
				},
			},
			{
				Name: "unset",
			},
		},
	}

	service.Canonicalize(task, tg, job)
	must.Eq(t, 22, service.Checks[0].CheckRestart.Limit)
	must.Eq(t, 22*time.Second, *service.Checks[0].CheckRestart.Grace)
	must.True(t, service.Checks[0].CheckRestart.IgnoreWarnings)

	must.Eq(t, 33, service.Checks[1].CheckRestart.Limit)
	must.Eq(t, 33*time.Second, *service.Checks[1].CheckRestart.Grace)
	must.True(t, service.Checks[1].CheckRestart.IgnoreWarnings)

	must.Eq(t, 11, service.Checks[2].CheckRestart.Limit)
	must.Eq(t, 11*time.Second, *service.Checks[2].CheckRestart.Grace)
	must.True(t, service.Checks[2].CheckRestart.IgnoreWarnings)
}

func TestService_Connect_proxy_settings(t *testing.T) {
	testutil.Parallel(t)

	job := &Job{Name: pointerOf("job")}
	tg := &TaskGroup{Name: pointerOf("group")}
	task := &Task{Name: "task"}
	service := &Service{
		Connect: &ConsulConnect{
			SidecarService: &ConsulSidecarService{
				Proxy: &ConsulProxy{
					Upstreams: []*ConsulUpstream{
						{
							DestinationName:  "upstream",
							LocalBindPort:    80,
							Datacenter:       "dc2",
							LocalBindAddress: "127.0.0.2",
						},
					},
					LocalServicePort: 8000,
				},
			},
		},
	}

	service.Canonicalize(task, tg, job)
	proxy := service.Connect.SidecarService.Proxy
	must.Eq(t, "upstream", proxy.Upstreams[0].DestinationName)
	must.Eq(t, 80, proxy.Upstreams[0].LocalBindPort)
	must.Eq(t, "dc2", proxy.Upstreams[0].Datacenter)
	must.Eq(t, "127.0.0.2", proxy.Upstreams[0].LocalBindAddress)
	must.Eq(t, 8000, proxy.LocalServicePort)
}

func TestService_Tags(t *testing.T) {
	testutil.Parallel(t)

	// canonicalize does not modify eto or tags
	job := &Job{Name: pointerOf("job")}
	tg := &TaskGroup{Name: pointerOf("group")}
	task := &Task{Name: "task"}
	service := &Service{
		Tags:              []string{"a", "b"},
		CanaryTags:        []string{"c", "d"},
		EnableTagOverride: true,
	}

	service.Canonicalize(task, tg, job)
	must.True(t, service.EnableTagOverride)
	must.Eq(t, []string{"a", "b"}, service.Tags)
	must.Eq(t, []string{"c", "d"}, service.CanaryTags)
}
