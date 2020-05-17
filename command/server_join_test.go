package command

import (
	"testing"

	"github.com/mitchellh/cli"
)

func TestServerJoinCommand_Implements(t *testing.T) {
	var _ cli.Command = &ServerJoinCommand{}
}
