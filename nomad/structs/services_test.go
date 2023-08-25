// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func TestServiceCheck_Hash(t *testing.T) {
	ci.Parallel(t)

	original := &ServiceCheck{
		Name:                   "check",
		SuccessBeforePassing:   3,
		FailuresBeforeCritical: 4,
	}

	type sc = ServiceCheck
	type tweaker = func(check *sc)

	hash := func(c *sc) string {
		return c.Hash("ServiceID")
	}

	t.Run("reflexive", func(t *testing.T) {
		require.Equal(t, hash(original), hash(original))
	})

	// these tests use tweaker to modify 1 field and make the false assertion
	// on comparing the resulting hash output

	try := func(t *testing.T, tweak tweaker) {
		originalHash := hash(original)
		modifiable := original.Copy()
		tweak(modifiable)
		tweakedHash := hash(modifiable)
		require.NotEqual(t, originalHash, tweakedHash)
	}

	t.Run("name", func(t *testing.T) {
		try(t, func(s *sc) { s.Name = "newName" })
	})

	t.Run("success_before_passing", func(t *testing.T) {
		try(t, func(s *sc) { s.SuccessBeforePassing = 99 })
	})

	t.Run("failures_before_critical", func(t *testing.T) {
		try(t, func(s *sc) { s.FailuresBeforeCritical = 99 })
	})
}

func TestServiceCheck_Canonicalize(t *testing.T) {
	ci.Parallel(t)

	t.Run("defaults", func(t *testing.T) {
		sc := &ServiceCheck{
			Args:     []string{},
			Header:   make(map[string][]string),
			Method:   "",
			OnUpdate: "",
		}
		sc.Canonicalize("MyService", "task1")
		must.Nil(t, sc.Args)
		must.Nil(t, sc.Header)
		must.Eq(t, `service: "MyService" check`, sc.Name)
		must.Eq(t, "", sc.Method)
		must.Eq(t, OnUpdateRequireHealthy, sc.OnUpdate)
	})

	t.Run("check name set", func(t *testing.T) {
		sc := &ServiceCheck{
			Name: "Some Check",
		}
		sc.Canonicalize("MyService", "task1")
		must.Eq(t, "Some Check", sc.Name)
	})

	t.Run("on_update is set", func(t *testing.T) {
		sc := &ServiceCheck{
			OnUpdate: OnUpdateIgnore,
		}
		sc.Canonicalize("MyService", "task1")
		must.Eq(t, OnUpdateIgnore, sc.OnUpdate)
	})
}

func TestServiceCheck_validate_PassingTypes(t *testing.T) {
	ci.Parallel(t)

	t.Run("valid", func(t *testing.T) {
		for _, checkType := range []string{"tcp", "http", "grpc"} {
			err := (&ServiceCheck{
				Name:                 "check",
				Type:                 checkType,
				Path:                 "/path",
				Interval:             1 * time.Second,
				Timeout:              2 * time.Second,
				SuccessBeforePassing: 3,
			}).validateConsul()
			require.NoError(t, err)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		err := (&ServiceCheck{
			Name:                 "check",
			Type:                 "script",
			Command:              "/nothing",
			Interval:             1 * time.Second,
			Timeout:              2 * time.Second,
			SuccessBeforePassing: 3,
		}).validateConsul()
		require.EqualError(t, err, `success_before_passing not supported for check of type "script"`)
	})
}

func TestServiceCheck_validate_FailingTypes(t *testing.T) {
	ci.Parallel(t)

	t.Run("valid", func(t *testing.T) {
		for _, checkType := range []string{"tcp", "http", "grpc"} {
			err := (&ServiceCheck{
				Name:                   "check",
				Type:                   checkType,
				Path:                   "/path",
				Interval:               1 * time.Second,
				Timeout:                2 * time.Second,
				FailuresBeforeCritical: 3,
			}).validateConsul()
			require.NoError(t, err)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		err := (&ServiceCheck{
			Name:                   "check",
			Type:                   "script",
			Command:                "/nothing",
			Interval:               1 * time.Second,
			Timeout:                2 * time.Second,
			SuccessBeforePassing:   0,
			FailuresBeforeCritical: 3,
		}).validateConsul()
		require.EqualError(t, err, `failures_before_critical not supported for check of type "script"`)
	})
}

func TestServiceCheck_validate_PassFailZero_on_scripts(t *testing.T) {
	ci.Parallel(t)

	t.Run("invalid", func(t *testing.T) {
		err := (&ServiceCheck{
			Name:                   "check",
			Type:                   "script",
			Command:                "/nothing",
			Interval:               1 * time.Second,
			Timeout:                2 * time.Second,
			SuccessBeforePassing:   0, // script checks should still pass validation
			FailuresBeforeCritical: 0, // script checks should still pass validation
		}).validateConsul()
		require.NoError(t, err)
	})
}

func TestServiceCheck_validate_OnUpdate_CheckRestart_Conflict(t *testing.T) {
	ci.Parallel(t)

	t.Run("invalid", func(t *testing.T) {
		err := (&ServiceCheck{
			Name:     "check",
			Type:     "script",
			Command:  "/nothing",
			Interval: 1 * time.Second,
			Timeout:  2 * time.Second,
			CheckRestart: &CheckRestart{
				IgnoreWarnings: false,
				Limit:          3,
				Grace:          5 * time.Second,
			},
			OnUpdate: OnUpdateIgnoreWarn,
		}).validateConsul()
		require.EqualError(t, err, `on_update value "ignore_warnings" not supported with check_restart ignore_warnings value "false"`)
	})

	t.Run("invalid", func(t *testing.T) {
		err := (&ServiceCheck{
			Name:     "check",
			Type:     "script",
			Command:  "/nothing",
			Interval: 1 * time.Second,
			Timeout:  2 * time.Second,
			CheckRestart: &CheckRestart{
				IgnoreWarnings: false,
				Limit:          3,
				Grace:          5 * time.Second,
			},
			OnUpdate: OnUpdateIgnore,
		}).validateConsul()
		require.EqualError(t, err, `on_update value "ignore" is not compatible with check_restart`)
	})

	t.Run("valid", func(t *testing.T) {
		err := (&ServiceCheck{
			Name:     "check",
			Type:     "script",
			Command:  "/nothing",
			Interval: 1 * time.Second,
			Timeout:  2 * time.Second,
			CheckRestart: &CheckRestart{
				IgnoreWarnings: true,
				Limit:          3,
				Grace:          5 * time.Second,
			},
			OnUpdate: OnUpdateIgnoreWarn,
		}).validateConsul()
		require.NoError(t, err)
	})
}

func TestServiceCheck_validateNomad(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name string
		sc   *ServiceCheck
		exp  string
	}{
		{name: "grpc", sc: &ServiceCheck{Type: ServiceCheckGRPC}, exp: `invalid check type ("grpc"), must be one of tcp, http`},
		{name: "script", sc: &ServiceCheck{Type: ServiceCheckScript}, exp: `invalid check type ("script"), must be one of tcp, http`},
		{
			name: "expose",
			sc: &ServiceCheck{
				Type:     ServiceCheckTCP,
				Expose:   true, // consul only
				Interval: 3 * time.Second,
				Timeout:  1 * time.Second,
			},
			exp: `expose may only be set for Consul service checks`,
		}, {
			name: "on_update ignore_warnings",
			sc: &ServiceCheck{
				Type:     ServiceCheckTCP,
				Interval: 3 * time.Second,
				Timeout:  1 * time.Second,
				OnUpdate: OnUpdateIgnoreWarn,
			},
			exp: `on_update may only be set to ignore_warnings for Consul service checks`,
		},
		{
			name: "success_before_passing",
			sc: &ServiceCheck{
				Type:                 ServiceCheckTCP,
				SuccessBeforePassing: 3, // consul only
				Interval:             3 * time.Second,
				Timeout:              1 * time.Second,
			},
			exp: `success_before_passing may only be set for Consul service checks`,
		},
		{
			name: "failures_before_critical",
			sc: &ServiceCheck{
				Type:                   ServiceCheckTCP,
				FailuresBeforeCritical: 3, // consul only
				Interval:               3 * time.Second,
				Timeout:                1 * time.Second,
			},
			exp: `failures_before_critical may only be set for Consul service checks`,
		},
		{
			name: "check_restart",
			sc: &ServiceCheck{
				Type:         ServiceCheckTCP,
				Interval:     3 * time.Second,
				Timeout:      1 * time.Second,
				CheckRestart: new(CheckRestart),
			},
		},
		{
			name: "check_restart ignore_warnings",
			sc: &ServiceCheck{
				Type:     ServiceCheckTCP,
				Interval: 3 * time.Second,
				Timeout:  1 * time.Second,
				CheckRestart: &CheckRestart{
					IgnoreWarnings: true,
				},
			},
			exp: `ignore_warnings on check_restart only supported for Consul service checks`,
		},
		{
			name: "address mode driver",
			sc: &ServiceCheck{
				Type:        ServiceCheckTCP,
				Interval:    3 * time.Second,
				Timeout:     1 * time.Second,
				AddressMode: "driver",
			},
			exp: `address_mode = driver may only be set for Consul service checks`,
		},
		{
			name: "http non GET",
			sc: &ServiceCheck{
				Type:     ServiceCheckHTTP,
				Interval: 3 * time.Second,
				Timeout:  1 * time.Second,
				Path:     "/health",
				Method:   "HEAD",
			},
		},
		{
			name: "http unknown method type",
			sc: &ServiceCheck{
				Type:     ServiceCheckHTTP,
				Interval: 3 * time.Second,
				Timeout:  1 * time.Second,
				Path:     "/health",
				Method:   "Invalid",
			},
			exp: `method type "Invalid" not supported in Nomad http check`,
		},
		{
			name: "http with headers",
			sc: &ServiceCheck{
				Type:     ServiceCheckHTTP,
				Interval: 3 * time.Second,
				Timeout:  1 * time.Second,
				Path:     "/health",
				Method:   "GET",
				Header: map[string][]string{
					"foo": {"bar"},
					"baz": nil,
				},
			},
		},
		{
			name: "http with body",
			sc: &ServiceCheck{
				Type:     ServiceCheckHTTP,
				Interval: 3 * time.Second,
				Timeout:  1 * time.Second,
				Path:     "/health",
				Method:   "POST",
				Body:     "this is a request payload!",
			},
		},
		{
			name: "http with tls_server_name",
			sc: &ServiceCheck{
				Type:          ServiceCheckHTTP,
				Interval:      3 * time.Second,
				Timeout:       1 * time.Second,
				Path:          "/health",
				TLSServerName: "foo",
			},
			exp: `tls_server_name may only be set for Consul service checks`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.sc.validateNomad()
			if testCase.exp == "" {
				must.NoError(t, err)
			} else {
				must.EqError(t, err, testCase.exp)
			}
		})
	}
}

