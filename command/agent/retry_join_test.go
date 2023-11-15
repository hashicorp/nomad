// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"context"
	"fmt"
	golog "log"
	"net"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/go-netaddrs"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

const stubAddress = "127.0.0.1"

type MockDiscover struct {
	ReceivedConfig string
}

func (m *MockDiscover) Addrs(s string, l *golog.Logger) ([]string, error) {
	m.ReceivedConfig = s
	return []string{stubAddress}, nil
}
func (m *MockDiscover) Help() string { return "" }
func (m *MockDiscover) Names() []string {
	return []string{""}
}

type MockNetaddrs struct {
	ReceivedConfig []string
}

func (m *MockNetaddrs) IPAddrs(ctx context.Context, cfg string, l netaddrs.Logger) ([]net.IPAddr, error) {
	m.ReceivedConfig = append(m.ReceivedConfig, cfg)

	ip := net.ParseIP(stubAddress)
	if ip == nil {
		return nil, fmt.Errorf("unable to transform the stubAddress into a valid IP")
	}

	return []net.IPAddr{{IP: ip}}, nil
}

func TestRetryJoin_Integration(t *testing.T) {
	ci.Parallel(t)

	// Create two agents and have one retry join the other
	agent := NewTestAgent(t, t.Name(), nil)
	defer agent.Shutdown()

	agent2 := NewTestAgent(t, t.Name(), func(c *Config) {
		c.NodeName = "foo"
		if c.Server.ServerJoin == nil {
			c.Server.ServerJoin = &ServerJoin{}
		}
		c.Server.ServerJoin.RetryJoin = []string{agent.Config.normalizedAddrs.Serf}
		c.Server.ServerJoin.RetryInterval = 1 * time.Second
	})
	defer agent2.Shutdown()

	// Create a fake command and have it wrap the second agent and run the retry
	// join handler
	cmd := &Command{
		Ui: &cli.BasicUi{
			Reader:      os.Stdin,
			Writer:      os.Stdout,
			ErrorWriter: os.Stderr,
		},
		agent: agent2.Agent,
	}

	if err := cmd.handleRetryJoin(agent2.Config); err != nil {
		t.Fatalf("handleRetryJoin failed: %v", err)
	}

	// Ensure the retry join occurred.
	testutil.WaitForResult(func() (bool, error) {
		mem := agent.server.Members()
		if len(mem) != 2 {
			return false, fmt.Errorf("bad :%#v", mem)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf(err.Error())
	})
}

func TestRetryJoin_Server_NonCloud(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	serverJoin := &ServerJoin{
		RetryMaxAttempts: 1,
		RetryJoin:        []string{"127.0.0.1"},
	}

	var output []string

	mockJoin := func(s []string) (int, error) {
		output = s
		return 0, nil
	}

	joiner := retryJoiner{
		autoDiscover:  autoDiscover{goDiscover: &MockDiscover{}},
		serverJoin:    mockJoin,
		serverEnabled: true,
		logger:        testlog.HCLogger(t),
		errCh:         make(chan struct{}),
	}

	joiner.RetryJoin(serverJoin)

	require.Equal(1, len(output))
	require.Equal(stubAddress, output[0])
}

func TestRetryJoin_Server_Cloud(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	serverJoin := &ServerJoin{
		RetryMaxAttempts: 1,
		RetryJoin:        []string{"provider=aws, tag_value=foo"},
	}

	var output []string

	mockJoin := func(s []string) (int, error) {
		output = s
		return 0, nil
	}

	mockDiscover := &MockDiscover{}
	joiner := retryJoiner{
		autoDiscover:  autoDiscover{goDiscover: mockDiscover},
		serverJoin:    mockJoin,
		serverEnabled: true,
		logger:        testlog.HCLogger(t),
		errCh:         make(chan struct{}),
	}

	joiner.RetryJoin(serverJoin)

	require.Equal(1, len(output))
	require.Equal("provider=aws, tag_value=foo", mockDiscover.ReceivedConfig)
	require.Equal(stubAddress, output[0])
}

func TestRetryJoin_Server_MixedProvider(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	serverJoin := &ServerJoin{
		RetryMaxAttempts: 1,
		RetryJoin:        []string{"provider=aws, tag_value=foo", "127.0.0.1"},
	}

	var output []string

	mockJoin := func(s []string) (int, error) {
		output = s
		return 0, nil
	}

	mockDiscover := &MockDiscover{}
	joiner := retryJoiner{
		autoDiscover:  autoDiscover{goDiscover: mockDiscover},
		serverJoin:    mockJoin,
		serverEnabled: true,
		logger:        testlog.HCLogger(t),
		errCh:         make(chan struct{}),
	}

	joiner.RetryJoin(serverJoin)

	require.Equal(2, len(output))
	require.Equal("provider=aws, tag_value=foo", mockDiscover.ReceivedConfig)
	require.Equal(stubAddress, output[0])
}

func TestRetryJoin_AutoDiscover(t *testing.T) {
	ci.Parallel(t)

	var joinAddrs []string
	mockJoin := func(s []string) (int, error) {
		joinAddrs = s
		return 0, nil
	}

	// 'exec=*'' tests autoDiscover go-netaddr support
	// 'provider=aws, tag_value=foo' ensures that provider-prefixed configs are routed to go-discover
	// 'localhost' ensures that bare hostnames are returned as-is
	// 'localhost2:4648' ensures hostname:port entries are returned as-is
	// '127.0.0.1:4648' ensures ip:port entiresare returned as-is
	// '100.100.100.100' ensures that bare IPs are returned as-is
	serverJoin := &ServerJoin{
		RetryMaxAttempts: 1,
		RetryJoin: []string{
			"exec=echo 127.0.0.1", "provider=aws, tag_value=foo",
			"localhost", "localhost2:4648", "127.0.0.1:4648", "100.100.100.100"},
	}

	mockDiscover := &MockDiscover{}
	mockNetaddrs := &MockNetaddrs{}
	joiner := retryJoiner{
		autoDiscover:  autoDiscover{goDiscover: mockDiscover, netAddrs: mockNetaddrs},
		serverJoin:    mockJoin,
		serverEnabled: true,
		logger:        testlog.HCLogger(t),
		errCh:         make(chan struct{}),
	}

	joiner.RetryJoin(serverJoin)

	must.Eq(t, []string{
		"127.0.0.1", "127.0.0.1", "localhost", "localhost2:4648",
		"127.0.0.1:4648", "100.100.100.100"},
		joinAddrs)
	must.Eq(t, []string{"exec=echo 127.0.0.1"}, mockNetaddrs.ReceivedConfig)
	must.Eq(t, "provider=aws, tag_value=foo", mockDiscover.ReceivedConfig)
}

func TestRetryJoin_Client(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	serverJoin := &ServerJoin{
		RetryMaxAttempts: 1,
		RetryJoin:        []string{"127.0.0.1"},
	}

	var output []string

	mockJoin := func(s []string) (int, error) {
		output = s
		return 0, nil
	}

	joiner := retryJoiner{
		autoDiscover:  autoDiscover{goDiscover: &MockDiscover{}},
		clientJoin:    mockJoin,
		clientEnabled: true,
		logger:        testlog.HCLogger(t),
		errCh:         make(chan struct{}),
	}

	joiner.RetryJoin(serverJoin)

	require.Equal(1, len(output))
	require.Equal(stubAddress, output[0])
}

func TestRetryJoin_Validate(t *testing.T) {
	ci.Parallel(t)
	type validateExpect struct {
		config  *Config
		isValid bool
		reason  string
	}

	scenarios := []*validateExpect{
		{
			config: &Config{
				Server: &ServerConfig{
					ServerJoin: &ServerJoin{
						RetryJoin:        []string{"127.0.0.1"},
						RetryMaxAttempts: 0,
						RetryInterval:    0,
						StartJoin:        []string{},
					},
					RetryJoin:        []string{"127.0.0.1"},
					RetryMaxAttempts: 0,
					RetryInterval:    0,
					StartJoin:        []string{},
				},
			},
			isValid: false,
			reason:  "server_join cannot be defined if retry_join is defined on the server block",
		},
		{
			config: &Config{
				Server: &ServerConfig{
					ServerJoin: &ServerJoin{
						RetryJoin:        []string{"127.0.0.1"},
						RetryMaxAttempts: 0,
						RetryInterval:    0,
						StartJoin:        []string{},
					},
					StartJoin:        []string{"127.0.0.1"},
					RetryMaxAttempts: 0,
					RetryInterval:    0,
					RetryJoin:        []string{},
				},
			},
			isValid: false,
			reason:  "server_join cannot be defined if start_join is defined on the server block",
		},
		{
			config: &Config{
				Server: &ServerConfig{
					ServerJoin: &ServerJoin{
						RetryJoin:        []string{"127.0.0.1"},
						RetryMaxAttempts: 0,
						RetryInterval:    0,
						StartJoin:        []string{},
					},
					StartJoin:        []string{},
					RetryMaxAttempts: 1,
					RetryInterval:    0,
					RetryJoin:        []string{},
				},
			},
			isValid: false,
			reason:  "server_join cannot be defined if retry_max_attempts is defined on the server block",
		},
		{
			config: &Config{
				Server: &ServerConfig{
					ServerJoin: &ServerJoin{
						RetryJoin:        []string{"127.0.0.1"},
						RetryMaxAttempts: 0,
						RetryInterval:    time.Duration(1),
						StartJoin:        []string{},
					},
					StartJoin:        []string{},
					RetryMaxAttempts: 0,
					RetryInterval:    3 * time.Second,
					RetryJoin:        []string{},
				},
			},
			isValid: false,
			reason:  "server_join cannot be defined if retry_interval is defined on the server block",
		},
		{
			config: &Config{
				Server: &ServerConfig{
					ServerJoin: &ServerJoin{
						RetryJoin:        []string{"127.0.0.1"},
						RetryMaxAttempts: 0,
						RetryInterval:    0,
						StartJoin:        []string{"127.0.0.1"},
					},
				},
			},
			isValid: false,
			reason:  "start_join and retry_join should not both be defined",
		},
		{
			config: &Config{
				Client: &ClientConfig{
					ServerJoin: &ServerJoin{
						RetryJoin:        []string{},
						RetryMaxAttempts: 0,
						RetryInterval:    0,
						StartJoin:        []string{"127.0.0.1"},
					},
				},
			},
			isValid: false,
			reason:  "start_join should not be defined on the client",
		},
		{
			config: &Config{
				Client: &ClientConfig{
					ServerJoin: &ServerJoin{
						RetryJoin:        []string{"127.0.0.1"},
						RetryMaxAttempts: 0,
						RetryInterval:    0,
					},
				},
			},
			isValid: true,
			reason:  "client server_join should be valid",
		},
		{
			config: &Config{
				Server: &ServerConfig{
					ServerJoin: &ServerJoin{
						RetryJoin:        []string{"127.0.0.1"},
						RetryMaxAttempts: 1,
						RetryInterval:    1,
						StartJoin:        []string{},
					},
				},
			},
			isValid: true,
			reason:  "server server_join should be valid",
		},
	}

	joiner := retryJoiner{}
	for _, scenario := range scenarios {
		t.Run(scenario.reason, func(t *testing.T) {
			err := joiner.Validate(scenario.config)
			if scenario.isValid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
