package command

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mitchellh/cli"
)

func TestHelpers_FormatKV(t *testing.T) {
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
	in := []string{"alpha|beta||delta"}
	out := formatList(in)

	expect := "alpha  beta  <none>  delta"

	if out != expect {
		t.Fatalf("expect: %s, got: %s", expect, out)
	}
}

func TestHelpers_NodeID(t *testing.T) {
	srv, _, _ := testServer(t, nil)
	defer srv.Stop()

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
	// Create the test reader
	in := &testReadCloser{data: make(chan []byte)}

	// Set up the reader such that it won't hit the line/buffer limit and could
	// only terminate if it hits the time limit
	limit := NewLineLimitReader(in, 1000, 1000, 100*time.Millisecond)

	expected := []byte("hello world")

	resultCh := make(chan struct{})
	go func() {
		outBytes, err := ioutil.ReadAll(limit)
		if err != nil {
			t.Fatalf("ReadAll failed: %v", err)
		}

		if reflect.DeepEqual(outBytes, expected) {
			close(resultCh)
			return
		}
	}()

	// Send the data
	in.data <- expected
	in.Close()

	select {
	case <-resultCh:
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
                }
        }
}`
)

// Test StructJob with local jobfile
func TestStructJobWithLocal(t *testing.T) {
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
	sj, err := j.StructJob(fh.Name())
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	err = sj.Validate()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
}

// Test StructJob with jobfile from HTTP Server
func TestStructJobWithHTTPServer(t *testing.T) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, job)
	})
	go http.ListenAndServe("127.0.0.1:12345", nil)

	// Wait until HTTP Server starts certainly
	time.Sleep(100 * time.Millisecond)

	j := &JobGetter{}
	sj, err := j.StructJob("http://127.0.0.1:12345/")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	err = sj.Validate()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
}
