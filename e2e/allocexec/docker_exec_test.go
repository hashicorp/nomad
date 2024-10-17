// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocexec

import (
	"archive/tar"
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	nomadapi "github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/v3/cluster3"
	"github.com/hashicorp/nomad/e2e/v3/jobs3"
	"github.com/shoenig/test/must"
)

func TestDockerAllocExec(t *testing.T) {
	cluster3.Establish(t,
		cluster3.Leader(),
		cluster3.LinuxClients(1),
	)

	t.Run("testDockerExecStdin", testDockerExecStdin)
}

func testDockerExecStdin(t *testing.T) {
	_, cleanup := jobs3.Submit(t, "./input/sleepytar.hcl")
	t.Cleanup(cleanup)

	client, err := nomadapi.NewClient(nomadapi.DefaultConfig())
	must.NoError(t, err)

	allocations, _, err := client.Allocations().List(nil)
	must.NoError(t, err)
	must.SliceLen(t, 1, allocations)

	// Use the first allocation for the example
	allocationID := allocations[0].ID
	allocation, _, err := client.Allocations().Info(allocationID, nil)
	must.NoError(t, err)

	// Command to execute
	command := []string{"tar", "--extract", "--verbose", "--file=/dev/stdin"}

	// Create a buffer to hold the tar archive
	var tarBuffer bytes.Buffer
	tarWriter := tar.NewWriter(&tarBuffer)

	// Create a tar header
	fileContentLength := 8100
	header := &tar.Header{
		Name: "filename.txt",
		Mode: 0600,
		Size: int64(len(strings.Repeat("a", fileContentLength))),
	}

	// Write the header to the tar archive
	must.NoError(t, tarWriter.WriteHeader(header))

	// Write the file content to the tar archive
	_, err = tarWriter.Write([]byte(strings.Repeat("a", fileContentLength)))
	must.NoError(t, err)

	// Close the tar writer
	must.Close(t, tarWriter)

	output := new(bytes.Buffer)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// execute the tar command inside the container
	exitCode, err := client.Allocations().Exec(
		ctx,
		allocation,
		"task",
		false,
		command,
		&tarBuffer,
		output,
		output,
		nil,
		nil,
	)
	must.NoError(t, err)
	must.Zero(t, exitCode)

	// check the output of tar
	s := output.String()
	must.Eq(t, "filename.txt\n", s)
}
