package api

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/stretchr/testify/require"
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
	s := &Service{}

	s.Canonicalize(task, tg, j)

	require.Equal(t, fmt.Sprintf("%s-%s-%s", *j.Name, *tg.Name, task.Name), s.Name)
	require.Equal(t, "auto", s.AddressMode)
	require.Equal(t, OnUpdateRequireHealthy, s.OnUpdate)
	require.Equal(t, ServiceProviderConsul, s.Provider)
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

	require.Equal(t, OnUpdateRequireHealthy, s.Checks[0].OnUpdate)
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
		require.Zero(t, s.Checks[0].SuccessBeforePassing)
		require.Zero(t, s.Checks[0].FailuresBeforeCritical)
	})

	t.Run("normal", func(t *testing.T) {
		s := &Service{
			Checks: []ServiceCheck{{
				SuccessBeforePassing:   3,
				FailuresBeforeCritical: 4,
			}},
		}

		s.Canonicalize(task, tg, job)
		require.Equal(t, 3, s.Checks[0].SuccessBeforePassing)
		require.Equal(t, 4, s.Checks[0].FailuresBeforeCritical)
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
	require.Equal(t, service.Checks[0].CheckRestart.Limit, 22)
	require.Equal(t, *service.Checks[0].CheckRestart.Grace, 22*time.Second)
	require.True(t, service.Checks[0].CheckRestart.IgnoreWarnings)

	require.Equal(t, service.Checks[1].CheckRestart.Limit, 33)
	require.Equal(t, *service.Checks[1].CheckRestart.Grace, 33*time.Second)
	require.True(t, service.Checks[1].CheckRestart.IgnoreWarnings)

	require.Equal(t, service.Checks[2].CheckRestart.Limit, 11)
	require.Equal(t, *service.Checks[2].CheckRestart.Grace, 11*time.Second)
	require.True(t, service.Checks[2].CheckRestart.IgnoreWarnings)
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
	require.Equal(t, proxy.Upstreams[0].DestinationName, "upstream")
	require.Equal(t, proxy.Upstreams[0].LocalBindPort, 80)
	require.Equal(t, proxy.Upstreams[0].Datacenter, "dc2")
	require.Equal(t, proxy.Upstreams[0].LocalBindAddress, "127.0.0.2")
	require.Equal(t, proxy.LocalServicePort, 8000)
}

func TestService_Tags(t *testing.T) {
	testutil.Parallel(t)
	r := require.New(t)

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
	r.True(service.EnableTagOverride)
	r.Equal([]string{"a", "b"}, service.Tags)
	r.Equal([]string{"c", "d"}, service.CanaryTags)
}