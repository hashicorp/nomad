package command

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDebugUtils(t *testing.T) {
	xs := argNodes("foo, bar")
	require.Equal(t, []string{"foo", "bar"}, xs)

	xs = argNodes("")
	require.Len(t, xs, 0)
	require.Empty(t, xs)

	// address calculation honors CONSUL_HTTP_SSL
	e := &external{addrVal: "http://127.0.0.1:8500", ssl: true}
	require.Equal(t, "https://127.0.0.1:8500", e.addr("foo"))

	e = &external{addrVal: "http://127.0.0.1:8500", ssl: false}
	require.Equal(t, "http://127.0.0.1:8500", e.addr("foo"))

	e = &external{addrVal: "127.0.0.1:8500", ssl: false}
	require.Equal(t, "http://127.0.0.1:8500", e.addr("foo"))

	e = &external{addrVal: "127.0.0.1:8500", ssl: true}
	require.Equal(t, "https://127.0.0.1:8500", e.addr("foo"))
}

func TestDebugSuccesses(t *testing.T) {
	t.Parallel()
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &OperatorDebugCommand{Meta: Meta{Ui: ui}}

	// NOTE -- duration must be shorter than default 2m to prevent testify from timing out

	// Debug on the leader
	code := cmd.Run([]string{"-address", url, "-duration", "250ms", "-server-id", "leader"})
	assert.Equal(t, 0, code) // take note of failed return code, but continue to see why
	require.Contains(t, ui.OutputWriter.String(), "Starting debugger")
	ui.OutputWriter.Reset()

	// Debug on all servers
	code = cmd.Run([]string{"-address", url, "-duration", "250ms", "-server-id", "all"})
	assert.Equal(t, 0, code)
	require.Contains(t, ui.OutputWriter.String(), "Starting debugger")
	ui.OutputWriter.Reset()
}

func TestDebugFails(t *testing.T) {
	t.Parallel()
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &OperatorDebugCommand{Meta: Meta{Ui: ui}}

	// Fails incorrect args
	code := cmd.Run([]string{"some", "bad", "args"})
	require.Equal(t, 1, code)

	// Fails illegal node ids
	code = cmd.Run([]string{"-node-id", "foo:bar"})
	require.Equal(t, 1, code)

	// Fails missing node ids
	code = cmd.Run([]string{"-node-id", "abc,def"})
	require.Equal(t, 1, code)

	// Fails bad durations
	code = cmd.Run([]string{"-duration", "foo"})
	require.Equal(t, 1, code)

	// Fails bad durations
	code = cmd.Run([]string{"-interval", "bar"})
	require.Equal(t, 1, code)

	// Fails existing output
	format := "2006-01-02-150405Z"
	stamped := "nomad-debug-" + time.Now().UTC().Format(format)
	path := filepath.Join(os.TempDir(), stamped)
	os.MkdirAll(path, 0755)
	defer os.Remove(path)
	code = cmd.Run([]string{"-output", os.TempDir()})
	require.Equal(t, 2, code)

	// Fails bad address
	code = cmd.Run([]string{"-address", url + "bogus"})
	assert.Equal(t, 1, code)
	ui.OutputWriter.Reset()
}

func TestDebugCapturedFiles(t *testing.T) {
	t.Parallel()
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &OperatorDebugCommand{Meta: Meta{Ui: ui}}

	code := cmd.Run([]string{
		"-address", url,
		"-output", os.TempDir(),
		"-server-id", "leader",
		"-duration", "1300ms",
		"-interval", "600ms",
	})

	path := cmd.collectDir
	defer os.Remove(path)

	require.Empty(t, ui.ErrorWriter.String())
	require.Equal(t, 0, code)
	ui.ErrorWriter.Reset()

	serverFiles := []string{
		// Version is always captured
		filepath.Join(path, "version", "agent-self.json"),

		// Consul and Vault contain results or errors
		filepath.Join(path, "version", "consul-agent-self.json"),
		filepath.Join(path, "version", "vault-sys-health.json"),

		// Monitor files are only created when selected
		filepath.Join(path, "server", "leader", "monitor.log"),
		filepath.Join(path, "server", "leader", "profile.prof"),
		filepath.Join(path, "server", "leader", "trace.prof"),
		filepath.Join(path, "server", "leader", "goroutine.prof"),
		filepath.Join(path, "server", "leader", "goroutine-debug1.txt"),
		filepath.Join(path, "server", "leader", "goroutine-debug2.txt"),

		// Multiple snapshots are collected, 00 is always created
		filepath.Join(path, "nomad", "0000", "jobs.json"),
		filepath.Join(path, "nomad", "0000", "nodes.json"),
		filepath.Join(path, "nomad", "0000", "metrics.json"),

		// Multiple snapshots are collected, 01 requires two intervals
		filepath.Join(path, "nomad", "0001", "jobs.json"),
		filepath.Join(path, "nomad", "0001", "nodes.json"),
		filepath.Join(path, "nomad", "0001", "metrics.json"),
	}

	testutil.WaitForFiles(t, serverFiles)
}
