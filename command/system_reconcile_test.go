package command

import (
	"testing"

	"github.com/mitchellh/cli"
)

func TestSystemReconcileCommand_Implements(t *testing.T) {
	var _ cli.Command = &SystemCommand{}
}
