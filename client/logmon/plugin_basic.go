// +build !linux

package logmon

import (
	"os/exec"

	hclog "github.com/hashicorp/go-hclog"
)

func newPluginCmd(nomadBin string, logger hclog.Logger) *exec.Cmd {
	cmd := exec.Command(nomadBin, "logmon")
	return cmd
}
