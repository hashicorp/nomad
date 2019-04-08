package testutils

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
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

	TestExecTaskStreamingBasicResponses(t, driver, taskID)
}

var ExecTaskStreamingBasicCases = []struct {
	Name     string
	Command  string
	Tty      bool
	Stdin    string
	Stdout   string
	Stderr   string
	ExitCode int
}{
	{
		Name:     "notty: basic",
		Command:  "echo hello stdout; echo hello stderr >&2; exit 43",
		Tty:      false,
		Stdout:   "hello stdout\n",
		Stderr:   "hello stderr\n",
		ExitCode: 43,
	},
	{
		Name:     "notty: streaming",
		Command:  "for n in 1 2 3; do echo $n; sleep 1; done",
		Tty:      false,
		Stdout:   "1\n2\n3\n",
		ExitCode: 0,
	},
	{
		Name:     "ntty: stty check",
		Command:  "stty size",
		Tty:      false,
		Stderr:   "stty: standard input: Inappropriate ioctl for device\n",
		ExitCode: 1,
	},
	{
		Name:     "notty: stdin passing",
		Command:  "echo hello from command; cat",
		Tty:      false,
		Stdin:    "hello from stdin\n",
		Stdout:   "hello from command\nhello from stdin\n",
		ExitCode: 0,
	},
	{
		Name:     "notty: stdin passing",
		Command:  "echo hello from command; cat",
		Tty:      false,
		Stdin:    "hello from stdin\n",
		Stdout:   "hello from command\nhello from stdin\n",
		ExitCode: 0,
	},
	{
		Name:    "notty: children processes",
		Command: "(( sleep 3; echo from background ) & ); echo from main; exec sleep 1",
		Tty:     false,
		// when not using tty; wait for all processes to exit matching behavior of `docker exec`
		Stdout:   "from main\nfrom background\n",
		ExitCode: 0,
	},
}

func TestExecTaskStreamingBasicResponses(t *testing.T, driver *DriverHarness, taskID string) {
	for _, c := range ExecTaskStreamingBasicCases {
		t.Run(c.Name, func(t *testing.T) {
			stdin, stdout, stderr, cleanupFn := NewIO(t, c.Tty)
			defer cleanupFn()

			stdin.WriteString(c.Stdin)

			resizeCh := make(chan drivers.TerminalSize)

			go func() {
				resizeCh <- drivers.TerminalSize{Height: 100, Width: 100}
			}()
			opts := drivers.ExecOptions{
				Command: []string{"/bin/sh", "-c", c.Command},
				Tty:     c.Tty,

				Stdin:  stdin,
				Stdout: stdout,
				Stderr: stderr,

				ResizeCh: resizeCh,
			}

			result, err := driver.ExecTaskStreaming(context.Background(), taskID, opts)
			require.NoError(t, err)
			require.Equal(t, c.ExitCode, result.ExitCode)

			// flush any pending writes
			stdout.Close()
			stderr.Close()

			stdoutFound, err := ioutil.ReadFile(stdout.Name())
			require.NoError(t, err)
			require.Equal(t, c.Stdout, string(stdoutFound))

			stderrFound, err := ioutil.ReadFile(stderr.Name())
			require.NoError(t, err)
			require.Equal(t, c.Stderr, string(stderrFound))
		})
	}
}

func NewIO(t *testing.T, tty bool) (stdin, stdout, stderr *os.File, cleanupFn func()) {
	tmpdir, err := ioutil.TempDir("", "nomad-exec-")
	cleanupFn = func() {
		os.RemoveAll(tmpdir)
	}

	stdin, err = os.Create(filepath.Join(tmpdir, "stdin"))
	require.NoError(t, err)

	stdout, err = os.Create(filepath.Join(tmpdir, "stdout"))
	require.NoError(t, err)

	stderr, err = os.Create(filepath.Join(tmpdir, "stderr"))
	require.NoError(t, err)

	return stdin, stdout, stderr, cleanupFn

}