func TestService_Hash(t *testing.T) {
	ci.Parallel(t)

	original := &Service{
		Name:      "myService",
		PortLabel: "portLabel",
		// AddressMode: "bridge", // not hashed (used internally by Nomad)
		Tags:       []string{"original", "tags"},
		CanaryTags: []string{"canary", "tags"},
		// Checks:      nil, // not hashed (managed independently)
		Connect: &ConsulConnect{
			// Native: false, // not hashed
			SidecarService: &ConsulSidecarService{
				Tags: []string{"original", "sidecar", "tags"},
				Port: "9000",
				Proxy: &ConsulProxy{
					LocalServiceAddress: "127.0.0.1",
					LocalServicePort:    24000,
					Config:              map[string]any{"foo": "bar"},
					Upstreams: []ConsulUpstream{{
						DestinationName:      "upstream1",
						DestinationNamespace: "ns2",
						LocalBindPort:        29000,
						Config:               map[string]any{"foo": "bar"},
					}},
				},
				Meta: map[string]string{
					"test-key": "test-value",
				},
			},
			// SidecarTask: nil // not hashed
		}}

	type svc = Service
	type tweaker = func(service *svc)

	hash := func(s *svc, canary bool) string {
		return s.Hash("AllocID", "TaskName", canary)
	}

	t.Run("matching and is canary", func(t *testing.T) {
		require.Equal(t, hash(original, true), hash(original, true))
	})

	t.Run("matching and is not canary", func(t *testing.T) {
		require.Equal(t, hash(original, false), hash(original, false))
	})

	t.Run("matching mod canary", func(t *testing.T) {
		require.NotEqual(t, hash(original, true), hash(original, false))
	})

	try := func(t *testing.T, tweak tweaker) {
		originalHash := hash(original, true)
		modifiable := original.Copy()
		tweak(modifiable)
		tweakedHash := hash(modifiable, true)
		require.NotEqual(t, originalHash, tweakedHash)
	}

	// these tests use tweaker to modify 1 field and make the false assertion
	// on comparing the resulting hash output

	t.Run("mod address", func(t *testing.T) {
		try(t, func(s *svc) { s.Address = "example.com" })
	})

	t.Run("mod name", func(t *testing.T) {
		try(t, func(s *svc) { s.Name = "newName" })
	})

	t.Run("mod port label", func(t *testing.T) {
		try(t, func(s *svc) { s.PortLabel = "newPortLabel" })
	})

	t.Run("mod tags", func(t *testing.T) {
		try(t, func(s *svc) { s.Tags = []string{"new", "tags"} })
	})

	t.Run("mod canary tags", func(t *testing.T) {
		try(t, func(s *svc) { s.CanaryTags = []string{"new", "tags"} })
	})

	t.Run("mod enable tag override", func(t *testing.T) {
		try(t, func(s *svc) { s.EnableTagOverride = true })
	})

	t.Run("mod connect sidecar tags", func(t *testing.T) {
		try(t, func(s *svc) { s.Connect.SidecarService.Tags = []string{"new", "tags"} })
	})

	t.Run("mod connect sidecar port", func(t *testing.T) {
		try(t, func(s *svc) { s.Connect.SidecarService.Port = "9090" })
	})

	t.Run("mod connect sidecar proxy local service address", func(t *testing.T) {
		try(t, func(s *svc) { s.Connect.SidecarService.Proxy.LocalServiceAddress = "1.1.1.1" })
	})

	t.Run("mod connect sidecar proxy local service port", func(t *testing.T) {
		try(t, func(s *svc) { s.Connect.SidecarService.Proxy.LocalServicePort = 9999 })
	})

	t.Run("mod connect sidecar proxy config", func(t *testing.T) {
		try(t, func(s *svc) { s.Connect.SidecarService.Proxy.Config = map[string]interface{}{"foo": "baz"} })
	})

	t.Run("mod connect sidecar proxy upstream destination name", func(t *testing.T) {
		try(t, func(s *svc) { s.Connect.SidecarService.Proxy.Upstreams[0].DestinationName = "dest2" })
	})

	t.Run("mod connect sidecar proxy upstream destination namespace", func(t *testing.T) {
		try(t, func(s *svc) { s.Connect.SidecarService.Proxy.Upstreams[0].DestinationNamespace = "ns3" })
	})

	t.Run("mod connect sidecar proxy upstream destination local bind port", func(t *testing.T) {
		try(t, func(s *svc) { s.Connect.SidecarService.Proxy.Upstreams[0].LocalBindPort = 29999 })
	})

	t.Run("mod connect sidecar proxy upstream config", func(t *testing.T) {
		try(t, func(s *svc) { s.Connect.SidecarService.Proxy.Upstreams[0].Config = map[string]any{"foo": "baz"} })
	})
}

