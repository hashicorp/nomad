package servicediscovery

import (
	"context"
	"regexp"
	"testing"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test/must"
)

func testChecksHappy(t *testing.T) {
	nomadClient := e2eutil.NomadClient(t)

	// Generate our unique job ID which will be used for this test.
	jobID := "nsd-check-happy-" + uuid.Short()
	jobIDs := []string{jobID}

	// Defer a cleanup function to remove the job. This will trigger if the
	// test fails, unless the cancel function is called.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer e2eutil.CleanupJobsAndGCWithContext(t, ctx, &jobIDs)

	// Register the happy checks job.
	allocStubs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient, jobChecksHappy, jobID, "")
	must.Len(t, 1, allocStubs)

	// wait for the alloc to be running
	e2eutil.WaitForAllocRunning(t, nomadClient, allocStubs[0].ID)

	// get the output of 'nomad alloc checks'
	output, err := e2eutil.AllocChecks(allocStubs[0].ID)
	must.NoError(t, err)

	// assert the output contains success
	statusRe := regexp.MustCompile(`Status\s+=\s+success`)
	must.RegexMatch(t, statusRe, output)

	// assert the output contains 200 status code
	statusCodeRe := regexp.MustCompile(`StatusCode\s+=\s+200`)
	must.RegexMatch(t, statusCodeRe, output)

	// assert output contains nomad's success string
	must.StrContains(t, output, `nomad: http ok`)
}

func testChecksSad(t *testing.T) {
	nomadClient := e2eutil.NomadClient(t)

	// Generate our unique job ID which will be used for this test.
	jobID := "nsd-check-sad-" + uuid.Short()
	jobIDs := []string{jobID}

	// Defer a cleanup function to remove the job. This will trigger if the
	// test fails, unless the cancel function is called.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer e2eutil.CleanupJobsAndGCWithContext(t, ctx, &jobIDs)

	// Register the sad checks job.
	allocStubs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient, jobChecksSad, jobID, "")
	must.Len(t, 1, allocStubs)

	// wait for the alloc to be running
	e2eutil.WaitForAllocRunning(t, nomadClient, allocStubs[0].ID)

	// get the output of 'nomad alloc checks'
	output, err := e2eutil.AllocChecks(allocStubs[0].ID)
	must.NoError(t, err)

	// assert the output contains failure
	statusRe := regexp.MustCompile(`Status\s+=\s+failure`)
	must.RegexMatch(t, statusRe, output)

	// assert the output contains 501 status code
	statusCodeRe := regexp.MustCompile(`StatusCode\s+=\s+501`)
	must.RegexMatch(t, statusCodeRe, output)

	// assert output contains error output from python http.server
	must.StrContains(t, output, `<p>Error code explanation: HTTPStatus.NOT_IMPLEMENTED - Server does not support this operation.</p>`)
}
