// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	sockaddr "github.com/hashicorp/go-sockaddr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/ci"
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
	ci.Parallel(t)

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
		GRPCCAFile:           "1",
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
		GRPCCAFile:           "2",
		CAFile:               "2",
		CertFile:             "2",
		KeyFile:              "2",
		ServerAutoJoin:       &yes,
		ClientAutoJoin:       &yes,
		UseIdentity:          &yes,
		ServiceIdentity: &WorkloadIdentity{
			Name:        "test",
			Audience:    []string{"consul.io", "nomad.dev"},
			Env:         false,
			File:        true,
			ServiceName: "test",
		},
		ExtraKeysHCL: []string{"b", "2"},
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
		GRPCCAFile:           "2",
		CAFile:               "2",
		CertFile:             "2",
		KeyFile:              "2",
		ServerAutoJoin:       &yes,
		ClientAutoJoin:       &yes,
		UseIdentity:          &yes,
		ServiceIdentity: &WorkloadIdentity{
			Name:        "test",
			Audience:    []string{"consul.io", "nomad.dev"},
			Env:         false,
			File:        true,
			ServiceName: "test",
		},
		ExtraKeysHCL: []string{"a", "1"}, // not merged
	}

	result := c1.Merge(c2)
	require.Equal(t, exp, result)
}

// TestConsulConfig_Defaults asserts Consul defaults are copied from their
// upstream API package defaults.
func TestConsulConfig_Defaults(t *testing.T) {
	ci.Parallel(t)

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
	ci.Parallel(t)

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

func TestConsulConfig_IpTemplateParse(t *testing.T) {
	ci.Parallel(t)

	privateIp, err := sockaddr.GetPrivateIP()
	require.NoError(t, err)

	testCases := []struct {
		name        string
		tmpl        string
		expectedOut string
		expectErr   bool
	}{
		{name: "string address keeps working", tmpl: "10.0.1.0:8500", expectedOut: "10.0.1.0:8500", expectErr: false},
		{name: "single ip sock-addr template", tmpl: "{{ GetPrivateIP }}:8500", expectedOut: privateIp + ":8500", expectErr: false},
		{name: "multi ip sock-addr template", tmpl: "10.0.1.0 10.0.1.1:8500", expectedOut: "", expectErr: true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t)
			conf := ConsulConfig{
				Addr: tc.tmpl,
			}
			out, err := conf.ApiConfig()

			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expectedOut, out.Address)
		})
	}
}
