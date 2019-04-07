package testutils

import (
	"context"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/stretchr/testify/require"
)

func ExecTaskStreamingConformanceTests(t *testing.T, driver *DriverHarness, taskID string) {
	t.Helper()

	if runtime.GOOS == "windows" {
		// tests assume unix-ism now
		t.Skip("test assume unix tasks")
	}

	ExecTaskStreamingBasicResponses(t, driver, taskID)
}

func ExecTaskStreamingBasicResponses(t *testing.T, driver *DriverHarness, taskID string) {
	cases := []struct {
		name        string
		command     string
		tty         bool
		stdin       string
		stdout      string
		stderr      string
		exitCode    int
		customizeFn func(*drivers.ExecOptions, chan drivers.TerminalSize)
	}{
		{
			name:     "notty: basic",
			command:  "echo hello stdout; echo hello stderr >&2; exit 43",
			tty:      false,
			stdout:   "hello stdout\n",
			stderr:   "hello stderr\n",
			exitCode: 43,
		},
		{
			name:     "notty: streaming",
			command:  "for n in 1 2 3; do echo $n; sleep 1; done",
			tty:      false,
			stdout:   "1\n2\n3\n",
			exitCode: 0,
		},
		{
			name:     "ntty: stty check",
			command:  "stty size",
			tty:      false,
			stderr:   "stty: standard input: Inappropriate ioctl for device\n",
			exitCode: 1,
		},
		{
			name:     "notty: stdin passing",
			command:  "echo hello from command; cat",
			tty:      false,
			stdin:    "hello from stdin\n",
			stdout:   "hello from command\nhello from stdin\n",
			exitCode: 0,
		},
		{
			name:     "notty: stdin passing",
			command:  "echo hello from command; cat",
			tty:      false,
			stdin:    "hello from stdin\n",
			stdout:   "hello from command\nhello from stdin\n",
			exitCode: 0,
		},
		{
			name:    "notty: children processes",
			command: "(( sleep 3; echo from background ) & ); echo from main; exec sleep 1",
			tty:     false,
			// when not using tty; wait for all processes to exit matching behavior of `docker exec`
			stdout:   "from main\nfrom background\n",
			exitCode: 0,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stdout, err := ioutil.TempFile("", "nomad-exec-*")
			require.NoError(t, err)
			defer os.Remove(stdout.Name())

			stderr, err := ioutil.TempFile("", "nomad-exec-*")
			require.NoError(t, err)
			defer os.Remove(stderr.Name())

			stdinReader := ioutil.NopCloser(strings.NewReader(c.stdin))

			resizeCh := make(chan drivers.TerminalSize)

			go func() {
				resizeCh <- drivers.TerminalSize{Height: 100, Width: 100}
			}()
			opts := drivers.ExecOptions{
				Command: []string{"/bin/sh", "-c", c.command},
				Tty:     c.tty,

				Stdin:  stdinReader,
				Stdout: stdout,
				Stderr: stderr,

				ResizeCh: resizeCh,
			}

			if c.customizeFn != nil {
				go c.customizeFn(&opts, resizeCh)
			}

			result, err := driver.ExecTaskStreaming(context.Background(), taskID, opts)
			require.NoError(t, err)
			require.Equal(t, c.exitCode, result.ExitCode)

			// flush any pending writes
			stdout.Close()
			stderr.Close()

			stdoutFound, err := ioutil.ReadFile(stdout.Name())
			require.NoError(t, err)
			require.Equal(t, c.stdout, string(stdoutFound))

			stderrFound, err := ioutil.ReadFile(stderr.Name())
			require.NoError(t, err)
			require.Equal(t, c.stderr, string(stderrFound))
		})
	}
}
