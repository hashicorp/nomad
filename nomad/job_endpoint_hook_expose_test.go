package nomad

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestJobExposeHook_Name(t *testing.T) {
	t.Parallel()

	require.Equal(t, "expose", new(jobExposeHook).Name())
}

func TestJobExposeHook_tgUsesBridgeNetwork(t *testing.T) {
	t.Parallel()

	t.Run("uses bridge", func(t *testing.T) {
		mode, name, result := tgUsesBridgeNetwork(&structs.TaskGroup{
			Name: "group",
			Networks: structs.Networks{{
				Mode: "bridge",
			}},
		})
		require.True(t, result)
		require.Equal(t, "bridge", mode)
		require.Equal(t, "group", name)
	})
	t.Run("uses host", func(t *testing.T) {
		mode, name, result := tgUsesBridgeNetwork(&structs.TaskGroup{
			Name: "group",
			Networks: structs.Networks{{
				Mode: "host",
			}},
		})
		require.False(t, result)
		require.Equal(t, "host", mode)
		require.Equal(t, "group", name)
	})
}

func TestJobExposeHook_serviceExposeConfig(t *testing.T) {
	t.Parallel()

	t.Run("nil service", func(t *testing.T) {
		require.Nil(t, serviceExposeConfig(nil))
	})
	t.Run("nil connect", func(t *testing.T) {
		require.Nil(t, serviceExposeConfig(&structs.Service{
			Connect: nil,
		}))
	})
	t.Run("nil sidecar", func(t *testing.T) {
		require.Nil(t, serviceExposeConfig(&structs.Service{
			Connect: &structs.ConsulConnect{
				SidecarService: nil,
			}}))
	})
	t.Run("nil proxy", func(t *testing.T) {
		require.Nil(t, serviceExposeConfig(&structs.Service{
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{
					Proxy: nil,
				}}}))
	})
	t.Run("expose", func(t *testing.T) {
		require.True(t, serviceExposeConfig(&structs.Service{
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{
					Proxy: &structs.ConsulProxy{
						Expose: &structs.ConsulExposeConfig{
							Checks: true,
						}}}}}).Checks)
	})
}

func TestJobExposeHook_serviceEnablesExposeChecks(t *testing.T) {
	t.Parallel()

	t.Run("expose checks true", func(t *testing.T) {
		require.True(t, serviceEnablesExposeChecks(&structs.Service{
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{
					Proxy: &structs.ConsulProxy{
						Expose: &structs.ConsulExposeConfig{
							Checks: true,
						}}}}}))
	})
	t.Run("expose checks false", func(t *testing.T) {
		require.False(t, serviceEnablesExposeChecks(&structs.Service{
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{
					Proxy: &structs.ConsulProxy{
						Expose: &structs.ConsulExposeConfig{
							Checks: false,
						}}}}}))
	})
}

func TestJobExposeHook_serviceEnablesExpose(t *testing.T) {
	t.Parallel()

	t.Run("no expose", func(t *testing.T) {
		require.False(t, serviceEnablesExpose(&structs.Service{
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{
					Proxy: &structs.ConsulProxy{
						Expose: &structs.ConsulExposeConfig{
							Checks: false,
							Paths:  nil,
						}}}}}))
	})

	t.Run("expose checks", func(t *testing.T) {
		require.True(t, serviceEnablesExpose(&structs.Service{
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{
					Proxy: &structs.ConsulProxy{
						Expose: &structs.ConsulExposeConfig{
							Checks: true,
							Paths:  nil,
						}}}}}))
	})

	t.Run("expose paths", func(t *testing.T) {
		require.True(t, serviceEnablesExpose(&structs.Service{
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{
					Proxy: &structs.ConsulProxy{
						Expose: &structs.ConsulExposeConfig{
							Checks: false,
							Paths: []structs.ConsulExposePath{{
								Path: "/example",
							}},
						}}}}}))
	})
}

func TestJobExposeHook_checkIsExposable(t *testing.T) {
	t.Parallel()

	t.Run("type http", func(t *testing.T) {
		require.True(t, checkIsExposable(&structs.ServiceCheck{
			Type: "http",
			Path: "/path",
		}))
	})

	t.Run("type grpc", func(t *testing.T) {
		require.True(t, checkIsExposable(&structs.ServiceCheck{
			Type: "grpc",
			Path: "/path",
		}))
	})

	t.Run("type http no path", func(t *testing.T) {
		require.False(t, checkIsExposable(&structs.ServiceCheck{
			Type: "http",
		}))
	})

	t.Run("type tcp", func(t *testing.T) {
		require.False(t, checkIsExposable(&structs.ServiceCheck{
			Type: "tcp",
		}))
	})
}

