package e2eutil

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// Register registers a jobspec from a file but with a unique ID.
// The caller is responsible for recording that ID for later cleanup.
func Register(jobID, jobFilePath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	return register(jobID, jobFilePath, exec.CommandContext(ctx, "nomad", "job", "run", "-detach", "-"))
}

// RegisterWithArgs registers a jobspec from a file but with a unique ID. The
// optional args are added to the run command. The caller is responsible for
// recording that ID for later cleanup.
func RegisterWithArgs(jobID, jobFilePath string, args ...string) error {

	baseArgs := []string{"job", "run", "-detach"}
	for i := range args {
		baseArgs = append(baseArgs, args[i])
	}
	baseArgs = append(baseArgs, "-")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	return register(jobID, jobFilePath, exec.CommandContext(ctx, "nomad", baseArgs...))
}

func register(jobID, jobFilePath string, cmd *exec.Cmd) error {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("could not open stdin?: %w", err)
	}

	content, err := ioutil.ReadFile(jobFilePath)
	if err != nil {
		return fmt.Errorf("could not open job file: %w", err)
	}

	// hack off the first line to replace with our unique ID
	var re = regexp.MustCompile(`(?m)^job ".*" \{`)
	jobspec := re.ReplaceAllString(string(content),
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
	for i := range args {
		baseArgs = append(baseArgs, args[i])
	}
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
