package command

import (
	"testing"

	"github.com/mitchellh/cli"
)

func TestSystemCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &SystemCommand{}
}
