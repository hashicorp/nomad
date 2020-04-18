package nomad

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestJobExposeCheckHook_Name(t *testing.T) {
	t.Parallel()

	require.Equal(t, "expose-check", new(jobExposeCheckHook).Name())
}

func TestJobExposeCheckHook_serviceUsesConnectEnvoy(t *testing.T) {
	t.Parallel()

	t.Run("connect is nil", func(t *testing.T) {
		require.False(t, serviceUsesConnectEnvoy(&structs.Service{
			Connect: nil,
		}))
	})

	t.Run("sidecar-task is overridden", func(t *testing.T) {
		require.False(t, serviceUsesConnectEnvoy(&structs.Service{
			Connect: &structs.ConsulConnect{
				SidecarTask: &structs.SidecarTask{
					Name: "my-sidecar",
				},
			},
		}))
	})

	t.Run("sidecar-task is nil", func(t *testing.T) {
		require.True(t, serviceUsesConnectEnvoy(&structs.Service{
			Connect: &structs.ConsulConnect{
				SidecarTask: nil,
			},
		}))
	})
}

func TestJobExposeCheckHook_tgUsesExposeCheck(t *testing.T) {
	t.Parallel()

	t.Run("no check.expose", func(t *testing.T) {
		require.False(t, tgUsesExposeCheck(&structs.TaskGroup{
			Services: []*structs.Service{{
				Checks: []*structs.ServiceCheck{{
					Expose: false,
				}},
			}},
		}))
	})

	t.Run("with check.expose", func(t *testing.T) {
		require.True(t, tgUsesExposeCheck(&structs.TaskGroup{
			Services: []*structs.Service{{
				Checks: []*structs.ServiceCheck{{
					Expose: false,
				}, {
					Expose: true,
				}},
			}},
		}))
	})
}

func TestJobExposeCheckHook_tgValidateUseOfBridgeMode(t *testing.T) {
	t.Parallel()

	s1 := &structs.Service{
		Name: "s1",
		Checks: []*structs.ServiceCheck{{
			Name:      "s1-check1",
			Type:      "http",
			PortLabel: "health",
			Expose:    true,
		}},
	}

	t.Run("no networks but no use of expose", func(t *testing.T) {
		require.Nil(t, tgValidateUseOfBridgeMode(&structs.TaskGroup{
			Networks: make(structs.Networks, 0),
		}))
	})

	t.Run("no networks and uses expose", func(t *testing.T) {
		require.EqualError(t, tgValidateUseOfBridgeMode(&structs.TaskGroup{
			Name:     "g1",
			Networks: make(structs.Networks, 0),
			Services: []*structs.Service{s1},
		}), `group "g1" must specify one bridge network for exposing service check(s)`)
	})

	t.Run("non-bridge network and uses expose", func(t *testing.T) {
		require.EqualError(t, tgValidateUseOfBridgeMode(&structs.TaskGroup{
			Name: "g1",
			Networks: structs.Networks{{
				Mode: "host",
			}},
			Services: []*structs.Service{s1},
		}), `group "g1" must use bridge network for exposing service check(s)`)
	})

	t.Run("bridge network uses expose", func(t *testing.T) {
		require.Nil(t, tgValidateUseOfBridgeMode(&structs.TaskGroup{
			Name: "g1",
			Networks: structs.Networks{{
				Mode: "bridge",
			}},
			Services: []*structs.Service{s1},
		}))
	})
}

func TestJobExposeCheckHook_tgValidateUseOfCheckExpose(t *testing.T) {
	t.Parallel()

	withCustomProxyTask := &structs.Service{
		Name: "s1",
		Connect: &structs.ConsulConnect{
			SidecarTask: &structs.SidecarTask{Name: "custom"},
		},
		Checks: []*structs.ServiceCheck{{
			Name:      "s1-check1",
			Type:      "http",
			PortLabel: "health",
			Expose:    true,
		}},
	}

	t.Run("group-service uses custom proxy", func(t *testing.T) {
		require.EqualError(t, tgValidateUseOfCheckExpose(&structs.TaskGroup{
			Name:     "g1",
			Services: []*structs.Service{withCustomProxyTask},
		}), `exposed service check g1->s1->s1-check1 requires use of Nomad's builtin Connect proxy`)
	})

	t.Run("group-service uses custom proxy but no expose", func(t *testing.T) {
		withCustomProxyTaskNoExpose := &(*withCustomProxyTask)
		withCustomProxyTask.Checks[0].Expose = false
		require.Nil(t, tgValidateUseOfCheckExpose(&structs.TaskGroup{
			Name:     "g1",
			Services: []*structs.Service{withCustomProxyTaskNoExpose},
		}))
	})

	t.Run("task-service sets expose", func(t *testing.T) {
		require.EqualError(t, tgValidateUseOfCheckExpose(&structs.TaskGroup{
			Name: "g1",
			Tasks: []*structs.Task{{
				Name: "t1",
				Services: []*structs.Service{{
					Name: "s2",
					Checks: []*structs.ServiceCheck{{
						Name:   "check1",
						Type:   "http",
						Expose: true,
					}},
				}},
			}},
		}), `exposed service check g1[t1]->s2->check1 is not a task-group service`)
	})
}

