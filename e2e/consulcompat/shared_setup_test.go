// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consulcompat

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	consulapi "github.com/hashicorp/consul/api"
	consulTestUtil "github.com/hashicorp/consul/sdk/testutil"
	nomadapi "github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

const (
	consulDataDir = "consul-data"
)

// startConsul runs a Consul agent with bootstrapped ACLs and returns a stop
// function, the HTTP address, and a HTTP API client
func startConsul(t *testing.T, b build, baseDir, ns string) (string, *consulapi.Client) {

	path := filepath.Join(baseDir, binDir, b.Version)
	cwd, _ := os.Getwd()
	os.Chdir(path)      // so that we can launch Consul from the current directory
	defer os.Chdir(cwd) // return to the test dir so we can find job files

	oldpath := os.Getenv("PATH")
	os.Setenv("PATH", path+":"+oldpath)
	t.Cleanup(func() {
		os.Setenv("PATH", oldpath)
	})

	consulDC1 := "dc1"
	rootToken := uuid.Generate()

	testconsul, err := consulTestUtil.NewTestServerConfigT(t,
		func(c *consulTestUtil.TestServerConfig) {
			c.ACL.Enabled = true
			c.ACL.DefaultPolicy = "deny"
			c.ACL.Tokens = consulTestUtil.TestTokens{
				InitialManagement: rootToken,
			}
			c.Datacenter = consulDC1
			c.DataDir = filepath.Join(baseDir, binDir, b.Version, consulDataDir)
			c.LogLevel = "debug"
			c.Connect = map[string]any{"enabled": true}
			c.Server = true

			if !testing.Verbose() {
				c.Stdout = io.Discard
				c.Stderr = io.Discard
			}
		})
	must.NoError(t, err, must.Sprint("error starting test consul server"))

	t.Cleanup(func() {
		testconsul.Stop()
		os.RemoveAll(filepath.Join(baseDir, binDir, b.Version, consulDataDir))
	})

	testconsul.WaitForLeader(t)
	testconsul.WaitForActiveCARoot(t)

	// TODO: we should run this entire test suite with mTLS everywhere
	consulClient, err := consulapi.NewClient(&consulapi.Config{
		Address:    testconsul.HTTPAddr,
		Scheme:     "http",
		Datacenter: consulDC1,
		HttpClient: consulapi.DefaultConfig().HttpClient,
		Token:      rootToken,
		Namespace:  ns,
		TLSConfig:  consulapi.TLSConfig{},
	})
	must.NoError(t, err)

	return testconsul.HTTPAddr, consulClient
}

// startNomad runs a Nomad agent in dev mode with bootstrapped ACLs
func startNomad(t *testing.T, consulConfig *testutil.Consul) *nomadapi.Client {

	rootToken := uuid.Generate()

	ts := testutil.NewTestServer(t, func(c *testutil.TestServerConfig) {
		c.DevMode = true
		c.LogLevel = testlog.HCLoggerTestLevel().String()
		c.Consul = consulConfig
		c.ACL = &testutil.ACLConfig{
			Enabled:        true,
			BootstrapToken: rootToken,
		}

		if !testing.Verbose() {
			c.Stdout = io.Discard
			c.Stderr = io.Discard
		}
	})

	t.Cleanup(ts.Stop)

	// TODO: we should run this entire test suite with mTLS everywhere
	nc, err := nomadapi.NewClient(&nomadapi.Config{
		Address:   "http://" + ts.HTTPAddr,
		TLSConfig: &nomadapi.TLSConfig{},
	})
	must.NoError(t, err, must.Sprint("unable to create nomad api client"))

	nc.SetSecretID(rootToken)
	return nc
}
