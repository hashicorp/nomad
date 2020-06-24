package structs

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper"
	"github.com/stretchr/testify/require"
)

func TestService_Hash(t *testing.T) {
	t.Parallel()

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
					Config:              map[string]interface{}{"foo": "bar"},
					Upstreams: []ConsulUpstream{{
						DestinationName: "upstream1",
						LocalBindPort:   29000,
					}},
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

	t.Run("mod connect sidecar proxy upstream dest name", func(t *testing.T) {
		try(t, func(s *svc) { s.Connect.SidecarService.Proxy.Upstreams[0].DestinationName = "dest2" })
	})

	t.Run("mod connect sidecar proxy upstream dest local bind port", func(t *testing.T) {
		try(t, func(s *svc) { s.Connect.SidecarService.Proxy.Upstreams[0].LocalBindPort = 29999 })
	})
}

func TestConsulConnect_Validate(t *testing.T) {
	t.Parallel()

	c := &ConsulConnect{}

	// An empty Connect stanza is invalid
	require.Error(t, c.Validate())

	// Native=true is valid
	c.Native = true
	require.NoError(t, c.Validate())

	// Native=true + Sidecar!=nil is invalid
	c.SidecarService = &ConsulSidecarService{}
	require.Error(t, c.Validate())

	// Native=false + Sidecar!=nil is valid
	c.Native = false
	require.NoError(t, c.Validate())
}

func TestConsulConnect_CopyEquals(t *testing.T) {
	t.Parallel()

	c := &ConsulConnect{
		SidecarService: &ConsulSidecarService{
			Tags: []string{"tag1", "tag2"},
			Port: "9001",
			Proxy: &ConsulProxy{
				LocalServiceAddress: "127.0.0.1",
				LocalServicePort:    8080,
				Upstreams: []ConsulUpstream{
					{
						DestinationName: "up1",
						LocalBindPort:   9002,
					},
					{
						DestinationName: "up2",
						LocalBindPort:   9003,
					},
				},
				Config: map[string]interface{}{
					"foo": 1,
				},
			},
		},
	}

	require.NoError(t, c.Validate())

	// Copies should be equivalent
	o := c.Copy()
	require.True(t, c.Equals(o))

	o.SidecarService.Proxy.Upstreams = nil
	require.False(t, c.Equals(o))
}

func TestSidecarTask_MergeIntoTask(t *testing.T) {
	t.Parallel()

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
		KillTimeout: helper.TimeToPtr(15 * time.Second),
		LogConfig: &LogConfig{
			MaxFiles: 3,
		},
		ShutdownDelay: helper.TimeToPtr(5 * time.Second),
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

func TestConsulUpstream_upstreamEquals(t *testing.T) {
	t.Parallel()

	up := func(name string, port int) ConsulUpstream {
		return ConsulUpstream{
			DestinationName: name,
			LocalBindPort:   port,
		}
	}

	t.Run("size mismatch", func(t *testing.T) {
		a := []ConsulUpstream{up("foo", 8000)}
		b := []ConsulUpstream{up("foo", 8000), up("bar", 9000)}
		require.False(t, upstreamsEquals(a, b))
	})

	t.Run("different", func(t *testing.T) {
		a := []ConsulUpstream{up("bar", 9000)}
		b := []ConsulUpstream{up("foo", 8000)}
		require.False(t, upstreamsEquals(a, b))
	})

	t.Run("identical", func(t *testing.T) {
		a := []ConsulUpstream{up("foo", 8000), up("bar", 9000)}
		b := []ConsulUpstream{up("foo", 8000), up("bar", 9000)}
		require.True(t, upstreamsEquals(a, b))
	})

	t.Run("unsorted", func(t *testing.T) {
		a := []ConsulUpstream{up("foo", 8000), up("bar", 9000)}
		b := []ConsulUpstream{up("bar", 9000), up("foo", 8000)}
		require.True(t, upstreamsEquals(a, b))
	})
}

func TestConsulExposePath_exposePathsEqual(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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

func TestConsulExposeConfig_Equals(t *testing.T) {
	t.Parallel()

	require.True(t, (*ConsulExposeConfig)(nil).Equals(nil))
	require.True(t, (&ConsulExposeConfig{
		Paths: []ConsulExposePath{{
			Path: "/health",
		}},
	}).Equals(&ConsulExposeConfig{
		Paths: []ConsulExposePath{{
			Path: "/health",
		}},
	}))
}

func TestConsulSidecarService_Copy(t *testing.T) {
	t.Parallel()

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
		}
		result := s.Copy()
		require.Equal(t, &ConsulSidecarService{
			Tags:  []string{"foo", "bar"},
			Port:  "port1",
			Proxy: &ConsulProxy{LocalServiceAddress: "10.0.0.1"},
		}, result)
	})
}
