package command

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestVarListCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &VarListCommand{}
}

// TestVarListCommand_Offline contains all of the tests that do not require a
// testagent to complete
func TestVarListCommand_Offline(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &VarListCommand{Meta: Meta{Ui: ui}}

	testCases := []testVarListTestCase{
		{
			name:        "help",
			args:        []string{"-help"},
			exitCode:    1,
			expectUsage: true,
		},
		{
			name:               "bad args",
			args:               []string{"some", "bad", "args"},
			exitCode:           1,
			expectUsageError:   true,
			expectStdErrPrefix: "This command takes flags and either no arguments or one: <prefix>",
		},
		{
			name:               "bad address",
			args:               []string{"-address", "nope"},
			exitCode:           1,
			expectStdErrPrefix: "Error retrieving vars",
		},
		{
			name:               "unparsable address",
			args:               []string{"-address", "http://10.0.0.1:bad"},
			exitCode:           1,
			expectStdErrPrefix: "Error initializing client: invalid address",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			tC := tC
			ec := cmd.Run(tC.args)
			stdOut := ui.OutputWriter.String()
			errOut := ui.ErrorWriter.String()
			defer resetUiWriters(ui)

			require.Equal(t, tC.exitCode, ec,
				"Expected exit code %v; got: %v\nstdout: %s\nstderr: %s",
				tC.exitCode, ec, stdOut, errOut,
			)
			if tC.expectUsage {
				help := cmd.Help()
				require.Equal(t, help, strings.TrimSpace(stdOut))
				// Test that stdout ends with a linefeed since we trim them for
				// convenience in the equality tests.
				require.True(t, strings.HasSuffix(stdOut, "\n"),
					"stdout does not end with a linefeed")
			}
			if tC.expectUsageError {
				require.Contains(t, errOut, commandErrorText(cmd))
			}
			if tC.expectStdOut != "" {
				require.Equal(t, tC.expectStdOut, strings.TrimSpace(stdOut))
				// Test that stdout ends with a linefeed since we trim them for
				// convenience in the equality tests.
				require.True(t, strings.HasSuffix(stdOut, "\n"),
					"stdout does not end with a linefeed")
			}
			if tC.expectStdErrPrefix != "" {
				require.True(t, strings.HasPrefix(errOut, tC.expectStdErrPrefix),
					"Expected stderr to start with %q; got %s",
					tC.expectStdErrPrefix, errOut)
				// Test that stderr ends with a linefeed since we trim them for
				// convenience in the equality tests.
				require.True(t, strings.HasSuffix(errOut, "\n"),
					"stderr does not end with a linefeed")
			}
		})
	}
}