func TestConsulConnect_Validate(t *testing.T) {
	ci.Parallel(t)

	c := &ConsulConnect{}

	// An empty Connect block is invalid
	require.Error(t, c.Validate())

	c.Native = true
	require.NoError(t, c.Validate())

	// Native=true + Sidecar!=nil is invalid
	c.SidecarService = &ConsulSidecarService{}
	require.Error(t, c.Validate())

	c.Native = false
	require.NoError(t, c.Validate())
}

func TestConsulConnect_CopyEqual(t *testing.T) {
	ci.Parallel(t)

	c := &ConsulConnect{
		SidecarService: &ConsulSidecarService{
			Tags: []string{"tag1", "tag2"},
			Port: "9001",
			Proxy: &ConsulProxy{
				LocalServiceAddress: "127.0.0.1",
				LocalServicePort:    8080,
				Upstreams: []ConsulUpstream{
					{
						DestinationName:      "up1",
						DestinationNamespace: "ns2",
						LocalBindPort:        9002,
					},
					{
						DestinationName:      "up2",
						DestinationNamespace: "ns2",
						LocalBindPort:        9003,
					},
				},
				Config: map[string]interface{}{
					"foo": 1,
				},
			},
			Meta: map[string]string{
				"test-key": "test-value",
			},
		},
	}

	require.NoError(t, c.Validate())

	// Copies should be equivalent
	o := c.Copy()
	require.True(t, c.Equal(o))

	o.SidecarService.Proxy.Upstreams = nil
	require.False(t, c.Equal(o))
}

func TestConsulConnect_GatewayProxy_CopyEqual(t *testing.T) {
	ci.Parallel(t)

	c := &ConsulGatewayProxy{
		ConnectTimeout:                  pointer.Of(1 * time.Second),
		EnvoyGatewayBindTaggedAddresses: false,
		EnvoyGatewayBindAddresses:       make(map[string]*ConsulGatewayBindAddress),
	}

	require.NoError(t, c.Validate())

	// Copies should be equivalent
	o := c.Copy()
	require.Equal(t, c, o)
	require.True(t, c.Equal(o))
}

func TestSidecarTask_MergeIntoTask(t *testing.T) {
	ci.Parallel(t)

	task := MockJob().TaskGroups[0].Tasks[0]
	sTask := &SidecarTask{
		Name:   "sidecar",
		Driver: "sidecar",
		User:   "test",
		Config: map[string]interface{}{
			"foo": "bar",
		},
		Resources: &Resources{
			CPU:      10000,
			MemoryMB: 10000,
		},
		Env: map[string]string{
			"sidecar": "proxy",
		},
		Meta: map[string]string{
			"abc": "123",
		},
		KillTimeout: pointer.Of(15 * time.Second),
		LogConfig: &LogConfig{
			MaxFiles: 3,
		},
		ShutdownDelay: pointer.Of(5 * time.Second),
		KillSignal:    "SIGABRT",
	}

	expected := task.Copy()
	expected.Name = "sidecar"
	expected.Driver = "sidecar"
	expected.User = "test"
	expected.Config = map[string]interface{}{
		"foo": "bar",
	}
	expected.Resources.CPU = 10000
	expected.Resources.MemoryMB = 10000
	expected.Env["sidecar"] = "proxy"
	expected.Meta["abc"] = "123"
	expected.KillTimeout = 15 * time.Second
	expected.LogConfig.MaxFiles = 3
	expected.ShutdownDelay = 5 * time.Second
	expected.KillSignal = "SIGABRT"

	sTask.MergeIntoTask(task)
	require.Exactly(t, expected, task)

	// Check that changing just driver config doesn't replace map
	sTask.Config["abc"] = 123
	expected.Config["abc"] = 123

	sTask.MergeIntoTask(task)
	require.Exactly(t, expected, task)
}

func TestSidecarTask_Equal(t *testing.T) {
	ci.Parallel(t)

	original := &SidecarTask{
		Name:        "sidecar-task-1",
		Driver:      "docker",
		User:        "nobody",
		Config:      map[string]interface{}{"foo": 1},
		Env:         map[string]string{"color": "blue"},
		Resources:   &Resources{MemoryMB: 300},
		Meta:        map[string]string{"index": "1"},
		KillTimeout: pointer.Of(2 * time.Second),
		LogConfig: &LogConfig{
			MaxFiles:      2,
			MaxFileSizeMB: 300,
		},
		ShutdownDelay: pointer.Of(10 * time.Second),
		KillSignal:    "SIGTERM",
	}

	t.Run("unmodified", func(t *testing.T) {
		duplicate := original.Copy()
		require.True(t, duplicate.Equal(original))
	})

	type st = SidecarTask
	type tweaker = func(task *st)

	try := func(t *testing.T, tweak tweaker) {
		modified := original.Copy()
		tweak(modified)
		require.NotEqual(t, original, modified)
	}

	t.Run("mod name", func(t *testing.T) {
		try(t, func(s *st) { s.Name = "sidecar-task-2" })
	})

	t.Run("mod driver", func(t *testing.T) {
		try(t, func(s *st) { s.Driver = "exec" })
	})

	t.Run("mod user", func(t *testing.T) {
		try(t, func(s *st) { s.User = "root" })
	})

	t.Run("mod config", func(t *testing.T) {
		try(t, func(s *st) { s.Config = map[string]interface{}{"foo": 2} })
	})

	t.Run("mod env", func(t *testing.T) {
		try(t, func(s *st) { s.Env = map[string]string{"color": "red"} })
	})

	t.Run("mod resources", func(t *testing.T) {
		try(t, func(s *st) { s.Resources = &Resources{MemoryMB: 200} })
	})

	t.Run("mod meta", func(t *testing.T) {
		try(t, func(s *st) { s.Meta = map[string]string{"index": "2"} })
	})

	t.Run("mod kill timeout", func(t *testing.T) {
		try(t, func(s *st) { s.KillTimeout = pointer.Of(3 * time.Second) })
	})

	t.Run("mod log config", func(t *testing.T) {
		try(t, func(s *st) { s.LogConfig = &LogConfig{MaxFiles: 3} })
	})

	t.Run("mod shutdown delay", func(t *testing.T) {
		try(t, func(s *st) { s.ShutdownDelay = pointer.Of(20 * time.Second) })
	})

	t.Run("mod kill signal", func(t *testing.T) {
		try(t, func(s *st) { s.KillSignal = "SIGHUP" })
	})
}

func TestConsulUpstream_upstreamEqual(t *testing.T) {
	ci.Parallel(t)

	up := func(name string, port int) ConsulUpstream {
		return ConsulUpstream{
			DestinationName: name,
			LocalBindPort:   port,
		}
	}

	t.Run("size mismatch", func(t *testing.T) {
		a := []ConsulUpstream{up("foo", 8000)}
		b := []ConsulUpstream{up("foo", 8000), up("bar", 9000)}
		must.False(t, upstreamsEquals(a, b))
	})

	t.Run("different", func(t *testing.T) {
		a := []ConsulUpstream{up("bar", 9000)}
		b := []ConsulUpstream{up("foo", 8000)}
		must.False(t, upstreamsEquals(a, b))
	})

	t.Run("different namespace", func(t *testing.T) {
		a := []ConsulUpstream{up("foo", 8000)}
		a[0].DestinationNamespace = "ns1"

		b := []ConsulUpstream{up("foo", 8000)}
		b[0].DestinationNamespace = "ns2"

		must.False(t, upstreamsEquals(a, b))
	})

	t.Run("different mesh_gateway", func(t *testing.T) {
		a := []ConsulUpstream{{DestinationName: "foo", MeshGateway: ConsulMeshGateway{Mode: "local"}}}
		b := []ConsulUpstream{{DestinationName: "foo", MeshGateway: ConsulMeshGateway{Mode: "remote"}}}
		must.False(t, upstreamsEquals(a, b))
	})

	t.Run("different opaque config", func(t *testing.T) {
		a := []ConsulUpstream{{Config: map[string]any{"foo": 1}}}
		b := []ConsulUpstream{{Config: map[string]any{"foo": 2}}}
		must.False(t, upstreamsEquals(a, b))
	})

	t.Run("identical", func(t *testing.T) {
		a := []ConsulUpstream{up("foo", 8000), up("bar", 9000)}
		b := []ConsulUpstream{up("foo", 8000), up("bar", 9000)}
		must.True(t, upstreamsEquals(a, b))
	})

	t.Run("unsorted", func(t *testing.T) {
		a := []ConsulUpstream{up("foo", 8000), up("bar", 9000)}
		b := []ConsulUpstream{up("bar", 9000), up("foo", 8000)}
		must.True(t, upstreamsEquals(a, b))
	})
}