func TestJobExposeCheckHook_Validate(t *testing.T) {
	s1 := &structs.Service{
		Name: "s1",
		Checks: []*structs.ServiceCheck{{
			Name:   "s1-check1",
			Type:   "http",
			Expose: true,
		}},
	}

	t.Run("double network", func(t *testing.T) {
		warnings, err := new(jobExposeCheckHook).Validate(&structs.Job{
			TaskGroups: []*structs.TaskGroup{{
				Name: "g1",
				Networks: structs.Networks{{
					Mode: "bridge",
				}, {
					Mode: "bridge",
				}},
				Services: []*structs.Service{s1},
			}},
		})
		require.Empty(t, warnings)
		require.EqualError(t, err, `group "g1" must specify one bridge network for exposing service check(s)`)
	})

	t.Run("expose in service check", func(t *testing.T) {
		warnings, err := new(jobExposeCheckHook).Validate(&structs.Job{
			TaskGroups: []*structs.TaskGroup{{
				Name: "g1",
				Networks: structs.Networks{{
					Mode: "bridge",
				}},
				Tasks: []*structs.Task{{
					Name: "t1",
					Services: []*structs.Service{{
						Name: "s2",
						Checks: []*structs.ServiceCheck{{
							Name:   "s2-check1",
							Type:   "http",
							Expose: true,
						}},
					}},
				}},
			}},
		})
		require.Empty(t, warnings)
		require.EqualError(t, err, `exposed service check g1[t1]->s2->s2-check1 is not a task-group service`)
	})

	t.Run("ok", func(t *testing.T) {
		warnings, err := new(jobExposeCheckHook).Validate(&structs.Job{
			TaskGroups: []*structs.TaskGroup{{
				Name: "g1",
				Networks: structs.Networks{{
					Mode: "bridge",
				}},
				Services: []*structs.Service{{
					Name:    "s1",
					Connect: &structs.ConsulConnect{},
					Checks: []*structs.ServiceCheck{{
						Name:   "check1",
						Type:   "http",
						Expose: true,
					}},
				}},
				Tasks: []*structs.Task{{
					Name: "t1",
					Services: []*structs.Service{{
						Name: "s2",
						Checks: []*structs.ServiceCheck{{
							Name:   "s2-check1",
							Type:   "http",
							Expose: false,
						}},
					}},
				}},
			}},
		})
		require.Empty(t, warnings)
		require.Nil(t, err)
	})
}

func TestJobExposeCheckHook_exposePathForCheck(t *testing.T) {
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
		require.EqualError(t, err, `unable to determine local service port for service check group1->service1->check1`)
	})
}

func TestJobExposeCheckHook_containsExposePath(t *testing.T) {
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

func TestJobExposeCheckHook_serviceExposeConfig(t *testing.T) {
	t.Parallel()

	t.Run("proxy is nil", func(t *testing.T) {
		require.NotNil(t, serviceExposeConfig(&structs.Service{
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{},
			},
		}))
	})

	t.Run("expose is nil", func(t *testing.T) {
		require.NotNil(t, serviceExposeConfig(&structs.Service{
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{
					Proxy: &structs.ConsulProxy{},
				},
			},
		}))
	})

	t.Run("expose pre-existing", func(t *testing.T) {
		exposeConfig := serviceExposeConfig(&structs.Service{
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{
					Proxy: &structs.ConsulProxy{
						Expose: &structs.ConsulExposeConfig{
							Paths: []structs.ConsulExposePath{{
								Path: "/health",
							}},
						},
					},
				},
			},
		})
		require.NotNil(t, exposeConfig)
		require.Equal(t, []structs.ConsulExposePath{{
			Path: "/health",
		}}, exposeConfig.Paths)
	})

	t.Run("append to paths is safe", func(t *testing.T) {
		// double check that serviceExposeConfig(s).Paths can be appended to
		// from a derived pointer without fear of the original underlying array
		// pointer being lost

		s := &structs.Service{
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{
					Proxy: &structs.ConsulProxy{
						Expose: &structs.ConsulExposeConfig{
							Paths: []structs.ConsulExposePath{{
								Path: "/one",
							}},
						},
					},
				},
			},
		}

		exposeConfig := serviceExposeConfig(s)
		exposeConfig.Paths = append(exposeConfig.Paths,
			structs.ConsulExposePath{Path: "/two"},
			structs.ConsulExposePath{Path: "/three"},
			structs.ConsulExposePath{Path: "/four"},
			structs.ConsulExposePath{Path: "/five"},
			structs.ConsulExposePath{Path: "/six"},
			structs.ConsulExposePath{Path: "/seven"},
			structs.ConsulExposePath{Path: "/eight"},
			structs.ConsulExposePath{Path: "/nine"},
		)

		// works, because exposeConfig.Paths gets re-assigned into exposeConfig
		// which is a pointer, meaning the field is modified also from the
		// service struct's perspective
		require.Equal(t, 9, len(s.Connect.SidecarService.Proxy.Expose.Paths))
	})
}

