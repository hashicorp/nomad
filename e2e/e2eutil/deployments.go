package e2eutil

import (
	"fmt"
	"time"

	"github.com/hashicorp/nomad/testutil"
)

func WaitForLastDeploymentStatus(jobID, status string, wc *WaitConfig) error {
	var got string
	var err error
	interval, retries := wc.OrDefault()
	testutil.WaitForResultRetries(retries, func() (bool, error) {
		time.Sleep(interval)

		out, err := Command("nomad", "job", "status", jobID)
		if err != nil {
			return false, fmt.Errorf("could not get job status: %v\n%v", err, out)
		}

		section, err := GetSection(out, "Latest Deployment")
		if err != nil {
			return false, fmt.Errorf("could not find Latest Deployment section: %w", err)
		}

		fields, err := ParseFields(section)
		if err != nil {
			return false, fmt.Errorf("could not parse Latest Deployment section: %w", err)
		}

		got = fields["Status"]
		return got == status, nil
	}, func(e error) {
		err = fmt.Errorf("deployment status check failed: got %#v", got)
	})
	return err
}
