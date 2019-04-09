package testutils

import (
	"context"
	"io"
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
		Command:  "echo hello from command; head -n1",
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

	// TTY cases - difference is new lines add `\r` and child process waiting is different
	{
		Name:     "tty: basic",
		Command:  "echo hello stdout; echo hello stderr >&2; exit 43",
		Tty:      true,
		Stdout:   "hello stdout\r\nhello stderr\r\n",
		ExitCode: 43,
	},
	{
		Name:     "tty: streaming",
		Command:  "for n in 1 2 3; do echo $n; sleep 1; done",
		Tty:      true,
		Stdout:   "1\r\n2\r\n3\r\n",
		ExitCode: 0,
	},
	{
		Name:     "tty: stty check",
		Command:  "sleep 1; stty size",
		Tty:      true,
		Stdout:   "100 100\r\n",
		ExitCode: 0,
	},
	{
		Name:     "tty: stdin passing",
		Command:  "head -n1",
		Tty:      true,
		Stdin:    "hello from stdin\n",
		Stdout:   "hello from stdin\r\nhello from stdin\r\n",
		ExitCode: 0,
	},
	{
		Name:    "tty: children processes",
		Command: "(( sleep 3; echo from background ) & ); echo from main; exec sleep 1",
		Tty:     true,
		// when using tty; wait for lead process only, like `docker exec -it`
		Stdout:   "from main\r\n",
		ExitCode: 0,
	},
}

func TestExecTaskStreamingBasicResponses(t *testing.T, driver *DriverHarness, taskID string) {
	for _, c := range ExecTaskStreamingBasicCases {
		t.Run(c.Name, func(t *testing.T) {
			stdin, stdout, stderr, readOutput, cleanupFn := NewIO(t, c.Tty, c.Stdin)
			defer cleanupFn()

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
			stdin.Close()
			stdout.Close()
			stderr.Close()

			stdoutFound, stderrFound := readOutput()
			require.Equal(t, c.Stdout, stdoutFound)
			require.Equal(t, c.Stderr, stderrFound)
		})
	}
}

func NewIO(t *testing.T, tty bool, stdinInput string) (stdin io.ReadCloser, stdout, stderr io.WriteCloser,
	read func() (stdout, stderr string), cleanup func()) {

	stdin, pw := io.Pipe()
	go func() {
		pw.Write([]byte(stdinInput))

		// don't close stdin in tty, because closing early may cause
		// tty to be closed and we would lose output
		if !tty {
			pw.Close()
		}
	}()

	tmpdir, err := ioutil.TempDir("", "nomad-exec-")
	require.NoError(t, err)
	cleanupFn := func() {
		os.RemoveAll(tmpdir)
	}

	stdoutF, err := os.Create(filepath.Join(tmpdir, "stdout"))
	require.NoError(t, err)

	stderrF, err := os.Create(filepath.Join(tmpdir, "stderr"))
	require.NoError(t, err)

	readFn := func() (string, string) {
		stdoutContent, err := ioutil.ReadFile(stdoutF.Name())
		require.NoError(t, err)

		stderrContent, err := ioutil.ReadFile(stderrF.Name())
		require.NoError(t, err)

		return string(stdoutContent), string(stderrContent)
	}

	return stdin, stdoutF, stderrF, readFn, cleanupFn
}
