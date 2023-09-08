// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"context"
	"fmt"
	"path"
	"strings"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_RPC(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()

	var out struct{}
	if err := s1.RPC("Status.Ping", &structs.GenericRequest{}, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestServer_RPC_TLS(t *testing.T) {
	ci.Parallel(t)

	const (
		cafile  = "../helper/tlsutil/testdata/nomad-agent-ca.pem"
		foocert = "../helper/tlsutil/testdata/regionFoo-server-nomad.pem"
		fookey  = "../helper/tlsutil/testdata/regionFoo-server-nomad-key.pem"
	)
	dir := t.TempDir()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.Region = "regionFoo"
		c.BootstrapExpect = 3
		c.DevMode = false
		c.DataDir = path.Join(dir, "node1")
		c.TLSConfig = &config.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		}
	})
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.Region = "regionFoo"
		c.BootstrapExpect = 3
		c.DevMode = false
		c.DataDir = path.Join(dir, "node2")
		c.TLSConfig = &config.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		}
	})
	defer cleanupS2()

	s3, cleanupS3 := TestServer(t, func(c *Config) {
		c.Region = "regionFoo"
		c.BootstrapExpect = 3
		c.DevMode = false
		c.DataDir = path.Join(dir, "node3")
		c.TLSConfig = &config.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		}
	})
	defer cleanupS3()

	TestJoin(t, s1, s2, s3)
	testutil.WaitForLeader(t, s1.RPC)

	// Part of a server joining is making an RPC request, so just by testing
	// that there is a leader we verify that the RPCs are working over TLS.
}

func TestServer_RPC_MixedTLS(t *testing.T) {
	ci.Parallel(t)

	const (
		cafile  = "../helper/tlsutil/testdata/nomad-agent-ca.pem"
		foocert = "../helper/tlsutil/testdata/regionFoo-server-nomad.pem"
		fookey  = "../helper/tlsutil/testdata/regionFoo-server-nomad-key.pem"
	)
	dir := t.TempDir()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.Region = "regionFoo"
		c.BootstrapExpect = 3
		c.DevMode = false
		c.DataDir = path.Join(dir, "node1")
		c.TLSConfig = &config.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		}
	})
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.Region = "regionFoo"
		c.BootstrapExpect = 3
		c.DevMode = false
		c.DataDir = path.Join(dir, "node2")
		c.TLSConfig = &config.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		}
	})
	defer cleanupS2()

	s3, cleanupS3 := TestServer(t, func(c *Config) {
		c.Region = "regionFoo"
		c.BootstrapExpect = 3
		c.DevMode = false
		c.DataDir = path.Join(dir, "node3")
	})
	defer cleanupS3()

	TestJoin(t, s1, s2, s3)

	// Ensure that we do not form a quorum
	start := time.Now()
	for {
		if time.Now().After(start.Add(2 * time.Second)) {
			break
		}

		args := &structs.GenericRequest{}
		var leader string
		err := s1.RPC("Status.Leader", args, &leader)
		if err == nil || leader != "" {
			t.Fatalf("Got leader or no error: %q %v", leader, err)
		}
	}
}