func TestConsulExposePath_exposePathsEqual(t *testing.T) {
	ci.Parallel(t)

	expose := func(path, protocol, listen string, local int) ConsulExposePath {
		return ConsulExposePath{
			Path:          path,
			Protocol:      protocol,
			LocalPathPort: local,
			ListenerPort:  listen,
		}
	}

	t.Run("size mismatch", func(t *testing.T) {
		a := []ConsulExposePath{expose("/1", "http", "myPort", 8000)}
		b := []ConsulExposePath{expose("/1", "http", "myPort", 8000), expose("/2", "http", "myPort", 8000)}
		require.False(t, exposePathsEqual(a, b))
	})

	t.Run("different", func(t *testing.T) {
		a := []ConsulExposePath{expose("/1", "http", "myPort", 8000)}
		b := []ConsulExposePath{expose("/2", "http", "myPort", 8000)}
		require.False(t, exposePathsEqual(a, b))
	})

	t.Run("identical", func(t *testing.T) {
		a := []ConsulExposePath{expose("/1", "http", "myPort", 8000)}
		b := []ConsulExposePath{expose("/1", "http", "myPort", 8000)}
		require.True(t, exposePathsEqual(a, b))
	})

	t.Run("unsorted", func(t *testing.T) {
		a := []ConsulExposePath{expose("/2", "http", "myPort", 8000), expose("/1", "http", "myPort", 8000)}
		b := []ConsulExposePath{expose("/1", "http", "myPort", 8000), expose("/2", "http", "myPort", 8000)}
		require.True(t, exposePathsEqual(a, b))
	})
}

func TestConsulExposeConfig_Copy(t *testing.T) {
	ci.Parallel(t)

	require.Nil(t, (*ConsulExposeConfig)(nil).Copy())
	require.Equal(t, &ConsulExposeConfig{
		Paths: []ConsulExposePath{{
			Path: "/health",
		}},
	}, (&ConsulExposeConfig{
		Paths: []ConsulExposePath{{
			Path: "/health",
		}},
	}).Copy())
}

func TestConsulExposeConfig_Equal(t *testing.T) {
	ci.Parallel(t)

	require.True(t, (*ConsulExposeConfig)(nil).Equal(nil))
	require.True(t, (&ConsulExposeConfig{
		Paths: []ConsulExposePath{{
			Path: "/health",
		}},
	}).Equal(&ConsulExposeConfig{
		Paths: []ConsulExposePath{{
			Path: "/health",
		}},
	}))
}

func TestConsulSidecarService_Copy(t *testing.T) {
	ci.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		s := (*ConsulSidecarService)(nil)
		result := s.Copy()
		require.Nil(t, result)
	})

	t.Run("not nil", func(t *testing.T) {
		s := &ConsulSidecarService{
			Tags:  []string{"foo", "bar"},
			Port:  "port1",
			Proxy: &ConsulProxy{LocalServiceAddress: "10.0.0.1"},
			Meta: map[string]string{
				"test-key": "test-value",
			},
		}
		result := s.Copy()
		require.Equal(t, &ConsulSidecarService{
			Tags:  []string{"foo", "bar"},
			Port:  "port1",
			Proxy: &ConsulProxy{LocalServiceAddress: "10.0.0.1"},
			Meta: map[string]string{
				"test-key": "test-value",
			},
		}, result)
	})
}

var (
	consulIngressGateway1 = &ConsulGateway{
		Proxy: &ConsulGatewayProxy{
			ConnectTimeout:                  pointer.Of(1 * time.Second),
			EnvoyGatewayBindTaggedAddresses: true,
			EnvoyGatewayBindAddresses: map[string]*ConsulGatewayBindAddress{
				"listener1": {Address: "10.0.0.1", Port: 2001},
				"listener2": {Address: "10.0.0.1", Port: 2002},
			},
			EnvoyGatewayNoDefaultBind: true,
			Config: map[string]interface{}{
				"foo": 1,
			},
		},
		Ingress: &ConsulIngressConfigEntry{
			TLS: &ConsulGatewayTLSConfig{
				Enabled: true,
			},
			Listeners: []*ConsulIngressListener{{
				Port:     3000,
				Protocol: "http",
				Services: []*ConsulIngressService{{
					Name:  "service1",
					Hosts: []string{"10.0.0.1", "10.0.0.1:3000"},
				}, {
					Name:  "service2",
					Hosts: []string{"10.0.0.2", "10.0.0.2:3000"},
				}},
			}, {
				Port:     3001,
				Protocol: "tcp",
				Services: []*ConsulIngressService{{
					Name: "service3",
				}},
			}},
		},
	}

	consulTerminatingGateway1 = &ConsulGateway{
		Proxy: &ConsulGatewayProxy{
			ConnectTimeout:            pointer.Of(1 * time.Second),
			EnvoyDNSDiscoveryType:     "STRICT_DNS",
			EnvoyGatewayBindAddresses: nil,
		},
		Terminating: &ConsulTerminatingConfigEntry{
			Services: []*ConsulLinkedService{{
				Name:     "linked-service1",
				CAFile:   "ca.pem",
				CertFile: "cert.pem",
				KeyFile:  "key.pem",
				SNI:      "service1.consul",
			}, {
				Name: "linked-service2",
			}},
		},
	}

	consulMeshGateway1 = &ConsulGateway{
		Proxy: &ConsulGatewayProxy{
			ConnectTimeout: pointer.Of(1 * time.Second),
		},
		Mesh: &ConsulMeshConfigEntry{
			// nothing
		},
	}
)

func TestConsulGateway_Prefix(t *testing.T) {
	ci.Parallel(t)

	t.Run("ingress", func(t *testing.T) {
		result := (&ConsulGateway{Ingress: new(ConsulIngressConfigEntry)}).Prefix()
		require.Equal(t, ConnectIngressPrefix, result)
	})

	t.Run("terminating", func(t *testing.T) {
		result := (&ConsulGateway{Terminating: new(ConsulTerminatingConfigEntry)}).Prefix()
		require.Equal(t, ConnectTerminatingPrefix, result)
	})

	t.Run("mesh", func(t *testing.T) {
		result := (&ConsulGateway{Mesh: new(ConsulMeshConfigEntry)}).Prefix()
		require.Equal(t, ConnectMeshPrefix, result)
	})
}