func TestJobExposeHook_tgEnablesExpose(t *testing.T) {
	t.Parallel()

	tg := &structs.TaskGroup{
		Services: []*structs.Service{
			{Name: "no_expose"},
			{
				Name: "with_expose", Connect: &structs.ConsulConnect{
					SidecarService: &structs.ConsulSidecarService{
						Proxy: &structs.ConsulProxy{
							Expose: &structs.ConsulExposeConfig{
								Checks: true,
							}}}},
			},
		},
	}

	t.Run("tg enables expose", func(t *testing.T) {
		require.True(t, tgEnablesExpose(tg))
	})

	t.Run("tg does not enable expose", func(t *testing.T) {
		tg.Services[1].Connect.SidecarService.Proxy.Expose.Checks = false
		require.False(t, tgEnablesExpose(tg))
	})
}

func TestJobExposeHook_containsExposePath(t *testing.T) {
	t.Parallel()

	t.Run("contains path", func(t *testing.T) {
		require.True(t, containsExposePath([]structs.ConsulExposePath{{
			Path:          "/v2/health",
			Protocol:      "grpc",
			LocalPathPort: 8080,
			ListenerPort:  "v2Port",
		}, {
			Path:          "/health",
			Protocol:      "http",
			LocalPathPort: 8080,
			ListenerPort:  "hcPort",
		}}, structs.ConsulExposePath{
			Path:          "/health",
			Protocol:      "http",
			LocalPathPort: 8080,
			ListenerPort:  "hcPort",
		}))
	})

	t.Run("no such path", func(t *testing.T) {
		require.False(t, containsExposePath([]structs.ConsulExposePath{{
			Path:          "/v2/health",
			Protocol:      "grpc",
			LocalPathPort: 8080,
			ListenerPort:  "v2Port",
		}, {
			Path:          "/health",
			Protocol:      "http",
			LocalPathPort: 8080,
			ListenerPort:  "hcPort",
		}}, structs.ConsulExposePath{
			Path:          "/v3/health",
			Protocol:      "http",
			LocalPathPort: 8080,
			ListenerPort:  "hcPort",
		}))
	})
}

func TestJobExposeHook_exposePathForCheck(t *testing.T) {
	t.Parallel()

	t.Run("not expose compatible", func(t *testing.T) {
		c := &structs.ServiceCheck{
			Type: "tcp", // not expose compatible
		}
		s := &structs.Service{
			Checks: []*structs.ServiceCheck{c},
		}
		ePath, err := exposePathForCheck(&structs.TaskGroup{
			Services: []*structs.Service{s},
		}, s, c)
		require.NoError(t, err)
		require.Nil(t, ePath)
	})

	t.Run("direct port", func(t *testing.T) {
		c := &structs.ServiceCheck{
			Name:      "check1",
			Type:      "http",
			Path:      "/health",
			PortLabel: "hcPort",
		}
		s := &structs.Service{
			Name:      "service1",
			PortLabel: "4000",
			Checks:    []*structs.ServiceCheck{c},
		}
		ePath, err := exposePathForCheck(&structs.TaskGroup{
			Name:     "group1",
			Services: []*structs.Service{s},
		}, s, c)
		require.NoError(t, err)
		require.Equal(t, &structs.ConsulExposePath{
			Path:          "/health",
			Protocol:      "", // often blank, consul does the Right Thing
			LocalPathPort: 4000,
			ListenerPort:  "hcPort",
		}, ePath)
	})

	t.Run("labeled port", func(t *testing.T) {
		c := &structs.ServiceCheck{
			Name:      "check1",
			Type:      "http",
			Path:      "/health",
			PortLabel: "hcPort",
		}
		s := &structs.Service{
			Name:      "service1",
			PortLabel: "sPort", // port label indirection
			Checks:    []*structs.ServiceCheck{c},
		}
		ePath, err := exposePathForCheck(&structs.TaskGroup{
			Name:     "group1",
			Services: []*structs.Service{s},
			Networks: structs.Networks{{
				Mode: "bridge",
				DynamicPorts: []structs.Port{
					{Label: "sPort", Value: 4000},
				},
			}},
		}, s, c)
		require.NoError(t, err)
		require.Equal(t, &structs.ConsulExposePath{
			Path:          "/health",
			Protocol:      "",
			LocalPathPort: 4000,
			ListenerPort:  "hcPort",
		}, ePath)
	})

	t.Run("missing port", func(t *testing.T) {
		c := &structs.ServiceCheck{
			Name:      "check1",
			Type:      "http",
			Path:      "/health",
			PortLabel: "hcPort",
		}
		s := &structs.Service{
			Name:      "service1",
			PortLabel: "sPort", // port label indirection
			Checks:    []*structs.ServiceCheck{c},
		}
		_, err := exposePathForCheck(&structs.TaskGroup{
			Name:     "group1",
			Services: []*structs.Service{s},
			Networks: structs.Networks{{
				Mode:         "bridge",
				DynamicPorts: []structs.Port{
					// service declares "sPort", but does not exist
				},
			}},
		}, s, c)
		require.EqualError(t, err, `unable to determine local service port for check "check1" of service "service1" in group "group1"`)
	})
}

