package provisioning

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// SSHRunner is a ProvisioningRunner that deploys via ssh.
// Terraform does all of this more elegantly and portably in its
// ssh communicator, but by shelling out we avoid pulling in TF's as
// a Nomad dependency, and avoid some long-standing issues with
// connections to Windows servers. The tradeoff is losing portability
// but in practice we're always going to run this from a Unixish
// machine.
type SSHRunner struct {
	Key  string // `json:"key"`
	User string // `json:"user"`
	Host string // `json:"host"`
	Port int    // `json:"port"`

	// none of these are available at time of construction, but
	// should be populated in Open().
	t               *testing.T
	controlSockPath string
	ctx             context.Context
	cancelFunc      context.CancelFunc
	copyMethod      func(*SSHRunner, string, string) error
	muxWait         chan struct{}
}

// Open establishes the ssh connection. We keep this connection open
// so that we can multiplex subsequent ssh connections.
func (runner *SSHRunner) Open(t *testing.T) error {
	runner.t = t
	runner.Logf("opening connection to %s", runner.Host)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	runner.ctx = ctx
	runner.cancelFunc = cancel
	runner.muxWait = make(chan struct{})

	home, _ := os.UserHomeDir()
	runner.controlSockPath = filepath.Join(
		home, ".ssh",
		fmt.Sprintf("ssh-control-%s-%d.sock", runner.Host, os.Getpid()))

	cmd := exec.CommandContext(ctx,
		"ssh",
		"-M", "-S", runner.controlSockPath,
		"-o", "StrictHostKeyChecking=no", // we're those terrible cloud devs
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-o", "ConnectTimeout=60", // give the target a while to come up
		"-i", runner.Key,
		"-p", fmt.Sprintf("%v", runner.Port),
		fmt.Sprintf("%s@%s", runner.User, runner.Host),
	)

	go func() {
		// will block until command completes, we cancel, or timeout.
		// there's no point in returning the error here as we only
		// hit it when we're done and Windows unfortunately tends to
		// return 1 even when the script is complete.
		cmd.Run()
		runner.muxWait <- struct{}{}
	}()
	return nil
}

func (runner *SSHRunner) Run(script string) error {
	commands := strings.Split(strings.TrimSpace(script), "\n")
	for _, command := range commands {
		err := runner.run(strings.TrimSpace(command))
		if err != nil {
			runner.cancelFunc()
			return err
		}
	}
	return nil
}

func (runner *SSHRunner) run(command string) error {
	if runner.controlSockPath == "" {
		return fmt.Errorf("Run failed: you need to call Open() first")
	}
	runner.Logf("running '%s'", command)
	cmd := exec.CommandContext(runner.ctx,
		"ssh",
		"-S", runner.controlSockPath,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-i", runner.Key,
		"-p", fmt.Sprintf("%v", runner.Port),
		fmt.Sprintf("%s@%s", runner.User, runner.Host),
		command)

	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil && err != context.Canceled {
		runner.LogErrOutput(string(stdoutStderr))
		return err
	}
	runner.LogOutput(string(stdoutStderr))
	return nil
}

// Copy uploads the local path to the remote path. We call into
// different copy methods for Linux vs Windows because their path
// semantics are slightly different and the typical ssh users have
// different permissions.
func (runner *SSHRunner) Copy(local, remote string) error {
	return runner.copyMethod(runner, local, remote)
}

// TODO: would be nice to set file owner/mode here
func copyLinux(runner *SSHRunner, local, remote string) error {
	t := runner.t
	runner.Logf("copying '%s' to '%s'", local, remote)
	remoteDir, remoteFileName := filepath.Split(remote)

	// we stage to /tmp so that we can handle root-owned files
	tempPath := fmt.Sprintf("/tmp/%s", remoteFileName)

	cmd := exec.CommandContext(runner.ctx,
		"scp", "-r",
		"-o", fmt.Sprintf("ControlPath=%s", runner.controlSockPath),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-i", runner.Key,
		"-P", fmt.Sprintf("%v", runner.Port),
		local,
		fmt.Sprintf("%s@%s:%s", runner.User, runner.Host, tempPath))

	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil && err != context.Canceled {
		runner.LogErrOutput(string(stdoutStderr))
		runner.cancelFunc()
		return err
	}

	fi, err := os.Stat(local)
	if err != nil {
		t.Fatalf("could not read '%s'", local)
	}
	if fi.IsDir() {
		// this is a little inefficient but it lets us merge the contents of
		// a bundled directory with existing directories
		err = runner.Run(
			fmt.Sprintf("sudo mkdir -p %s; sudo cp -R %s %s; sudo rm -r %s",
				remote, tempPath, remoteDir, tempPath))
	} else {
		err = runner.run(fmt.Sprintf("sudo mv %s %s", tempPath, remoteDir))
	}
	return err
}

// staging to Windows tempdirs is a little messier, but "fortunately"
// nobody seems to complain about connecting via ssh as Administrator on
// Windows so we can just bypass the problem.
func copyWindows(runner *SSHRunner, local, remote string) error {
	runner.Logf("copying '%s' to '%s'", local, remote)
	remoteDir, _ := filepath.Split(remote)
	fi, err := os.Stat(local)
	if err != nil {
		runner.t.Fatalf("could not read '%s'", local)
	}
	remotePath := remote
	if fi.IsDir() {
		remotePath = remoteDir
	}
	cmd := exec.CommandContext(runner.ctx,
		"scp", "-r",
		"-o", fmt.Sprintf("ControlPath=%s", runner.controlSockPath),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-i", runner.Key,
		"-P", fmt.Sprintf("%v", runner.Port),
		local,
		fmt.Sprintf("%s@%s:'%s'", runner.User, runner.Host, remotePath))

	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil && err != context.Canceled {
		runner.LogErrOutput(string(stdoutStderr))
		runner.cancelFunc()
		return err
	}
	return err
}

func (runner *SSHRunner) Close() {
	runner.Log("closing connection")
	runner.cancelFunc()
	<-runner.muxWait
}

// 'go test -v' only emits logs after the entire test run is complete,
// but that makes it much harder to debug hanging deployments. These
// methods wrap the test logger or just emit directly w/ fmt.Print if
// the '-v' flag was set.

func (runner *SSHRunner) Log(args ...interface{}) {
	if runner.t == nil {
		log.Fatal("no t.Testing configured for SSHRunner")
	}
	if testing.Verbose() {
		fmt.Printf("[" + runner.Host + "] ")
		fmt.Println(args...)
	} else {
		runner.t.Log(args...)
	}
}

func (runner *SSHRunner) Logf(format string, args ...interface{}) {
	if runner.t == nil {
		log.Fatal("no t.Testing configured for SSHRunner")
	}
	if testing.Verbose() {
		fmt.Printf("["+runner.Host+"] "+format+"\n", args...)
	} else {
		runner.t.Logf("["+runner.Host+"] "+format, args...)
	}
}

func (runner *SSHRunner) LogOutput(output string) {
	if testing.Verbose() {
		fmt.Println("\033[32m" + output + "\033[0m")
	} else {
		runner.t.Log(output)
	}
}

func (runner *SSHRunner) LogErrOutput(output string) {
	if testing.Verbose() {
		fmt.Println("\033[31m" + output + "\033[0m")
	} else {
		runner.t.Log(output)
	}
}
