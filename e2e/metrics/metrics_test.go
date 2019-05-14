package metrics

import (
	"testing"

	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/stretchr/testify/require"
)

var metrics = flag.Bool("metrics", false, "run metrics tests")

func WaitForCluster(t *testing.T, nomadClient *api.Client) {
	// Ensure cluster has leader before running tests
	e2eutil.WaitForLeader(t, nomadClient)
	// Ensure that we have four client nodes in ready state
	e2eutil.WaitForNodesReady(t, nomadClient, 1)
}

// TestMetrics runs fabio/prometheus and waits for those to succeed
// After that a series of jobs added to the input directory are executed
// Unlike other e2e tests this test does not clean up after itself.
// This test is meant for AWS environments and will not work locally
func TestMetrics(t *testing.T) {
	if !*metrics {
		t.Skip("skipping test in non-metrics mode.")
	}
	require := require.New(t)
	// Build Nomad api client
	nomadClient, err := api.NewClient(api.DefaultConfig())
	require.Nil(err)
	WaitForCluster(t, nomadClient)

	// Run fabio
	fabioAllocs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient, "../fabio/fabio.nomad", "fabio")
	require.NotEmpty(fabioAllocs)

	// Run prometheus
	prometheusAllocs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient, "../prometheus/prometheus.nomad", "prometheus")
	require.NotEmpty(prometheusAllocs)

	// List all job jobFiles in the input directory and run them and wait for allocations
	var jobFiles []string

	root := "input/"
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
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
		allocs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient, file, jobId)
		require.NotEmpty(allocs)
	}

	// Get a client node IP address
	nodesAPI := nomadClient.Nodes()
	nodes, _, err := nodesAPI.List(nil)
	require.Nil(err)
	for _, node := range nodes {
		nodeDetails, _, err := nodesAPI.Info(node.ID, nil)
		require.Nil(err)
		clientPublicIP := nodeDetails.Attributes["unique.platform.aws.public-ipv4"]
		fmt.Printf("Prometheus Metrics available at http://%s:9999\n", clientPublicIP)
	}

}
