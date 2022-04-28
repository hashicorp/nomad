package command

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	hclog "github.com/hashicorp/go-hclog"
)

// environment captures all the information needed to execute terraform
// in order to setup a test environment
type environment struct {
	provider string // provider ex. aws
	name     string // environment name ex. generic

	tf      string // location of terraform binary
	tfPath  string // path to terraform configuration
	tfState string // path to terraform state file
	logger  hclog.Logger
}

func (env *environment) canonicalName() string {
	return fmt.Sprintf("%s/%s", env.provider, env.name)
}

// envResults are the fields returned after provisioning a test environment
type envResults struct {
	nomadAddr  string
	consulAddr string
	vaultAddr  string
}

// newEnv takes a path to the environments directory, environment name and provider,
// path to terraform state file and a logger and builds the environment stuct used
// to initial terraform calls
func newEnv(envPath, provider, name, tfStatePath string, logger hclog.Logger) (*environment, error) {
	// Make sure terraform is on the PATH
	tf, err := exec.LookPath("terraform")
	if err != nil {
		return nil, fmt.Errorf("failed to lookup terraform binary: %v", err)
	}

	logger = logger.Named("provision").With("provider", provider, "name", name)

	// set the path to the terraform module
	tfPath := path.Join(envPath, provider, name)
	logger.Debug("using tf path", "path", tfPath)
	if _, err := os.Stat(tfPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to lookup terraform configuration dir %s: %v", tfPath, err)
	}

	// set the path to state file
	tfState := path.Join(tfStatePath, fmt.Sprintf("e2e.%s.%s.tfstate", provider, name))

	env := &environment{
		provider: provider,
		name:     name,
		tf:       tf,
		tfPath:   tfPath,
		tfState:  tfState,
		logger:   logger,
	}
	return env, nil
}

// envsFromGlob allows for the discovery of multiple environments using globs (*).
// ex. aws/* for all environments in aws.
func envsFromGlob(envPath, glob, tfStatePath string, logger hclog.Logger) ([]*environment, error) {
	results, err := filepath.Glob(filepath.Join(envPath, glob))
	if err != nil {
		return nil, err
	}

	envs := []*environment{}

	for _, p := range results {
		elems := strings.Split(p, "/")
		name := elems[len(elems)-1]
		provider := elems[len(elems)-2]
		env, err := newEnv(envPath, provider, name, tfStatePath, logger)
		if err != nil {
			return nil, err
		}

		envs = append(envs, env)
	}

	return envs, nil
}

// provision calls terraform to setup the environment with the given nomad binary
func (env *environment) provision(nomadPath string) (*envResults, error) {
	tfArgs := []string{"apply", "-auto-approve", "-input=false", "-no-color",
		"-state", env.tfState,
		"-var", fmt.Sprintf("nomad_binary=%s", path.Join(nomadPath, "nomad")),
		env.tfPath,
	}

	// Setup the 'terraform apply' command
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, env.tf, tfArgs...)

	// Funnel the stdout/stderr to logging
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stderr pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %v", err)
	}

	// Run 'terraform apply'
	cmd.Start()
	go tfLog(env.logger.Named("tf.stderr"), stderr)
	go tfLog(env.logger.Named("tf.stdout"), stdout)

	sigChan := make(chan os.Signal, 2)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	cmdChan := make(chan error)
	go func() {
		cmdChan <- cmd.Wait()
	}()

	// if an interrupt is received before terraform finished, forward signal to
	// child pid
	select {
	case sig := <-sigChan:
		env.logger.Error("interrupt received, forwarding signal to child process",
			"pid", cmd.Process.Pid)
		cmd.Process.Signal(sig)
		if err := procWaitTimeout(cmd.Process, 5*time.Second); err != nil {
			env.logger.Error("child process did not exit in time, killing forcefully",
				"pid", cmd.Process.Pid)
			cmd.Process.Kill()
		}
		return nil, fmt.Errorf("interrupt received")
	case err := <-cmdChan:
		if err != nil {
			return nil, fmt.Errorf("terraform exited with a non-zero status: %v", err)
		}
	}

	// Setup and run 'terraform output' to get the module output
	cmd = exec.CommandContext(ctx, env.tf, "output", "-json", "-state", env.tfState)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("terraform exited with a non-zero status: %v", err)
	}

	// Parse the json and pull out results
	tfOutput := make(map[string]map[string]interface{})
	err = json.Unmarshal(out, &tfOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to parse terraform output: %v", err)
	}

	results := &envResults{}
	if nomadAddr, ok := tfOutput["nomad_addr"]; ok {
		results.nomadAddr = nomadAddr["value"].(string)
	} else {
		return nil, fmt.Errorf("'nomad_addr' field expected in terraform output, but was missing")
	}

	return results, nil
}

// destroy calls terraform to destroy the environment
func (env *environment) destroy() error {
	tfArgs := []string{"destroy", "-auto-approve", "-no-color",
		"-state", env.tfState,
		"-var", "nomad_binary=",
		env.tfPath,
	}
	cmd := exec.Command(env.tf, tfArgs...)

	// Funnel the stdout/stderr to logging
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %v", err)
	}

	// Run 'terraform destroy'
	cmd.Start()
	go tfLog(env.logger.Named("tf.stderr"), stderr)
	go tfLog(env.logger.Named("tf.stdout"), stdout)

	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("terraform exited with a non-zero status: %v", err)
	}

	return nil
}

func tfLog(logger hclog.Logger, r io.ReadCloser) {
	defer r.Close()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		logger.Debug(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		logger.Error("scan error", "error", err)
	}

}
