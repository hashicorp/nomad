package agent

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/nomad/version"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

type MockDiscover struct {
	ReceivedAddrs string
}

const stubAddress = "127.0.0.1"

func (m *MockDiscover) Addrs(s string, l *log.Logger) ([]string, error) {
	m.ReceivedAddrs = s
	return []string{stubAddress}, nil
}
func (m *MockDiscover) Help() string { return "" }
func (m *MockDiscover) Names() []string {
	return []string{""}
}

func TestRetryJoin_Integration(t *testing.T) {
	t.Parallel()
	agent := NewTestAgent(t, t.Name(), nil)
	defer agent.Shutdown()

	doneCh := make(chan struct{})
	shutdownCh := make(chan struct{})

	defer func() {
		close(shutdownCh)
		<-doneCh
	}()

	cmd := &Command{
		Version:    version.GetVersion(),
		ShutdownCh: shutdownCh,
		Ui: &cli.BasicUi{
			Reader:      os.Stdin,
			Writer:      os.Stdout,
			ErrorWriter: os.Stderr,
		},
	}

	serfAddr := agent.Config.normalizedAddrs.Serf

	args := []string{
		"-dev",
		"-node", "foo",
		"-retry-join", serfAddr,
		"-retry-interval", "1s",
	}

	go func() {
		if code := cmd.Run(args); code != 0 {
			t.Logf("bad: %d", code)
		}
		close(doneCh)
	}()

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
	t.Parallel()
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
		discover:      &MockDiscover{},
		serverJoin:    mockJoin,
		serverEnabled: true,
		logger:        log.New(ioutil.Discard, "", 0),
		errCh:         make(chan struct{}),
	}

	joiner.RetryJoin(serverJoin)

	require.Equal(1, len(output))
	require.Equal(stubAddress, output[0])
}

func TestRetryJoin_Server_Cloud(t *testing.T) {
	t.Parallel()
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
		discover:      mockDiscover,
		serverJoin:    mockJoin,
		serverEnabled: true,
		logger:        log.New(ioutil.Discard, "", 0),
		errCh:         make(chan struct{}),
	}

	joiner.RetryJoin(serverJoin)

	require.Equal(1, len(output))
	require.Equal("provider=aws, tag_value=foo", mockDiscover.ReceivedAddrs)
	require.Equal(stubAddress, output[0])
}

func TestRetryJoin_Server_MixedProvider(t *testing.T) {
	t.Parallel()
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
		discover:      mockDiscover,
		serverJoin:    mockJoin,
		serverEnabled: true,
		logger:        log.New(ioutil.Discard, "", 0),
		errCh:         make(chan struct{}),
	}

	joiner.RetryJoin(serverJoin)

	require.Equal(2, len(output))
	require.Equal("provider=aws, tag_value=foo", mockDiscover.ReceivedAddrs)
	require.Equal(stubAddress, output[0])
}

func TestRetryJoin_Client(t *testing.T) {
	t.Parallel()
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
		discover:      &MockDiscover{},
		clientJoin:    mockJoin,
		clientEnabled: true,
		logger:        log.New(ioutil.Discard, "", 0),
		errCh:         make(chan struct{}),
	}

	joiner.RetryJoin(serverJoin)

	require.Equal(1, len(output))
	require.Equal(stubAddress, output[0])
}

func TestRetryJoin_Validate(t *testing.T) {
	t.Parallel()
	require := require.New(t)

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
						RetryInterval:    "0",
						StartJoin:        []string{},
					},
					RetryJoin:        []string{"127.0.0.1"},
					RetryMaxAttempts: 0,
					RetryInterval:    "0",
					StartJoin:        []string{},
				},
			},
			isValid: false,
			reason:  "server_join cannot be defined if retry_join is defined on the server stanza",
		},
		{
			config: &Config{
				Server: &ServerConfig{
					ServerJoin: &ServerJoin{
						RetryJoin:        []string{"127.0.0.1"},
						RetryMaxAttempts: 0,
						RetryInterval:    "0",
						StartJoin:        []string{},
					},
					StartJoin:        []string{"127.0.0.1"},
					RetryMaxAttempts: 0,
					RetryInterval:    "0",
					RetryJoin:        []string{},
				},
			},
			isValid: false,
			reason:  "server_join cannot be defined if start_join is defined on the server stanza",
		},
		{
			config: &Config{
				Server: &ServerConfig{
					ServerJoin: &ServerJoin{
						RetryJoin:        []string{"127.0.0.1"},
						RetryMaxAttempts: 0,
						RetryInterval:    "0",
						StartJoin:        []string{},
					},
					StartJoin:        []string{},
					RetryMaxAttempts: 1,
					RetryInterval:    "0",
					RetryJoin:        []string{},
				},
			},
			isValid: false,
			reason:  "server_join cannot be defined if retry_max_attempts is defined on the server stanza",
		},
		{
			config: &Config{
				Server: &ServerConfig{
					ServerJoin: &ServerJoin{
						RetryJoin:        []string{"127.0.0.1"},
						RetryMaxAttempts: 0,
						RetryInterval:    "0",
						StartJoin:        []string{},
					},
					StartJoin:        []string{},
					RetryMaxAttempts: 0,
					RetryInterval:    "1",
					RetryJoin:        []string{},
				},
			},
			isValid: false,
			reason:  "server_join cannot be defined if retry_interval is defined on the server stanza",
		},
		{
			config: &Config{
				Client: &ClientConfig{
					ServerJoin: &ServerJoin{
						RetryJoin:        []string{"127.0.0.1"},
						RetryMaxAttempts: 0,
						RetryInterval:    "0",
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
						RetryInterval:    "0",
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
						RetryMaxAttempts: 0,
						RetryInterval:    "0",
						StartJoin:        []string{},
					},
					StartJoin:        []string{},
					RetryMaxAttempts: 0,
					RetryInterval:    "0",
					RetryJoin:        []string{},
				},
			},
			isValid: true,
			reason:  "server server_join should be valid",
		},
		{
			config: &Config{
				Server: &ServerConfig{
					StartJoin:        []string{"127.0.0.1"},
					RetryMaxAttempts: 1,
					RetryInterval:    "0",
					RetryJoin:        []string{},
				},
			},
			isValid: true,
			reason:  "server deprecated retry_join configuration should be valid",
		},
	}

	joiner := retryJoiner{}
	for _, scenario := range scenarios {
		err := joiner.Validate(scenario.config)
		require.Equal(err == nil, scenario.isValid, scenario.reason)
	}
}