// TestVarListCommand_Online contains all of the tests that use a testagent.
// They reuse the same testagent so that they can run in parallel and minimize
// test startup time costs.
func TestVarListCommand_Online(t *testing.T) {
	ci.Parallel(t)

	// Create a server
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &VarListCommand{Meta: Meta{Ui: ui}}

	nsList := []string{api.DefaultNamespace, "ns1"}
	pathList := []string{"a/b/c", "a/b/c/d", "z/y", "z/y/x"}
	toJSON := func(in interface{}) string {
		b, err := json.MarshalIndent(in, "", "    ")
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(b))
	}
	variables := setupTestVariables(client, nsList, pathList)

	testTmpl := `{{ range $i, $e := . }}{{if ne $i 0}}{{print "•"}}{{end}}{{printf "%v\t%v" .Namespace .Path}}{{end}}`

	pathsEqual := func(t *testing.T, expect any) testVarListJSONTestExpectFn {
		out := func(t *testing.T, check any) {

			expect := expect
			exp, ok := expect.(NSPather)
			require.True(t, ok, "expect is not an NSPather, got %T", expect)
			in, ok := check.(NSPather)
			require.True(t, ok, "check is not an NSPather, got %T", check)
			require.ElementsMatch(t, exp.NSPaths(), in.NSPaths())
		}
		return out
	}

	hasLength := func(t *testing.T, length int) testVarListJSONTestExpectFn {
		out := func(t *testing.T, check any) {

			length := length
			in, ok := check.(NSPather)
			require.True(t, ok, "check is not an NSPather, got %T", check)
			inLen := in.NSPaths().Len()
			require.Equal(t, length, inLen,
				"expected length of %v, got %v. \nvalues: %v",
				length, inLen, in.NSPaths())
		}
		return out
	}

	testCases := []testVarListTestCase{
		{
			name:         "plaintext/not found",
			args:         []string{"does/not/exist"},
			expectStdOut: msgSecureVariableNotFound,
		},
		{
			name: "plaintext/single variable",
			args: []string{"a/b/c/d"},
			expectStdOut: formatList([]string{
				"Namespace|Path|Last Updated",
				fmt.Sprintf(
					"default|a/b/c/d|%s",
					time.Unix(0, variables.HavingPrefix("a/b/c/d")[0].ModifyTime),
				),
			},
			),
		},
		{
			name:         "plaintext/quiet",
			args:         []string{"-q"},
			expectStdOut: strings.Join(variables.HavingNamespace(api.DefaultNamespace).Strings(), "\n"),
		},
		{
			name:         "plaintext/quiet/prefix",
			args:         []string{"-q", "a/b/c"},
			expectStdOut: strings.Join(variables.HavingNSPrefix(api.DefaultNamespace, "a/b/c").Strings(), "\n"),
		},
		{
			name:               "plaintext/quiet/filter",
			args:               []string{"-q", "-filter", "SecureVariableMetadata.Path == \"a/b/c\""},
			expectStdOut:       "a/b/c",
			expectStdErrPrefix: msgWarnFilterPerformance,
		},
		{
			name:               "plaintext/quiet/paginated",
			args:               []string{"-q", "-per-page=1"},
			expectStdOut:       "a/b/c",
			expectStdErrPrefix: "Next page token",
		},
		{
			name:         "plaintext/quiet/prefix/wildcard ns",
			args:         []string{"-q", "-namespace", "*", "a/b/c/d"},
			expectStdOut: strings.Join(variables.HavingPrefix("a/b/c/d").Strings(), "\n"),
		},
		{
			name:               "plaintext/quiet/paginated/prefix/wildcard ns",
			args:               []string{"-q", "-per-page=1", "-namespace", "*", "a/b/c/d"},
			expectStdOut:       variables.HavingPrefix("a/b/c/d").Strings()[0],
			expectStdErrPrefix: "Next page token",
		},
		{
			name: "json/not found",
			args: []string{"-json", "does/not/exist"},
			jsonTest: &testVarListJSONTest{
				jsonDest: &SVMSlice{},
				expectFns: []testVarListJSONTestExpectFn{
					hasLength(t, 0),
				},
			},
		},
		{
			name: "json/prefix",
			args: []string{"-json", "a"},
			jsonTest: &testVarListJSONTest{
				jsonDest: &SVMSlice{},
				expectFns: []testVarListJSONTestExpectFn{
					pathsEqual(t, variables.HavingNSPrefix(api.DefaultNamespace, "a")),
				},
			},
		},
		{
			name: "json/paginated",
			args: []string{"-json", "-per-page", "1"},
			jsonTest: &testVarListJSONTest{
				jsonDest: &PaginatedSVMSlice{},
				expectFns: []testVarListJSONTestExpectFn{
					hasLength(t, 1),
				},
			},
		},
		{
			name:         "json/quiet",
			args:         []string{"-q", "-json"},
			expectStdOut: toJSON(variables.HavingNamespace(api.DefaultNamespace).Strings()),
		},
		{
			name: "json/quiet/paginated",
			args: []string{"-q", "-json", "-per-page", "1"},
			jsonTest: &testVarListJSONTest{
				jsonDest: &PaginatedSVQuietSlice{},
				expectFns: []testVarListJSONTestExpectFn{
					hasLength(t, 1),
				},
			},
		},
		{
			name: "json/quiet/wildcard-ns",
			args: []string{"-q", "-json", "-namespace", "*"},
			jsonTest: &testVarListJSONTest{
				jsonDest: &SVMSlice{},
				expectFns: []testVarListJSONTestExpectFn{
					hasLength(t, variables.Len()),
					pathsEqual(t, variables),
				},
			},
		},
		{
			name: "json/quiet/paginated/wildcard-ns",
			args: []string{"-q", "-json", "-per-page=1", "-namespace", "*"},
			jsonTest: &testVarListJSONTest{
				jsonDest: &PaginatedSVMSlice{},
				expectFns: []testVarListJSONTestExpectFn{
					hasLength(t, 1),
					pathsEqual(t, SVMSlice{variables[0]}),
				},
			},
		},
		{
			name:         "template/not found",
			args:         []string{"-t", testTmpl, "does/not/exist"},
			expectStdOut: "",
		},
		{
			name:         "template/prefix",
			args:         []string{"-t", testTmpl, "a/b/c/d"},
			expectStdOut: "default\ta/b/c/d",
		},
		{
			name:               "template/filter",
			args:               []string{"-t", testTmpl, "-filter", "SecureVariableMetadata.Path == \"a/b/c\""},
			expectStdOut:       "default\ta/b/c",
			expectStdErrPrefix: msgWarnFilterPerformance,
		},
		{
			name:               "template/paginated",
			args:               []string{"-t", testTmpl, "-per-page=1"},
			expectStdOut:       "default\ta/b/c",
			expectStdErrPrefix: "Next page token",
		},
		{
			name:         "template/prefix/wildcard namespace",
			args:         []string{"-namespace", "*", "-t", testTmpl, "a/b/c/d"},
			expectStdOut: "default\ta/b/c/d•ns1\ta/b/c/d",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			tC := tC
			// address always needs to be provided and since the test cases
			// might pass a positional parameter, we need to jam it in the
			// front.
			tcArgs := append([]string{"-address=" + url}, tC.args...)

			code := cmd.Run(tcArgs)
			stdOut := ui.OutputWriter.String()
			errOut := ui.ErrorWriter.String()
			defer resetUiWriters(ui)

			require.Equal(t, tC.exitCode, code,
				"Expected exit code %v; got: %v\nstdout: %s\nstderr: %s",
				tC.exitCode, code, stdOut, errOut)

			if tC.expectStdOut != "" {
				require.Equal(t, tC.expectStdOut, strings.TrimSpace(stdOut))

				// Test that stdout ends with a linefeed since we trim them for
				// convenience in the equality tests.
				require.True(t, strings.HasSuffix(stdOut, "\n"),
					"stdout does not end with a linefeed")
			}

			if tC.expectStdErrPrefix != "" {
				require.True(t, strings.HasPrefix(errOut, tC.expectStdErrPrefix),
					"Expected stderr to start with %q; got %s",
					tC.expectStdErrPrefix, errOut)

				// Test that stderr ends with a linefeed since this test only
				// considers prefixes.
				require.True(t, strings.HasSuffix(stdOut, "\n"),
					"stderr does not end with a linefeed")
			}

			if tC.jsonTest != nil {
				jtC := tC.jsonTest
				err := json.Unmarshal([]byte(stdOut), &jtC.jsonDest)
				require.NoError(t, err, "stdout: %s", stdOut)

				for _, fn := range jtC.expectFns {
					fn(t, jtC.jsonDest)
				}
			}
		})
	}
}

