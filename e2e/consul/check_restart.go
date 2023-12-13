// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"

	e2e "github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
)

const ns = ""

type CheckRestartE2ETest struct {
	framework.TC
	jobIds []string
}

func (tc *CheckRestartE2ETest) BeforeAll(f *framework.F) {
	e2e.WaitForLeader(f.T(), tc.Nomad())
	e2e.WaitForNodesReady(f.T(), tc.Nomad(), 1)
}

func (tc *CheckRestartE2ETest) AfterEach(f *framework.F) {
	if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
		return
	}

	for _, id := range tc.jobIds {
		err := e2e.StopJob(id, "-purge")
		f.Assert().NoError(err)
	}
	tc.jobIds = []string{}
	_, err := e2e.Command("nomad", "system", "gc")
	f.Assert().NoError(err)
}

// TestGroupCheckRestart runs a job with a group service that will never
// become healthy. Both tasks should be restarted up to the 'restart' limit.
func (tc *CheckRestartE2ETest) TestGroupCheckRestart(f *framework.F) {

	jobID := "test-group-check-restart-" + uuid.Short()
	f.NoError(e2e.Register(jobID, "consul/input/checks_group_restart.nomad"))
	tc.jobIds = append(tc.jobIds, jobID)

	f.NoError(
		e2e.WaitForAllocStatusComparison(
			func() ([]string, error) { return e2e.AllocStatuses(jobID, ns) },
			func(got []string) bool { return reflect.DeepEqual(got, []string{"failed"}) },
			&e2e.WaitConfig{Interval: time.Second * 10, Retries: 30},
		))

	allocs, err := e2e.AllocsForJob(jobID, ns)
	f.NoError(err)
	f.Len(allocs, 1)

	allocID := allocs[0]["ID"]
	expected := "Exceeded allowed attempts 2 in interval 5m0s and mode is \"fail\""

	out, err := e2e.Command("nomad", "alloc", "status", allocID)
	f.NoError(err, "could not get allocation status")
	f.Contains(out, expected,
		fmt.Errorf("expected '%s', got\n%v", expected, out))

	re := regexp.MustCompile(`Total Restarts += (.*)\n`)
	match := re.FindAllStringSubmatch(out, -1)
	for _, m := range match {
		f.Equal("2", strings.TrimSpace(m[1]),
			fmt.Errorf("expected exactly 2 restarts for both tasks, got:\n%v", out))
	}
}

// TestTaskCheckRestart runs a job with a task service that will never become
// healthy. Only the failed task should be restarted up to the 'restart'
// limit.
func (tc *CheckRestartE2ETest) TestTaskCheckRestart(f *framework.F) {

	jobID := "test-task-check-restart-" + uuid.Short()
	f.NoError(e2e.Register(jobID, "consul/input/checks_task_restart.nomad"))
	tc.jobIds = append(tc.jobIds, jobID)

	var allocID string

	f.NoError(
		e2e.WaitForAllocStatusComparison(
			func() ([]string, error) { return e2e.AllocStatuses(jobID, ns) },
			func(got []string) bool { return reflect.DeepEqual(got, []string{"failed"}) },
			&e2e.WaitConfig{Interval: time.Second * 10, Retries: 30},
		))

	expected := "Exceeded allowed attempts 2 in interval 5m0s and mode is \"fail\""

	out, err := e2e.Command("nomad", "alloc", "status", allocID)
	f.NoError(err, "could not get allocation status")
	f.Contains(out, expected,
		fmt.Errorf("expected '%s', got\n%v", expected, out))

	re := regexp.MustCompile(`Total Restarts += (.*)\n`)
	match := re.FindAllStringSubmatch(out, -1)
	f.Equal("2", strings.TrimSpace(match[0][1]),
		fmt.Errorf("expected exactly 2 restarts for failed task, got:\n%v", out))

	f.Equal("0", strings.TrimSpace(match[1][1]),
		fmt.Errorf("expected exactly no restarts for healthy task, got:\n%v", out))
}