func TestServer_Regions(t *testing.T) {
	ci.Parallel(t)

	// Make the servers
	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.Region = "region1"
	})
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.Region = "region2"
	})
	defer cleanupS2()

	// Join them together
	s2Addr := fmt.Sprintf("127.0.0.1:%d",
		s2.config.SerfConfig.MemberlistConfig.BindPort)
	if n, err := s1.Join([]string{s2Addr}); err != nil || n != 1 {
		t.Fatalf("Failed joining: %v (%d joined)", err, n)
	}

	// Try listing the regions
	testutil.WaitForResult(func() (bool, error) {
		out := s1.Regions()
		if len(out) != 2 || out[0] != "region1" || out[1] != "region2" {
			return false, fmt.Errorf("unexpected regions: %v", out)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestServer_Reload_Vault(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.Region = "global"
	})
	defer cleanupS1()

	if s1.vault.Running() {
		t.Fatalf("Vault client should not be running")
	}

	tr := true
	config := DefaultConfig()
	config.VaultConfig.Enabled = &tr
	config.VaultConfig.Token = uuid.Generate()
	config.VaultConfig.Namespace = "nondefault"

	if err := s1.Reload(config); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	if !s1.vault.Running() {
		t.Fatalf("Vault client should be running")
	}

	if s1.vault.GetConfig().Namespace != "nondefault" {
		t.Fatalf("Vault client did not get new namespace")
	}
}

func connectionReset(msg string) bool {
	return strings.Contains(msg, "EOF") || strings.Contains(msg, "connection reset by peer")
}

// Tests that the server will successfully reload its network connections,
// upgrading from plaintext to TLS if the server's TLS configuration changes.
func TestServer_Reload_TLSConnections_PlaintextToTLS(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	const (
		cafile  = "../helper/tlsutil/testdata/nomad-agent-ca.pem"
		foocert = "../helper/tlsutil/testdata/regionFoo-client-nomad.pem"
		fookey  = "../helper/tlsutil/testdata/regionFoo-client-nomad-key.pem"
	)
	dir := t.TempDir()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.DataDir = path.Join(dir, "nodeA")
	})
	defer cleanupS1()

	// assert that the server started in plaintext mode
	assert.Equal(s1.config.TLSConfig.CertFile, "")

	newTLSConfig := &config.TLSConfig{
		EnableHTTP:           true,
		EnableRPC:            true,
		VerifyServerHostname: true,
		CAFile:               cafile,
		CertFile:             foocert,
		KeyFile:              fookey,
	}

	err := s1.reloadTLSConnections(newTLSConfig)
	assert.Nil(err)
	assert.True(s1.config.TLSConfig.CertificateInfoIsEqual(newTLSConfig))

	codec := rpcClient(t, s1)

	node := mock.Node()
	req := &structs.NodeRegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	var resp structs.GenericResponse
	err = msgpackrpc.CallWithCodec(codec, "Node.Register", req, &resp)
	assert.NotNil(err)
	assert.True(connectionReset(err.Error()))
}

// Tests that the server will successfully reload its network connections,
// downgrading from TLS to plaintext if the server's TLS configuration changes.
func TestServer_Reload_TLSConnections_TLSToPlaintext_RPC(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	const (
		cafile  = "../helper/tlsutil/testdata/nomad-agent-ca.pem"
		foocert = "../helper/tlsutil/testdata/regionFoo-client-nomad.pem"
		fookey  = "../helper/tlsutil/testdata/regionFoo-client-nomad-key.pem"
	)

	dir := t.TempDir()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.DataDir = path.Join(dir, "nodeB")
		c.TLSConfig = &config.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		}
	})
	defer cleanupS1()

	newTLSConfig := &config.TLSConfig{}

	err := s1.reloadTLSConnections(newTLSConfig)
	assert.Nil(err)
	assert.True(s1.config.TLSConfig.CertificateInfoIsEqual(newTLSConfig))

	codec := rpcClient(t, s1)

	node := mock.Node()
	req := &structs.NodeRegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	var resp structs.GenericResponse
	err = msgpackrpc.CallWithCodec(codec, "Node.Register", req, &resp)
	assert.Nil(err)
}

// Tests that the server will successfully reload its network connections,
// downgrading only RPC connections
func TestServer_Reload_TLSConnections_TLSToPlaintext_OnlyRPC(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	const (
		cafile  = "../helper/tlsutil/testdata/nomad-agent-ca.pem"
		foocert = "../helper/tlsutil/testdata/regionFoo-client-nomad.pem"
		fookey  = "../helper/tlsutil/testdata/regionFoo-client-nomad-key.pem"
	)

	dir := t.TempDir()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.DataDir = path.Join(dir, "nodeB")
		c.TLSConfig = &config.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		}
	})
	defer cleanupS1()

	newTLSConfig := &config.TLSConfig{
		EnableHTTP:           true,
		EnableRPC:            false,
		VerifyServerHostname: true,
		CAFile:               cafile,
		CertFile:             foocert,
		KeyFile:              fookey,
	}

	err := s1.reloadTLSConnections(newTLSConfig)
	assert.Nil(err)
	assert.True(s1.config.TLSConfig.CertificateInfoIsEqual(newTLSConfig))

	codec := rpcClient(t, s1)

	node := mock.Node()
	req := &structs.NodeRegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	var resp structs.GenericResponse
	err = msgpackrpc.CallWithCodec(codec, "Node.Register", req, &resp)
	assert.Nil(err)
}

