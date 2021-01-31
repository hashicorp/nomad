package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	if os.Getenv("NOMAD_ENV_TEST") != "1" {
		os.Exit(m.Run())
	}

	// Encode the default config as json to stdout for testing env var
	// handling.
	if err := json.NewEncoder(os.Stdout).Encode(DefaultConsulConfig()); err != nil {
		fmt.Fprintf(os.Stderr, "error encoding config: %v", err)
		os.Exit(2)
	}

	os.Exit(0)
}

func TestConsulConfig_Merge(t *testing.T) {
	yes, no := true, false

	c1 := &ConsulConfig{
		ServerServiceName:    "1",
		ServerHTTPCheckName:  "1",
		ServerSerfCheckName:  "1",
		ServerRPCCheckName:   "1",
		ClientServiceName:    "1",
		ClientHTTPCheckName:  "1",
		Tags:                 []string{"a", "1"},
		AutoAdvertise:        &no,
		ChecksUseAdvertise:   &no,
		Addr:                 "1",
		GRPCAddr:             "1",
		Timeout:              time.Duration(1),
		TimeoutHCL:           "1",
		Token:                "1",
		AllowUnauthenticated: &no,
		Auth:                 "1",
		EnableSSL:            &no,
		VerifySSL:            &no,
		CAFile:               "1",
		CertFile:             "1",
		KeyFile:              "1",
		ServerAutoJoin:       &no,
		ClientAutoJoin:       &no,
		ExtraKeysHCL:         []string{"a", "1"},
	}

	c2 := &ConsulConfig{
		ServerServiceName:    "2",
		ServerHTTPCheckName:  "2",
		ServerSerfCheckName:  "2",
		ServerRPCCheckName:   "2",
		ClientServiceName:    "2",
		ClientHTTPCheckName:  "2",
		Tags:                 []string{"b", "2"},
		AutoAdvertise:        &yes,
		ChecksUseAdvertise:   &yes,
		Addr:                 "2",
		GRPCAddr:             "2",
		Timeout:              time.Duration(2),
		TimeoutHCL:           "2",
		Token:                "2",
		AllowUnauthenticated: &yes,
		Auth:                 "2",
		EnableSSL:            &yes,
		VerifySSL:            &yes,
		CAFile:               "2",
		CertFile:             "2",
		KeyFile:              "2",
		ServerAutoJoin:       &yes,
		ClientAutoJoin:       &yes,
		ExtraKeysHCL:         []string{"b", "2"},
	}

	exp := &ConsulConfig{
		ServerServiceName:    "2",
		ServerHTTPCheckName:  "2",
		ServerSerfCheckName:  "2",
		ServerRPCCheckName:   "2",
		ClientServiceName:    "2",
		ClientHTTPCheckName:  "2",
		Tags:                 []string{"a", "1", "b", "2"},
		AutoAdvertise:        &yes,
		ChecksUseAdvertise:   &yes,
		Addr:                 "2",
		GRPCAddr:             "2",
		Timeout:              time.Duration(2),
		TimeoutHCL:           "2",
		Token:                "2",
		AllowUnauthenticated: &yes,
		Auth:                 "2",
		EnableSSL:            &yes,
		VerifySSL:            &yes,
		CAFile:               "2",
		CertFile:             "2",
		KeyFile:              "2",
		ServerAutoJoin:       &yes,
		ClientAutoJoin:       &yes,
		ExtraKeysHCL:         []string{"a", "1"}, // not merged
	}

	result := c1.Merge(c2)
	require.Equal(t, exp, result)
}

// TestConsulConfig_Defaults asserts Consul defaults are copied from their
// upstream API package defaults.
func TestConsulConfig_Defaults(t *testing.T) {
	t.Parallel()

	nomadDef := DefaultConsulConfig()
	consulDef := consulapi.DefaultConfig()

	require.Equal(t, consulDef.Address, nomadDef.Addr)
	require.NotZero(t, nomadDef.Addr)
	require.Equal(t, consulDef.Scheme == "https", *nomadDef.EnableSSL)
	require.Equal(t, !consulDef.TLSConfig.InsecureSkipVerify, *nomadDef.VerifySSL)
	require.Equal(t, consulDef.TLSConfig.CAFile, nomadDef.CAFile)
}

// TestConsulConfig_Exec asserts Consul defaults use env vars when they are
// set by forking a subprocess.
func TestConsulConfig_Exec(t *testing.T) {
	t.Parallel()

	self, err := os.Executable()
	if err != nil {
		t.Fatalf("error finding test binary: %v", err)
	}

	cmd := exec.Command(self)
	cmd.Env = []string{
		"NOMAD_ENV_TEST=1",
		"CONSUL_CACERT=cacert",
		"CONSUL_HTTP_ADDR=addr",
		"CONSUL_HTTP_SSL=1",
		"CONSUL_HTTP_SSL_VERIFY=1",
	}

	out, err := cmd.Output()
	if err != nil {
		if eerr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("exit error code %d; output:\n%s", eerr.ExitCode(), string(eerr.Stderr))
		}
		t.Fatalf("error running command %q: %v", self, err)
	}

	conf := ConsulConfig{}
	require.NoError(t, json.Unmarshal(out, &conf))
	assert.Equal(t, "cacert", conf.CAFile)
	assert.Equal(t, "addr", conf.Addr)
	require.NotNil(t, conf.EnableSSL)
	assert.True(t, *conf.EnableSSL)
	require.NotNil(t, conf.VerifySSL)
	assert.True(t, *conf.VerifySSL)
}
