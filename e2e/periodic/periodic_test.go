// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package periodic

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/v3/jobs3"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

func TestPeriodicDispatch_Basic(t *testing.T) {

	sub, cleanup := jobs3.Submit(t, "input/simple.nomad", jobs3.Dispatcher())
	t.Cleanup(cleanup)

	// force dispatch and wait for the dispatched job to finish
	must.NoError(t, e2eutil.PeriodicForce(sub.JobID()))
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			children, err := e2eutil.PreviouslyLaunched(sub.JobID())
			if err != nil {
				return err
			}

			for _, c := range children {
				if c["Status"] == "dead" {
					return nil
				}
			}
			return fmt.Errorf("expected periodic job to be dead")

		}),
		wait.Timeout(30*time.Second),
		wait.Gap(time.Second),
	))

	// Assert there are no pending children
	summary, err := e2eutil.ChildrenJobSummary(sub.JobID())
	must.NoError(t, err)
	must.Len(t, 1, summary)
	must.Eq(t, "0", summary[0]["Pending"])
	must.Eq(t, "0", summary[0]["Running"])
	must.Eq(t, "1", summary[0]["Dead"])
}
