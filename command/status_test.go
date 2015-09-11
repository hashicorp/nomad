package command

import (
	"testing"

	"github.com/mitchellh/cli"
)

func TestStatusCommand_Implements(t *testing.T) {
	var _ cli.Command = &StatusCommand{}
}
