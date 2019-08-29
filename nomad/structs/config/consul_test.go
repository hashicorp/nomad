package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"testing"

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
