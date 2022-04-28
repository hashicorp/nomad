package command

import (
	"testing"

	"github.com/mitchellh/cli"
)

func TestServerJoinCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &ServerJoinCommand{}
}
