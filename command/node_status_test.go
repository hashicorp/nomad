package command

import (
	"testing"

	"github.com/mitchellh/cli"
)

func TestNodeStatusCommand_Implements(t *testing.T) {
	var _ cli.Command = &NodeStatusCommand{}
}
