package jobspec2

import (
	"io/ioutil"
	"os"
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
