package jobspec2

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/jobspec"
	"github.com/stretchr/testify/require"
)

func TestEquivalentToHCL1(t *testing.T) {
	hclSpecDir := "../jobspec/test-fixtures/"
	fis, err := ioutil.ReadDir(hclSpecDir)
	require.NoError(t, err)

	for _, fi := range fis {
		name := fi.Name()

		t.Run(name, func(t *testing.T) {
			f, err := os.Open(hclSpecDir + name)
			require.NoError(t, err)
			defer f.Close()

			job1, err := jobspec.Parse(f)
			if err != nil {
				t.Skip("file is not parsable in v1")
			}

			f.Seek(0, 0)

			job2, err := Parse(name, f)
			require.NoError(t, err)

			require.Equal(t, job1, job2)
		})
	}
}

func TestParse_Variables(t *testing.T) {
	hcl := `
job "example" {
  datacenters = [for s in ["dc1", "dc2"] : upper(s)]
  region      = vars.region_var
}
`

	out, err := ParseWithArgs("input.hcl", strings.NewReader(hcl), map[string]string{"region_var": "aug"}, true)
	require.NoError(t, err)

	require.Equal(t, []string{"DC1", "DC2"}, out.Datacenters)
	require.Equal(t, "aug", *out.Region)
}

func TestParse_VarsAndFunctions(t *testing.T) {
	hcl := `
job "example" {
  datacenters = [for s in ["dc1", "dc2"] : upper(s)]
  region      = vars.region_var
}
`

	out, err := ParseWithArgs("input.hcl", strings.NewReader(hcl), map[string]string{"region_var": "aug"}, true)
	require.NoError(t, err)

	require.Equal(t, []string{"DC1", "DC2"}, out.Datacenters)
	require.NotNil(t, out.Region)
	require.Equal(t, "aug", *out.Region)
}

func TestParse_FileOperators(t *testing.T) {
	hcl := `
job "example" {
  region      = file("parse_test.go")
}
`

	out, err := ParseWithArgs("input.hcl", strings.NewReader(hcl), nil, true)
	require.NoError(t, err)

	expected, err := ioutil.ReadFile("parse_test.go")
	require.NoError(t, err)

	require.NotNil(t, out.Region)
	require.Equal(t, string(expected), *out.Region)
}
