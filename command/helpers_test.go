package command

import (
	"io/ioutil"
	"strings"
	"testing"

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

func TestHelpers_LineLimitReader(t *testing.T) {
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
		limit := NewLineLimitReader(in, c.Lines, c.SearchLimit)
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