func resetUiWriters(ui *cli.MockUi) {
	ui.ErrorWriter.Reset()
	ui.OutputWriter.Reset()
}

type testVarListTestCase struct {
	name               string
	args               []string
	exitCode           int
	expectUsage        bool
	expectUsageError   bool
	expectStdOut       string
	expectStdErrPrefix string
	jsonTest           *testVarListJSONTest
}

type testVarListJSONTest struct {
	jsonDest  interface{}
	expectFns []testVarListJSONTestExpectFn
}

type testVarListJSONTestExpectFn func(*testing.T, interface{})

type testSVNamespacePath struct {
	Namespace string
	Path      string
}

func setupTestVariables(c *api.Client, nsList, pathList []string) SVMSlice {

	out := make(SVMSlice, 0, len(nsList)*len(pathList))

	for _, ns := range nsList {
		c.Namespaces().Register(&api.Namespace{Name: ns}, nil)
		for _, p := range pathList {
			setupTestVariable(c, ns, p, &out)
		}
	}

	return out
}

func setupTestVariable(c *api.Client, ns, p string, out *SVMSlice) {
	testVar := &api.SecureVariable{Items: map[string]string{"k": "v"}}
	c.Raw().Write("/v1/var/"+p, testVar, nil, &api.WriteOptions{Namespace: ns})
	v, _, _ := c.SecureVariables().Read(p, &api.QueryOptions{Namespace: ns})
	*out = append(*out, *v.Metadata())
}

