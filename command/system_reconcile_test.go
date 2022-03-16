package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
)

func TestSystemReconcileCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &SystemCommand{}
}