func TestConsulGateway_Copy(t *testing.T) {
	ci.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		g := (*ConsulGateway)(nil)
		result := g.Copy()
		require.Nil(t, result)
	})

	t.Run("as ingress", func(t *testing.T) {
		result := consulIngressGateway1.Copy()
		require.Equal(t, consulIngressGateway1, result)
		require.True(t, result.Equal(consulIngressGateway1))
		require.True(t, consulIngressGateway1.Equal(result))
	})

	t.Run("as terminating", func(t *testing.T) {
		result := consulTerminatingGateway1.Copy()
		require.Equal(t, consulTerminatingGateway1, result)
		require.True(t, result.Equal(consulTerminatingGateway1))
		require.True(t, consulTerminatingGateway1.Equal(result))
	})

	t.Run("as mesh", func(t *testing.T) {
		result := consulMeshGateway1.Copy()
		require.Equal(t, consulMeshGateway1, result)
		require.True(t, result.Equal(consulMeshGateway1))
		require.True(t, consulMeshGateway1.Equal(result))
	})
}

func TestConsulGateway_Equal_mesh(t *testing.T) {
	ci.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		a := (*ConsulGateway)(nil)
		b := (*ConsulGateway)(nil)
		require.True(t, a.Equal(b))
		require.False(t, a.Equal(consulMeshGateway1))
		require.False(t, consulMeshGateway1.Equal(a))
	})

	t.Run("reflexive", func(t *testing.T) {
		require.True(t, consulMeshGateway1.Equal(consulMeshGateway1))
	})
}

func TestConsulGateway_Equal_ingress(t *testing.T) {
	ci.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		a := (*ConsulGateway)(nil)
		b := (*ConsulGateway)(nil)
		require.True(t, a.Equal(b))
		require.False(t, a.Equal(consulIngressGateway1))
		require.False(t, consulIngressGateway1.Equal(a))
	})

	original := consulIngressGateway1.Copy()

	type cg = ConsulGateway
	type tweaker = func(g *cg)

	t.Run("reflexive", func(t *testing.T) {
		require.True(t, original.Equal(original))
	})

	try := func(t *testing.T, tweak tweaker) {
		modifiable := original.Copy()
		tweak(modifiable)
		require.False(t, original.Equal(modifiable))
		require.False(t, modifiable.Equal(original))
		require.True(t, modifiable.Equal(modifiable))
	}

	// proxy block equality checks

	t.Run("mod gateway timeout", func(t *testing.T) {
		try(t, func(g *cg) { g.Proxy.ConnectTimeout = pointer.Of(9 * time.Second) })
	})

	t.Run("mod gateway envoy_gateway_bind_tagged_addresses", func(t *testing.T) {
		try(t, func(g *cg) { g.Proxy.EnvoyGatewayBindTaggedAddresses = false })
	})

	t.Run("mod gateway envoy_gateway_bind_addresses", func(t *testing.T) {
		try(t, func(g *cg) {
			g.Proxy.EnvoyGatewayBindAddresses = map[string]*ConsulGatewayBindAddress{
				"listener3": {Address: "9.9.9.9", Port: 9999},
			}
		})
	})

	t.Run("mod gateway envoy_gateway_no_default_bind", func(t *testing.T) {
		try(t, func(g *cg) { g.Proxy.EnvoyGatewayNoDefaultBind = false })
	})

	t.Run("mod gateway config", func(t *testing.T) {
		try(t, func(g *cg) {
			g.Proxy.Config = map[string]interface{}{
				"foo": 2,
			}
		})
	})

	// ingress config entry equality checks

	t.Run("mod ingress tls", func(t *testing.T) {
		try(t, func(g *cg) { g.Ingress.TLS = nil })
		try(t, func(g *cg) { g.Ingress.TLS.Enabled = false })
	})

	t.Run("mod ingress listeners count", func(t *testing.T) {
		try(t, func(g *cg) { g.Ingress.Listeners = g.Ingress.Listeners[:1] })
	})

	t.Run("mod ingress listeners port", func(t *testing.T) {
		try(t, func(g *cg) { g.Ingress.Listeners[0].Port = 7777 })
	})

	t.Run("mod ingress listeners protocol", func(t *testing.T) {
		try(t, func(g *cg) { g.Ingress.Listeners[0].Protocol = "tcp" })
	})

	t.Run("mod ingress listeners services count", func(t *testing.T) {
		try(t, func(g *cg) { g.Ingress.Listeners[0].Services = g.Ingress.Listeners[0].Services[:1] })
	})

	t.Run("mod ingress listeners services name", func(t *testing.T) {
		try(t, func(g *cg) { g.Ingress.Listeners[0].Services[0].Name = "serviceX" })
	})

	t.Run("mod ingress listeners services hosts count", func(t *testing.T) {
		try(t, func(g *cg) { g.Ingress.Listeners[0].Services[0].Hosts = g.Ingress.Listeners[0].Services[0].Hosts[:1] })
	})

	t.Run("mod ingress listeners services hosts content", func(t *testing.T) {
		try(t, func(g *cg) { g.Ingress.Listeners[0].Services[0].Hosts[0] = "255.255.255.255" })
	})
}

func TestConsulGateway_Equal_terminating(t *testing.T) {
	ci.Parallel(t)

	original := consulTerminatingGateway1.Copy()

	type cg = ConsulGateway
	type tweaker = func(c *cg)

	t.Run("reflexive", func(t *testing.T) {
		require.True(t, original.Equal(original))
	})

	try := func(t *testing.T, tweak tweaker) {
		modifiable := original.Copy()
		tweak(modifiable)
		require.False(t, original.Equal(modifiable))
		require.False(t, modifiable.Equal(original))
		require.True(t, modifiable.Equal(modifiable))
	}

	// proxy block equality checks

	t.Run("mod dns discovery type", func(t *testing.T) {
		try(t, func(g *cg) { g.Proxy.EnvoyDNSDiscoveryType = "LOGICAL_DNS" })
	})

	// terminating config entry equality checks

	t.Run("mod terminating services count", func(t *testing.T) {
		try(t, func(g *cg) { g.Terminating.Services = g.Terminating.Services[:1] })
	})

	t.Run("mod terminating services name", func(t *testing.T) {
		try(t, func(g *cg) { g.Terminating.Services[0].Name = "foo" })
	})

	t.Run("mod terminating services ca_file", func(t *testing.T) {
		try(t, func(g *cg) { g.Terminating.Services[0].CAFile = "foo.pem" })
	})

	t.Run("mod terminating services cert_file", func(t *testing.T) {
		try(t, func(g *cg) { g.Terminating.Services[0].CertFile = "foo.pem" })
	})

	t.Run("mod terminating services key_file", func(t *testing.T) {
		try(t, func(g *cg) { g.Terminating.Services[0].KeyFile = "foo.pem" })
	})

	t.Run("mod terminating services sni", func(t *testing.T) {
		try(t, func(g *cg) { g.Terminating.Services[0].SNI = "foo.consul" })
	})
}

func TestConsulGateway_ingressServicesEqual(t *testing.T) {
	ci.Parallel(t)

	igs1 := []*ConsulIngressService{{
		Name:  "service1",
		Hosts: []string{"host1", "host2"},
	}, {
		Name:  "service2",
		Hosts: []string{"host3"},
	}}

	require.False(t, ingressServicesEqual(igs1, nil))
	require.True(t, ingressServicesEqual(igs1, igs1))

	reversed := []*ConsulIngressService{
		igs1[1], igs1[0], // services reversed
	}

	require.True(t, ingressServicesEqual(igs1, reversed))

	hostOrder := []*ConsulIngressService{{
		Name:  "service1",
		Hosts: []string{"host2", "host1"}, // hosts reversed
	}, {
		Name:  "service2",
		Hosts: []string{"host3"},
	}}

	require.True(t, ingressServicesEqual(igs1, hostOrder))
}

