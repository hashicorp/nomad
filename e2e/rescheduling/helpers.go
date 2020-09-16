package rescheduling

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/testutil"
)

// allocStatuses returns a slice of client statuses
func allocStatuses(f *framework.F, jobID string) []string {

	out, err := e2eutil.Command("nomad", "job", "status", "-verbose", "-all-allocs", jobID)
	f.NoError(err, "nomad job status failed", err)
	section, err := e2eutil.GetSection(out, "Allocations")
	f.NoError(err, "could not find Allocations section", err)

	allocs, err := e2eutil.ParseColumns(section)
	f.NoError(err, "could not parse Allocations section", err)

	statuses := []string{}
	for _, alloc := range allocs {
		statuses = append(statuses, alloc["Status"])
	}

	return statuses
}

// allocStatusesRescheduled is a helper function that pulls
// out client statuses only from rescheduled allocs.
func allocStatusesRescheduled(f *framework.F, jobID string) []string {

	out, err := e2eutil.Command("nomad", "job", "status", "-verbose", jobID)
	f.NoError(err, "nomad job status failed", err)
	section, err := e2eutil.GetSection(out, "Allocations")
	f.NoError(err, "could not find Allocations section", err)

	allocs, err := e2eutil.ParseColumns(section)
	f.NoError(err, "could not parse Allocations section", err)

	statuses := []string{}
	for _, alloc := range allocs {

		allocID := alloc["ID"]

		// reschedule tracker isn't exposed in the normal CLI output
		out, err := e2eutil.Command("nomad", "alloc", "status", "-json", allocID)
		f.NoError(err, "nomad alloc status failed", err)

		dec := json.NewDecoder(strings.NewReader(out))
		alloc := &api.Allocation{}
		err = dec.Decode(alloc)
		f.NoError(err, "could not decode alloc status JSON: %w", err)

		if (alloc.RescheduleTracker != nil &&
			len(alloc.RescheduleTracker.Events) > 0) || alloc.FollowupEvalID != "" {
			statuses = append(statuses, alloc.ClientStatus)
		}
	}
	return statuses
}

// register is a helper that registers a jobspec with a unique ID
// and records that ID in the testcase for later cleanup
func register(f *framework.F, jobFile, jobID string) {

	cmd := exec.Command("nomad", "job", "run", "-")
	stdin, err := cmd.StdinPipe()
	f.NoError(err, fmt.Sprintf("could not open stdin?: %v", err))

	content, err := ioutil.ReadFile(jobFile)
	f.NoError(err, fmt.Sprintf("could not open job file: %v", err))

	// hack off the first line to replace with our unique ID
	var re = regexp.MustCompile(`^job "\w+" \{`)
	jobspec := re.ReplaceAllString(string(content),
		fmt.Sprintf("job \"%s\" {", jobID))

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, jobspec)
	}()

	out, err := cmd.CombinedOutput()
	f.NoError(err, "could not register job: %v\n%v", err, string(out))
}

func waitForAllocStatusComparison(query func() ([]string, error), comparison func([]string) bool) error {
	var got []string
	var err error
	testutil.WaitForResultRetries(50, func() (bool, error) {
		time.Sleep(time.Millisecond * 100)
		got, err = query()
		if err != nil {
			return false, err
		}
		return comparison(got), nil
	}, func(e error) {
		err = fmt.Errorf("alloc status check failed: got %#v", got)
	})
	return err
}

func waitForLastDeploymentStatus(f *framework.F, jobID, status string) error {
	var got string
	var err error
	testutil.WaitForResultRetries(50, func() (bool, error) {
		time.Sleep(time.Millisecond * 100)

		out, err := e2eutil.Command("nomad", "job", "status", jobID)
		f.NoError(err, "could not get job status: %v\n%v", err, out)

		section, err := e2eutil.GetSection(out, "Latest Deployment")
		f.NoError(err, "could not find Latest Deployment section", err)

		fields, err := e2eutil.ParseFields(section)
		f.NoError(err, "could not parse Latest Deployment section", err)

		got = fields["Status"]
		return got == status, nil
	}, func(e error) {
		err = fmt.Errorf("deployment status check failed: got %#v", got)
	})
	return err
}
