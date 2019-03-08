package metrics

import (
	"os"
	"path/filepath"

	"strings"

	"fmt"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/stretchr/testify/require"
)

type MetricsTest struct {
	framework.TC
	jobIds []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component: "Metrics",
		Constraints: framework.Constraints{
			Tags: []string{"metrics"},
		},
		CanRunLocal: false,
		Cases: []framework.TestCase{
			new(MetricsTest),
		},
	})
}

func (tc *MetricsTest) BeforeAll(f *framework.F) {
	// Ensure cluster has leader before running tests
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	// Ensure that we have four client nodes in ready state
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 4)
}

// TestMetrics runs fabio/prometheus and waits for those to succeed
// After that a series of jobs added to the input directory are executed
// Unlike other e2e tests this test does not clean up after itself.
func (tc *MetricsTest) TestMetrics(f *framework.F) {
	nomadClient := tc.Nomad()
	require := require.New(f.T())

	// Run fabio
	fabioAllocs := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, "fabio/fabio.nomad", "fabio")
	require.NotEmpty(fabioAllocs)

	// Run prometheus
	prometheusAllocs := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, "prometheus/prometheus.nomad", "prometheus")
	require.NotEmpty(prometheusAllocs)

	// List all job jobFiles in the input directory and run them and wait for allocations
	var jobFiles []string

	root := "metrics/input"
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if strings.HasSuffix(path, ".nomad") {
			jobFiles = append(jobFiles, path)
		}
		return nil
	})
	require.Nil(err)
	for _, file := range jobFiles {
		uuid := uuid.Generate()
		jobId := "metrics" + uuid[0:8]
		fmt.Println("Registering ", file)
		allocs := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, file, jobId)
		require.NotEmpty(allocs)
	}
}
