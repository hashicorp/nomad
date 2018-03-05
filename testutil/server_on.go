//+build bin_test

package testutil

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"

	cleanhttp "github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/nomad/helper/discover"
	testing "github.com/mitchellh/go-testing-interface"
)

// NewTestServer creates a new TestServer, and makes a call to
// an optional callback function to modify the configuration.
func NewTestServer(t testing.T, cb ServerConfigCallback) *TestServer {
	path, err := discover.NomadExecutable()
	if err != nil {
		t.Skipf("nomad not found, skipping: %v", err)
	}

	// Do a sanity check that we are actually running nomad
	vcmd := exec.Command(path, "-version")
	vcmd.Stdout = nil
	vcmd.Stderr = nil
	if err := vcmd.Run(); err != nil {
		t.Skipf("nomad version failed. Did you run your test with -tags nomad_test (%v)", err)
	}

	dataDir, err := ioutil.TempDir("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	configFile, err := ioutil.TempFile(dataDir, "nomad")
	if err != nil {
		defer os.RemoveAll(dataDir)
		t.Fatalf("err: %s", err)
	}
	defer configFile.Close()

	nomadConfig := defaultServerConfig(t)
	nomadConfig.DataDir = dataDir

	if cb != nil {
		cb(nomadConfig)
	}

	configContent, err := json.Marshal(nomadConfig)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if _, err := configFile.Write(configContent); err != nil {
		t.Fatalf("err: %s", err)
	}
	configFile.Close()

	stdout := io.Writer(os.Stdout)
	if nomadConfig.Stdout != nil {
		stdout = nomadConfig.Stdout
	}

	stderr := io.Writer(os.Stderr)
	if nomadConfig.Stderr != nil {
		stderr = nomadConfig.Stderr
	}

	args := []string{"agent", "-config", configFile.Name()}
	if nomadConfig.DevMode {
		args = append(args, "-dev")
	}

	// Start the server
	cmd := exec.Command(path, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	client := cleanhttp.DefaultClient()

	server := &TestServer{
		Config: nomadConfig,
		cmd:    cmd,
		t:      t,

		HTTPAddr:   fmt.Sprintf("127.0.0.1:%d", nomadConfig.Ports.HTTP),
		SerfAddr:   fmt.Sprintf("127.0.0.1:%d", nomadConfig.Ports.Serf),
		HTTPClient: client,
	}

	// Wait for the server to be ready
	if nomadConfig.Server.Enabled && nomadConfig.Server.BootstrapExpect != 0 {
		server.waitForLeader()
	} else {
		server.waitForAPI()
	}

	// Wait for the client to be ready
	if nomadConfig.DevMode {
		server.waitForClient()
	}
	return server
}
