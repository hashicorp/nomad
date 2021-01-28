package e2eutil

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/testutil"
	"github.com/kr/pretty"
)

// WaitForAllocStatusExpected polls 'nomad job status' and exactly compares
// the status of all allocations (including any previous versions) against the
// expected list.
func WaitForAllocStatusExpected(jobID, ns string, expected []string) error {
	err := WaitForAllocStatusComparison(
		func() ([]string, error) { return AllocStatuses(jobID, ns) },
		func(got []string) bool { return reflect.DeepEqual(got, expected) },
		nil,
	)
	if err != nil {
		allocs, _ := AllocsForJob(jobID, ns)
		err = fmt.Errorf("%v\nallocs: %v", err, pretty.Sprint(allocs))
	}
	return err
}

// WaitForAllocStatusComparison is a convenience wrapper that polls the query
// function until the comparison function returns true.
func WaitForAllocStatusComparison(query func() ([]string, error), comparison func([]string) bool, wc *WaitConfig) error {
	var got []string
	var err error
	interval, retries := wc.OrDefault()
	testutil.WaitForResultRetries(retries, func() (bool, error) {
		time.Sleep(interval)
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

// AllocsForJob returns a slice of key->value maps, each describing the values
// of the 'nomad job status' Allocations section (not actual
// structs.Allocation objects, query the API if you want those)
func AllocsForJob(jobID, ns string) ([]map[string]string, error) {

	var nsArg = []string{}
	if ns != "" {
		nsArg = []string{"-namespace", ns}
	}

	cmd := []string{"nomad", "job", "status"}
	params := []string{"-verbose", "-all-allocs", jobID}
	cmd = append(cmd, nsArg...)
	cmd = append(cmd, params...)

	out, err := Command(cmd[0], cmd[1:]...)
	if err != nil {
		return nil, fmt.Errorf("'nomad job status' failed: %w", err)
	}

	section, err := GetSection(out, "Allocations")
	if err != nil {
		return nil, fmt.Errorf("could not find Allocations section: %w", err)
	}

	allocs, err := ParseColumns(section)
	if err != nil {
		return nil, fmt.Errorf("could not parse Allocations section: %w", err)
	}
	return allocs, nil
}

// AllocsForNode returns a slice of key->value maps, each describing the values
// of the 'nomad node status' Allocations section (not actual
// structs.Allocation objects, query the API if you want those)
func AllocsForNode(nodeID string) ([]map[string]string, error) {

	out, err := Command("nomad", "node", "status", "-verbose", nodeID)
	if err != nil {
		return nil, fmt.Errorf("'nomad node status' failed: %w", err)
	}

	section, err := GetSection(out, "Allocations")
	if err != nil {
		return nil, fmt.Errorf("could not find Allocations section: %w", err)
	}

	allocs, err := ParseColumns(section)
	if err != nil {
		return nil, fmt.Errorf("could not parse Allocations section: %w", err)
	}
	return allocs, nil
}

// AllocStatuses returns a slice of client statuses
func AllocStatuses(jobID, ns string) ([]string, error) {

	allocs, err := AllocsForJob(jobID, ns)
	if err != nil {
		return nil, err
	}

	statuses := []string{}
	for _, alloc := range allocs {
		statuses = append(statuses, alloc["Status"])
	}
	return statuses, nil
}

// AllocStatusesRescheduled is a helper function that pulls
// out client statuses only from rescheduled allocs.
func AllocStatusesRescheduled(jobID, ns string) ([]string, error) {

	var nsArg = []string{}
	if ns != "" {
		nsArg = []string{"-namespace", ns}
	}

	cmd := []string{"nomad", "job", "status"}
	params := []string{"-verbose", jobID}
	cmd = append(cmd, nsArg...)
	cmd = append(cmd, params...)

	out, err := Command(cmd[0], cmd[1:]...)
	if err != nil {
		return nil, fmt.Errorf("nomad job status failed: %w", err)
	}

	section, err := GetSection(out, "Allocations")
	if err != nil {
		return nil, fmt.Errorf("could not find Allocations section: %w", err)
	}

	allocs, err := ParseColumns(section)
	if err != nil {
		return nil, fmt.Errorf("could not parse Allocations section: %w", err)
	}

	statuses := []string{}
	for _, alloc := range allocs {

		allocID := alloc["ID"]

		cmd := []string{"nomad", "alloc", "status"}
		params := []string{"-json", allocID}
		cmd = append(cmd, nsArg...)
		cmd = append(cmd, params...)

		// reschedule tracker isn't exposed in the normal CLI output
		out, err := Command(cmd[0], cmd[1:]...)
		if err != nil {
			return nil, fmt.Errorf("nomad alloc status failed: %w", err)
		}

		dec := json.NewDecoder(strings.NewReader(out))
		alloc := &api.Allocation{}
		err = dec.Decode(alloc)
		if err != nil {
			return nil, fmt.Errorf("could not decode alloc status JSON: %w", err)
		}

		if (alloc.RescheduleTracker != nil &&
			len(alloc.RescheduleTracker.Events) > 0) || alloc.FollowupEvalID != "" {
			statuses = append(statuses, alloc.ClientStatus)
		}
	}
	return statuses, nil
}

type LogStream int

const (
	LogsStdErr LogStream = iota
	LogsStdOut
)

func AllocLogs(allocID string, logStream LogStream) (string, error) {
	cmd := []string{"nomad", "alloc", "logs"}
	if logStream == LogsStdErr {
		cmd = append(cmd, "-stderr")
	}
	cmd = append(cmd, allocID)
	return Command(cmd[0], cmd[1:]...)
}

// AllocExec is a convenience wrapper that runs 'nomad alloc exec' with the
// passed execCmd via '/bin/sh -c', retrying if the task isn't ready
func AllocExec(allocID, taskID, execCmd, ns string, wc *WaitConfig) (string, error) {
	var got string
	var err error
	interval, retries := wc.OrDefault()

	var nsArg = []string{}
	if ns != "" {
		nsArg = []string{"-namespace", ns}
	}

	cmd := []string{"nomad", "exec"}
	params := []string{"-task", taskID, allocID, "/bin/sh", "-c", execCmd}
	cmd = append(cmd, nsArg...)
	cmd = append(cmd, params...)

	testutil.WaitForResultRetries(retries, func() (bool, error) {
		time.Sleep(interval)
		got, err = Command(cmd[0], cmd[1:]...)
		return err == nil, err
	}, func(e error) {
		err = fmt.Errorf("exec failed: '%s': %v\nGot: %v", strings.Join(cmd, " "), e, got)
	})
	return got, err
}

// WaitForAllocFile is a helper that grabs a file via alloc fs and tests its
// contents; useful for checking the results of rendered templates
func WaitForAllocFile(allocID, path string, test func(string) bool, wc *WaitConfig) error {
	var err error
	var out string
	interval, retries := wc.OrDefault()

	testutil.WaitForResultRetries(retries, func() (bool, error) {
		time.Sleep(interval)
		out, err = Command("nomad", "alloc", "fs", allocID, path)
		if err != nil {
			return false, fmt.Errorf("could not get file %q from allocation %q: %v",
				path, allocID, err)
		}
		return test(out), nil
	}, func(e error) {
		err = fmt.Errorf("test for file content failed: got %#v\nerror: %v", out, e)
	})
	return err
}
