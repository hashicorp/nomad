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

func TestRetryJoin_NonCloud(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	newConfig := &Config{
		Server: &ServerConfig{
			RetryMaxAttempts: 1,
			RetryJoin:        []string{"127.0.0.1"},
			Enabled:          true,
		},
	}

	var output []string

	mockJoin := func(s []string) (int, error) {
		output = s
		return 0, nil
	}

	joiner := retryJoiner{
		discover: &MockDiscover{},
		join:     mockJoin,
		logger:   log.New(ioutil.Discard, "", 0),
		errCh:    make(chan struct{}),
	}

	joiner.RetryJoin(newConfig)

	require.Equal(1, len(output))
	require.Equal(stubAddress, output[0])
}

func TestRetryJoin_Cloud(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	newConfig := &Config{
		Server: &ServerConfig{
			RetryMaxAttempts: 1,
			RetryJoin:        []string{"provider=aws, tag_value=foo"},
			Enabled:          true,
		},
	}

	var output []string

	mockJoin := func(s []string) (int, error) {
		output = s
		return 0, nil
	}

	mockDiscover := &MockDiscover{}
	joiner := retryJoiner{
		discover: mockDiscover,
		join:     mockJoin,
		logger:   log.New(ioutil.Discard, "", 0),
		errCh:    make(chan struct{}),
	}

	joiner.RetryJoin(newConfig)

	require.Equal(1, len(output))
	require.Equal("provider=aws, tag_value=foo", mockDiscover.ReceivedAddrs)
	require.Equal(stubAddress, output[0])
}

func TestRetryJoin_MixedProvider(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	newConfig := &Config{
		Server: &ServerConfig{
			RetryMaxAttempts: 1,
			RetryJoin:        []string{"provider=aws, tag_value=foo", "127.0.0.1"},
			Enabled:          true,
		},
	}

	var output []string

	mockJoin := func(s []string) (int, error) {
		output = s
		return 0, nil
	}

	mockDiscover := &MockDiscover{}
	joiner := retryJoiner{
		discover: mockDiscover,
		join:     mockJoin,
		logger:   log.New(ioutil.Discard, "", 0),
		errCh:    make(chan struct{}),
	}

	joiner.RetryJoin(newConfig)

	require.Equal(2, len(output))
	require.Equal("provider=aws, tag_value=foo", mockDiscover.ReceivedAddrs)
	require.Equal(stubAddress, output[0])
}