type NSPather interface {
	Len() int
	NSPaths() testSVNamespacePaths
}

type testSVNamespacePaths []testSVNamespacePath

func (ps testSVNamespacePaths) Len() int { return len(ps) }
func (ps testSVNamespacePaths) NSPaths() testSVNamespacePaths {
	return ps
}

type SVMSlice []api.SecureVariableMetadata

func (s SVMSlice) Len() int { return len(s) }
func (s SVMSlice) NSPaths() testSVNamespacePaths {

	out := make(testSVNamespacePaths, len(s))
	for i, v := range s {
		out[i] = testSVNamespacePath{v.Namespace, v.Path}
	}
	return out
}

func (ps SVMSlice) Strings() []string {
	ns := make(map[string]struct{})
	outNS := make([]string, len(ps))
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Path
		outNS[i] = p.Namespace + "|" + p.Path
		ns[p.Namespace] = struct{}{}
	}
	if len(ns) > 1 {
		return strings.Split(formatList(outNS), "\n")
	}
	return out
}

func (ps *SVMSlice) HavingNamespace(ns string) SVMSlice {
	return *ps.having("namespace", ns)
}

func (ps *SVMSlice) HavingPrefix(prefix string) SVMSlice {
	return *ps.having("prefix", prefix)
}

func (ps *SVMSlice) HavingNSPrefix(ns, p string) SVMSlice {
	return *ps.having("namespace", ns).having("prefix", p)
}

func (ps SVMSlice) having(field, val string) *SVMSlice {

	out := make(SVMSlice, 0, len(ps))
	for _, p := range ps {
		if field == "namespace" && p.Namespace == val {
			out = append(out, p)
		}
		if field == "prefix" && strings.HasPrefix(p.Path, val) {
			out = append(out, p)
		}
	}
	return &out
}

type PaginatedSVMSlice struct {
	Data      SVMSlice
	QueryMeta api.QueryMeta
}

func (s *PaginatedSVMSlice) Len() int { return len(s.Data) }
func (s *PaginatedSVMSlice) NSPaths() testSVNamespacePaths {

	out := make(testSVNamespacePaths, len(s.Data))
	for i, v := range s.Data {
		out[i] = testSVNamespacePath{v.Namespace, v.Path}
	}
	return out
}

type PaginatedSVQuietSlice struct {
	Data      []string
	QueryMeta api.QueryMeta
}

func (ps PaginatedSVQuietSlice) Len() int { return len(ps.Data) }
func (s *PaginatedSVQuietSlice) NSPaths() testSVNamespacePaths {

	out := make(testSVNamespacePaths, len(s.Data))
	for i, v := range s.Data {
		out[i] = testSVNamespacePath{"", v}
	}
	return out
}
