package nomad

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tmpDir(t *testing.T) string {
	t.Helper()
	dir, err := ioutil.TempDir("", "nomad")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return dir
}

func TestServer_RPC(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
	defer s1.Shutdown()

	var out struct{}
	if err := s1.RPC("Status.Ping", struct{}{}, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestServer_RPC_TLS(t *testing.T) {
	t.Parallel()
	const (
		cafile  = "../helper/tlsutil/testdata/ca.pem"
		foocert = "../helper/tlsutil/testdata/nomad-foo.pem"
		fookey  = "../helper/tlsutil/testdata/nomad-foo-key.pem"
	)
	dir := tmpDir(t)
	defer os.RemoveAll(dir)
	s1 := TestServer(t, func(c *Config) {
		c.Region = "regionFoo"
		c.BootstrapExpect = 3
		c.DevMode = false
		c.DevDisableBootstrap = true
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
	defer s1.Shutdown()

	s2 := TestServer(t, func(c *Config) {
		c.Region = "regionFoo"
		c.BootstrapExpect = 3
		c.DevMode = false
		c.DevDisableBootstrap = true
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
	defer s2.Shutdown()
	s3 := TestServer(t, func(c *Config) {
		c.Region = "regionFoo"
		c.BootstrapExpect = 3
		c.DevMode = false
		c.DevDisableBootstrap = true
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
	defer s3.Shutdown()

	TestJoin(t, s1, s2, s3)
	testutil.WaitForLeader(t, s1.RPC)

	// Part of a server joining is making an RPC request, so just by testing
	// that there is a leader we verify that the RPCs are working over TLS.
}

func TestServer_RPC_MixedTLS(t *testing.T) {
	t.Parallel()
	const (
		cafile  = "../helper/tlsutil/testdata/ca.pem"
		foocert = "../helper/tlsutil/testdata/nomad-foo.pem"
		fookey  = "../helper/tlsutil/testdata/nomad-foo-key.pem"
	)
	dir := tmpDir(t)
	defer os.RemoveAll(dir)
	s1 := TestServer(t, func(c *Config) {
		c.Region = "regionFoo"
		c.BootstrapExpect = 3
		c.DevMode = false
		c.DevDisableBootstrap = true
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
	defer s1.Shutdown()

	s2 := TestServer(t, func(c *Config) {
		c.Region = "regionFoo"
		c.BootstrapExpect = 3
		c.DevMode = false
		c.DevDisableBootstrap = true
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
	defer s2.Shutdown()
	s3 := TestServer(t, func(c *Config) {
		c.Region = "regionFoo"
		c.BootstrapExpect = 3
		c.DevMode = false
		c.DevDisableBootstrap = true
		c.DataDir = path.Join(dir, "node3")
	})
	defer s3.Shutdown()

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
	t.Parallel()
	// Make the servers
	s1 := TestServer(t, func(c *Config) {
		c.Region = "region1"
	})
	defer s1.Shutdown()

	s2 := TestServer(t, func(c *Config) {
		c.Region = "region2"
	})
	defer s2.Shutdown()

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
	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
		c.Region = "region1"
	})
	defer s1.Shutdown()

	if s1.vault.Running() {
		t.Fatalf("Vault client should not be running")
	}

	tr := true
	config := s1.config
	config.VaultConfig.Enabled = &tr
	config.VaultConfig.Token = uuid.Generate()

	if err := s1.Reload(config); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	if !s1.vault.Running() {
		t.Fatalf("Vault client should be running")
	}
}

func connectionReset(msg string) bool {
	return strings.Contains(msg, "EOF") || strings.Contains(msg, "connection reset by peer")
}

// Tests that the server will successfully reload its network connections,
// upgrading from plaintext to TLS if the server's TLS configuration changes.
func TestServer_Reload_TLSConnections_PlaintextToTLS(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	const (
		cafile  = "../helper/tlsutil/testdata/ca.pem"
		foocert = "../helper/tlsutil/testdata/nomad-foo.pem"
		fookey  = "../helper/tlsutil/testdata/nomad-foo-key.pem"
	)
	dir := tmpDir(t)
	defer os.RemoveAll(dir)

	s1 := TestServer(t, func(c *Config) {
		c.DataDir = path.Join(dir, "nodeA")
	})
	defer s1.Shutdown()

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
	t.Parallel()
	assert := assert.New(t)

	const (
		cafile  = "../helper/tlsutil/testdata/ca.pem"
		foocert = "../helper/tlsutil/testdata/nomad-foo.pem"
		fookey  = "../helper/tlsutil/testdata/nomad-foo-key.pem"
	)

	dir := tmpDir(t)
	defer os.RemoveAll(dir)

	s1 := TestServer(t, func(c *Config) {
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
	defer s1.Shutdown()

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
	t.Parallel()
	assert := assert.New(t)

	const (
		cafile  = "../helper/tlsutil/testdata/ca.pem"
		foocert = "../helper/tlsutil/testdata/nomad-foo.pem"
		fookey  = "../helper/tlsutil/testdata/nomad-foo-key.pem"
	)

	dir := tmpDir(t)
	defer os.RemoveAll(dir)

	s1 := TestServer(t, func(c *Config) {
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
	defer s1.Shutdown()

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
	t.Parallel()
	assert := assert.New(t)

	const (
		cafile  = "../helper/tlsutil/testdata/ca.pem"
		foocert = "../helper/tlsutil/testdata/nomad-foo.pem"
		fookey  = "../helper/tlsutil/testdata/nomad-foo-key.pem"
	)

	dir := tmpDir(t)
	defer os.RemoveAll(dir)

	s1 := TestServer(t, func(c *Config) {
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
	defer s1.Shutdown()

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
	assert := assert.New(t)
	t.Parallel()
	const (
		cafile  = "../../helper/tlsutil/testdata/ca.pem"
		foocert = "../../helper/tlsutil/testdata/nomad-foo.pem"
		fookey  = "../../helper/tlsutil/testdata/nomad-foo-key.pem"
		barcert = "../dev/tls_cluster/certs/nomad.pem"
		barkey  = "../dev/tls_cluster/certs/nomad-key.pem"
	)
	dir := tmpDir(t)
	defer os.RemoveAll(dir)

	s1 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 2
		c.DevMode = false
		c.DevDisableBootstrap = true
		c.DataDir = path.Join(dir, "node1")
		c.NodeName = "node1"
		c.Region = "regionFoo"
	})
	defer s1.Shutdown()

	s2 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 2
		c.DevMode = false
		c.DevDisableBootstrap = true
		c.DataDir = path.Join(dir, "node2")
		c.NodeName = "node2"
		c.Region = "regionFoo"
	})
	defer s2.Shutdown()

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

func TestServer_InvalidSchedulers(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	config := DefaultConfig()
	config.DevMode = true
	config.LogOutput = testlog.NewWriter(t)
	config.SerfConfig.MemberlistConfig.BindAddr = "127.0.0.1"
	logger := log.New(config.LogOutput, "", log.LstdFlags)
	catalog := consul.NewMockCatalog(logger)

	// Set the config to not have the core scheduler
	config.EnabledSchedulers = []string{"batch"}
	_, err := NewServer(config, catalog, logger)
	require.NotNil(err)
	require.Contains(err.Error(), "scheduler not enabled")

	// Set the config to have an unknown scheduler
	config.EnabledSchedulers = []string{"batch", structs.JobTypeCore, "foo"}
	_, err = NewServer(config, catalog, logger)
	require.NotNil(err)
	require.Contains(err.Error(), "foo")
}
