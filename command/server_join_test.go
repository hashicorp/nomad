package command

import (
	"testing"

	"github.com/hashicorp/nomad/Godeps/_workspace/src/github.com/mitchellh/cli"
)

func TestServerJoinCommand_Implements(t *testing.T) {
	var _ cli.Command = &ServerJoinCommand{}
}