func TestJobExposeCheckHook_checkIsExposable(t *testing.T) {
	t.Parallel()

	t.Run("grpc", func(t *testing.T) {
		require.True(t, checkIsExposable(&structs.ServiceCheck{
			Type: "grpc",
			Path: "/health",
		}))
		require.True(t, checkIsExposable(&structs.ServiceCheck{
			Type: "gRPC",
			Path: "/health",
		}))
	})

	t.Run("http", func(t *testing.T) {
		require.True(t, checkIsExposable(&structs.ServiceCheck{
			Type: "http",
			Path: "/health",
		}))
		require.True(t, checkIsExposable(&structs.ServiceCheck{
			Type: "HTTP",
			Path: "/health",
		}))
	})

	t.Run("tcp", func(t *testing.T) {
		require.False(t, checkIsExposable(&structs.ServiceCheck{
			Type: "tcp",
			Path: "/health",
		}))
	})

	t.Run("no path slash prefix", func(t *testing.T) {
		require.False(t, checkIsExposable(&structs.ServiceCheck{
			Type: "http",
			Path: "health",
		}))
	})
}

func TestJobExposeCheckHook_Mutate(t *testing.T) {
	t.Parallel()

	t.Run("typical", func(t *testing.T) {
		result, warnings, err := new(jobExposeCheckHook).Mutate(&structs.Job{
			TaskGroups: []*structs.TaskGroup{{
				Name: "group0",
				Networks: structs.Networks{{
					Mode: "host",
				}},
			}, {
				Name: "group1",
				Networks: structs.Networks{{
					Mode: "bridge",
				}},
				Services: []*structs.Service{{
					Name:      "service1",
					PortLabel: "8000",
					Checks: []*structs.ServiceCheck{{
						Name:      "check1",
						Type:      "tcp",
						PortLabel: "8100",
					}, {
						Name:      "check2",
						Type:      "http",
						PortLabel: "health",
						Path:      "/health",
						Expose:    true,
					}, {
						Name:      "check3",
						Type:      "grpc",
						PortLabel: "health",
						Path:      "/v2/health",
						Expose:    true,
					}},
					Connect: &structs.ConsulConnect{
						SidecarService: &structs.ConsulSidecarService{
							Proxy: &structs.ConsulProxy{
								Expose: &structs.ConsulExposeConfig{
									Paths: []structs.ConsulExposePath{{
										Path:          "/pre-existing",
										Protocol:      "http",
										LocalPathPort: 9000,
										ListenerPort:  "otherPort",
									}}}}}}}, {
					Name:      "service2",
					PortLabel: "3000",
					Checks: []*structs.ServiceCheck{{
						Name:      "check1",
						Type:      "grpc",
						Protocol:  "http2",
						Path:      "/ok",
						PortLabel: "health",
						Expose:    true,
					}},
					Connect: &structs.ConsulConnect{
						SidecarService: &structs.ConsulSidecarService{
							Proxy: &structs.ConsulProxy{},
						},
					},
				}}}},
		})

		require.NoError(t, err)
		require.Empty(t, warnings)
		require.Equal(t, []structs.ConsulExposePath{{
			Path:          "/pre-existing",
			LocalPathPort: 9000,
			Protocol:      "http",
			ListenerPort:  "otherPort",
		}, {
			Path:          "/health",
			LocalPathPort: 8000,
			ListenerPort:  "health",
		}, {
			Path:          "/v2/health",
			LocalPathPort: 8000,
			ListenerPort:  "health",
		}}, result.TaskGroups[1].Services[0].Connect.SidecarService.Proxy.Expose.Paths)
		require.Equal(t, []structs.ConsulExposePath{{
			Path:          "/ok",
			LocalPathPort: 3000,
			Protocol:      "http2",
			ListenerPort:  "health",
		}}, result.TaskGroups[1].Services[1].Connect.SidecarService.Proxy.Expose.Paths)
	})
}