func TestConsulGateway_ingressListenersEqual(t *testing.T) {
	ci.Parallel(t)

	ils1 := []*ConsulIngressListener{{
		Port:     2000,
		Protocol: "http",
		Services: []*ConsulIngressService{{
			Name:  "service1",
			Hosts: []string{"host1", "host2"},
		}},
	}, {
		Port:     2001,
		Protocol: "tcp",
		Services: []*ConsulIngressService{{
			Name: "service2",
		}},
	}}

	require.False(t, ingressListenersEqual(ils1, nil))

	reversed := []*ConsulIngressListener{
		ils1[1], ils1[0],
	}

	require.True(t, ingressListenersEqual(ils1, reversed))
}

func TestConsulGateway_Validate(t *testing.T) {
	ci.Parallel(t)

	t.Run("bad proxy", func(t *testing.T) {
		err := (&ConsulGateway{
			Proxy: &ConsulGatewayProxy{
				ConnectTimeout: nil,
			},
			Ingress: nil,
		}).Validate()
		require.EqualError(t, err, "Consul Gateway Proxy connection_timeout must be set")
	})

	t.Run("bad ingress config entry", func(t *testing.T) {
		err := (&ConsulGateway{
			Ingress: &ConsulIngressConfigEntry{
				Listeners: nil,
			},
		}).Validate()
		require.EqualError(t, err, "Consul Ingress Gateway requires at least one listener")
	})

	t.Run("bad terminating config entry", func(t *testing.T) {
		err := (&ConsulGateway{
			Terminating: &ConsulTerminatingConfigEntry{
				Services: nil,
			},
		}).Validate()
		require.EqualError(t, err, "Consul Terminating Gateway requires at least one service")
	})

	t.Run("no config entry set", func(t *testing.T) {
		err := (&ConsulGateway{
			Ingress:     nil,
			Terminating: nil,
			Mesh:        nil,
		}).Validate()
		require.EqualError(t, err, "One Consul Gateway Configuration must be set")
	})

	t.Run("multiple config entries set", func(t *testing.T) {
		err := (&ConsulGateway{
			Ingress: &ConsulIngressConfigEntry{
				Listeners: []*ConsulIngressListener{{
					Port:     1111,
					Protocol: "tcp",
					Services: []*ConsulIngressService{{
						Name: "service1",
					}},
				}},
			},
			Terminating: &ConsulTerminatingConfigEntry{
				Services: []*ConsulLinkedService{{
					Name: "linked-service1",
				}},
			},
		}).Validate()
		require.EqualError(t, err, "One Consul Gateway Configuration must be set")
	})

	t.Run("ok mesh", func(t *testing.T) {
		err := (&ConsulGateway{
			Mesh: new(ConsulMeshConfigEntry),
		}).Validate()
		require.NoError(t, err)
	})
}

func TestConsulGatewayBindAddress_Validate(t *testing.T) {
	ci.Parallel(t)

	t.Run("no address", func(t *testing.T) {
		err := (&ConsulGatewayBindAddress{
			Address: "",
			Port:    2000,
		}).Validate()
		require.EqualError(t, err, "Consul Gateway Bind Address must be set")
	})

	t.Run("invalid port", func(t *testing.T) {
		err := (&ConsulGatewayBindAddress{
			Address: "10.0.0.1",
			Port:    0,
		}).Validate()
		require.EqualError(t, err, "Consul Gateway Bind Address must set valid Port")
	})

	t.Run("ok", func(t *testing.T) {
		err := (&ConsulGatewayBindAddress{
			Address: "10.0.0.1",
			Port:    2000,
		}).Validate()
		require.NoError(t, err)
	})
}

func TestConsulGatewayProxy_Validate(t *testing.T) {
	ci.Parallel(t)

	t.Run("no timeout", func(t *testing.T) {
		err := (&ConsulGatewayProxy{
			ConnectTimeout: nil,
		}).Validate()
		require.EqualError(t, err, "Consul Gateway Proxy connection_timeout must be set")
	})

	t.Run("invalid bind address", func(t *testing.T) {
		err := (&ConsulGatewayProxy{
			ConnectTimeout: pointer.Of(1 * time.Second),
			EnvoyGatewayBindAddresses: map[string]*ConsulGatewayBindAddress{
				"service1": {
					Address: "10.0.0.1",
					Port:    0,
				}},
		}).Validate()
		require.EqualError(t, err, "Consul Gateway Bind Address must set valid Port")
	})

	t.Run("invalid dns discovery type", func(t *testing.T) {
		err := (&ConsulGatewayProxy{
			ConnectTimeout:        pointer.Of(1 * time.Second),
			EnvoyDNSDiscoveryType: "RANDOM_DNS",
		}).Validate()
		require.EqualError(t, err, "Consul Gateway Proxy Envoy DNS Discovery type must be STRICT_DNS or LOGICAL_DNS")
	})

	t.Run("ok with nothing set", func(t *testing.T) {
		err := (&ConsulGatewayProxy{
			ConnectTimeout: pointer.Of(1 * time.Second),
		}).Validate()
		require.NoError(t, err)
	})

	t.Run("ok with everything set", func(t *testing.T) {
		err := (&ConsulGatewayProxy{
			ConnectTimeout: pointer.Of(1 * time.Second),
			EnvoyGatewayBindAddresses: map[string]*ConsulGatewayBindAddress{
				"service1": {
					Address: "10.0.0.1",
					Port:    2000,
				}},
			EnvoyGatewayBindTaggedAddresses: true,
			EnvoyGatewayNoDefaultBind:       true,
		}).Validate()
		require.NoError(t, err)
	})
}

func TestConsulIngressService_Validate(t *testing.T) {
	ci.Parallel(t)

	t.Run("invalid name", func(t *testing.T) {
		err := (&ConsulIngressService{
			Name: "",
		}).Validate("http")
		must.EqError(t, err, "Consul Ingress Service requires a name")
	})

	t.Run("tcp extraneous hosts", func(t *testing.T) {
		err := (&ConsulIngressService{
			Name:  "service1",
			Hosts: []string{"host1"},
		}).Validate("tcp")
		must.EqError(t, err, `Consul Ingress Service doesn't support associating hosts to a service for the "tcp" protocol`)
	})

	t.Run("tcp ok", func(t *testing.T) {
		err := (&ConsulIngressService{
			Name: "service1",
		}).Validate("tcp")
		must.NoError(t, err)
	})

	t.Run("tcp with wildcard service", func(t *testing.T) {
		err := (&ConsulIngressService{
			Name: "*",
		}).Validate("tcp")
		must.EqError(t, err, `Consul Ingress Service doesn't support wildcard name for "tcp" protocol`)
	})

	// non-"tcp" protocols should be all treated the same.
	for _, proto := range []string{"http", "http2", "grpc"} {
		t.Run(proto+" ok", func(t *testing.T) {
			err := (&ConsulIngressService{
				Name:  "service1",
				Hosts: []string{"host1"},
			}).Validate(proto)
			must.NoError(t, err)
		})

		t.Run(proto+" without hosts", func(t *testing.T) {
			err := (&ConsulIngressService{
				Name: "service1",
			}).Validate(proto)
			must.NoError(t, err, must.Sprintf(`"%s" protocol should not require hosts`, proto))
		})

		t.Run(proto+" wildcard service", func(t *testing.T) {
			err := (&ConsulIngressService{
				Name: "*",
			}).Validate(proto)
			must.NoError(t, err, must.Sprintf(`"%s" protocol should allow wildcard service`, proto))
		})

		t.Run(proto+" wildcard service and host", func(t *testing.T) {
			err := (&ConsulIngressService{
				Name:  "*",
				Hosts: []string{"any"},
			}).Validate(proto)
			must.EqError(t, err, `Consul Ingress Service with a wildcard "*" service name can not also specify hosts`)
		})
	}
}

