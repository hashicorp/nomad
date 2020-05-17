package command

import (
	"testing"

	"github.com/mitchellh/cli"
)

func TestServerForceLeaveCommand_Implements(t *testing.T) {
	var _ cli.Command = &ServerForceLeaveCommand{}
}
