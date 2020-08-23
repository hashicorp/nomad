package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestService_Check_PassFail(t *testing.T) {
	t.Parallel()

	job := &Job{Name: stringToPtr("job")}
	tg := &TaskGroup{Name: stringToPtr("group")}
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
	t.Parallel()

	job := &Job{Name: stringToPtr("job")}
	tg := &TaskGroup{Name: stringToPtr("group")}
	task := &Task{Name: "task"}
	service := &Service{
		CheckRestart: &CheckRestart{
			Limit:          11,
			Grace:          timeToPtr(11 * time.Second),
			IgnoreWarnings: true,
		},
		Checks: []ServiceCheck{
			{
				Name: "all-set",
				CheckRestart: &CheckRestart{
					Limit:          22,
					Grace:          timeToPtr(22 * time.Second),
					IgnoreWarnings: true,
				},
			},
			{
				Name: "some-set",
				CheckRestart: &CheckRestart{
					Limit: 33,
					Grace: timeToPtr(33 * time.Second),
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

func TestService_Connect_Canonicalize(t *testing.T) {
	t.Parallel()

	t.Run("nil connect", func(t *testing.T) {
		cc := (*ConsulConnect)(nil)
		cc.Canonicalize()
		require.Nil(t, cc)
	})

	t.Run("empty connect", func(t *testing.T) {
		cc := new(ConsulConnect)
		cc.Canonicalize()
		require.Empty(t, cc.Native)
		require.Nil(t, cc.SidecarService)
		require.Nil(t, cc.SidecarTask)
	})
}

func TestService_Connect_ConsulSidecarService_Canonicalize(t *testing.T) {
	t.Parallel()

	t.Run("nil sidecar_service", func(t *testing.T) {
		css := (*ConsulSidecarService)(nil)
		css.Canonicalize()
		require.Nil(t, css)
	})

	t.Run("empty sidecar_service", func(t *testing.T) {
		css := new(ConsulSidecarService)
		css.Canonicalize()
		require.Empty(t, css.Tags)
		require.Nil(t, css.Proxy)
	})

	t.Run("non-empty sidecar_service", func(t *testing.T) {
		css := &ConsulSidecarService{
			Tags: make([]string, 0),
			Port: "port",
			Proxy: &ConsulProxy{
				LocalServiceAddress: "lsa",
				LocalServicePort:    80,
			},
		}
		css.Canonicalize()
		require.Equal(t, &ConsulSidecarService{
			Tags: nil,
			Port: "port",
			Proxy: &ConsulProxy{
				LocalServiceAddress: "lsa",
				LocalServicePort:    80},
		}, css)
	})
}

func TestService_Connect_ConsulProxy_Canonicalize(t *testing.T) {
	t.Parallel()

	t.Run("nil proxy", func(t *testing.T) {
		cp := (*ConsulProxy)(nil)
		cp.Canonicalize()
		require.Nil(t, cp)
	})

	t.Run("empty proxy", func(t *testing.T) {
		cp := new(ConsulProxy)
		cp.Canonicalize()
		require.Empty(t, cp.LocalServiceAddress)
		require.Zero(t, cp.LocalServicePort)
		require.Nil(t, cp.ExposeConfig)
		require.Nil(t, cp.Upstreams)
		require.Empty(t, cp.Config)
	})

	t.Run("non empty proxy", func(t *testing.T) {
		cp := &ConsulProxy{
			LocalServiceAddress: "127.0.0.1",
			LocalServicePort:    80,
			ExposeConfig:        new(ConsulExposeConfig),
			Upstreams:           make([]*ConsulUpstream, 0),
			Config:              make(map[string]interface{}),
		}
		cp.Canonicalize()
		require.Equal(t, "127.0.0.1", cp.LocalServiceAddress)
		require.Equal(t, 80, cp.LocalServicePort)
		require.Equal(t, &ConsulExposeConfig{}, cp.ExposeConfig)
		require.Nil(t, cp.Upstreams)
		require.Nil(t, cp.Config)
	})
}

func TestService_Connect_proxy_settings(t *testing.T) {
	t.Parallel()

	job := &Job{Name: stringToPtr("job")}
	tg := &TaskGroup{Name: stringToPtr("group")}
	task := &Task{Name: "task"}
	service := &Service{
		Connect: &ConsulConnect{
			SidecarService: &ConsulSidecarService{
				Proxy: &ConsulProxy{
					Upstreams: []*ConsulUpstream{
						{
							DestinationName: "upstream",
							LocalBindPort:   80,
						},
					},
					LocalServicePort: 8000,
				},
			},
		},
	}

	service.Canonicalize(task, tg, job)
	proxy := service.Connect.SidecarService.Proxy
	require.Equal(t, proxy.Upstreams[0].LocalBindPort, 80)
	require.Equal(t, proxy.Upstreams[0].DestinationName, "upstream")
	require.Equal(t, proxy.LocalServicePort, 8000)
}

func TestService_Tags(t *testing.T) {
	t.Parallel()
	r := require.New(t)

	// canonicalize does not modify eto or tags
	job := &Job{Name: stringToPtr("job")}
	tg := &TaskGroup{Name: stringToPtr("group")}
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

func TestService_Connect_SidecarTask_Canonicalize(t *testing.T) {
	t.Parallel()

	t.Run("nil sidecar_task", func(t *testing.T) {
		st := (*SidecarTask)(nil)
		st.Canonicalize()
		require.Nil(t, st)
	})

	t.Run("empty sidecar_task", func(t *testing.T) {
		st := new(SidecarTask)
		st.Canonicalize()
		require.Nil(t, st.Config)
		require.Nil(t, st.Env)
		require.Equal(t, DefaultResources(), st.Resources)
		require.Equal(t, DefaultLogConfig(), st.LogConfig)
		require.Nil(t, st.Meta)
		require.Equal(t, 5*time.Second, *st.KillTimeout)
		require.Equal(t, 0*time.Second, *st.ShutdownDelay)
	})

	t.Run("non empty sidecar_task resources", func(t *testing.T) {
		exp := DefaultResources()
		exp.MemoryMB = intToPtr(333)
		st := &SidecarTask{
			Resources: &Resources{MemoryMB: intToPtr(333)},
		}
		st.Canonicalize()
		require.Equal(t, exp, st.Resources)
	})
}
