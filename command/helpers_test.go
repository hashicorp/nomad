package command

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/flatmap"
	"github.com/kr/pretty"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestHelpers_FormatKV(t *testing.T) {
	t.Parallel()
	in := []string{"alpha|beta", "charlie|delta", "echo|"}
	out := formatKV(in)

	expect := "alpha   = beta\n"
	expect += "charlie = delta\n"
	expect += "echo    = <none>"

	if out != expect {
		t.Fatalf("expect: %s, got: %s", expect, out)
	}
}

func TestHelpers_FormatList(t *testing.T) {
	t.Parallel()
	in := []string{"alpha|beta||delta"}
	out := formatList(in)

	expect := "alpha  beta  <none>  delta"

	if out != expect {
		t.Fatalf("expect: %s, got: %s", expect, out)
	}
}

func TestHelpers_NodeID(t *testing.T) {
	t.Parallel()
	srv, _, _ := testServer(t, false, nil)
	defer srv.Shutdown()

	meta := Meta{Ui: new(cli.MockUi)}
	client, err := meta.Client()
	if err != nil {
		t.FailNow()
	}

	// This is because there is no client
	if _, err := getLocalNodeID(client); err == nil {
		t.Fatalf("getLocalNodeID() should fail")
	}
}

func TestHelpers_LineLimitReader_NoTimeLimit(t *testing.T) {
	t.Parallel()
	helloString := `hello
world
this
is
a
test`

	noLines := "jskdfhjasdhfjkajkldsfdlsjkahfkjdsafa"

	cases := []struct {
		Input       string
		Output      string
		Lines       int
		SearchLimit int
	}{
		{
			Input:       helloString,
			Output:      helloString,
			Lines:       6,
			SearchLimit: 1000,
		},
		{
			Input: helloString,
			Output: `world
this
is
a
test`,
			Lines:       5,
			SearchLimit: 1000,
		},
		{
			Input:       helloString,
			Output:      `test`,
			Lines:       1,
			SearchLimit: 1000,
		},
		{
			Input:       helloString,
			Output:      "",
			Lines:       0,
			SearchLimit: 1000,
		},
		{
			Input:       helloString,
			Output:      helloString,
			Lines:       6,
			SearchLimit: 1, // Exceed the limit
		},
		{
			Input:       noLines,
			Output:      noLines,
			Lines:       10,
			SearchLimit: 1000,
		},
		{
			Input:       noLines,
			Output:      noLines,
			Lines:       10,
			SearchLimit: 2,
		},
	}

	for i, c := range cases {
		in := ioutil.NopCloser(strings.NewReader(c.Input))
		limit := NewLineLimitReader(in, c.Lines, c.SearchLimit, 0)
		outBytes, err := ioutil.ReadAll(limit)
		if err != nil {
			t.Fatalf("case %d failed: %v", i, err)
		}

		out := string(outBytes)
		if out != c.Output {
			t.Fatalf("case %d: got %q; want %q", i, out, c.Output)
		}
	}
}

type testReadCloser struct {
	data chan []byte
}

func (t *testReadCloser) Read(p []byte) (n int, err error) {
	select {
	case b, ok := <-t.data:
		if !ok {
			return 0, io.EOF
		}

		return copy(p, b), nil
	case <-time.After(10 * time.Millisecond):
		return 0, nil
	}
}

func (t *testReadCloser) Close() error {
	close(t.data)
	return nil
}

func TestHelpers_LineLimitReader_TimeLimit(t *testing.T) {
	t.Parallel()
	// Create the test reader
	in := &testReadCloser{data: make(chan []byte)}

	// Set up the reader such that it won't hit the line/buffer limit and could
	// only terminate if it hits the time limit
	limit := NewLineLimitReader(in, 1000, 1000, 100*time.Millisecond)

	expected := []byte("hello world")

	errCh := make(chan error)
	resultCh := make(chan []byte)
	go func() {
		defer close(resultCh)
		defer close(errCh)
		outBytes, err := ioutil.ReadAll(limit)
		if err != nil {
			errCh <- fmt.Errorf("ReadAll failed: %v", err)
			return
		}
		resultCh <- outBytes
	}()

	// Send the data
	in.data <- expected
	in.Close()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
	case outBytes := <-resultCh:
		if !reflect.DeepEqual(outBytes, expected) {
			t.Fatalf("got:%s, expected,%s", string(outBytes), string(expected))
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("did not exit by time limit")
	}
}

const (
	job = `job "job1" {
        type = "service"
        datacenters = [ "dc1" ]
        group "group1" {
                count = 1
                task "task1" {
                        driver = "exec"
                        resources = {}
                }
                restart{
                        attempts = 10
                        mode = "delay"
						interval = "15s"
                }
        }
}`
)