// Tests that the server will successfully reload its network connections,
// upgrading only RPC connections
func TestServer_Reload_TLSConnections_PlaintextToTLS_OnlyRPC(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	const (
		cafile  = "../helper/tlsutil/testdata/nomad-agent-ca.pem"
		foocert = "../helper/tlsutil/testdata/regionFoo-client-nomad.pem"
		fookey  = "../helper/tlsutil/testdata/regionFoo-client-nomad-key.pem"
	)

	dir := t.TempDir()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.DataDir = path.Join(dir, "nodeB")
		c.TLSConfig = &config.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            false,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		}
	})
	defer cleanupS1()

	newTLSConfig := &config.TLSConfig{
		EnableHTTP:           true,
		EnableRPC:            true,
		VerifyServerHostname: true,
		CAFile:               cafile,
		CertFile:             foocert,
		KeyFile:              fookey,
	}

	err := s1.reloadTLSConnections(newTLSConfig)
	assert.Nil(err)
	assert.True(s1.config.TLSConfig.EnableRPC)
	assert.True(s1.config.TLSConfig.CertificateInfoIsEqual(newTLSConfig))

	codec := rpcClient(t, s1)

	node := mock.Node()
	req := &structs.NodeRegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	var resp structs.GenericResponse
	err = msgpackrpc.CallWithCodec(codec, "Node.Register", req, &resp)
	assert.NotNil(err)
	assert.True(connectionReset(err.Error()))
}

// Test that Raft connections are reloaded as expected when a Nomad server is
// upgraded from plaintext to TLS
func TestServer_Reload_TLSConnections_Raft(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	const (
		cafile  = "../../helper/tlsutil/testdata/nomad-agent-ca.pem"
		foocert = "../../helper/tlsutil/testdata/regionFoo-client-nomad.pem"
		fookey  = "../../helper/tlsutil/testdata/regionFoo-client-nomad-key.pem"
		barcert = "../dev/tls_cluster/certs/nomad.pem"
		barkey  = "../dev/tls_cluster/certs/nomad-key.pem"
	)
	dir := t.TempDir()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 2
		c.DevMode = false
		c.DataDir = path.Join(dir, "node1")
		c.NodeName = "node1"
		c.Region = "regionFoo"
	})
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 2
		c.DevMode = false
		c.DataDir = path.Join(dir, "node2")
		c.NodeName = "node2"
		c.Region = "regionFoo"
	})
	defer cleanupS2()

	TestJoin(t, s1, s2)
	servers := []*Server{s1, s2}

	testutil.WaitForLeader(t, s1.RPC)

	newTLSConfig := &config.TLSConfig{
		EnableHTTP:        true,
		VerifyHTTPSClient: true,
		CAFile:            cafile,
		CertFile:          foocert,
		KeyFile:           fookey,
	}

	err := s1.reloadTLSConnections(newTLSConfig)
	assert.Nil(err)

	{
		for _, serv := range servers {
			testutil.WaitForResult(func() (bool, error) {
				args := &structs.GenericRequest{}
				var leader string
				err := serv.RPC("Status.Leader", args, &leader)
				if leader != "" && err != nil {
					return false, fmt.Errorf("Should not have found leader but got %s", leader)
				}
				return true, nil
			}, func(err error) {
				t.Fatalf("err: %v", err)
			})
		}
	}

	secondNewTLSConfig := &config.TLSConfig{
		EnableHTTP:        true,
		VerifyHTTPSClient: true,
		CAFile:            cafile,
		CertFile:          barcert,
		KeyFile:           barkey,
	}

	// Now, transition the other server to TLS, which should restore their
	// ability to communicate.
	err = s2.reloadTLSConnections(secondNewTLSConfig)
	assert.Nil(err)

	testutil.WaitForLeader(t, s2.RPC)
}

func TestServer_ReloadRaftConfig(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.RaftConfig.TrailingLogs = 10
	})
	defer cleanupS1()

	testutil.WaitForLeader(t, s1.RPC)
	rc := s1.raft.ReloadableConfig()
	must.Eq(t, rc.TrailingLogs, uint64(10))
	cfg := s1.GetConfig()
	cfg.RaftConfig.TrailingLogs = 100

	// Hot-reload the configuration
	s1.Reload(cfg)

	// Check it from the raft library
	rc = s1.raft.ReloadableConfig()
	must.Eq(t, rc.TrailingLogs, uint64(100))
}

func TestServer_InvalidSchedulers(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Set the config to not have the core scheduler
	config := DefaultConfig()
	logger := testlog.HCLogger(t)
	s := &Server{
		config: config,
		logger: logger,
	}

	config.EnabledSchedulers = []string{"batch"}
	err := s.setupWorkers(s.shutdownCtx)
	require.NotNil(err)
	require.Contains(err.Error(), "scheduler not enabled")

	// Set the config to have an unknown scheduler
	config.EnabledSchedulers = []string{"batch", structs.JobTypeCore, "foo"}
	err = s.setupWorkers(s.shutdownCtx)
	require.NotNil(err)
	require.Contains(err.Error(), "foo")
}

