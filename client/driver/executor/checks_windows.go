// +build windows

package executor

import (
	"fmt"
	"os/exec"
	"time"

	"golang.org/x/sys/windows"

	"github.com/armon/circbuf"

	cstructs "github.com/hashicorp/nomad/client/driver/structs"
)

// Run runs an exec script check
func (e *ExecScriptCheck) Run() *cstructs.CheckResult {
	buf, _ := circbuf.NewBuffer(int64(cstructs.CheckBufSize))
	cmd := exec.Command(e.cmd, e.args...)
	cmd.Stdout = buf
	cmd.Stderr = buf
	e.setChroot(cmd)
	ts := time.Now()
	if err := cmd.Start(); err != nil {
		return &cstructs.CheckResult{Err: err}
	}
	errCh := make(chan error, 2)
	go func() {
		errCh <- cmd.Wait()
	}()
	for {
		select {
		case err := <-errCh:
			endTime := time.Now()
			if err == nil {
				return &cstructs.CheckResult{
					ExitCode:  0,
					Output:    string(buf.Bytes()),
					Timestamp: ts,
				}
			}
			exitCode := 1
			if exitErr, ok := err.(*exec.ExitError); ok {
				if status, ok := exitErr.Sys().(windows.WaitStatus); ok {
					exitCode = status.ExitStatus()
				}
			}
			return &cstructs.CheckResult{
				ExitCode:  exitCode,
				Output:    string(buf.Bytes()),
				Timestamp: ts,
				Duration:  endTime.Sub(ts),
			}
		case <-time.After(e.Timeout()):
			errCh <- fmt.Errorf("timed out after waiting 30s")
		}
	}
	return nil
}