func TestConsulIngressListener_Validate(t *testing.T) {
	ci.Parallel(t)

	t.Run("invalid port", func(t *testing.T) {
		err := (&ConsulIngressListener{
			Port:     0,
			Protocol: "tcp",
			Services: []*ConsulIngressService{{
				Name: "service1",
			}},
		}).Validate()
		require.EqualError(t, err, "Consul Ingress Listener requires valid Port")
	})

	t.Run("invalid protocol", func(t *testing.T) {
		err := (&ConsulIngressListener{
			Port:     2000,
			Protocol: "gopher",
			Services: []*ConsulIngressService{{
				Name: "service1",
			}},
		}).Validate()
		require.EqualError(t, err, `Consul Ingress Listener requires protocol of tcp, http, http2, grpc, got "gopher"`)
	})

	t.Run("no services", func(t *testing.T) {
		err := (&ConsulIngressListener{
			Port:     2000,
			Protocol: "tcp",
			Services: nil,
		}).Validate()
		require.EqualError(t, err, "Consul Ingress Listener requires one or more services")
	})

	t.Run("invalid service", func(t *testing.T) {
		err := (&ConsulIngressListener{
			Port:     2000,
			Protocol: "tcp",
			Services: []*ConsulIngressService{{
				Name: "",
			}},
		}).Validate()
		require.EqualError(t, err, "Consul Ingress Service requires a name")
	})

	t.Run("ok", func(t *testing.T) {
		err := (&ConsulIngressListener{
			Port:     2000,
			Protocol: "tcp",
			Services: []*ConsulIngressService{{
				Name: "service1",
			}},
		}).Validate()
		require.NoError(t, err)
	})
}

func TestConsulIngressConfigEntry_Validate(t *testing.T) {
	ci.Parallel(t)

	t.Run("no listeners", func(t *testing.T) {
		err := (&ConsulIngressConfigEntry{}).Validate()
		require.EqualError(t, err, "Consul Ingress Gateway requires at least one listener")
	})

	t.Run("invalid listener", func(t *testing.T) {
		err := (&ConsulIngressConfigEntry{
			Listeners: []*ConsulIngressListener{{
				Port:     9000,
				Protocol: "tcp",
			}},
		}).Validate()
		require.EqualError(t, err, "Consul Ingress Listener requires one or more services")
	})

	t.Run("full", func(t *testing.T) {
		err := (&ConsulIngressConfigEntry{
			TLS: &ConsulGatewayTLSConfig{
				Enabled: true,
			},
			Listeners: []*ConsulIngressListener{{
				Port:     9000,
				Protocol: "tcp",
				Services: []*ConsulIngressService{{
					Name: "service1",
				}},
			}},
		}).Validate()
		require.NoError(t, err)
	})
}

func TestConsulLinkedService_Validate(t *testing.T) {
	ci.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		err := (*ConsulLinkedService)(nil).Validate()
		require.Nil(t, err)
	})

	t.Run("missing name", func(t *testing.T) {
		err := (&ConsulLinkedService{}).Validate()
		require.EqualError(t, err, "Consul Linked Service requires Name")
	})

	t.Run("missing ca_file", func(t *testing.T) {
		err := (&ConsulLinkedService{
			Name:     "linked-service1",
			CertFile: "cert_file.pem",
			KeyFile:  "key_file.pem",
		}).Validate()
		require.EqualError(t, err, "Consul Linked Service TLS requires CAFile")
	})

	t.Run("mutual cert key", func(t *testing.T) {
		err := (&ConsulLinkedService{
			Name:     "linked-service1",
			CAFile:   "ca_file.pem",
			CertFile: "cert_file.pem",
		}).Validate()
		require.EqualError(t, err, "Consul Linked Service TLS Cert and Key must both be set")
	})

	t.Run("sni without ca_file", func(t *testing.T) {
		err := (&ConsulLinkedService{
			Name: "linked-service1",
			SNI:  "service.consul",
		}).Validate()
		require.EqualError(t, err, "Consul Linked Service TLS SNI requires CAFile")
	})

	t.Run("minimal", func(t *testing.T) {
		err := (&ConsulLinkedService{
			Name: "linked-service1",
		}).Validate()
		require.NoError(t, err)
	})

	t.Run("tls minimal", func(t *testing.T) {
		err := (&ConsulLinkedService{
			Name:   "linked-service1",
			CAFile: "ca_file.pem",
		}).Validate()
		require.NoError(t, err)
	})

	t.Run("tls mutual", func(t *testing.T) {
		err := (&ConsulLinkedService{
			Name:     "linked-service1",
			CAFile:   "ca_file.pem",
			CertFile: "cert_file.pem",
			KeyFile:  "key_file.pem",
		}).Validate()
		require.NoError(t, err)
	})

	t.Run("tls sni", func(t *testing.T) {
		err := (&ConsulLinkedService{
			Name:   "linked-service1",
			CAFile: "ca_file.pem",
			SNI:    "linked-service.consul",
		}).Validate()
		require.NoError(t, err)
	})

	t.Run("tls complete", func(t *testing.T) {
		err := (&ConsulLinkedService{
			Name:     "linked-service1",
			CAFile:   "ca_file.pem",
			CertFile: "cert_file.pem",
			KeyFile:  "key_file.pem",
			SNI:      "linked-service.consul",
		}).Validate()
		require.NoError(t, err)
	})
}

func TestConsulLinkedService_Copy(t *testing.T) {
	ci.Parallel(t)

	require.Nil(t, (*ConsulLinkedService)(nil).Copy())
	require.Equal(t, &ConsulLinkedService{
		Name:     "service1",
		CAFile:   "ca.pem",
		CertFile: "cert.pem",
		KeyFile:  "key.pem",
		SNI:      "service1.consul",
	}, (&ConsulLinkedService{
		Name:     "service1",
		CAFile:   "ca.pem",
		CertFile: "cert.pem",
		KeyFile:  "key.pem",
		SNI:      "service1.consul",
	}).Copy())
}

func TestConsulLinkedService_linkedServicesEqual(t *testing.T) {
	ci.Parallel(t)

	services := []*ConsulLinkedService{{
		Name:   "service1",
		CAFile: "ca.pem",
	}, {
		Name:   "service2",
		CAFile: "ca.pem",
	}}

	require.False(t, linkedServicesEqual(services, nil))
	require.True(t, linkedServicesEqual(services, services))

	reversed := []*ConsulLinkedService{
		services[1], services[0], // reversed
	}

	require.True(t, linkedServicesEqual(services, reversed))

	different := []*ConsulLinkedService{
		services[0], {
			Name:   "service2",
			CAFile: "ca.pem",
			SNI:    "service2.consul",
		},
	}

	require.False(t, linkedServicesEqual(services, different))
}

func TestConsulTerminatingConfigEntry_Validate(t *testing.T) {
	ci.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		err := (*ConsulTerminatingConfigEntry)(nil).Validate()
		require.NoError(t, err)
	})

	t.Run("no services", func(t *testing.T) {
		err := (&ConsulTerminatingConfigEntry{
			Services: make([]*ConsulLinkedService, 0),
		}).Validate()
		require.EqualError(t, err, "Consul Terminating Gateway requires at least one service")
	})

	t.Run("service invalid", func(t *testing.T) {
		err := (&ConsulTerminatingConfigEntry{
			Services: []*ConsulLinkedService{{
				Name: "",
			}},
		}).Validate()
		require.EqualError(t, err, "Consul Linked Service requires Name")
	})

	t.Run("ok", func(t *testing.T) {
		err := (&ConsulTerminatingConfigEntry{
			Services: []*ConsulLinkedService{{
				Name: "service1",
			}},
		}).Validate()
		require.NoError(t, err)
	})
}

func TestConsulMeshGateway_Copy(t *testing.T) {
	ci.Parallel(t)

	require.Nil(t, (*ConsulMeshGateway)(nil))
	require.Equal(t, &ConsulMeshGateway{
		Mode: "remote",
	}, &ConsulMeshGateway{
		Mode: "remote",
	})
}

