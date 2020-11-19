package e2eutil

import (
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"regexp"
)

// Register registers a jobspec from a file but with a unique ID.
// The caller is responsible for recording that ID for later cleanup.
func Register(jobID, jobFilePath string) error {

	cmd := exec.Command("nomad", "job", "run", "-")
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

// Register registers a jobspec from a string, also with a unique ID.
// The caller is responsible for recording that ID for later cleanup.
func RegisterFromJobspec(jobID, jobspec string) error {

	cmd := exec.Command("nomad", "job", "run", "-")
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
