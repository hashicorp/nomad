// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package e2eutil

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	api "github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/discover"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type e2eJob struct {
	framework.TC
	jobfile string
	jobID   string
}

func (j *e2eJob) Name() string {
	return filepath.Base(j.jobfile)
}

// Ensure cluster has leader and at least 1 client node
// in a ready state before running tests
func (j *e2eJob) BeforeAll(f *framework.F) {
	WaitForLeader(f.T(), j.Nomad())
	WaitForNodesReady(f.T(), j.Nomad(), 1)
	j.jobID = "e2eutil-" + uuid.Generate()[0:8]
}

func (j *e2eJob) TestJob(f *framework.F) {
	file, err := os.Open(j.jobfile)
	t := f.T()
	require.NoError(t, err)

	scanner := bufio.NewScanner(file)
	var e2eJobLine string
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "//e2e:") {
			e2eJobLine = scanner.Text()[6:]
		}
		require.NoError(t, scanner.Err())
	}

	switch {
	case strings.HasPrefix(e2eJobLine, "batch"):
		parseBatchJobLine(t, j, e2eJobLine).Run(f)
	case strings.HasPrefix(e2eJobLine, "service"):
		parseServiceJobLine(t, j, e2eJobLine).Run(f)
	default:
		require.Fail(t, "could not parse e2e job line: %q", e2eJobLine)
	}
}

type e2eBatchJob struct {
	*e2eJob

	shouldFail bool
}

func (j *e2eBatchJob) Run(f *framework.F) {
	t := f.T()
	require := require.New(t)
	nomadClient := j.Nomad()

	allocs := RegisterAndWaitForAllocs(f.T(), nomadClient, j.jobfile, j.jobID, "")
	require.Equal(1, len(allocs))
	allocID := allocs[0].ID

	// wait for the job to stop
	WaitForAllocStopped(t, nomadClient, allocID)
	alloc, _, err := nomadClient.Allocations().Info(allocID, nil)
	require.NoError(err)
	if j.shouldFail {
		require.NotEqual(structs.AllocClientStatusComplete, alloc.ClientStatus)
	} else {
		require.Equal(structs.AllocClientStatusComplete, alloc.ClientStatus)
	}
}

type e2eServiceJob struct {
	*e2eJob

	script          string
	runningDuration time.Duration
}

func (j *e2eServiceJob) Run(f *framework.F) {
	t := f.T()
	nomadClient := j.Nomad()

	allocs := RegisterAndWaitForAllocs(f.T(), nomadClient, j.jobfile, j.jobID, "")
	require.Equal(t, 1, len(allocs))
	allocID := allocs[0].ID

	var alloc *api.Allocation
	WaitForAllocRunning(t, nomadClient, allocID)
	testutil.AssertUntil(j.runningDuration, func() (bool, error) {
		var err error
		alloc, _, err = nomadClient.Allocations().Info(allocID, nil)
		if err != nil {
			return false, err
		}

		return alloc.ClientStatus == structs.AllocClientStatusRunning, fmt.Errorf("expected status running, but was: %s", alloc.ClientStatus)
	}, func(err error) {
		require.NoError(t, err, "failed to keep alloc running")
	})

	scriptPath := filepath.Join(filepath.Dir(j.jobfile), j.script)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	cmd := exec.CommandContext(ctx, scriptPath)
	nmdBin, err := discover.NomadExecutable()
	assert.NoError(t, err)
	cmd.Env = append(os.Environ(),
		"NOMAD_BIN="+nmdBin,
		"NOMAD_ALLOC_ID="+allocID,
		"NOMAD_ADDR="+nomadClient.Address(),
	)

	assert.NoError(t, cmd.Start())
	waitCh := make(chan error)
	go func() {
		select {
		case waitCh <- cmd.Wait():
		case <-ctx.Done():
		}
	}()

	select {
	case <-ctx.Done():
	case err := <-waitCh:
		assert.NoError(t, err)
		assert.Zero(t, cmd.ProcessState.ExitCode())
	}

	// stop the job
	_, _, err = nomadClient.Jobs().Deregister(j.jobID, false, nil)
	require.NoError(t, err)
	WaitForAllocStopped(t, nomadClient, allocID)
}

//e2e:batch fail=false
//e2e:service running=5s check=script.sh

func NewE2EJob(jobfile string) framework.TestCase {
	return &e2eJob{
		jobfile: jobfile,
	}

}

func parseServiceJobLine(t *testing.T, j *e2eJob, line string) *e2eServiceJob {
	job := &e2eServiceJob{
		e2eJob:          j,
		runningDuration: time.Second * 5,
	}
	for _, options := range strings.Split(line, " ")[1:] {
		o := strings.SplitN(options, "=", 2)
		switch o[0] {
		case "script":
			job.script = o[1]
		case "running":
			dur, err := time.ParseDuration(o[1])
			if err != nil {
				t.Logf("could not parse running duration %q for e2e job spec: %v", o[1], err)
			} else {
				job.runningDuration = dur
			}
		}
	}

	return job
}

func parseBatchJobLine(t *testing.T, j *e2eJob, line string) *e2eBatchJob {
	job := &e2eBatchJob{
		e2eJob: j,
	}
	for _, options := range strings.Split(line, " ")[1:] {
		o := strings.SplitN(options, "=", 2)
		switch o[0] {
		case "shouldFail":
			job.shouldFail, _ = strconv.ParseBool(o[1])
		}
	}

	return job
}