func TestConsulMeshGateway_Equal(t *testing.T) {
	ci.Parallel(t)

	c := ConsulMeshGateway{Mode: "local"}
	require.False(t, c.Equal(ConsulMeshGateway{}))
	require.True(t, c.Equal(c))

	o := ConsulMeshGateway{Mode: "remote"}
	require.False(t, c.Equal(o))
}

func TestConsulMeshGateway_Validate(t *testing.T) {
	ci.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		err := (*ConsulMeshGateway)(nil).Validate()
		require.NoError(t, err)
	})

	t.Run("mode invalid", func(t *testing.T) {
		err := (&ConsulMeshGateway{Mode: "banana"}).Validate()
		require.EqualError(t, err, `Connect mesh_gateway mode "banana" not supported`)
	})

	t.Run("ok", func(t *testing.T) {
		err := (&ConsulMeshGateway{Mode: "local"}).Validate()
		require.NoError(t, err)
	})
}

func TestService_Validate(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		input     *Service
		expErr    bool
		expErrStr string
		name      string
	}{
		{
			name: "base service",
			input: &Service{
				Name: "testservice",
			},
			expErr: false,
		},
		{
			name: "Native Connect without task name",
			input: &Service{
				Name: "testservice",
				Connect: &ConsulConnect{
					Native: true,
				},
			},
			expErr: false, // gets set automatically
		},
		{
			name: "Native Connect with task name",
			input: &Service{
				Name:     "testservice",
				TaskName: "testtask",
				Connect: &ConsulConnect{
					Native: true,
				},
			},
			expErr: false,
		},
		{
			name: "Native Connect with Sidecar",
			input: &Service{
				Name:     "testservice",
				TaskName: "testtask",
				Connect: &ConsulConnect{
					Native:         true,
					SidecarService: &ConsulSidecarService{},
				},
			},
			expErr:    true,
			expErrStr: "Consul Connect must be exclusively native",
		},
		{
			name: "provider nomad with checks",
			input: &Service{
				Name:      "testservice",
				Provider:  "nomad",
				PortLabel: "port",
				Checks: []*ServiceCheck{
					{
						Name:     "servicecheck",
						Type:     "http",
						Path:     "/",
						Interval: 1 * time.Second,
						Timeout:  3 * time.Second,
					},
					{
						Name:     "servicecheck",
						Type:     "tcp",
						Interval: 1 * time.Second,
						Timeout:  3 * time.Second,
					},
				},
			},
			expErr: false,
		},
		{
			name: "provider nomad with invalid check type",
			input: &Service{
				Name:     "testservice",
				Provider: "nomad",
				Checks: []*ServiceCheck{
					{
						Name: "servicecheck",
						Type: "script",
					},
				},
			},
			expErr: true,
		},
		{
			name: "provider nomad with tls skip verify",
			input: &Service{
				Name:     "testservice",
				Provider: "nomad",
				Checks: []*ServiceCheck{
					{
						Name:          "servicecheck",
						Type:          "http",
						TLSSkipVerify: true,
					},
				},
			},
			expErr: true,
		},
		{
			name: "provider nomad with connect",
			input: &Service{
				Name:     "testservice",
				Provider: "nomad",
				Connect: &ConsulConnect{
					Native: true,
				},
			},
			expErr:    true,
			expErrStr: "Service with provider nomad cannot include Connect blocks",
		},
		{
			name: "provider nomad valid",
			input: &Service{
				Name:     "testservice",
				Provider: "nomad",
			},
			expErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.input.Canonicalize("testjob", "testgroup", "testtask", "testnamespace")
			err := tc.input.Validate()
			if tc.expErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expErrStr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestService_Validate_Address(t *testing.T) {
	ci.Parallel(t)

	try := func(mode, advertise string, exp error) {
		s := &Service{Name: "s1", Provider: "consul", AddressMode: mode, Address: advertise}
		result := s.Validate()
		if exp == nil {
			require.NoError(t, result)
		} else {
			// would be nice if multierror worked with errors.Is
			require.Contains(t, result.Error(), exp.Error())
		}
	}

	// advertise not set
	try("", "", nil)
	try("auto", "", nil)
	try("host", "", nil)
	try("alloc", "", nil)
	try("driver", "", nil)

	// advertise is set
	try("", "example.com", nil)
	try("auto", "example.com", nil)
	try("host", "example.com", errors.New(`Service address_mode must be "auto" if address is set`))
	try("alloc", "example.com", errors.New(`Service address_mode must be "auto" if address is set`))
	try("driver", "example.com", errors.New(`Service address_mode must be "auto" if address is set`))
}

func TestService_Equal(t *testing.T) {
	ci.Parallel(t)

	s := Service{
		Name:            "testservice",
		TaggedAddresses: make(map[string]string),
	}

	s.Canonicalize("testjob", "testgroup", "testtask", "default")

	o := s.Copy()

	// Base service should be equal to copy of itself
	require.True(t, s.Equal(o))

	// create a helper to assert a diff and reset the struct
	assertDiff := func() {
		require.False(t, s.Equal(o))
		o = s.Copy()
		require.True(t, s.Equal(o), "bug in copy")
	}

	// Changing any field should cause inequality
	o.Name = "diff"
	assertDiff()

	o.Address = "diff"
	assertDiff()

	o.PortLabel = "diff"
	assertDiff()

	o.AddressMode = AddressModeDriver
	assertDiff()

	o.Tags = []string{"diff"}
	assertDiff()

	o.CanaryTags = []string{"diff"}
	assertDiff()

	o.Checks = []*ServiceCheck{{Name: "diff"}}
	assertDiff()

	o.Connect = &ConsulConnect{Native: true}
	assertDiff()

	o.EnableTagOverride = true
	assertDiff()

	o.Provider = "nomad"
	assertDiff()

	o.TaggedAddresses = map[string]string{"foo": "bar"}
	assertDiff()
}

func TestService_validateNomadService(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		inputService         *Service
		inputErr             *multierror.Error
		expectedOutputErrors []error
		name                 string
	}{
		{
			inputService: &Service{
				Name:      "webapp",
				PortLabel: "http",
				Namespace: "default",
				Provider:  "nomad",
			},
			inputErr:             &multierror.Error{},
			expectedOutputErrors: nil,
			name:                 "valid service",
		},
		{
			inputService: &Service{
				Name:      "webapp",
				PortLabel: "http",
				Namespace: "default",
				Provider:  "nomad",
				Checks: []*ServiceCheck{{
					Name:     "webapp",
					Type:     ServiceCheckHTTP,
					Method:   "GET",
					Path:     "/health",
					Interval: 3 * time.Second,
					Timeout:  1 * time.Second,
				}},
			},
			inputErr:             &multierror.Error{},
			expectedOutputErrors: nil,
			name:                 "valid service with checks",
		},
		{
			inputService: &Service{
				Name:      "webapp",
				PortLabel: "http",
				Namespace: "default",
				Provider:  "nomad",
				Connect: &ConsulConnect{
					Native: true,
				},
			},
			inputErr:             &multierror.Error{},
			expectedOutputErrors: []error{errors.New("Service with provider nomad cannot include Connect blocks")},
			name:                 "invalid service due to connect",
		},
		{
			inputService: &Service{
				Name:      "webapp",
				PortLabel: "http",
				Namespace: "default",
				Provider:  "nomad",
				Checks: []*ServiceCheck{
					{Name: "some-check"},
				},
			},
			inputErr: &multierror.Error{},
			expectedOutputErrors: []error{
				errors.New(`invalid check type (""), must be one of tcp, http`),
			},
			name: "bad nomad check",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.inputService.validateNomadService(tc.inputErr)
			must.Eq(t, tc.expectedOutputErrors, tc.inputErr.Errors)
		})
	}
}
