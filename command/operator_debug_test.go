package command

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mitchellh/cli"
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

func TestDebugFails(t *testing.T) {
	t.Parallel()
	srv, _, _ := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
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
}

func TestDebugCapturedFiles(t *testing.T) {
	t.Parallel()
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
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

	// Version is always captured
	require.FileExists(t, filepath.Join(path, "version", "agent-self.json"))

	// Consul and Vault contain results or errors
	_, err := os.Stat(filepath.Join(path, "version", "consul-agent-self.json"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(path, "version", "vault-sys-health.json"))
	require.NoError(t, err)

	// Monitor files are only created when selected
	require.FileExists(t, filepath.Join(path, "server", "leader", "monitor.log"))
	require.FileExists(t, filepath.Join(path, "server", "leader", "profile.prof"))
	require.FileExists(t, filepath.Join(path, "server", "leader", "trace.prof"))
	require.FileExists(t, filepath.Join(path, "server", "leader", "goroutine.prof"))

	// Multiple snapshots are collected, 00 is always created
	require.FileExists(t, filepath.Join(path, "nomad", "0000", "jobs.json"))
	require.FileExists(t, filepath.Join(path, "nomad", "0000", "nodes.json"))

	// Multiple snapshots are collected, 01 requires two intervals
	require.FileExists(t, filepath.Join(path, "nomad", "0001", "jobs.json"))
	require.FileExists(t, filepath.Join(path, "nomad", "0001", "nodes.json"))
}