func TestJobExposeHook_Validate(t *testing.T) {
	t.Parallel()

	t.Run("expose without bridge", func(t *testing.T) {
		warnings, err := new(jobExposeHook).Validate(&structs.Job{
			TaskGroups: []*structs.TaskGroup{{
				Name: "group",
				Networks: structs.Networks{{
					Mode: "host",
				}},
				Services: []*structs.Service{{
					Name: "service",
					Connect: &structs.ConsulConnect{
						SidecarService: &structs.ConsulSidecarService{
							Proxy: &structs.ConsulProxy{
								Expose: &structs.ConsulExposeConfig{
									Checks: true,
								}}}}}}}},
		})
		require.Empty(t, warnings)
		require.EqualError(t, err, `expose configuration requires bridge network, found "host" network in task group "group"`)
	})

	t.Run("valid", func(t *testing.T) {
		warnings, err := new(jobExposeHook).Validate(&structs.Job{
			TaskGroups: []*structs.TaskGroup{{
				Name: "group",
				Networks: structs.Networks{{
					Mode: "bridge",
				}},
				Services: []*structs.Service{{
					Name: "service",
					Connect: &structs.ConsulConnect{
						SidecarService: &structs.ConsulSidecarService{
							Proxy: &structs.ConsulProxy{
								Expose: &structs.ConsulExposeConfig{
									Checks: true,
								}}}}}}}},
		})
		require.Empty(t, warnings)
		require.NoError(t, err)
	})
}

func TestJobExposeHook_Mutate(t *testing.T) {
	t.Parallel()

	result, warnings, err := new(jobExposeHook).Mutate(&structs.Job{
		TaskGroups: []*structs.TaskGroup{{
			Name: "group1",
			Services: []*structs.Service{{
				Name:      "service1",
				PortLabel: "8080",
				Checks: []*structs.ServiceCheck{{
					Name:      "check1",
					Type:      "tcp",
					PortLabel: "5000",
				}, {
					Name:      "check2",
					Type:      "http",
					Protocol:  "http",
					PortLabel: "forChecks",
					Path:      "/health",
				}, {
					Name:      "check3",
					Type:      "grpc",
					Protocol:  "grpc",
					PortLabel: "forChecks",
					Path:      "/v2/health",
				}},
				Connect: &structs.ConsulConnect{
					SidecarService: &structs.ConsulSidecarService{
						Proxy: &structs.ConsulProxy{
							Expose: &structs.ConsulExposeConfig{
								Checks: true,
								Paths: []structs.ConsulExposePath{{
									Path:          "/pre-existing",
									Protocol:      "http",
									LocalPathPort: 9000,
									ListenerPort:  "otherPort",
								}}}}}}}}}},
	})

	require.NoError(t, err)
	require.Empty(t, warnings)
	require.Equal(t, []structs.ConsulExposePath{{
		Path:          "/pre-existing",
		Protocol:      "http",
		LocalPathPort: 9000,
		ListenerPort:  "otherPort",
	}, {
		Path:          "/health",
		Protocol:      "http",
		LocalPathPort: 8080,
		ListenerPort:  "forChecks",
	}, {
		Path:          "/v2/health",
		Protocol:      "grpc",
		LocalPathPort: 8080,
		ListenerPort:  "forChecks",
	}}, result.TaskGroups[0].Services[0].Connect.SidecarService.Proxy.Expose.Paths)
}
