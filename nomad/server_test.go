package nomad

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
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