func TestServer_RPCNameAndRegionValidation(t *testing.T) {
	ci.Parallel(t)
	for _, tc := range []struct {
		name     string
		region   string
		expected bool
	}{
		// OK
		{name: "client.global.nomad", region: "global", expected: true},
		{name: "server.global.nomad", region: "global", expected: true},
		{name: "server.other.nomad", region: "global", expected: true},
		{name: "server.other.region.nomad", region: "other.region", expected: true},

		// Bad
		{name: "client.other.nomad", region: "global", expected: false},
		{name: "client.global.nomad.other", region: "global", expected: false},
		{name: "server.global.nomad.other", region: "global", expected: false},
		{name: "other.global.nomad", region: "global", expected: false},
		{name: "server.nomad", region: "global", expected: false},
		{name: "localhost", region: "global", expected: false},
	} {
		assert.Equal(t, tc.expected, validateRPCRegionPeer(tc.name, tc.region),
			"expected %q in region %q to validate as %v",
			tc.name, tc.region, tc.expected)
	}
}

func TestServer_ReloadSchedulers_NumSchedulers(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 8
	})
	defer cleanupS1()

	require.Equal(t, s1.config.NumSchedulers, len(s1.workers))

	config := DefaultConfig()
	config.NumSchedulers = 4
	require.NoError(t, s1.Reload(config))

	time.Sleep(1 * time.Second)
	require.Equal(t, config.NumSchedulers, len(s1.workers))
}

func TestServer_ReloadSchedulers_EnabledSchedulers(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.EnabledSchedulers = []string{structs.JobTypeCore, structs.JobTypeSystem}
	})
	defer cleanupS1()

	require.Equal(t, s1.config.NumSchedulers, len(s1.workers))

	config := DefaultConfig()
	config.EnabledSchedulers = []string{structs.JobTypeCore, structs.JobTypeSystem, structs.JobTypeBatch}
	require.NoError(t, s1.Reload(config))

	time.Sleep(1 * time.Second)
	require.Equal(t, config.NumSchedulers, len(s1.workers))
	require.ElementsMatch(t, config.EnabledSchedulers, s1.GetSchedulerWorkerConfig().EnabledSchedulers)

}

func TestServer_ReloadSchedulers_InvalidSchedulers(t *testing.T) {
	ci.Parallel(t)

	// Set the config to not have the core scheduler
	config := DefaultConfig()
	logger := testlog.HCLogger(t)
	s := &Server{
		config: config,
		logger: logger,
	}
	s.config.NumSchedulers = 0
	s.shutdownCtx, s.shutdownCancel = context.WithCancel(context.Background())
	s.shutdownCh = s.shutdownCtx.Done()

	config.EnabledSchedulers = []string{"_core", "batch"}
	err := s.setupWorkers(s.shutdownCtx)
	require.Nil(t, err)
	origWC := s.GetSchedulerWorkerConfig()
	reloadSchedulers(s, &SchedulerWorkerPoolArgs{NumSchedulers: config.NumSchedulers, EnabledSchedulers: []string{"batch"}})
	currentWC := s.GetSchedulerWorkerConfig()
	require.Equal(t, origWC, currentWC)

	// Set the config to have an unknown scheduler
	reloadSchedulers(s, &SchedulerWorkerPoolArgs{NumSchedulers: config.NumSchedulers, EnabledSchedulers: []string{"_core", "foo"}})
	currentWC = s.GetSchedulerWorkerConfig()
	require.Equal(t, origWC, currentWC)
}

func TestServer_PreventRaftDowngrade(t *testing.T) {
	ci.Parallel(t)

	dir := t.TempDir()
	_, cleanupv3 := TestServer(t, func(c *Config) {
		c.DevMode = false
		c.DataDir = dir
		c.RaftConfig.ProtocolVersion = 3
	})
	cleanupv3()

	_, cleanupv2, err := TestServerErr(t, func(c *Config) {
		c.DevMode = false
		c.DataDir = dir
		c.RaftConfig.ProtocolVersion = 2
	})
	if cleanupv2 != nil {
		defer cleanupv2()
	}

	// Downgrading Raft should prevent the server from starting.
	require.Error(t, err)
}
