// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package e2eutil

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shoenig/test"
)

// Register registers a jobspec from a file but with a unique ID.
// The caller is responsible for recording that ID for later cleanup.
func Register(jobID, jobFilePath string) error {
	_, err := RegisterGetOutput(jobID, jobFilePath)
	return err
}

// RegisterGetOutput registers a jobspec from a file but with a unique ID.
// The caller is responsible for recording that ID for later cleanup.
// Also returns the CLI output from running 'job run'.
func RegisterGetOutput(jobID, jobFilePath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	b, err := execCmd(jobID, jobFilePath, exec.CommandContext(ctx, "nomad", "job", "run", "-detach", "-"))
	return string(b), err
}

// RegisterWithArgs registers a jobspec from a file but with a unique ID. The
// optional args are added to the run command. The caller is responsible for
// recording that ID for later cleanup.
func RegisterWithArgs(jobID, jobFilePath string, args ...string) error {

	baseArgs := []string{"job", "run", "-detach"}
	baseArgs = append(baseArgs, args...)
	baseArgs = append(baseArgs, "-")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	_, err := execCmd(jobID, jobFilePath, exec.CommandContext(ctx, "nomad", baseArgs...))
	return err
}

// Revert reverts the job to the given version.
func Revert(jobID, jobFilePath string, version int) error {
	args := []string{"job", "revert", "-detach", jobID, strconv.Itoa(version)}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	_, err := execCmd(jobID, jobFilePath, exec.CommandContext(ctx, "nomad", args...))
	return err
}

func execCmd(jobID, jobFilePath string, cmd *exec.Cmd) ([]byte, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("could not open stdin?: %w", err)
	}

	content, err := os.ReadFile(jobFilePath)
	if err != nil {
		return nil, fmt.Errorf("could not open job file: %w", err)
	}

	// hack off the job block to replace with our unique ID
	var re = regexp.MustCompile(`(?m)^job ".*" \{`)
	jobspec := re.ReplaceAllString(string(content),
		fmt.Sprintf("job \"%s\" {", jobID))

	go func() {
		defer func() {
			_ = stdin.Close()
		}()
		_, _ = io.WriteString(stdin, jobspec)
	}()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("could not register job: %w\n%v", err, string(out))
	}
	return out, nil
}

// PeriodicForce forces a periodic job to dispatch
func PeriodicForce(jobID string) error {
	// nomad job periodic force
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	cmd := exec.CommandContext(ctx, "nomad", "job", "periodic", "force", jobID)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not register job: %w\n%v", err, string(out))
	}

	return nil
}

// Dispatch dispatches a parameterized job
func Dispatch(jobID string, meta map[string]string, payload string) error {
	// nomad job periodic force
	args := []string{"job", "dispatch"}
	for k, v := range meta {
		args = append(args, "-meta", fmt.Sprintf("%v=%v", k, v))
	}
	args = append(args, jobID)
	if payload != "" {
		args = append(args, "-")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	cmd := exec.CommandContext(ctx, "nomad", args...)
	cmd.Stdin = strings.NewReader(payload)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not dispatch job: %w\n%v", err, string(out))
	}

	return nil
}

// JobInspectTemplate runs nomad job inspect and formats the output
// using the specified go template
func JobInspectTemplate(jobID, template string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	cmd := exec.CommandContext(ctx, "nomad", "job", "inspect", "-t", template, jobID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("could not inspect job: %w\n%v", err, string(out))
	}
	outStr := string(out)
	outStr = strings.TrimSuffix(outStr, "\n")
	return outStr, nil
}

// RegisterFromJobspec registers a jobspec from a string, also with a unique
// ID. The caller is responsible for recording that ID for later cleanup.
func RegisterFromJobspec(jobID, jobspec string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	cmd := exec.CommandContext(ctx, "nomad", "job", "run", "-detach", "-")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("could not open stdin?: %w", err)
	}

	// hack off the first line to replace with our unique ID
	var re = regexp.MustCompile(`^job "\w+" \{`)
	jobspec = re.ReplaceAllString(jobspec,
		fmt.Sprintf("job \"%s\" {", jobID))

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, jobspec)
	}()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not register job: %w\n%v", err, string(out))
	}
	return nil
}

