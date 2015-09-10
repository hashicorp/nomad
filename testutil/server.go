package testutil

// TestServer is a test helper. It uses a fork/exec model to create
// a test Nomad server instance in the background and initialize it
// with some data and/or services. The test server can then be used
// to run a unit test, and offers an easy API to tear itself down
// when the test has completed. The only prerequisite is to have a nomad
// binary available on the $PATH.
//
// This package does not use Nomad's official API client. This is
// because we use TestServer to test the API client, which would
// otherwise cause an import cycle.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"sync/atomic"
	"testing"
)

// offset is used to atomically increment the port numbers.
var offset uint64

// TestServerConfig is the main server configuration struct.
type TestServerConfig struct {
	HTTPAddr          string    `json:"http_addr,omitempty"`
	Bootstrap         bool      `json:"bootstrap,omitempty"`
	DataDir           string    `json:"data_dir,omitempty"`
	Region            string    `json:"region,omitempty"`
	DisableCheckpoint bool      `json:"disable_update_check"`
	LogLevel          string    `json:"log_level,omitempty"`
	Stdout, Stderr    io.Writer `json:"-"`
}

// ServerConfigCallback is a function interface which can be
// passed to NewTestServerConfig to modify the server config.
type ServerConfigCallback func(c *TestServerConfig)

// defaultServerConfig returns a new TestServerConfig struct
// with all of the listen ports incremented by one.
func defaultServerConfig() *TestServerConfig {
	idx := int(atomic.AddUint64(&offset, 1))

	return &TestServerConfig{
		DisableCheckpoint: true,
		Bootstrap:         true,
		LogLevel:          "DEBUG",
		HTTPAddr:          fmt.Sprintf("127.0.0.1:%d", 20000+idx),
	}
}

// TestServer is the main server wrapper struct.
type TestServer struct {
	PID    int
	Config *TestServerConfig
	t      *testing.T

	HTTPAddr   string
	HttpClient *http.Client
}

// NewTestServerConfig creates a new TestServer, and makes a call to
// an optional callback function to modify the configuration.
func NewTestServer(t *testing.T, cb ServerConfigCallback) *TestServer {
	if path, err := exec.LookPath("nomad"); err != nil || path == "" {
		t.Skip("nomad not found on $PATH, skipping")
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

	nomadConfig := defaultServerConfig()
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

	// Start the server
	cmd := exec.Command("nomad", "agent", "-dev", "-config", configFile.Name())
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	var client *http.Client
	client = http.DefaultClient

	server := &TestServer{
		Config: nomadConfig,
		PID:    cmd.Process.Pid,
		t:      t,

		HTTPAddr:   nomadConfig.HTTPAddr,
		HttpClient: client,
	}

	// Wait for the server to be ready
	if nomadConfig.Bootstrap {
		server.waitForLeader()
	} else {
		server.waitForAPI()
	}
	return server
}

// Stop stops the test Nomad server, and removes the Nomad data
// directory once we are done.
func (s *TestServer) Stop() {
	defer os.RemoveAll(s.Config.DataDir)

	cmd := exec.Command("kill", "-9", fmt.Sprintf("%d", s.PID))
	if err := cmd.Run(); err != nil {
		s.t.Errorf("err: %s", err)
	}
}

// waitForAPI waits for only the agent HTTP endpoint to start
// responding. This is an indication that the agent has started,
// but will likely return before a leader is elected.
func (s *TestServer) waitForAPI() {
	WaitForResult(func() (bool, error) {
		resp, err := s.HttpClient.Get(s.url("/v1/jobs?stale"))
		if err != nil {
			return false, err
		}
		defer resp.Body.Close()
		if err := s.requireOK(resp); err != nil {
			return false, err
		}
		return true, nil
	}, func(err error) {
		defer s.Stop()
		s.t.Fatalf("err: %s", err)
	})
}

// waitForLeader waits for the Nomad server's HTTP API to become
// available, and then waits for a known leader and an index of
// 1 or more to be observed to confirm leader election is done.
func (s *TestServer) waitForLeader() {
	WaitForResult(func() (bool, error) {
		// Query the API and check the status code
		resp, err := s.HttpClient.Get(s.url("/v1/jobs"))
		if err != nil {
			return false, err
		}
		defer resp.Body.Close()
		if err := s.requireOK(resp); err != nil {
			return false, err
		}

		// Ensure we have a leader and a node registeration
		if leader := resp.Header.Get("X-Nomad-KnownLeader"); leader != "true" {
			fmt.Println(leader)
			return false, fmt.Errorf("Nomad leader status: %#v", leader)
		}
		return true, nil
	}, func(err error) {
		defer s.Stop()
		s.t.Fatalf("err: %s", err)
	})
}

// url is a helper function which takes a relative URL and
// makes it into a proper URL against the local Nomad server.
func (s *TestServer) url(path string) string {
	return fmt.Sprintf("http://%s%s", s.HTTPAddr, path)
}

// requireOK checks the HTTP response code and ensures it is acceptable.
func (s *TestServer) requireOK(resp *http.Response) error {
	if resp.StatusCode != 200 {
		return fmt.Errorf("Bad status code: %d", resp.StatusCode)
	}
	return nil
}

// put performs a new HTTP PUT request.
func (s *TestServer) put(path string, body io.Reader) *http.Response {
	req, err := http.NewRequest("PUT", s.url(path), body)
	if err != nil {
		s.t.Fatalf("err: %s", err)
	}
	resp, err := s.HttpClient.Do(req)
	if err != nil {
		s.t.Fatalf("err: %s", err)
	}
	if err := s.requireOK(resp); err != nil {
		defer resp.Body.Close()
		s.t.Fatal(err)
	}
	return resp
}

// get performs a new HTTP GET request.
func (s *TestServer) get(path string) *http.Response {
	resp, err := s.HttpClient.Get(s.url(path))
	if err != nil {
		s.t.Fatalf("err: %s", err)
	}
	if err := s.requireOK(resp); err != nil {
		defer resp.Body.Close()
		s.t.Fatal(err)
	}
	return resp
}

// encodePayload returns a new io.Reader wrapping the encoded contents
// of the payload, suitable for passing directly to a new request.
func (s *TestServer) encodePayload(payload interface{}) io.Reader {
	var encoded bytes.Buffer
	enc := json.NewEncoder(&encoded)
	if err := enc.Encode(payload); err != nil {
		s.t.Fatalf("err: %s", err)
	}
	return &encoded
}