var (
	expectedApiJob = &api.Job{
		ID:          helper.StringToPtr("job1"),
		Name:        helper.StringToPtr("job1"),
		Type:        helper.StringToPtr("service"),
		Datacenters: []string{"dc1"},
		TaskGroups: []*api.TaskGroup{
			{
				Name:  helper.StringToPtr("group1"),
				Count: helper.IntToPtr(1),
				RestartPolicy: &api.RestartPolicy{
					Attempts: helper.IntToPtr(10),
					Interval: helper.TimeToPtr(15 * time.Second),
					Mode:     helper.StringToPtr("delay"),
				},

				Tasks: []*api.Task{
					{
						Driver:    "exec",
						Name:      "task1",
						Resources: &api.Resources{},
					},
				},
			},
		},
	}
)

// Test APIJob with local jobfile
func TestJobGetter_LocalFile(t *testing.T) {
	t.Parallel()
	fh, err := ioutil.TempFile("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(fh.Name())
	_, err = fh.WriteString(job)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	j := &JobGetter{}
	aj, err := j.ApiJob(fh.Name())
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if !reflect.DeepEqual(expectedApiJob, aj) {
		eflat := flatmap.Flatten(expectedApiJob, nil, false)
		aflat := flatmap.Flatten(aj, nil, false)
		t.Fatalf("got:\n%v\nwant:\n%v", aflat, eflat)
	}
}

// Test StructJob with jobfile from HTTP Server
func TestJobGetter_HTTPServer(t *testing.T) {
	t.Parallel()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, job)
	})
	go http.ListenAndServe("127.0.0.1:12345", nil)

	// Wait until HTTP Server starts certainly
	time.Sleep(100 * time.Millisecond)

	j := &JobGetter{}
	aj, err := j.ApiJob("http://127.0.0.1:12345/")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if !reflect.DeepEqual(expectedApiJob, aj) {
		for _, d := range pretty.Diff(expectedApiJob, aj) {
			t.Logf(d)
		}
		t.Fatalf("Unexpected file")
	}
}

func TestPrettyTimeDiff(t *testing.T) {
	// Grab the time and truncate to the nearest second. This allows our tests
	// to be deterministic since we don't have to worry about rounding.
	now := time.Now().Truncate(time.Second)

	test_cases := []struct {
		t1  time.Time
		t2  time.Time
		exp string
	}{
		{now, time.Unix(0, 0), ""}, // This is the upgrade path case
		{now, now.Add(-10 * time.Millisecond), "0s ago"},
		{now, now.Add(-740 * time.Second), "12m20s ago"},
		{now, now.Add(-12 * time.Minute), "12m ago"},
		{now, now.Add(-60 * time.Minute), "1h ago"},
		{now, now.Add(-80 * time.Minute), "1h20m ago"},
		{now, now.Add(-6 * time.Hour), "6h ago"},
		{now.Add(-6 * time.Hour), now, "6h from now"},
		{now, now.Add(-22165 * time.Second), "6h9m ago"},
		{now, now.Add(-100 * time.Hour), "4d4h ago"},
		{now, now.Add(-438000 * time.Minute), "10mo4d ago"},
		{now, now.Add(-20460 * time.Hour), "2y4mo ago"},
	}
	for _, tc := range test_cases {
		t.Run(tc.exp, func(t *testing.T) {
			out := prettyTimeDiff(tc.t2, tc.t1)
			if out != tc.exp {
				t.Fatalf("expected :%v but got :%v", tc.exp, out)
			}
		})
	}

	var t1 time.Time
	out := prettyTimeDiff(t1, time.Now())

	if out != "" {
		t.Fatalf("Expected empty output but got:%v", out)
	}

}

// TestUiErrorWriter asserts that writer buffers and
func TestUiErrorWriter(t *testing.T) {
	t.Parallel()

	var outBuf, errBuf bytes.Buffer
	ui := &cli.BasicUi{
		Writer:      &outBuf,
		ErrorWriter: &errBuf,
	}

	w := &uiErrorWriter{ui: ui}

	inputs := []string{
		"some line\n",
		"multiple\nlines\r\nhere",
		" with  followup\nand",
		" more lines ",
		" without new line ",
		"until here\nand then",
		"some more",
	}

	partialAcc := ""
	for _, in := range inputs {
		n, err := w.Write([]byte(in))
		require.NoError(t, err)
		require.Equal(t, len(in), n)

		// assert that writer emits partial result until last new line
		partialAcc += strings.ReplaceAll(in, "\r\n", "\n")
		lastNL := strings.LastIndex(partialAcc, "\n")
		require.Equal(t, partialAcc[:lastNL+1], errBuf.String())
	}

	require.Empty(t, outBuf.String())

	// note that the \r\n got replaced by \n
	expectedErr := "some line\nmultiple\nlines\nhere with  followup\nand more lines  without new line until here\n"
	require.Equal(t, expectedErr, errBuf.String())

	// close emits the final line
	err := w.Close()
	require.NoError(t, err)

	expectedErr += "and thensome more\n"
	require.Equal(t, expectedErr, errBuf.String())
}