func ChildrenJobSummary(jobID string) ([]map[string]string, error) {
	out, err := Command("nomad", "job", "status", jobID)
	if err != nil {
		return nil, fmt.Errorf("nomad job status failed: %w", err)
	}

	section, err := GetSection(out, "Children Job Summary")
	if err != nil {
		section, err = GetSection(out, "Parameterized Job Summary")
		if err != nil {
			return nil, fmt.Errorf("could not find children job summary section: %w", err)
		}
	}

	summary, err := ParseColumns(section)
	if err != nil {
		return nil, fmt.Errorf("could not parse children job summary section: %w", err)
	}

	return summary, nil
}

func PreviouslyLaunched(jobID string) ([]map[string]string, error) {
	out, err := Command("nomad", "job", "status", jobID)
	if err != nil {
		return nil, fmt.Errorf("nomad job status failed: %w", err)
	}

	section, err := GetSection(out, "Previously Launched Jobs")
	if err != nil {
		return nil, fmt.Errorf("could not find previously launched jobs section: %w", err)
	}

	summary, err := ParseColumns(section)
	if err != nil {
		return nil, fmt.Errorf("could not parse previously launched jobs section: %w", err)
	}

	return summary, nil
}

func DispatchedJobs(jobID string) ([]map[string]string, error) {
	out, err := Command("nomad", "job", "status", jobID)
	if err != nil {
		return nil, fmt.Errorf("nomad job status failed: %w", err)
	}

	section, err := GetSection(out, "Dispatched Jobs")
	if err != nil {
		return nil, fmt.Errorf("could not find previously launched jobs section: %w", err)
	}

	summary, err := ParseColumns(section)
	if err != nil {
		return nil, fmt.Errorf("could not parse previously launched jobs section: %w", err)
	}

	return summary, nil
}

func StopJob(jobID string, args ...string) error {

	// Build our argument list in the correct order, ensuring the jobID is last
	// and the Nomad subcommand are first.
	baseArgs := []string{"job", "stop"}
	baseArgs = append(baseArgs, args...)
	baseArgs = append(baseArgs, jobID)

	// Execute the command. We do not care about the stdout, only stderr.
	_, err := Command("nomad", baseArgs...)

	if err != nil {
		// When stopping a job and monitoring the resulting deployment, we
		// expect that the monitor fails and exits with status code one because
		// technically the deployment has failed. Overwrite the error to be
		// nil.
		if strings.Contains(err.Error(), "Description = Cancelled because job is stopped") ||
			strings.Contains(err.Error(), "Description = Failed due to progress deadline") {
			err = nil
		}
	}
	return err
}

// CleanupJobsAndGC stops and purges the list of jobIDs and runs a
// system gc. Returns a func so that the return value can be used
// in t.Cleanup
func CleanupJobsAndGC(t *testing.T, jobIDs *[]string) func() {
	return func() {
		for _, jobID := range *jobIDs {
			err := StopJob(jobID, "-purge", "-detach")
			test.NoError(t, err)
		}
		_, err := Command("nomad", "system", "gc")
		test.NoError(t, err)
	}
}

// MaybeCleanupJobsAndGC stops and purges the list of jobIDs and runs a
// system gc. Returns a func so that the return value can be used
// in t.Cleanup. Similar to CleanupJobsAndGC, but this one does not assert
// on a successful stop and gc, which is useful for tests that want to stop and
// gc the jobs themselves but we want a backup Cleanup just in case.
func MaybeCleanupJobsAndGC(jobIDs *[]string) func() {
	return func() {
		for _, jobID := range *jobIDs {
			_ = StopJob(jobID, "-purge", "-detach")
		}
		_, _ = Command("nomad", "system", "gc")
	}
}

// CleanupJobsAndGCWithContext stops and purges the list of jobIDs and runs a
// system gc. The passed context allows callers to cancel the execution of the
// cleanup as they desire. This is useful for tests which attempt to remove the
// job as part of their run, but may fail before that point is reached.
func CleanupJobsAndGCWithContext(t *testing.T, ctx context.Context, jobIDs *[]string) {

	// Check the context before continuing. If this has been closed return,
	// otherwise fallthrough and complete the work.
	select {
	case <-ctx.Done():
		return
	default:
	}
	for _, jobID := range *jobIDs {
		err := StopJob(jobID, "-purge", "-detach")
		test.NoError(t, err)
	}
	_, err := Command("nomad", "system", "gc")
	test.NoError(t, err)
}
