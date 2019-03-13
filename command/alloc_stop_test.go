package command

import (
	"testing"

	"github.com/mitchellh/cli"
)

func TestAllocStopCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &AllocStopCommand{}
}
